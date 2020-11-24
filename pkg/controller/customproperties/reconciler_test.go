package customproperties

import (
	"context"
	"os"
	"testing"

	"github.com/Dynatrace/dynatrace-operator/pkg/apis"
	dynatracev1alpha1 "github.com/Dynatrace/dynatrace-operator/pkg/apis/dynatrace/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testName      = "test-name"
	testNamespace = "test-namespace"
	testValue     = "test-value"
	testKey       = "test-key"
	testOwner     = "test"
	namespace     = "dynatrace"
)

func init() {
	_ = apis.AddToScheme(scheme.Scheme) // Register OneAgent and Istio object schemas.
	_ = os.Setenv(k8sutil.WatchNamespaceEnvVar, namespace)
}

func TestReconciler_Reconcile(t *testing.T) {
	t.Run(`Reconile works with minimal setup`, func(t *testing.T) {
		r := NewReconciler(nil, nil, nil, "", dynatracev1alpha1.DynaKubeValueSource{}, nil)
		err := r.Reconcile()
		assert.NoError(t, err)
	})
	t.Run(`Reconile creates custom properties secret`, func(t *testing.T) {
		valueSource := dynatracev1alpha1.DynaKubeValueSource{Value: testValue}
		instance := &dynatracev1alpha1.DynaKube{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testName,
				Namespace: testNamespace,
			}}
		fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme, instance)
		r := NewReconciler(fakeClient, instance, nil, testOwner, valueSource, scheme.Scheme)
		err := r.Reconcile()

		assert.NoError(t, err)

		var customPropertiesSecret corev1.Secret
		err = fakeClient.Get(context.TODO(), client.ObjectKey{Name: r.buildCustomPropertiesName(testName), Namespace: testNamespace}, &customPropertiesSecret)

		assert.NoError(t, err)
		assert.NotNil(t, customPropertiesSecret)
		assert.NotEmpty(t, customPropertiesSecret.Data)
		assert.Contains(t, customPropertiesSecret.Data, DataKey)
		assert.Equal(t, customPropertiesSecret.Data[DataKey], []byte(testValue))
	})
	t.Run(`Reconcile updates custom properties only if data changed`, func(t *testing.T) {
		valueSource := dynatracev1alpha1.DynaKubeValueSource{Value: testValue}
		instance := &dynatracev1alpha1.DynaKube{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testName,
				Namespace: testNamespace,
			}}
		fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme, instance)
		r := NewReconciler(fakeClient, instance, nil, testOwner, valueSource, scheme.Scheme)
		err := r.Reconcile()

		assert.NoError(t, err)

		var customPropertiesSecret corev1.Secret
		err = fakeClient.Get(context.TODO(), client.ObjectKey{Name: r.buildCustomPropertiesName(testName), Namespace: testNamespace}, &customPropertiesSecret)

		assert.NoError(t, err)
		assert.NotNil(t, customPropertiesSecret)
		assert.NotEmpty(t, customPropertiesSecret.Data)
		assert.Contains(t, customPropertiesSecret.Data, DataKey)
		assert.Equal(t, customPropertiesSecret.Data[DataKey], []byte(testValue))

		err = r.Reconcile()

		assert.NoError(t, err)

		err = fakeClient.Get(context.TODO(), client.ObjectKey{Name: r.buildCustomPropertiesName(testName), Namespace: testNamespace}, &customPropertiesSecret)

		assert.NoError(t, err)
		assert.NotNil(t, customPropertiesSecret)
		assert.NotEmpty(t, customPropertiesSecret.Data)
		assert.Contains(t, customPropertiesSecret.Data, DataKey)
		assert.Equal(t, customPropertiesSecret.Data[DataKey], []byte(testValue))

		r.customPropertiesSource.Value = testKey
		err = r.Reconcile()

		assert.NoError(t, err)

		err = fakeClient.Get(context.TODO(), client.ObjectKey{Name: r.buildCustomPropertiesName(testName), Namespace: testNamespace}, &customPropertiesSecret)

		assert.NoError(t, err)
		assert.NotNil(t, customPropertiesSecret)
		assert.NotEmpty(t, customPropertiesSecret.Data)
		assert.Contains(t, customPropertiesSecret.Data, DataKey)
		assert.Equal(t, customPropertiesSecret.Data[DataKey], []byte(testKey))
	})
}
