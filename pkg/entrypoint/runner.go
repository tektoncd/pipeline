package entrypoint

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

// TODO(jasonhall): Test that original exit code is propagated and that
// stdout/stderr are collected -- needs e2e tests.

// RealRunner actually runs commands.
type RealRunner struct{}

var _ Runner = (*RealRunner)(nil)

func (r *RealRunner) Run(args ...string) error {
	if len(args) == 0 {
		return nil
	}
	name, args := args[0], args[1:]

	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

type LoggingRunner struct{}

var _ Runner = (*LoggingRunner)(nil)

func (l *LoggingRunner) Run(args ...string) error {
	if len(args) == 0 {
		return nil
	}
	name, args := args[0], args[1:]

	b, err := ioutil.ReadFile("/tekton/logCfg/labels")
	if err != nil {
		return err
	}
	labels := map[string]string{}
	for _, l := range strings.Split(string(b), "\n") {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if !strings.HasPrefix(parts[0], "tekton.dev/") {
			continue
		}
		labels[parts[0]] = strings.Trim(parts[1], "\"")
	}

	cmd := exec.Command(name, args...)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	go teeOut(stdout, os.Stdout, labels)
	go teeOut(stderr, os.Stderr, labels)

	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func teeOut(r io.Reader, w io.Writer, labels map[string]string) {
	s := bufio.NewScanner(r)
	var b []byte
	var err error
	for s.Scan() {
		labels["message"] = s.Text()
		b, err = json.Marshal(labels)
		if err != nil {
			log.Printf("error marshalling text %v: %s", labels, err)
			return
		}
		fmt.Fprintln(w, string(b))
	}
}
