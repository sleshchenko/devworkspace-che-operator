package manager

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/che-incubator/devworkspace-che-operator/apis/che-controller/v1alpha1"
	"github.com/che-incubator/devworkspace-che-operator/pkg/defaults"
)

func (r *CheReconciler) singlehostFinalize(ctx context.Context, manager *v1alpha1.CheManager) error {
	// we need to stop the reconcile if there are workspaces handled by it.
	// we detect that by the presence of the gateway configmaps in the namespace of the manager
	list := corev1.ConfigMapList{}

	err := r.client.List(ctx, &list, &client.ListOptions{
		Namespace:     manager.Namespace,
		LabelSelector: labels.SelectorFromSet(defaults.GetLabelsForComponent(manager, "gateway-config")),
	})
	if err != nil {
		return err
	}

	workspaceCount := 0

	for _, c := range list.Items {
		if c.Annotations[defaults.ConfigAnnotationCheManagerName] == manager.Name && c.Annotations[defaults.ConfigAnnotationCheManagerNamespace] == manager.Namespace {
			workspaceCount++
		}
	}

	if workspaceCount > 0 {
		return fmt.Errorf("there are %d workspaces associated with this Che manager", workspaceCount)
	}

	return nil
}
