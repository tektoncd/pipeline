package labels

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
)

const (
	addLabel    = "/jx-label "
	removeLabel = " remove"
	emptyString = ""
)

// ConvertLabelComments converts comments to labels for git providers which don't support native labels
func ConvertLabelComments(cs []*scm.Comment) ([]*scm.Label, error) {
	sort.SliceStable(cs, func(i, j int) bool {
		return cs[i].Created.UnixNano() < cs[j].Created.UnixNano()
	})
	m := make(map[string]bool)
	for _, com := range cs {
		if strings.HasPrefix(com.Body, addLabel) {
			t := strings.ReplaceAll(com.Body, addLabel, emptyString)
			if strings.HasSuffix(t, removeLabel) {
				t = strings.ReplaceAll(t, removeLabel, emptyString)
				m[t] = true
			} else {
				m[t] = false
			}
		}
	}
	var ls []*scm.Label
	for l, i := range m {
		if !i {
			ls = append(ls, &scm.Label{Name: l})
		}
	}
	return ls, nil
}

// CreateLabelAddComment creates a label comment for git providers which don't support labels with comment with /jx-label <name>
func CreateLabelAddComment(label string) *scm.CommentInput {
	return &scm.CommentInput{
		Body: fmt.Sprintf("%s%s", addLabel, label),
	}
}

// CreateLabelRemoveComment adds a comment with /jx-label <name> remove
func CreateLabelRemoveComment(label string) *scm.CommentInput {
	input := &scm.CommentInput{
		Body: fmt.Sprintf("%s%s%s", addLabel, label, removeLabel),
	}
	return input
}
