package infrastructure

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Type specifies what kind of infrastructure we're operating in.
type Type uint

// Generation the major version of the infrastructure
type Generation uint

// Kind represents the kind of infrastructure we're running on
type Kind struct {
	Type       Type
	Generation Generation
}

const (
	// Undetected is the type of the infrastructure that could not be detected
	Undetected Type = 0

	// Kubernetes represents Kubernetes infrastructure
	Kubernetes Type = 1

	// OpenShift represents the OpenShift infrastrcture
	OpenShift Type = 2

	// Unknown represents a generation of the infrastructure that either could not be detected or has only a single possible value
	Unknown Generation = 0

	// V3 represents OpenShift v3
	V3 Generation = 1

	// V4 represents OpenShift v4
	V4 Generation = 2
)

var (
	// Current is the infrastructure that we're currently running on. Can have an Undetected type if the detection fails.
	Current Kind
)

func init() {
	Current = detect()
}

// IsLatest returns true if the infrastructure is at its latest detected generation
func (k Kind) IsLatest() bool {
	if k.Type == OpenShift {
		return k.Generation == V4
	}
	return true
}

// GetOperatorNamespace returns the namespace the operator should be running in.
//
// This function was ported over from Operator SDK 0.17.0 and modified.
func GetOperatorNamespace() (string, error) {
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("could not read namespace from mounted serviceaccount info")
		}
		return "", err
	}
	ns := strings.TrimSpace(string(nsBytes))
	return ns, nil
}

func detect() Kind {
	kubeCfg, err := config.GetConfig()
	if err != nil {
		return Kind{Type: Undetected, Generation: Unknown}
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeCfg)
	if err != nil {
		return Kind{Type: Undetected, Generation: Unknown}
	}
	apiList, err := discoveryClient.ServerGroups()
	if err != nil {
		return Kind{Type: Undetected, Generation: Unknown}
	}
	if findAPIGroup(apiList.Groups, "route.openshift.io") == nil {
		return Kind{Type: Kubernetes, Generation: Unknown}
	} else {
		if findAPIGroup(apiList.Groups, "config.openshift.io") == nil {
			return Kind{Type: OpenShift, Generation: V3}
		} else {
			return Kind{Type: OpenShift, Generation: V4}
		}
	}
}

func findAPIGroup(source []metav1.APIGroup, apiName string) *metav1.APIGroup {
	for i := 0; i < len(source); i++ {
		if source[i].Name == apiName {
			return &source[i]
		}
	}
	return nil
}
