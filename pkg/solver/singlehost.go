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
	"context"
	"fmt"
	"path"
	"strings"

	dwoche "github.com/che-incubator/devworkspace-che-operator/apis/che-controller/v1alpha1"
	"github.com/che-incubator/devworkspace-che-operator/pkg/defaults"
	"github.com/che-incubator/devworkspace-che-operator/pkg/sync"
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	uniqueEndpointAttributeName = "unique"
	endpointURLPrefixPattern    = "/%s/%s/%d"
	// note - che-theia DEPENDS on this format - we should not change this unless crosschecked with the che-theia impl
	uniqueEndpointURLPrefixPattern = "/%s/%s/%s"
)

var (
	configMapDiffOpts = cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta", "ObjectMeta")
)

func (c *CheRoutingSolver) singlehostSpecObjects(cheManager *dwoche.CheManager, routing *dwo.DevWorkspaceRouting, workspaceMeta solvers.WorkspaceMetadata) (solvers.RoutingObjects, error) {
	objs := solvers.RoutingObjects{}

	objs.Services = solvers.GetDiscoverableServicesForEndpoints(routing.Spec.Endpoints, workspaceMeta)

	commonService := solvers.GetServiceForEndpoints(routing.Spec.Endpoints, workspaceMeta, false, dw.PublicEndpointExposure, dw.InternalEndpointExposure)
	if commonService != nil {
		objs.Services = append(objs.Services, *commonService)
	}

	annos := map[string]string{}
	annos[defaults.ConfigAnnotationCheManagerName] = cheManager.Name
	annos[defaults.ConfigAnnotationCheManagerNamespace] = cheManager.Namespace

	additionalLabels := defaults.GetLabelsForComponent(cheManager, "exposure")

	for i := range objs.Services {
		// need to use a ref otherwise s would be a copy
		s := &objs.Services[i]

		if s.Labels == nil {
			s.Labels = map[string]string{}
		}

		for k, v := range additionalLabels {

			if len(s.Labels[k]) == 0 {
				s.Labels[k] = v
			}
		}

		if s.Annotations == nil {
			s.Annotations = map[string]string{}
		}

		for k, v := range annos {

			if len(s.Annotations[k]) == 0 {
				s.Annotations[k] = v
			}
		}
	}

	// k, now we have to create our own objects for configuring the gateway
	configMaps, err := c.getGatewayConfigMaps(cheManager, workspaceMeta.WorkspaceId, routing)
	if err != nil {
		return solvers.RoutingObjects{}, err
	}

	syncer := sync.New(c.client, c.scheme)

	for _, cm := range configMaps {
		_, _, err := syncer.Sync(context.TODO(), nil, &cm, configMapDiffOpts)
		if err != nil {
			return solvers.RoutingObjects{}, err
		}

	}
	return objs, nil
}

func (c *CheRoutingSolver) singlehostExposedEndpoints(manager *dwoche.CheManager, workspaceID string, endpoints map[string]dwo.EndpointList, routingObj solvers.RoutingObjects) (exposedEndpoints map[string]dwo.ExposedEndpointList, ready bool, err error) {
	if manager.Status.GatewayPhase != dwoche.GatewayPhaseEstablished {
		return nil, false, nil
	}

	host := manager.Status.GatewayHost
	if host == "" {
		return nil, false, nil
	}

	exposed := map[string]dwo.ExposedEndpointList{}

	for machineName, endpoints := range endpoints {
		exposedEndpoints := dwo.ExposedEndpointList{}
		for _, endpoint := range endpoints {
			if endpoint.Exposure != dw.PublicEndpointExposure {
				continue
			}

			var scheme string
			if endpoint.Protocol == "" {
				scheme = "http"
			} else {
				scheme = string(endpoint.Protocol)
			}

			if scheme != "http" && scheme != "https" {
				// we cannot expose non-http endpoints publicly, because ingresses/routes only support http(s)
				continue
			}

			if endpoint.Secure {
				scheme = "https"

				// TODO this should also do the magic of ensuring user authentication however we are going to do it
				// in the future
			}

			publicURLPrefix := getPublicURLPrefixForEndpoint(workspaceID, machineName, endpoint)

			publicURL := scheme + "://" + path.Join(host, publicURLPrefix, endpoint.Path)

			// path.Join() removes the trailing slashes, so make sure to reintroduce that if required.
			if endpoint.Path == "" || strings.HasSuffix(endpoint.Path, "/") {
				publicURL = publicURL + "/"
			}

			exposedEndpoints = append(exposedEndpoints, dwo.ExposedEndpoint{
				Name:       endpoint.Name,
				Url:        publicURL,
				Attributes: endpoint.Attributes,
			})
		}
		exposed[machineName] = exposedEndpoints
	}

	return exposed, true, nil
}

