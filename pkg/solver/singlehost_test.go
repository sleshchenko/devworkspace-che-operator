package solver

import (
	"context"
	"fmt"
	"testing"

	"github.com/che-incubator/devworkspace-che-operator/apis/che-controller/v1alpha1"
	"github.com/che-incubator/devworkspace-che-operator/pkg/defaults"
	"github.com/che-incubator/devworkspace-che-operator/pkg/manager"
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/workspacerouting/solvers"
	"github.com/devfile/devworkspace-operator/pkg/config"
	routev1 "github.com/openshift/api/route/v1"
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
	"sigs.k8s.io/yaml"
)

func createTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(extensions.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(rbac.AddToScheme(scheme))
	utilruntime.Must(dw.AddToScheme(scheme))
	utilruntime.Must(dwo.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))

	return scheme
}

func getSpecObjects(t *testing.T, routing *dwo.WorkspaceRouting) (client.Client, solvers.RoutingSolver, solvers.RoutingObjects) {
	scheme := createTestScheme()
	cheManager := &v1alpha1.CheManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{manager.FinalizerName},
		},
		Spec: v1alpha1.CheManagerSpec{
			Host: "over.the.rainbow",
		},
	}

	cl := fake.NewFakeClientWithScheme(scheme, cheManager)

	solver, err := Getter(scheme).GetSolver(cl, "che")
	if err != nil {
		t.Fatal(err)
	}

	meta := solvers.WorkspaceMetadata{
		WorkspaceId:   routing.Spec.WorkspaceId,
		Namespace:     routing.GetNamespace(),
		PodSelector:   routing.Spec.PodSelector,
		RoutingSuffix: routing.Spec.RoutingSuffix,
	}

	// we need to do 1 round of che manager reconciliation so that the solver gets initialized
	cheRecon := manager.New(cl, scheme)
	_, err = cheRecon.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "che", Namespace: "ns"}})
	if err != nil {
		t.Fatal(err)
	}

	objs, err := solver.GetSpecObjects(routing, meta)
	if err != nil {
		t.Fatal(err)
	}

	// now we need a second round of che manager reconciliation so that it proclaims the che gateway as established
	cheRecon.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "che", Namespace: "ns"}})

	return cl, solver, objs
}

func simpleWorkspaceRouting() *dwo.WorkspaceRouting {
	return &dwo.WorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
		},
		Spec: dwo.WorkspaceRoutingSpec{
			WorkspaceId:  "wsid",
			RoutingClass: "che",
			Endpoints: map[string]dwo.EndpointList{
				"m1": {
					{
						Name:       "e1",
						TargetPort: 9999,
						Exposure:   dw.PublicEndpointExposure,
						Protocol:   "https",
						Path:       "/1/",
					},
					{
						Name:       "e2",
						TargetPort: 9999,
						Exposure:   dw.PublicEndpointExposure,
						Protocol:   "http",
						Path:       "/2.js",
						Secure:     true,
					},
					{
						Name:       "e3",
						TargetPort: 9999,
						Exposure:   dw.PublicEndpointExposure,
					},
				},
			},
		},
	}
}

