module github.com/tektoncd/pipeline

go 1.13

require (
	cloud.google.com/go v0.47.0 // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.1.0 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.12.8 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/GoogleCloudPlatform/cloud-builders/gcs-fetcher v0.0.0-20191203181535-308b93ad1f39
	github.com/cloudevents/sdk-go v0.0.0-20190509003705-56931988abe3
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.19.4 // indirect
	github.com/gobuffalo/envy v1.7.1 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/google/go-cmp v0.3.1
	github.com/google/go-containerregistry v0.0.0-20200115214256-379933c9c22b
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/gregjones/httpcache v0.0.0-20190212212710-3befbb6ad0cc // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.3
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/jenkins-x/go-scm v1.5.65
	github.com/json-iterator/go v1.1.8 // indirect
	github.com/markbates/inflect v1.0.4 // indirect
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/nbio/st v0.0.0-20140626010706-e9e8d9816f32 // indirect
	github.com/onsi/ginkgo v1.10.1 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/common v0.7.0 // indirect
	github.com/prometheus/procfs v0.0.5 // indirect
	github.com/shurcooL/githubv4 v0.0.0-20191102174205-af46314aec7b // indirect
	github.com/tektoncd/plumbing v0.0.0-20191216083742-847dcf196de9
	github.com/vdemeester/k8s-pkg-credentialprovider v1.13.12-1 // indirect
	go.opencensus.io v0.22.1
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/net v0.0.0-20191119073136-fc4aabc6c914 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e // indirect
	golang.org/x/sys v0.0.0-20191210023423-ac6580df4449 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	google.golang.org/api v0.10.0 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.2.5 // indirect
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.1
	k8s.io/client-go v0.17.0
	k8s.io/code-generator v0.17.1
	k8s.io/gengo v0.0.0-20191108084044-e500ee069b5c // indirect
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	knative.dev/caching v0.0.0-20190719140829-2032732871ff
	knative.dev/eventing-contrib v0.6.1-0.20190723221543-5ce18048c08b
	knative.dev/pkg v0.0.0-20191111150521-6d806b998379
)

// Knative deps

replace (
	contrib.go.opencensus.io/exporter/stackdriver => contrib.go.opencensus.io/exporter/stackdriver v0.12.5
	knative.dev/pkg => knative.dev/pkg v0.0.0-20190909195211-528ad1c1dd62
	knative.dev/pkg/vendor/github.com/spf13/pflag => github.com/spf13/pflag v1.0.5
)

// Pin k8s deps to 1.13.4

replace (
	k8s.io/api => k8s.io/api v0.0.0-20191004102255-dacd7df5a50b
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20191004074956-01f8b7d1121a
	k8s.io/client-go => k8s.io/client-go v0.0.0-20191004102537-eb5b9a8cfde7
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20181117043124-c2090bec4d9b
	k8s.io/gengo => k8s.io/gengo v0.0.0-20190327210449-e17681d19d3a
)
