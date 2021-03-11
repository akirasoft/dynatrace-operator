#!/bin/sh

set -e

CLI="kubectl"
ENABLE_K8S_MONITORING="false"
SET_APP_LOG_CONTENT_ACCESS="false"
SKIP_CERT_CHECK="false"
ENABLE_VOLUME_STORAGE="false"

for arg in "$@"; do
  case $arg in
  --api-url)
    API_URL="$2"
    shift 2
    ;;
  --api-token)
    API_TOKEN="$2"
    shift 2
    ;;
  --paas-token)
    PAAS_TOKEN="$2"
    shift 2
    ;;
  --enable-k8s-monitoring)
    ENABLE_K8S_MONITORING="true"
    shift
    ;;
  --set-app-log-content-access)
    SET_APP_LOG_CONTENT_ACCESS="true"
    shift
    ;;
  --skip-cert-check)
    SKIP_CERT_CHECK="true"
    shift
    ;;
  --enable-volume-storage)
    ENABLE_VOLUME_STORAGE="true"
    shift
    ;;
  --openshift)
    CLI="oc"
    shift
    ;;
  esac
done

if [ -z "$API_URL" ]; then
  echo "Error: api-url not set!"
  exit 1
fi

if [ -z "$API_TOKEN" ]; then
  echo "Error: api-token not set!"
  exit 1
fi

if [ -z "$PAAS_TOKEN" ]; then
  echo "Error: paas-token not set!"
  exit 1
fi

set -u

checkIfNSExists() {
  if ! "${CLI}" get ns dynatrace >/dev/null 2>&1; then
    if [ "${CLI}" = "kubectl" ]; then
      "${CLI}" create namespace dynatrace
    else
      "${CLI}" adm new-project --node-selector="" dynatrace
    fi
  else
    echo "Namespace already exists"
  fi
}

applyOneAgentOperator() {
  if [ "${CLI}" = "kubectl" ]; then
    "${CLI}" apply -f https://github.com/Dynatrace/dynatrace-oneagent-operator/releases/latest/download/kubernetes.yaml
  else
    printf "\nCreating Dynatrace OneAgent Operator Operator Group...\n"
    applyOneAgentOperatorOG
    printf "\nCreating Dynatrace OneAgent Operator Subscription...\n"
    applyOneAgentOperatorSub
  fi

  "${CLI}" -n dynatrace create secret generic oneagent --from-literal="apiToken=${API_TOKEN}" --from-literal="paasToken=${PAAS_TOKEN}" --dry-run=client -o yaml | "${CLI}" apply -f -
}

applyOneAgentOperatorOG() {
  cat <<EOF | "${CLI}" apply -f -
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: dynatrace-abcd
  namespace: dynatrace
spec:
  targetNamespaces:
  - dynatrace
EOF
}

applyOneAgentOperatorSub() {
  cat <<EOF | "${CLI}" apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: oneagent-certified
  namespace: dynatrace
spec:
  channel: alpha
  name: oneagent-certified
  source: certified-operators
  sourceNamespace: openshift-marketplace
EOF
}


applyOneAgentCR() {
  cat <<EOF | "${CLI}" apply -f -
apiVersion: dynatrace.com/v1alpha1
kind: OneAgent
metadata:
  name: oneagent
  namespace: dynatrace
spec:
  apiUrl: ${API_URL}
  tolerations:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
    operator: Exists
  skipCertCheck: ${SKIP_CERT_CHECK}
  args:
  - --set-app-log-content-access=${SET_APP_LOG_CONTENT_ACCESS}
  env:
  - name: ONEAGENT_ENABLE_VOLUME_STORAGE
    value: "${ENABLE_VOLUME_STORAGE}"
EOF
}

applyDynatraceOperator() {
    if [ "${CLI}" = "kubectl" ]; then
      "${CLI}" apply -f https://github.com/Dynatrace/dynatrace-operator/releases/latest/download/kubernetes.yaml
    else
      "${CLI}" apply -f https://github.com/Dynatrace/dynatrace-operator/releases/download/v0.1.0/openshift.yaml
    fi
}

applyDynaKubeCR() {
  cat <<EOF | "${CLI}" apply -f -
apiVersion: dynatrace.com/v1alpha1
kind: DynaKube
metadata:
  name: dynakube
  namespace: dynatrace
spec:
  apiUrl: ${API_URL}
  tokens: oneagent
  kubernetesMonitoring:
    enabled: true
    replicas: 1
EOF
}

addK8sConfiguration() {
  K8S_ENDPOINT="$("${CLI}" config view --minify -o jsonpath='{.clusters[0].cluster.server}')"
  if [ -z "$K8S_ENDPOINT" ]; then
    echo "Error: failed to get kubernetes endpoint!"
    exit 1
  fi

  CONNECTION_NAME="$(echo "${K8S_ENDPOINT}" | awk -F[/:] '{print $4}')"

  K8S_SECRET_NAME="$(for token in $("${CLI}" get sa dynatrace-kubernetes-monitoring -o jsonpath='{.secrets[*].name}' -n dynatrace); do echo "$token"; done | grep token)"
  if [ -z "$K8S_SECRET_NAME" ]; then
    echo "Error: failed to get kubernetes-monitoring secret!"
    exit 1
  fi

  K8S_BEARER="$("${CLI}" get secret "${K8S_SECRET_NAME}" -o jsonpath='{.data.token}' -n dynatrace | base64 --decode)"
  if [ -z "$K8S_BEARER" ]; then
    echo "Error: failed to get bearer token!"
    exit 1
  fi

  json="$(
    cat <<EOF
{
  "label": "${CONNECTION_NAME}",
  "endpointUrl": "${K8S_ENDPOINT}",
  "eventsFieldSelectors": [
    {
      "label": "Node events",
      "fieldSelector": "involvedObject.kind=Node",
      "active": true
    }
  ],
  "workloadIntegrationEnabled": true,
  "eventsIntegrationEnabled": false,
  "authToken": "${K8S_BEARER}",
  "active": true,
  "certificateCheckEnabled": "${SKIP_CERT_CHECK}"
}
EOF
  )"

  response="$(curl -sS -X POST "${API_URL}/config/v1/kubernetes/credentials" \
    -H "accept: application/json; charset=utf-8" \
    -H "Authorization: Api-Token ${API_TOKEN}" \
    -H "Content-Type: application/json; charset=utf-8" \
    -d "${json}")"

  if echo "$response" | grep "${CONNECTION_NAME}" >/dev/null 2>&1; then
    echo "Kubernetes monitoring successfully setup."
  else
    echo "Error adding Kubernetes cluster to Dynatrace: $response"
  fi
}

####### MAIN #######
printf "\nCreating Dynatrace namespace...\n"
checkIfNSExists
printf "\nApplying Dynatrace OneAgent Operator...\n"
applyOneAgentOperator
printf "\nSleeping for 60s to wait for operator deployment...\n"
sleep 60
printf "\nApplying OneAgent CustomResource...\n"
applyOneAgentCR

if [ "${ENABLE_K8S_MONITORING}" = "true" ]; then
  printf "\nApplying Dynatrace Operator...\n"
  applyDynatraceOperator
  printf "\nApplying DynaKube CustomResource...\n"
  applyDynaKubeCR
  printf "\nAdding cluster to Dynatrace...\n"
  addK8sConfiguration
fi
