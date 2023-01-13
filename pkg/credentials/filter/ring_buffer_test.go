package filter_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/credentials/filter"
)

func TestCanAddAndPop(t *testing.T) {
	buf := filter.NewRingBuffer(5)
	buf.Add(1)
	buf.Add(2)
	buf.Add(3)

	one := buf.PopFirst()
	two := buf.PopFirst()
	three := buf.PopFirst()

	if one != 1 {
		t.Errorf("First byte should be 1, was %b", one)
	}
	if two != 2 {
		t.Errorf("Second byte should be 2, was %b", two)
	}
	if three != 3 {
		t.Errorf("Third byte should be 3, was %b", three)
	}
}
