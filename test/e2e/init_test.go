// +build e2e

package e2e

import (
	"log"
	"os"
	"testing"

	"gotest.tools/v3/env"
	"gotest.tools/v3/fs"
)

var originalDir string
var dir = &fs.Dir{}
var t = &testing.T{}

const (
	tknContext = "github.com/tektoncd/cli/cmd/tkn"
)

func TestMain(m *testing.M) {
	log.Println("Running main e2e Test suite ")
	SetupProcess()
	v := m.Run()
	TearDownProcess()
	os.Exit(v)
}

func SetupProcess() {
	tkncli := NewTknRunner(tknContext)
	dir := fs.NewDir(t, "testcli")
	env.ChangeWorkingDir(t, dir.Path())
	tkncli.BuildTknClient()
}

func TearDownProcess() {
	dir.Remove()
}
