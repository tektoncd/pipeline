package e2e

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	knativetest "knative.dev/pkg/test"

	"knative.dev/pkg/test/logging"

	"github.com/tektoncd/pipeline/pkg/names"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var initMetrics sync.Once

const (
	interval = 1 * time.Second
	timeout  = 10 * time.Minute
)

func Setup(t *testing.T) (*Clients, string) {
	t.Helper()
	namespace := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("testcli")

	initializeLogsAndMetrics()
	c := NewClients(knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster, namespace)
	CreateNamespace(namespace, c.KubeClient)
	VerifyServiceAccountExistence(namespace, c.KubeClient)
	return c, namespace
}

func Header(logf logging.FormatLogger, text string) {
	left := "### "
	right := " ###"
	txt := left + text + right
	bar := strings.Repeat("#", len(txt))
	logf(bar)
	logf(txt)
	logf(bar)
}

//Create Service Account
func CreateServiceAccountSecret(c *knativetest.KubeClient, namespace string, secretName string) (bool, error) {

	file := os.Getenv("SERVICE_ACCOUNT_KEY_PATH")
	if file == "" {
		log.Printf("Not creating service account secret, relying on default credentials in namespace %s.", namespace)
		return false, nil
	}

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"username": string(`cHJhdmVlbjRnMA==`),
			"password": string(`UHJhdmVlbkA0ZzA=`),
		},
	}

	_, err := c.Kube.CoreV1().Secrets(namespace).Create(sec)
	if err != nil {
		log.Fatalf("Error in creating Secrets into namespace "+namespace+" err: %s", err)
	}

	log.Printf("Creating service account secret")
	return true, err
}

func TearDown(t *testing.T, cs *Clients, namespace string) {
	t.Helper()
	if cs.KubeClient == nil {
		return
	}
	if t.Failed() {
		Header(t.Logf, fmt.Sprintf("Dumping objects from %s", namespace))
		bs, err := getCRDYaml(cs, namespace)
		if err != nil {
			t.Error(err)
		} else {
			t.Log(string(bs))
		}
		Header(t.Logf, fmt.Sprintf("Dumping logs from Pods in the %s", namespace))
		taskruns, err := cs.TaskRunClient.List(metav1.ListOptions{})
		if err != nil {
			t.Errorf("Error getting TaskRun list %s", err)
		}
		for _, tr := range taskruns.Items {
			if tr.Status.PodName != "" {
				CollectPodLogs(cs, tr.Status.PodName, namespace, t.Logf)
			}
		}
	}

	t.Logf("Deleting namespace %s", namespace)
	if err := cs.KubeClient.Kube.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{}); err != nil {
		t.Errorf("Failed to delete namespace %s: %s", namespace, err)
	}

}

func initializeLogsAndMetrics() {
	initMetrics.Do(func() {
		flag.Parse()
		flag.Set("alsologtostderr", "true")
		logging.InitializeLogger(knativetest.Flags.LogVerbose)

		if knativetest.Flags.EmitMetrics {
			logging.InitializeMetricExporter("")
		}
	})
}

func CreateNamespace(namespace string, kubeClient *knativetest.KubeClient) {
	log.Printf("Create namespace %s to deploy to", namespace)
	if _, err := kubeClient.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}); err != nil {
		log.Printf("Failed to create namespace %s for tests: %s", namespace, err)
	}
}

func DeleteNamespace(namespace string, cs *Clients) {
	log.Printf("Deleting namespace %s", namespace)
	if err := cs.KubeClient.Kube.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{}); err != nil {
		log.Printf("Failed to delete namespace %s: %s", namespace, err)
	}
}

func VerifyServiceAccountExistence(namespace string, kubeClient *knativetest.KubeClient) {
	defaultSA := "default"
	log.Printf("Verify SA %q is created in namespace %q", defaultSA, namespace)

	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		_, err := kubeClient.Kube.CoreV1().ServiceAccounts(namespace).Get(defaultSA, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	}); err != nil {
		log.Printf("Failed to get SA %q in namespace %q for tests: %s", defaultSA, namespace, err)
	}
}

func VerifyServiceAccountExistenceForSecrets(namespace string, kubeClient *knativetest.KubeClient, SA string) {
	defaultSA := SA
	log.Printf("Verify SA %q is created in namespace %q", defaultSA, namespace)

	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		_, err := kubeClient.Kube.CoreV1().ServiceAccounts(namespace).Get(defaultSA, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	}); err != nil {
		log.Printf("Failed to get SA %q in namespace %q for tests: %s", defaultSA, namespace, err)
	}
}

func getCRDYaml(cs *Clients, ns string) ([]byte, error) {
	var output []byte
	printOrAdd := func(kind, name string, i interface{}) {
		bs, err := yaml.Marshal(i)
		if err != nil {
			return
		}
		output = append(output, []byte("\n---\n")...)
		output = append(output, bs...)
	}

	ps, err := cs.PipelineClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.Errorf("could not get pipeline: %w", err)
	}
	for _, i := range ps.Items {
		printOrAdd("Pipeline", i.Name, i)
	}

	prs, err := cs.PipelineResourceClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.Errorf("could not get pipelinerun resource: %w", err)
	}
	for _, i := range prs.Items {
		printOrAdd("PipelineResource", i.Name, i)
	}

	prrs, err := cs.PipelineRunClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.Errorf("could not get pipelinerun: %w", err)
	}
	for _, i := range prrs.Items {
		printOrAdd("PipelineRun", i.Name, i)
	}

	ts, err := cs.TaskClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.Errorf("could not get tasks: %w", err)
	}
	for _, i := range ts.Items {
		printOrAdd("Task", i.Name, i)
	}
	trs, err := cs.TaskRunClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.Errorf("could not get taskrun: %w", err)
	}
	for _, i := range trs.Items {
		printOrAdd("TaskRun", i.Name, i)
	}

	pods, err := cs.KubeClient.Kube.CoreV1().Pods(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.Errorf("could not get pods: %w", err)
	}
	for _, i := range pods.Items {
		printOrAdd("Pod", i.Name, i)
	}

	return output, nil
}
