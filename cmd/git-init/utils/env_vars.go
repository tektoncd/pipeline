package utils

import (
	"fmt"
	"strings"

)

type PullRefs struct {
	BaseBranch string
	BaseSha string
	ToMerge map[string] string
}

// ParsePullRefs parses the Prow PULL_REFS env var formatted string and converts to a map of branch:sha
func ParsePullRefs(pullRefs string) (*PullRefs, error) {
	kvs := strings.Split(pullRefs, ",")
	answer := PullRefs{
		ToMerge: make(map[string]string),
	}
	for i, kv := range kvs {
		s := strings.Split(kv, ":")
		if len(s) != 2 {
			return nil, fmt.Errorf("incorrect format for branch:sha %s", kv)
		}
		if i == 0 {
			answer.BaseBranch = s[0]
			answer.BaseSha = s[1]
		} else {
			answer.ToMerge[s[0]] = s[1]
		}
	}
	return &answer, nil
}
