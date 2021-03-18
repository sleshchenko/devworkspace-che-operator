//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package solver

import (
	"fmt"

	dwoche "github.com/che-incubator/devworkspace-che-operator/apis/che-controller/v1alpha1"
	dw "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
)

func (c *CheRoutingSolver) multihostSpecObjects(cheManager *dwoche.CheManager, routing *dw.DevWorkspaceRouting, workspaceMeta solvers.WorkspaceMetadata) (solvers.RoutingObjects, error) {
	return solvers.RoutingObjects{}, fmt.Errorf("multihost currently not supported")
}

func (c *CheRoutingSolver) multihostExposedEndpoints(manager *dwoche.CheManager, workspaceID string, endpoints map[string]dw.EndpointList, routingObj solvers.RoutingObjects) (exposedEndpoints map[string]dw.ExposedEndpointList, ready bool, err error) {
	return nil, false, fmt.Errorf("multihost currently not supported")
}

func (c *CheRoutingSolver) multihostFinalize(cheManager *dwoche.CheManager, routing *dw.DevWorkspaceRouting) error {
	return nil
}
