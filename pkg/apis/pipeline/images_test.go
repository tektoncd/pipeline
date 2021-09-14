package pipeline_test

import (
	"flag"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
)

func TestValidate(t *testing.T) {
	fs := flag.FlagSet{}
	valid := pipeline.NewFlagOptions(&fs)
	fs.Parse([]string{
		"-entrypoint-image", "set",
		"-nop-image", "set",
		"-git-image", "set",
		"-kubeconfig-writer-image", "set",
		"-shell-image", "set",
		"-shell-image-win", "set",
		"-gsutil-image", "set",
		"-pr-image", "set",
		"-imagedigest-exporter-image", "set",
	})
	if err := valid.Images.Validate(); err != nil {
		t.Errorf("valid Images returned error: %v", err)
	}

	fs = flag.FlagSet{}
	invalid := pipeline.NewFlagOptions(&fs)
	fs.Parse([]string{
		"-entrypoint-image", "set",
		"-nop-image", "set",
		// "-git-image", "unset",
		"-kubeconfig-writer-image", "set",
		// "-shell-image", "unset",
		"-shell-image-win", "set",
		"-gsutil-image", "set",
		// "-pr-image", "unset",
		"-imagedigest-exporter-image", "set",
	})
	wantErr := "found unset image flags: [git-image pr-image shell-image]"
	if err := invalid.Images.Validate(); err == nil {
		t.Error("invalid Images expected error, got nil")
	} else if err.Error() != wantErr {
		t.Errorf("Unexpected error message: got %q, want %q", err.Error(), wantErr)
	}
}
