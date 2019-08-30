package gitkit

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type HookInfo struct {
	RepoName string
	RepoPath string
	OldRev   string
	NewRev   string
	Ref      string
	RefType  string
	RefName  string
}

func ReadHookInput(input io.Reader) (*HookInfo, error) {
	reader := bufio.NewReader(input)

	line, _, err := reader.ReadLine()
	if err != nil {
		return nil, err
	}

	chunks := strings.Split(string(line), " ")
	if len(chunks) != 3 {
		return nil, fmt.Errorf("Invalid hook input")
	}
	refchunks := strings.Split(chunks[2], "/")

	dir, _ := os.Getwd()
	info := HookInfo{
		RepoName: filepath.Base(dir),
		RepoPath: dir,
		OldRev:   chunks[0],
		NewRev:   chunks[1],
		Ref:      chunks[2],
		RefType:  refchunks[1],
		RefName:  refchunks[2],
	}

	return &info, nil
}

func (h *HookInfo) Action() string {
	action := "push"
	context := "branch"

	if h.RefType == "tags" {
		context = "tag"
	}

	if h.OldRev == ZeroSHA && h.NewRev != ZeroSHA {
		action = "create"
	} else if h.OldRev != ZeroSHA && h.NewRev == ZeroSHA {
		action = "delete"
	}

	return context + "." + action
}
