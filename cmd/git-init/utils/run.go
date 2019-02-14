package utils

import (
	"go.uber.org/zap"
	"os/exec"
	"strings"
)

func RunAndError(logger *zap.SugaredLogger, cmd string, args ...string) {
	if output, err := Run(cmd, "", args...); err != nil {
		logger.Errorf("Error running %v %v: %v\n%v", cmd, args, err, output)
	}
}

func Run(cmd string, dir string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	if dir != "" {
		c.Dir = dir
	}
	out, err :=  c.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func RunOrFail(logger *zap.SugaredLogger, cmd string, args ...string) {
	if output, err := Run(cmd, "", args...); err != nil {
		logger.Fatalf("Unexpected error running %v %v: %v\n%v", cmd, args, err, output)
	}
}
