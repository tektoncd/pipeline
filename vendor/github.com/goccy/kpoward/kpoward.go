package kpoward

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Kpoward.
type Kpoward struct {
	cfg        *rest.Config
	namespace  string
	podName    string
	remotePort uint16
	localPort  uint16
	stdout     io.Writer
	stderr     io.Writer
}

// SetNamespace changes target namespace from default.
func (k *Kpoward) SetNamespace(ns string) {
	k.namespace = ns
}

// SetLocalPort specify fixed local port.
func (k *Kpoward) SetLocalPort(port uint16) {
	k.localPort = port
}

// SetStdout changes output destination for stdout.
func (k *Kpoward) SetStdout(stdout io.Writer) {
	k.stdout = stdout
}

// SetStderr changes output destination for stderr.
func (k *Kpoward) SetStderr(stderr io.Writer) {
	k.stderr = stderr
}

// New create Kpoward instance.
func New(cfg *rest.Config, podName string, remotePort uint16) *Kpoward {
	return &Kpoward{
		cfg:        cfg,
		namespace:  "default",
		podName:    podName,
		remotePort: remotePort,
		stdout:     io.Discard,
		stderr:     io.Discard,
	}
}

// Run will call back the local bound free port.
// You can send any request to this port in the callback.
// Upon exiting the callback, port forwarding will automatically end and the port will be released.
func (k *Kpoward) Run(ctx context.Context, cb func(ctx context.Context, localPort uint16) error) error {
	clientset, err := kubernetes.NewForConfig(k.cfg)
	if err != nil {
		return fmt.Errorf("kpoward: failed to create clientset: %w", err)
	}

	pod, err := clientset.CoreV1().Pods(k.namespace).Get(ctx, k.podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("kpoward: failed to get pod by name(%s): %w", k.podName, err)
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("kpoward: specified pod is not running(%v)", pod.Status.Phase)
	}

	reqURL, err := url.Parse(
		fmt.Sprintf(
			"%s/api/v1/namespaces/%s/pods/%s/portforward",
			k.cfg.Host,
			k.namespace,
			k.podName,
		),
	)
	if err != nil {
		return fmt.Errorf("could not build URL for portforward: %w", err)
	}
	transport, upgrader, err := spdy.RoundTripperFor(k.cfg)
	if err != nil {
		return fmt.Errorf("kpoward: failed to process round tripper: %w", err)
	}
	stopChannel := make(chan struct{}, 1)
	readyChannel := make(chan struct{})
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", reqURL)
	fw, err := portforward.NewOnAddresses(
		dialer,
		[]string{"127.0.0.1"},
		[]string{fmt.Sprintf("%d:%d", k.localPort, k.remotePort)},
		stopChannel,
		readyChannel,
		k.stdout,
		k.stderr,
	)
	if err != nil {
		return fmt.Errorf("kpoward: failed to create port forwarder: %w", err)
	}
	defer func() {
		stopChannel <- struct{}{}
	}()
	go func() {
		fw.ForwardPorts()
	}()
	select {
	case <-readyChannel:
	case <-ctx.Done():
		return fmt.Errorf("kpoward: failed to start port forwarder: %w", ctx.Err())
	}
	ports, err := fw.GetPorts()
	if err != nil {
		return fmt.Errorf("kpoward: failed to get ports: %w", err)
	}
	if len(ports) != 1 {
		return fmt.Errorf("kpoward: failed to get expected ports: %+v", ports)
	}
	if err := cb(ctx, ports[0].Local); err != nil {
		return err
	}
	return nil
}
