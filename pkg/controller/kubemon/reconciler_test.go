package kubemon

import (
	"context"
	"github.com/Dynatrace/dynatrace-operator/pkg/apis"
	"github.com/Dynatrace/dynatrace-operator/pkg/apis/dynatrace/v1alpha1"
	"github.com/Dynatrace/dynatrace-operator/pkg/controller/kubesystem"
	"github.com/Dynatrace/dynatrace-operator/pkg/dtclient"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

func init() {
	_ = apis.AddToScheme(scheme.Scheme) // Register OneAgent and Istio object schemas.
	_ = os.Setenv(k8sutil.WatchNamespaceEnvVar, v1alpha1.DynatraceNamespace)
}

func TestReconciler_Reconcile(t *testing.T) {
	log := logf.Log.WithName("TestReconciler")
	request := reconcile.Request{}
	dtcMock := &dtclient.MockDynatraceClient{}
	instance := &v1alpha1.DynaKube{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		}}
	fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			UID:  testUID,
			Name: kubesystem.Namespace,
		}},
		instance)
	reconciler := NewReconciler(
		fakeClient, fakeClient, scheme.Scheme, dtcMock, log, &corev1.Secret{}, instance,
	)
	connectionInfo := dtclient.ConnectionInfo{TenantUUID: testUID}
	tenantInfo := &dtclient.TenantInfo{ID: testUID}

	dtcMock.
		On("GetConnectionInfo").
		Return(connectionInfo, nil)

	dtcMock.
		On("GetTenantInfo").
		Return(tenantInfo, nil)

	assert.NotNil(t, reconciler)

	result, err := reconciler.Reconcile(request)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	var statefulSet v1.StatefulSet
	err = fakeClient.Get(context.TODO(), client.ObjectKey{Name: v1alpha1.Name, Namespace: instance.Namespace}, &statefulSet)

	expected := *newStatefulSet(*instance, tenantInfo, testUID)
	expected.Spec.Template.Spec.Volumes = nil

	assert.NoError(t, err)
	assert.NotNil(t, statefulSet)
	assert.Equal(t, expected.ObjectMeta.Name, statefulSet.ObjectMeta.Name)
	assert.Equal(t, expected.ObjectMeta.Namespace, statefulSet.ObjectMeta.Namespace)
	assert.Equal(t, expected.Spec, statefulSet.Spec)
}
