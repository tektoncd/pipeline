module github.com/tektoncd/pipeline

go 1.13

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.1 // indirect
	github.com/GoogleCloudPlatform/cloud-builders/gcs-fetcher v0.0.0-20191203181535-308b93ad1f39
	github.com/aws/aws-sdk-go v1.30.16 // indirect
	github.com/cloudevents/sdk-go/v2 v2.0.0
	github.com/docker/cli v0.0.0-20200210162036-a4bedce16568 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/google/go-cmp v0.4.1
	github.com/google/go-containerregistry v0.0.0-20200331213917-3d03ed9b1ca2
	github.com/google/uuid v1.1.1
	github.com/grpc-ecosystem/grpc-gateway v1.12.2 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/jenkins-x/go-scm v1.5.117
	github.com/mailru/easyjson v0.7.1-0.20191009090205-6c0755d89d1e // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/tektoncd/plumbing v0.0.0-20200430135134-e53521e1d887
	go.opencensus.io v0.22.3
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20200323165209-0ec3e9974c59 // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	gomodules.xyz/jsonpatch/v2 v2.1.0
	google.golang.org/protobuf v1.22.0 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
	k8s.io/api v0.17.6
	k8s.io/apiextensions-apiserver v0.17.6 // indirect
	k8s.io/apimachinery v0.17.6
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/code-generator v0.18.0
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29
	k8s.io/utils v0.0.0-20200124190032-861946025e34 // indirect
	knative.dev/caching v0.0.0-20200521155757-e78d17bc250e
	knative.dev/pkg v0.0.0-20200528142800-1c6815d7e4c9
	sigs.k8s.io/yaml v1.2.0 // indirect
)

// Needed for the sarama dependency above to work...
replace github.com/cloudevents/sdk-go/v2 => github.com/cloudevents/sdk-go/v2 v2.0.0

// Knative deps (release-0.15)
replace (
	contrib.go.opencensus.io/exporter/stackdriver => contrib.go.opencensus.io/exporter/stackdriver v0.12.9-0.20191108183826-59d068f8d8ff
	github.com/Azure/azure-sdk-for-go => github.com/Azure/azure-sdk-for-go v38.2.0+incompatible
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.4.0+incompatible
	knative.dev/caching => knative.dev/caching v0.0.0-20200521155757-e78d17bc250e
	knative.dev/pkg => knative.dev/pkg v0.0.0-20200528142800-1c6815d7e4c9
)

// Pin k8s deps to 1.16.5
replace (
	k8s.io/api => k8s.io/api v0.16.5
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.5
	k8s.io/client-go => k8s.io/client-go v0.16.5
	k8s.io/code-generator => k8s.io/code-generator v0.16.5
)
