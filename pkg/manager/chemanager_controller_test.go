package manager

import (
	"context"
	"reflect"
	"testing"

	"github.com/che-incubator/devworkspace-che-operator/apis/che-controller/v1alpha1"
	"github.com/che-incubator/devworkspace-che-operator/pkg/defaults"
	"github.com/che-incubator/devworkspace-che-operator/pkg/gateway"
	"github.com/che-incubator/devworkspace-che-operator/pkg/sync"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func createTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(extensions.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(rbac.AddToScheme(scheme))
	return scheme
}

func TestCreatesObjectsInSingleHost(t *testing.T) {
	managerName := "che"
	ns := "default"
	scheme := createTestScheme()
	ctx := context.TODO()
	cl := fake.NewFakeClientWithScheme(scheme, &v1alpha1.CheManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managerName,
			Namespace: ns,
		},
		Spec: v1alpha1.CheManagerSpec{
			Host:    "over.the.rainbow",
			Routing: v1alpha1.SingleHost,
		},
	})

	reconciler := CheReconciler{client: cl, scheme: scheme, gateway: gateway.New(cl, scheme), syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	gateway.TestGatewayObjectsExist(t, ctx, cl, managerName, ns)
}

func TestUpdatesObjectsInSingleHost(t *testing.T) {
	managerName := "che"
	ns := "default"

	scheme := createTestScheme()

	cl := fake.NewFakeClientWithScheme(scheme,
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
				Labels: map[string]string{
					"some":                   "label",
					"app.kubernetes.io/name": "not what we expect",
				},
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
		},
		&rbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
		},
		&rbac.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
		},
		&v1alpha1.CheManager{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
			Spec: v1alpha1.CheManagerSpec{
				Host:    "over.the.rainbow",
				Routing: v1alpha1.SingleHost,
			},
		})

	ctx := context.TODO()

	reconciler := CheReconciler{client: cl, scheme: scheme, gateway: gateway.New(cl, scheme), syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	gateway.TestGatewayObjectsExist(t, ctx, cl, managerName, ns)

	depl := &appsv1.Deployment{}
	if err = cl.Get(ctx, client.ObjectKey{Name: managerName, Namespace: ns}, depl); err != nil {
		t.Fatalf("Failed to read the che manager deployment that should exist")
	}

	// checking that we got the update we wanted on the labels...
	expectedLabels := defaults.GetLabelsFromNames(managerName, "deployment")
	expectedLabels["some"] = "label"

	if !reflect.DeepEqual(expectedLabels, depl.GetLabels()) {
		t.Errorf("The deployment should have had its labels reset by the reconciler.")
	}
}

func TestDoesntCreateObjectsInMultiHost(t *testing.T) {
	managerName := "che"
	ns := "default"
	scheme := createTestScheme()
	ctx := context.TODO()
	cl := fake.NewFakeClientWithScheme(scheme, &v1alpha1.CheManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managerName,
			Namespace: ns,
		},
		Spec: v1alpha1.CheManagerSpec{
			Host:    "over.the.rainbow",
			Routing: v1alpha1.MultiHost,
		},
	})

	reconciler := CheReconciler{client: cl, scheme: scheme, gateway: gateway.New(cl, scheme), syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	gateway.TestGatewayObjectsDontExist(t, ctx, cl, managerName, ns)
}

func TestDeletesObjectsInMultiHost(t *testing.T) {
	managerName := "che"
	ns := "default"

	scheme := createTestScheme()

	cl := fake.NewFakeClientWithScheme(scheme,
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
		},
		&rbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
		},
		&rbac.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
		},
		&v1alpha1.CheManager{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managerName,
				Namespace: ns,
			},
			Spec: v1alpha1.CheManagerSpec{
				Host:    "over.the.rainbow",
				Routing: v1alpha1.MultiHost,
			},
		})

	ctx := context.TODO()

	reconciler := CheReconciler{client: cl, scheme: scheme, gateway: gateway.New(cl, scheme), syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	gateway.TestGatewayObjectsDontExist(t, ctx, cl, managerName, ns)
}

func TestNoManagerSharedWhenReconcilingNonExistent(t *testing.T) {
	// clear the map before the test
	for k := range currentManagers {
		delete(currentManagers, k)
	}

	managerName := "che"
	ns := "default"
	scheme := createTestScheme()
	cl := fake.NewFakeClientWithScheme(scheme)

	ctx := context.TODO()

	reconciler := CheReconciler{client: cl, scheme: scheme, gateway: gateway.New(cl, scheme), syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	// there is nothing in our context, so the map should still be empty
	managers := GetCurrentManagers()
	if len(managers) != 0 {
		t.Fatalf("There should have been no managers after a reconcile of a non-existent manager.")
	}

	// now add some manager and reconcile a non-existent one
	cl.Create(ctx, &v1alpha1.CheManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managerName + "-not-me",
			Namespace: ns,
		},
		Spec: v1alpha1.CheManagerSpec{
			Host:    "over.the.rainbow",
			Routing: v1alpha1.MultiHost,
		},
	})

	_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers = GetCurrentManagers()
	if len(managers) != 0 {
		t.Fatalf("There should have been no managers after a reconcile of a non-existent manager.")
	}
}

