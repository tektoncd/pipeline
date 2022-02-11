module github.com/tektoncd/pipeline

go 1.16

require (
	github.com/cloudevents/sdk-go/v2 v2.5.0
	github.com/containerd/containerd v1.5.9
	github.com/ghodss/yaml v1.0.0
	github.com/google/go-cmp v0.5.7
	github.com/google/go-containerregistry v0.8.1-0.20220211173031-41f8d92709b7
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20220120151853-ac864e57b117
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v0.5.4
	github.com/jenkins-x/go-scm v1.10.10
	github.com/mitchellh/go-homedir v1.1.0
	github.com/opencontainers/image-spec v1.0.3-0.20220114050600-8b9d41f48198
	github.com/pkg/errors v0.9.1
	github.com/tektoncd/plumbing v0.0.0-20211012143332-c7cc43d9bc0c
	go.opencensus.io v0.23.0
	go.uber.org/zap v1.19.1
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	gomodules.xyz/jsonpatch/v2 v2.2.0
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	k8s.io/code-generator v0.22.5
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20220114203427-a0453230fd26
	knative.dev/pkg v0.0.0-20220131144930-f4b57aef0006
)

require (
	cloud.google.com/go/compute v1.1.0 // indirect
	github.com/Azure/azure-sdk-for-go v61.3.0+incompatible // indirect
	github.com/aws/aws-sdk-go-v2/config v1.13.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.14.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.11.0 // indirect
	github.com/emicklei/go-restful v2.15.0+incompatible // indirect
	github.com/google/go-containerregistry/pkg/authn/kubernetes v0.0.0-20220120123041-d22850aca581 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/time v0.0.0-20211116232009-f0f3c7e86c11 // indirect
	k8s.io/utils v0.0.0-20211208161948-7d6a63dca704 // indirect
)
