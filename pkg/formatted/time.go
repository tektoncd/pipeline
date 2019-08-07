package formatted

import (
	"github.com/hako/durafmt"
	"github.com/jonboulle/clockwork"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Age(t *metav1.Time, c clockwork.Clock) string {
	if t.IsZero() {
		return "---"
	}

	dur := c.Since(t.Time)
	return durafmt.ParseShort(dur).String() + " ago"
}

func Duration(t1, t2 *metav1.Time) string {
	if t1.IsZero() || t2.IsZero() {
		return "---"
	}

	dur := t2.Time.Sub(t1.Time)
	return durafmt.ParseShort(dur).String()
}
