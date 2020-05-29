module github.com/tektoncd/pipeline

go 1.13

require (
	cloud.google.com/go/storage v1.6.0 // indirect
	contrib.go.opencensus.io/exporter/ocagent v0.6.0 // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.1.0 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.13.1 // indirect
	github.com/GoogleCloudPlatform/cloud-builders/gcs-fetcher v0.0.0-20191203181535-308b93ad1f39
	github.com/aws/aws-sdk-go v1.30.16 // indirect
	github.com/cloudevents/sdk-go/v2 v2.0.0
	github.com/docker/cli v0.0.0-20200210162036-a4bedce16568 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.19.4 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/google/go-cmp v0.4.1
	github.com/google/go-containerregistry v0.0.0-20200331213917-3d03ed9b1ca2
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.12.2 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/jenkins-x/go-scm v1.5.117
	github.com/mailru/easyjson v0.7.1-0.20191009090205-6c0755d89d1e // indirect
	github.com/markbates/inflect v1.0.4 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/nbio/st v0.0.0-20140626010706-e9e8d9816f32 // indirect
	github.com/openzipkin/zipkin-go v0.2.2 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.5.0 // indirect
	github.com/shurcooL/githubv4 v0.0.0-20191102174205-af46314aec7b // indirect
	github.com/tektoncd/plumbing v0.0.0-20200430135134-e53521e1d887
	go.opencensus.io v0.22.3
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20200323165209-0ec3e9974c59 // indirect
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a // indirect
	gomodules.xyz/jsonpatch/v2 v2.1.0
	google.golang.org/api v0.20.0 // indirect
	google.golang.org/grpc v1.28.0 // indirect
	google.golang.org/protobuf v1.22.0 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
	k8s.io/api v0.17.6
	k8s.io/apiextensions-apiserver v0.17.6 // indirect
	k8s.io/apimachinery v0.17.6
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/code-generator v0.18.0
	k8s.io/gengo v0.0.0-20191108084044-e500ee069b5c // indirect
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29
	k8s.io/utils v0.0.0-20200124190032-861946025e34 // indirect
	knative.dev/caching v0.0.0-20200521155757-e78d17bc250e
	knative.dev/pkg v0.0.0-20200528190300-08a86da47d28
	sigs.k8s.io/yaml v1.2.0 // indirect
)

// Needed for the sarama dependency above to work...
replace github.com/cloudevents/sdk-go/v2 => github.com/cloudevents/sdk-go/v2 v2.0.0

// Knative deps (release-0.14)
replace (
	contrib.go.opencensus.io/exporter/stackdriver => contrib.go.opencensus.io/exporter/stackdriver v0.12.9-0.20191108183826-59d068f8d8ff
	knative.dev/caching => knative.dev/caching v0.0.0-20200116200605-67bca2c83dfa
	knative.dev/pkg => knative.dev/pkg v0.0.0-20200410152005-2a1db869228c
	knative.dev/pkg/vendor/github.com/spf13/pflag => github.com/spf13/pflag v1.0.5
)

// Pin k8s deps to 1.16.5
replace (
	k8s.io/api => k8s.io/api v0.16.5
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.5
	k8s.io/client-go => k8s.io/client-go v0.16.5
	k8s.io/code-generator => k8s.io/code-generator v0.16.5
	k8s.io/gengo => k8s.io/gengo v0.0.0-20190327210449-e17681d19d3a
)
