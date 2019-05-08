package printer

import (
	"io"

	"k8s.io/apimachinery/pkg/runtime"
	cliopts "k8s.io/cli-runtime/pkg/genericclioptions"
)

func PrintObject(out io.Writer, o runtime.Object, f *cliopts.PrintFlags) error {
	printer, err := f.ToPrinter()
	if err != nil {
		return err
	}
	return printer.PrintObj(o, out)
}
