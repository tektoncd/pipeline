module github.com/tektoncd/pipeline

go 1.16

replace (
	k8s.io/api => k8s.io/api v0.22.5
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.5
	k8s.io/client-go => k8s.io/client-go v0.22.5
	k8s.io/code-generator => k8s.io/code-generator v0.22.5
)

require (
	github.com/cloudevents/sdk-go/v2 v2.5.0
	github.com/containerd/containerd v1.5.10
	github.com/ghodss/yaml v1.0.0
	github.com/google/go-cmp v0.5.7
	github.com/google/go-containerregistry v0.8.1-0.20220216220642-00c59d91847c
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20220216220642-00c59d91847c
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
	golang.org/x/oauth2 v0.0.0-20220223155221-ee480838109b
	gomodules.xyz/jsonpatch/v2 v2.2.0
	k8s.io/api v0.23.4
	k8s.io/apimachinery v0.23.4
	k8s.io/client-go v0.23.4
	k8s.io/code-generator v0.22.5
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20220124234850-424119656bbf
	knative.dev/pkg v0.0.0-20220131144930-f4b57aef0006
)

require (
	cloud.google.com/go/compute v1.3.0 // indirect
	github.com/Azure/azure-sdk-for-go v61.5.0+incompatible // indirect
	github.com/docker/distribution v2.8.0+incompatible // indirect
	github.com/emicklei/go-restful v2.15.0+incompatible // indirect
	github.com/golang-jwt/jwt/v4 v4.3.0 // indirect
	github.com/google/go-containerregistry/pkg/authn/kubernetes v0.0.0-20220223122423-dd8d514a9b24 // indirect
	github.com/klauspost/compress v1.14.4 // indirect
	go.uber.org/goleak v1.1.12 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/crypto v0.0.0-20220214200702-86341886e292 // indirect
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f // indirect
	golang.org/x/sys v0.0.0-20220227234510-4e6760a101f9 // indirect
	golang.org/x/time v0.0.0-20220210224613-90d013bbcef8 // indirect
	google.golang.org/genproto v0.0.0-20220303160752-862486edd9cc // indirect
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9 // indirect
)
