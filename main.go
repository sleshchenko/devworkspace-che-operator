//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package main

import (
	"flag"
	"os"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/workspacerouting"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/che-incubator/devworkspace-che-operator/apis/che-controller/v1alpha1"
	"github.com/che-incubator/devworkspace-che-operator/pkg/infrastructure"
	"github.com/che-incubator/devworkspace-che-operator/pkg/manager"
	"github.com/che-incubator/devworkspace-che-operator/pkg/solver"
	"github.com/devfile/devworkspace-operator/pkg/config"
	routev1 "github.com/openshift/api/route/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(controllerv1alpha1.AddToScheme(scheme))
	utilruntime.Must(extensions.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(rbac.AddToScheme(scheme))

	if infrastructure.Current.Type == infrastructure.OpenShift {
		utilruntime.Must(routev1.AddToScheme(scheme))
	}
}

func main() {

	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	if infrastructure.Current.Type == infrastructure.Undetected {
		setupLog.Error(nil, "Unable to detect the Kubernetes infrastructure.")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "8d217f94.devfile.io",
	})

	if err != nil {
		setupLog.Error(err, "unable to start the operator manager")
		os.Exit(1)
	}

	if err = setupControllerConfig(mgr); err != nil {
		setupLog.Error(err, "unable to read controller configuration")
		os.Exit(1)
	}

	cheReconciler := &manager.CheReconciler{}
	if err = cheReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Che")
		os.Exit(1)
	}

	routingReconciler := &workspacerouting.WorkspaceRoutingReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("WorkspaceRouting"),
		Scheme:       mgr.GetScheme(),
		SolverGetter: solver.Getter(scheme),
	}

	if err = routingReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CheWorkspaceRoutingSolver")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// Welcome to the world of co-operating controllers running in multiple processes.
// We're "inheriting" a lot of code from the devworkspace-operator and are hooking into its
// workspace routing solver loop, which means that such code is running in a different operator
// than it expects and therefore some of the config read by the devworkspace-operator might
// not be available in this operator.
// We just need to hope that is not the case (it should not be at the time of writing of this comment).
func setupControllerConfig(mgr ctrl.Manager) error {
	// This is how the devworkspace-operator watches for changes in its config map. We obviously cannot
	// use it, because we're most probably running in a different namespace and might not even have enough
	// permissions to read that config map. It would also be weird to create our own separate copy of the config map,
	// because then there would be 2 places to configure the same thing.

	// operatorNamespace, err := infrastructure.GetOperatorNamespace()
	// if err == nil {
	// 	config.ConfigMapReference.Namespace = operatorNamespace
	// } else {
	// 	config.ConfigMapReference.Namespace = os.Getenv(infrastructure.ControllerConfigNamespaceEnvVar)
	// }
	// config.ConfigMapReference.Name = "devworkspace-che-operator-config"
	// err = config.WatchControllerConfig(mgr)
	// if err != nil {
	// 	return err
	// }

	// Fortunately, there is a way of telling the controller config (used in devworkspace-operator codepaths)
	// that we're running on openshift. We have our own way of figuring that out, because devworkspace-operator
	// does that in an internal package not accessible to us.
	config.ControllerCfg.SetIsOpenShift(infrastructure.Current.Type == infrastructure.OpenShift)

	// The validation can be done only when controller config knows if it is running on openshift.
	// But we don't actually want to validate the config, because we're not reading any atm.

	// err = config.ControllerCfg.Validate()
	// if err != nil {
	// 	setupLog.Error(err, "Controller configuration is invalid")
	// 	return err
	// }

	return nil
}
