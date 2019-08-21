package flags

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestFlags_add_shell_completion(t *testing.T) {
	newflag := "newflag"
	shellfunc := "__test_function"
	cmd := cobra.Command{}
	cmd.PersistentFlags().String(newflag, "", "Completion pinpon pinpon 🎤")

	pflag := cmd.PersistentFlags().Lookup(newflag)
	AddShellCompletion(pflag, shellfunc)

	if pflag.Annotations[cobra.BashCompCustom] == nil {
		t.Errorf("annotation should be have been added to the flag")
	}

	if pflag.Annotations[cobra.BashCompCustom][0] != shellfunc {
		t.Errorf("annotation should have been added to the flag")
	}

}