func TestCreateObjects(t *testing.T) {
	cl, _, objs := getSpecObjects(t, simpleWorkspaceRouting())

	t.Run("noIngresses", func(t *testing.T) {
		if len(objs.Ingresses) != 0 {
			t.Error()
		}
	})

	t.Run("noRoutes", func(t *testing.T) {
		if len(objs.Routes) != 0 {
			t.Error()
		}
	})

	t.Run("noPodAdditions", func(t *testing.T) {
		if objs.PodAdditions != nil {
			t.Error()
		}
	})

	for i := range objs.Services {
		t.Run(fmt.Sprintf("service-%d", i), func(t *testing.T) {
			svc := &objs.Services[i]
			if svc.Annotations[defaults.ConfigAnnotationCheManagerName] != "che" {
				t.Errorf("The name of the associated che manager should have been recorded in the service annotation")
			}

			if svc.Annotations[defaults.ConfigAnnotationCheManagerNamespace] != "ns" {
				t.Errorf("The namespace of the associated che manager should have been recorded in the service annotation")
			}

			if svc.Labels[config.WorkspaceIDLabel] != "wsid" {
				t.Errorf("The workspace ID should be recorded in the service labels")
			}
		})
	}

	t.Run("traefikConfig", func(t *testing.T) {
		cms := &corev1.ConfigMapList{}
		cl.List(context.TODO(), cms)

		if len(cms.Items) != 2 {
			t.Errorf("there should be 2 configmaps created for the gateway config of the workspace and che but there were: %d", len(cms.Items))
		}

		var cheMgrCfg *corev1.ConfigMap
		var workspaceCfg *corev1.ConfigMap

		for _, cfg := range cms.Items {
			if cfg.Name == "che" {
				cheMgrCfg = &cfg
			}

			if cfg.Name == "wsid" {
				workspaceCfg = &cfg
			}
		}

		if cheMgrCfg == nil {
			t.Error("traefik configuration for che manager not found")
		}

		if workspaceCfg == nil {
			t.Fatalf("traefik configuration for the workspace not found")
		}

		traefikWorkspaceConfig := workspaceCfg.Data["wsid.yml"]

		if len(traefikWorkspaceConfig) == 0 {
			t.Fatal("No traefik config file found in the workspace config configmap")
		}

		workspaceConfig := traefikConfig{}
		if err := yaml.Unmarshal([]byte(traefikWorkspaceConfig), &workspaceConfig); err != nil {
			t.Fatal(err)
		}

		if len(workspaceConfig.HTTP.Routers) != 1 {
			t.Fatalf("Expected exactly one traefik router but got %d", len(workspaceConfig.HTTP.Routers))
		}

		if _, ok := workspaceConfig.HTTP.Routers["wsid-m1-9999"]; !ok {
			t.Fatal("traefik config doesn't contain expected workspace configuration")
		}
	})
}

func TestReportExposedEndpoints(t *testing.T) {
	routing := simpleWorkspaceRouting()
	_, solver, objs := getSpecObjects(t, routing)

	exposed, ready, err := solver.GetExposedEndpoints(routing.Spec.Endpoints, objs)
	if err != nil {
		t.Fatal(err)
	}

	if !ready {
		t.Errorf("The exposed endpoints should have been ready.")
	}

	if len(exposed) != 1 {
		t.Errorf("There should have been 1 exposed endpoins but found %d", len(exposed))
	}

	m1, ok := exposed["m1"]
	if !ok {
		t.Errorf("The exposed endpoints should have been defined on the m1 machine.")
	}

	if len(m1) != 3 {
		t.Fatalf("There should have been 3 endpoints for m1.")
	}

	e1 := m1[0]
	if e1.Name != "e1" {
		t.Errorf("The first endpoint should have been e1 but is %s", e1.Name)
	}
	if e1.Url != "https://over.the.rainbow/wsid/m1/9999/1/" {
		t.Errorf("The e1 endpoint should have the following URL: '%s' but has '%s'.", "https://over.the.rainbow/wsid/m1/9999/1/", e1.Url)
	}

	e2 := m1[1]
	if e2.Name != "e2" {
		t.Errorf("The second endpoint should have been e2 but is %s", e1.Name)
	}
	if e2.Url != "https://over.the.rainbow/wsid/m1/9999/2.js" {
		t.Errorf("The e2 endpoint should have the following URL: '%s' but has '%s'.", "https://over.the.rainbow/wsid/m1/9999/2.js", e2.Url)
	}

	e3 := m1[2]
	if e3.Name != "e3" {
		t.Errorf("The third endpoint should have been e3 but is %s", e1.Name)
	}
	if e3.Url != "http://over.the.rainbow/wsid/m1/9999/" {
		t.Errorf("The e3 endpoint should have the following URL: '%s' but has '%s'.", "https://over.the.rainbow/wsid/m1/9999/", e3.Url)
	}
}

func TestFinalize(t *testing.T) {
	routing := simpleWorkspaceRouting()
	cl, slv, _ := getSpecObjects(t, routing)

	// the create test checks that during the above call, the solver created the 2 traefik configmaps
	// (1 for the main config and the second for the workspace)

	// now, let the solver finalize the routing
	if err := slv.Finalize(routing); err != nil {
		t.Fatal(err)
	}

	cms := &corev1.ConfigMapList{}
	cl.List(context.TODO(), cms)

	if len(cms.Items) != 1 {
		t.Fatalf("There should be just 1 configmap after routing finalization, but there were %d found", len(cms.Items))
	}

	cm := cms.Items[0]
	if cm.Name != "che" {
		t.Fatal("The only configmap left should be the main traefik config, but the configmap has unexpected name")
	}
}
