module github.com/tektoncd/pipeline

go 1.13

require (
	cloud.google.com/go/storage v1.8.0
	contrib.go.opencensus.io/exporter/stackdriver v0.13.1 // indirect
	github.com/GoogleCloudPlatform/cloud-builders/gcs-fetcher v0.0.0-20191203181535-308b93ad1f39
	github.com/cloudevents/sdk-go/v2 v2.1.0
	github.com/docker/cli v0.0.0-20200210162036-a4bedce16568 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/google/go-cmp v0.4.1
	github.com/google/go-containerregistry v0.1.1
	github.com/google/uuid v1.1.1
	github.com/grpc-ecosystem/grpc-gateway v1.12.2 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/jenkins-x/go-scm v1.5.117
	github.com/mailru/easyjson v0.7.1-0.20191009090205-6c0755d89d1e // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/tektoncd/plumbing v0.0.0-20200430135134-e53521e1d887
	go.opencensus.io v0.22.4
	go.uber.org/zap v1.15.0
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/text v0.3.3 // indirect
	gomodules.xyz/jsonpatch/v2 v2.1.0
	google.golang.org/api v0.25.0
	k8s.io/api v0.17.6
	k8s.io/apimachinery v0.17.6
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/code-generator v0.18.0
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29
	knative.dev/pkg v0.0.0-20200702222342-ea4d6e985ba0
)

// Knative deps (release-0.16)
replace (
	contrib.go.opencensus.io/exporter/stackdriver => contrib.go.opencensus.io/exporter/stackdriver v0.12.9-0.20191108183826-59d068f8d8ff
	github.com/Azure/azure-sdk-for-go => github.com/Azure/azure-sdk-for-go v38.2.0+incompatible
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.4.0+incompatible
)

// Pin k8s deps to 1.17.6
replace (
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/apiserver => k8s.io/apiserver v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29
)
