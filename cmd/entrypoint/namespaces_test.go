// +build linux

package main

import (
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// This isn't a great unit test, but it's the best I can think of.
// It attempts to verify there is no network access by making a network
// request. If the test were to run in an offline environment, or an already
// sandboxed environment, the test could pass even if the dropNetworking
// function did nothing.
func TestDropNetworking(t *testing.T) {
	cmd := exec.Command("curl", "google.com")
	dropNetworking(cmd)
	b, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected an error making a network connection. Got %s", string(b))
	}

	// Other things (env, etc.) should all be the same
	cmds := []string{"env", "whoami", "pwd", "uname"}
	for _, cmd := range cmds {
		withNetworking := exec.Command(cmd)
		withoutNetworking := exec.Command(cmd)
		dropNetworking(withoutNetworking)

		b1, err1 := withNetworking.CombinedOutput()
		b2, err2 := withoutNetworking.CombinedOutput()
		if err1 != err2 {
			t.Errorf("Expected no errors, got %v %v", err1, err2)
		}
		if diff := cmp.Diff(string(b1), string(b2)); diff != "" {
			t.Error(diff)
		}
	}
}
