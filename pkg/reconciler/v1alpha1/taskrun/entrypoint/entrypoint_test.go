package entrypoint_test

import (
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/entrypoint"
	"github.com/knative/build/pkg/apis/build/v1alpha1"
	"k8s.io/api/core/v1"
	"testing"
)
const (
	kanikoImage = "gcr.io/kaniko-project/executor"
	kanikoEntrypoint = "/kaniko/executor"
)

func TestAddEntrypoint(t *testing.T) {
	inputs := []v1.Container{
		{
			Image: kanikoImage,
		},
		{
			Image: kanikoImage,
			Args: []string{"abcd"},
		},
		{
			Image: kanikoImage,
			Command: []string{"abcd"},
			Args: []string{"efgh"},
		},
	}
	// The first test case showcases the downloading of the entrypoint for the
	// image. The second test shows downloading the image as well as the args
	// being passed in. The third command shows a set Command overriding the
	// remote one.
	envVarStrings := []string{
		`{"args":null,"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
		`{"args":["abcd"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
		`{"args":["abcd","efgh"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,

	}
	err := entrypoint.RedirectSteps(inputs)
	if err != nil {
		t.Errorf("failed to get resources: %v", err)
	}
	for i, input := range inputs {
		if len(input.Command) == 0 || input.Command[0] != entrypoint.BinaryLocation {
			t.Errorf("command incorrectly set: %q", input.Command)
		}
		if len(input.Args) > 0 {
			t.Errorf("containers should have no args")
		}
		if len(input.Env) == 0 {
			t.Error("there should be atleast one envvar")
		}
		for _, e := range input.Env {
			if e.Name == entrypoint.JSONConfigEnvVar && e.Value != envVarStrings[i] {
				t.Errorf("envvar \n%s\n does not match \n%s", e.Value, envVarStrings[i])
			}
		}
		found := false
		for _, vm := range input.VolumeMounts {
			if vm.Name == entrypoint.MountName {
				found = true
				break
			}
		}
		if !found {
			t.Error("could not find tools volume mount")
		}
	}
}

func TestGetRemoteEntrypoint(t *testing.T) {
	ep, err := entrypoint.GetRemoteEntrypoint(entrypoint.NewCache(), kanikoImage)
	if err != nil {
		t.Errorf("couldn't get entrypoint remote: %v", err)
	}
	if len(ep) != 1 {
		t.Errorf("remote entrypoint should only have one item")
	}
	if ep[0] != kanikoEntrypoint {
		t.Errorf("entrypoints do not match: %s should be %s", ep[0], kanikoEntrypoint)
	}
}

func TestAddCopyStep(t *testing.T) {
	bs := &v1alpha1.BuildSpec{
		Steps: []v1.Container{
			{
				Name: "test",
			},
			{
				Name: "test",
			},
		},
	}
	expectedSteps := len(bs.Steps) + 1
	entrypoint.AddCopyStep(bs)
	if len(bs.Steps) != 3 {
		t.Errorf("BuildSpec has the wrong step count: %d should be %d", len(bs.Steps), expectedSteps)
	}
	if bs.Steps[0].Name != entrypoint.InitContainerName {
		t.Errorf("entrypoint is incorrect: %s should be %s", bs.Steps[0].Name, entrypoint.InitContainerName)
	}
}