func (c *CheRoutingSolver) getGatewayConfigMaps(cheManager *dwoche.CheManager, workspaceID string, routing *dwo.DevWorkspaceRouting) ([]corev1.ConfigMap, error) {
	restrictedAnno, setRestrictedAnno := routing.Annotations[constants.WorkspaceRestrictedAccessAnnotation]

	labels := defaults.GetLabelsForComponent(cheManager, "gateway-config")
	labels[constants.WorkspaceIDLabel] = workspaceID
	if setRestrictedAnno {
		labels[constants.WorkspaceRestrictedAccessAnnotation] = restrictedAnno
	}

	configMap := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      defaults.GetGatewayWorkpaceConfigMapName(workspaceID),
			Namespace: cheManager.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				defaults.ConfigAnnotationDevWorkspaceRoutingName:      routing.Name,
				defaults.ConfigAnnotationDevWorkspaceRoutingNamespace: routing.Namespace,
			},
		},
		Data: map[string]string{},
	}

	rtrs := map[string]traefikConfigRouter{}
	srvcs := map[string]traefikConfigService{}
	mdls := map[string]traefikConfigMiddleware{}

	for machineName, endpoints := range routing.Spec.Endpoints {
		// we need to support unique endpoints - so 1 port can actually be accessible
		// multiple times, each time using a different resulting external URL.
		// non-unique endpoints are all represented using a single external URL
		ports := map[int32]map[string]bool{}
		for _, e := range endpoints {
			i := int32(e.TargetPort)

			name := ""
			if e.Attributes.GetBoolean(uniqueEndpointAttributeName, nil) {
				name = e.Name
			}

			if ports[i] == nil {
				ports[i] = map[string]bool{}
			}

			ports[i][name] = true
		}

		for port, names := range ports {
			for endpointName := range names {
				var name string
				var prefix string
				var serviceURL string

				if endpointName == "" {
					name = fmt.Sprintf("%s-%s-%d", workspaceID, machineName, port)
				} else {
					name = fmt.Sprintf("%s-%s-%d-%s", workspaceID, machineName, port, endpointName)
				}
				prefix = getPublicURLPrefix(workspaceID, machineName, port, endpointName)
				serviceURL = getServiceURL(port, workspaceID, routing.Namespace)

				rtrs[name] = traefikConfigRouter{
					Rule:        fmt.Sprintf("PathPrefix(`%s`)", prefix),
					Service:     name,
					Middlewares: []string{name},
					Priority:    100,
				}

				srvcs[name] = traefikConfigService{
					LoadBalancer: traefikConfigLoadbalancer{
						Servers: []traefikConfigLoadbalancerServer{
							{
								URL: serviceURL,
							},
						},
					},
				}

				mdls[name] = traefikConfigMiddleware{
					StripPrefix: traefikConfigStripPrefix{
						Prefixes: []string{prefix},
					},
				}

			}
		}
	}

	config := traefikConfig{
		HTTP: traefikConfigHTTP{
			Routers:     rtrs,
			Services:    srvcs,
			Middlewares: mdls,
		},
	}

	contents, err := yaml.Marshal(config)
	if err != nil {
		return []corev1.ConfigMap{}, err
	}

	configMap.Data[workspaceID+".yml"] = string(contents)

	return []corev1.ConfigMap{configMap}, nil
}

func (c *CheRoutingSolver) singlehostFinalize(cheManager *dwoche.CheManager, routing *dwo.DevWorkspaceRouting) error {
	configs := &corev1.ConfigMapList{}

	selector, err := labels.Parse(fmt.Sprintf("%s=%s", constants.WorkspaceIDLabel, routing.Spec.WorkspaceId))
	if err != nil {
		return err
	}

	listOpts := &client.ListOptions{
		Namespace:     cheManager.Namespace,
		LabelSelector: selector,
	}

	err = c.client.List(context.TODO(), configs, listOpts)
	if err != nil {
		return err
	}

	for _, cm := range configs.Items {
		err = c.client.Delete(context.TODO(), &cm)
		if err != nil {
			return err
		}
	}

	return nil
}

func getServiceURL(port int32, workspaceID string, workspaceNamespace string) string {
	// the default .cluster.local suffix of the internal domain names seems to be configurable, so let's just
	// not use it so we don't have to know about it...
	return fmt.Sprintf("http://%s.%s.svc:%d", common.ServiceName(workspaceID), workspaceNamespace, port)
}

func getPublicURLPrefixForEndpoint(workspaceID string, machineName string, endpoint dw.Endpoint) string {
	endpointName := ""
	if endpoint.Attributes.GetString(uniqueEndpointAttributeName, nil) == "true" {
		endpointName = endpoint.Name
	}

	return getPublicURLPrefix(workspaceID, machineName, int32(endpoint.TargetPort), endpointName)
}

func getPublicURLPrefix(workspaceID string, machineName string, port int32, uniqueEndpointName string) string {
	if uniqueEndpointName == "" {
		return fmt.Sprintf(endpointURLPrefixPattern, workspaceID, machineName, port)
	}
	return fmt.Sprintf(uniqueEndpointURLPrefixPattern, workspaceID, machineName, uniqueEndpointName)
}
