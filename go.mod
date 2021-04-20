module github.com/che-incubator/devworkspace-che-operator

go 1.13

require (
	github.com/devfile/api/v2 v2.0.0-20210402145809-1f335562c475
	github.com/devfile/devworkspace-operator v0.2.1-0.20210408131455-c6a22a3a7d8b
	github.com/google/go-cmp v0.5.0
	github.com/openshift/api v0.0.0-20200205133042-34f0ec8dab87
	k8s.io/api v0.18.8
	k8s.io/apiextensions-apiserver v0.18.8
	k8s.io/apimachinery v0.18.8
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/devfile/devworkspace-operator v0.2.1-0.20210408131455-c6a22a3a7d8b => github.com/tinakurian/devworkspace-operator v0.0.0-20210420133618-ecc7e209d26c