func TestAddsManagerToSharedMapOnCreate(t *testing.T) {
	// clear the map before the test
	for k := range currentManagers {
		delete(currentManagers, k)
	}

	managerName := "che"
	ns := "default"
	scheme := createTestScheme()
	cl := fake.NewFakeClientWithScheme(scheme, &v1alpha1.CheManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managerName,
			Namespace: ns,
		},
		Spec: v1alpha1.CheManagerSpec{
			Host:    "over.the.rainbow",
			Routing: v1alpha1.MultiHost,
		},
	})

	reconciler := CheReconciler{client: cl, scheme: scheme, gateway: gateway.New(cl, scheme), syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers := GetCurrentManagers()
	if len(managers) != 1 {
		t.Fatalf("There should have been exactly 1 manager after a reconcile but there is %d.", len(managers))
	}

	mgr, ok := managers[types.NamespacedName{Name: managerName, Namespace: ns}]
	if !ok {
		t.Fatalf("The map of the current managers doesn't contain the expected one.")
	}

	if mgr.Name != managerName {
		t.Fatalf("Found a manager that we didn't reconcile. Curious (and buggy). We found %s but should have found %s", mgr.Name, managerName)
	}
}

func TestUpdatesManagerInSharedMapOnUpdate(t *testing.T) {
	// clear the map before the test
	for k := range currentManagers {
		delete(currentManagers, k)
	}

	managerName := "che"
	ns := "default"
	scheme := createTestScheme()

	cl := fake.NewFakeClientWithScheme(scheme, &v1alpha1.CheManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managerName,
			Namespace: ns,
		},
		Spec: v1alpha1.CheManagerSpec{
			Host:    "over.the.rainbow",
			Routing: v1alpha1.MultiHost,
		},
	})

	reconciler := CheReconciler{client: cl, scheme: scheme, gateway: gateway.New(cl, scheme), syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers := GetCurrentManagers()
	if len(managers) != 1 {
		t.Fatalf("There should have been exactly 1 manager after a reconcile but there is %d.", len(managers))
	}

	mgr, ok := managers[types.NamespacedName{Name: managerName, Namespace: ns}]
	if !ok {
		t.Fatalf("The map of the current managers doesn't contain the expected one.")
	}

	if mgr.Name != managerName {
		t.Fatalf("Found a manager that we didn't reconcile. Curious (and buggy). We found %s but should have found %s", mgr.Name, managerName)
	}

	if mgr.Spec.Host != "over.the.rainbow" {
		t.Fatalf("Unexpected host value: expected: over.the.rainbow, actual: %s", mgr.Spec.Host)
	}

	// now update the manager and reconcile again. See that the map contains the updated value
	mgr = *mgr.DeepCopy()
	mgr.Spec.Host = "over.the.shoulder"
	err = cl.Update(context.TODO(), &mgr)
	if err != nil {
		t.Fatalf("Failed to update. Wat? %s", err)
	}

	// before the reconcile, the map still should containe the old value
	managers = GetCurrentManagers()
	mgr, ok = managers[types.NamespacedName{Name: managerName, Namespace: ns}]
	if !ok {
		t.Fatalf("The map of the current managers doesn't contain the expected one.")
	}

	if mgr.Name != managerName {
		t.Fatalf("Found a manager that we didn't reconcile. Curious (and buggy). We found %s but should have found %s", mgr.Name, managerName)
	}

	if mgr.Spec.Host != "over.the.rainbow" {
		t.Fatalf("Unexpected host value: expected: over.the.rainbow, actual: %s", mgr.Spec.Host)
	}

	// now reconcile and see that the value in the map is now updated

	_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers = GetCurrentManagers()
	mgr, ok = managers[types.NamespacedName{Name: managerName, Namespace: ns}]
	if !ok {
		t.Fatalf("The map of the current managers doesn't contain the expected one.")
	}

	if mgr.Name != managerName {
		t.Fatalf("Found a manager that we didn't reconcile. Curious (and buggy). We found %s but should have found %s", mgr.Name, managerName)
	}

	if mgr.Spec.Host != "over.the.shoulder" {
		t.Fatalf("Unexpected host value: expected: over.the.shoulder, actual: %s", mgr.Spec.Host)
	}
}

func TestRemovesManagerFromSharedMapOnDelete(t *testing.T) {
	// clear the map before the test
	for k := range currentManagers {
		delete(currentManagers, k)
	}

	managerName := "che"
	ns := "default"
	scheme := createTestScheme()

	cl := fake.NewFakeClientWithScheme(scheme, &v1alpha1.CheManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managerName,
			Namespace: ns,
		},
		Spec: v1alpha1.CheManagerSpec{
			Host:    "over.the.rainbow",
			Routing: v1alpha1.MultiHost,
		},
	})

	reconciler := CheReconciler{client: cl, scheme: scheme, gateway: gateway.New(cl, scheme), syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers := GetCurrentManagers()
	if len(managers) != 1 {
		t.Fatalf("There should have been exactly 1 manager after a reconcile but there is %d.", len(managers))
	}

	mgr, ok := managers[types.NamespacedName{Name: managerName, Namespace: ns}]
	if !ok {
		t.Fatalf("The map of the current managers doesn't contain the expected one.")
	}

	if mgr.Name != managerName {
		t.Fatalf("Found a manager that we didn't reconcile. Curious (and buggy). We found %s but should have found %s", mgr.Name, managerName)
	}

	cl.Delete(context.TODO(), &mgr)

	// now reconcile and see that the value is no longer in the map

	_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers = GetCurrentManagers()
	_, ok = managers[types.NamespacedName{Name: managerName, Namespace: ns}]
	if ok {
		t.Fatalf("The map of the current managers should no longer contain the manager after it has been deleted.")
	}
}
