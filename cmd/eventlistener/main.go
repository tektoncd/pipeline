package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"sync"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/joeshaw/envdecode"
	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	"github.com/knative/pkg/logging"
	"github.com/pkg/errors"
	gh "gopkg.in/go-playground/webhooks.v5/github"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	SupportCloudEventVersion = "0.2"
	listenerPath             = "/events"

	CloudEventType = "cloud-event"
)

type Config struct {
	Event            string `env:"EVENT,default=cloudevent"`
	EventType        string `env:"EVENT_TYPE,default=com.github.checksuite"`
	MasterURL        string `env:"MASTER_URL"`
	Kubeconfig       string `env:"KUBECONFIG"`
	Namespace        string `env:"NAMESPACE"`
	ListenerResource string `env:"LISTENER_RESOURCE"`
	Port             int    `env:"PORT"`
}

// EventListener boots cloudevent receiver and awaits a particular event to build
type EventListener struct {
	event     string
	eventType string
	namespace string
	clientset clientset.Interface
	mux       *sync.Mutex
	runSpec   v1alpha1.PipelineRunSpec
}

func main() {
	var cfg Config
	err := envdecode.Decode(&cfg)
	if err != nil {
		log.Fatalf("Failed loading env config: %q", err)
	}

	logger, _ := logging.NewLogger("", "event-listener")
	defer logger.Sync()

	if cfg.Namespace == "" {
		log.Fatal("NAMESPACE env var can not be empty")
	}

	clientcfg, err := clientcmd.BuildConfigFromFlags(cfg.MasterURL, cfg.Kubeconfig)
	if err != nil {
		logger.Fatalf("Error building kubeconfig: %v", err)
	}

	pipelineClient, err := clientset.NewForConfig(clientcfg)
	if err != nil {
		logger.Fatalf("Error building pipeline clientset: %v", err)
	}

	e := &EventListener{
		event:     cfg.Event,
		eventType: cfg.EventType,
		port:      cfg.Port,
		namespace: cfg.Namespace,
		mux:       &sync.Mutex{},
		clientset: pipelineClient,
	}

	listener, err := e.clientset.Tekton().PipelineListeners(e.namespace).Get(cfg.ListenerResource)
	if err != nil {
		log.Fatalf("failed to get pipeline listener spec: %q", err)
	}
	e.runSpec = listener.Spec

	switch e.event {
	case cloudEventType:
		startCloudEventListener() // handle cloud events
	default:
		log.Fatalf("invalid event type: %q", err)
	}
}

func startCloudEventListener() {
	log.Printf("Starting listener on port %d", listenerPort)

	t, err := http.New(
		http.WithPort(listenerPort),
		http.WithPath(listenerPath),
	)
	if err != nil {
		log.Fatalf("failed to create http client, %v", err)
	}
	client, err := client.New(t, client.WithTimeNow(), client.WithUUIDs())
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	log.Fatalf("Failed to start cloudevent receiver: %q", client.StartReceiver(context.Background(), c.HandleRequest))
}

// HandleRequest will decode the body of the cloudevent into the correct payload type based on event type,
// match on the event type and submit build from repo/branch.
// Only check_suite events are supported by this proposal.
func (r *CloudEventListener) HandleRequest(ctx context.Context, event cloudevents.Event) error {
	// todo: contribute nil check upstream
	if event.Context == nil {
		return errors.New("Empty event context")
	}

	if event.SpecVersion() != "0.2" {
		return errors.New("Only cloudevents version 0.2 supported")
	}
	if event.Type() != r.eventType {
		return errors.New("Mismatched event type submitted")

	}
	ec, ok := event.Context.(cloudevents.EventContextV02)
	if !ok {
		return errors.New("Cloudevent context missing")
	}

	log.Printf("Handling event ID: %q Type: %q", ec.ID, ec.GetType())

	switch event.Type() {
	case "com.github.checksuite":
		cs := &gh.CheckSuitePayload{}
		if err := event.DataAs(cs); err != nil {
			return errors.Wrap(err, "Error handling check suite payload")
		}
		if err := r.handleCheckSuite(event, cs); err != nil {
			return err
		}
	}

	return nil
}

func (r *CloudEventListener) handleCheckSuite(event cloudevents.Event, cs *gh.CheckSuitePayload) error {
	if cs.CheckSuite.Conclusion == "success" {
		if cs.CheckSuite.HeadBranch != r.branch {
			return fmt.Errorf("Mismatched branches. Expected %s Received %s", cs.CheckSuite.HeadBranch, r.branch)

		}

		build, err := r.createPipelineRun(cs.CheckSuite.HeadSHA)
		if err != nil {
			return errors.Wrapf(err, "Error creating build for check_suite event ID: %q", event.Context.AsV02().ID)
		}

		log.Printf("Created build %q!", build.Name)
	}
	return nil
}

func (r *CloudEventListener) createPipelineRun(sha string) (*v1alpha1.Build, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	pr := v1alpha1.PipelineRun{
		Name:      "",
		Namespace: "",
	}

	build := r.build.DeepCopy()
	// Set the builds git revision to the github events SHA
	build.Spec.Source.Git.Revision = sha
	// Set namespace from config. If they dont match, create will fail.
	build.Namespace = r.namespace

	log.Printf("Creating build %q sha %q namespace %q", build.Name, sha, build.Namespace)

	newBuild, err := r.clientset.V1alpha1().PipelineRun(build.Namespace).Create(pr)
	if err != nil {
		return nil, err
	}
	return newBuild, nil
}

// Read in the build spec info we have prepared to handle builds!
func loadBuildSpec(path string) (*v1alpha1.Build, error) {
	b := new(v1alpha1.Build)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(data), &b); err != nil {
		return nil, err
	}
	return b, nil
}
