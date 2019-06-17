package pods

import (
	"bufio"
	"fmt"
	"github.com/tektoncd/cli/pkg/helper/pods/stream"
	"io"
	corev1 "k8s.io/api/core/v1"
)

type Container struct {
	name        string
	NewStreamer stream.NewStreamerFunc
	pod         *Pod
}

func (c *Container) Status() error {
	pod, err := c.pod.Get()
	if err != nil {
		return err
	}

	container := c.name
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name != container {
			continue
		}

		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode == 1 {
			msg := ""

			if cs.State.Terminated.Reason != "" {
				msg = msg + " : " + cs.State.Terminated.Reason
			}

			if cs.State.Terminated.Message != "" {
				msg = msg + " : " + cs.State.Terminated.Message
			}

			return fmt.Errorf("container %s has failed %s", container, msg)
		}
	}

	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.Name != container {
			continue
		}

		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode == 1 {
			return fmt.Errorf("container %s has failed: %s", container, cs.State.Terminated.Reason)
		}
	}

	return nil
}

// Log represents one log message from a pod
type Log struct {
	PodName       string
	ContainerName string
	Log           string
}
type LogReader struct {
	containerName string
	pod           *Pod
	follow        bool
}

func (c *Container) LogReader(follow bool) *LogReader {
	return &LogReader{c.name, c.pod, follow}
}

func (lr *LogReader) Read() (<-chan Log, <-chan error, error) {
	pod := lr.pod
	opts := &corev1.PodLogOptions{
		Follow:    lr.follow,
		Container: lr.containerName,
	}

	stream, err := pod.Stream(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("error getting logs for pod %s(%s) : %s", pod.Name, lr.containerName, err)
	}

	logC := make(chan Log)
	errC := make(chan error)

	go func() {
		defer close(logC)
		defer close(errC)
		defer stream.Close()

		r := bufio.NewReader(stream)
		for {
			line, _, err := r.ReadLine()

			if err != nil {
				if err != io.EOF {
					errC <- err
				}
				return
			}

			logC <- Log{
				PodName:       pod.Name,
				ContainerName: lr.containerName,
				Log:           string(line),
			}
		}
	}()

	return logC, errC, nil
}
