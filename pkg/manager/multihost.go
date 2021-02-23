package manager

import (
	"context"
	"fmt"

	"github.com/che-incubator/devworkspace-che-operator/apis/che-controller/v1alpha1"
)

func (r *CheReconciler) multihostFinalize(ctx context.Context, manager *v1alpha1.CheManager) error {
	// Multihost not supported at the moment
	return fmt.Errorf("Multihost mode not supported at the moment.")
}
