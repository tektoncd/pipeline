package e2e

import (
	"gotest.tools/v3/icmd"
	"log"
	"testing"
	"time"
)

type TknRunner struct {
	path string
}

func NewTknRunner(tknPath string) TknRunner {
	return TknRunner{
		path: tknPath,
	}
}

func (tkn *TknRunner) BuildTknClient() string {

	res := icmd.RunCmd(icmd.Command("go", "build", tkn.path))

	if res.ExitCode != 0 {
		log.Fatalf("Go Build Failed....")
	}

	return " tekton client build successfully!"
}

func Prepare(t *testing.T) func(args ...string) icmd.Cmd {
	run := func(args ...string) icmd.Cmd {
		return icmd.Cmd{Command: append([]string{"./tkn"}, args...), Timeout: 10 * time.Minute}
	}
	return run
}
