package util

import "github.com/che-incubator/devworkspace-che-operator/apis/che-controller/v1alpha1"

// IsSingleHost is a helper function to figure out if the manager is configured for the singlehost mode
func IsSingleHost(mgr *v1alpha1.CheManager) bool {
	routing := mgr.Spec.Routing
	return routing == "" || routing == v1alpha1.SingleHost
}
