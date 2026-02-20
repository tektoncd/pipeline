/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package args

import (
	"errors"
	"path/filepath"

	"k8s.io/gengo/v2"
	"k8s.io/klog/v2"

	"github.com/spf13/pflag"
)

// Args is used by the gengo framework to pass args specific to this generator.
type Args struct {
	InputDirs         []string
	OutputPackagePath string
	OutputDir         string
	GoHeaderFilePath  string

	VersionedClientSetPackage        string
	ExternalVersionsInformersPackage string
	ListersPackage                   string
	ForceKinds                       string
	ListerHasPointerElem             bool
	DisableInformerInit              bool

	// PluralExceptions define a list of pluralizer exceptions in Type:PluralType format.
	// The default list is "Endpoints:Endpoints"
	PluralExceptions []string

	Boilerplate []byte
}

func New() *Args {
	return &Args{}
}

// AddFlags add the generator flags to the flag set.
func (a *Args) AddFlags(fs *pflag.FlagSet) {
	fs.StringSliceVarP(&a.InputDirs, "input-dirs", "i", a.InputDirs, "Comma-separated list of import paths to get input types from.")
	fs.StringVarP(&a.OutputPackagePath, "output-package", "p", a.OutputPackagePath, "Base package path.")
	fs.StringVarP(&a.OutputDir, "output-dir", "o", a.OutputDir, "Output directory.")
	fs.StringVarP(&a.GoHeaderFilePath, "go-header-file", "h", a.GoHeaderFilePath, "File containing boilerplate header text. The string YEAR will be replaced with the current 4-digit year.")

	fs.StringVar(&a.VersionedClientSetPackage, "versioned-clientset-package", a.VersionedClientSetPackage, "the full package name for the versioned injection clientset to use")
	fs.StringVar(&a.ExternalVersionsInformersPackage, "external-versions-informers-package", a.ExternalVersionsInformersPackage, "the full package name for the external versions injection informer to use")
	fs.StringVar(&a.ListersPackage, "listers-package", a.ListersPackage, "the full package name for client listers to use")
	fs.StringVar(&a.ForceKinds, "force-genreconciler-kinds", a.ForceKinds, `force kinds will override the genreconciler tag setting for the given set of kinds, comma separated: "Foo,Bar,Baz"`)

	fs.BoolVar(&a.ListerHasPointerElem, "lister-has-pointer-elem", false, "")
	fs.MarkDeprecated("lister-has-pointer-elem", "this flag has no effect")

	fs.BoolVar(&a.DisableInformerInit, "disable-informer-init", false, "disable generating the init function for the informer")
	fs.StringSliceVar(&a.PluralExceptions, "plural-exceptions", a.PluralExceptions,
		"list of comma separated plural exception definitions in Type:PluralizedType format")
}

// Validate checks the given arguments.
func (a *Args) Validate() error {
	var err error
	a.Boilerplate, err = gengo.GoBoilerplate(a.GoHeaderFilePath, "", gengo.StdGeneratedBy)
	if err != nil {
		klog.Fatalf("Failed loading boilerplate: %v", err)
	}

	if len(a.OutputPackagePath) == 0 {
		return errors.New("output package cannot be empty")
	}
	if len(a.VersionedClientSetPackage) == 0 {
		return errors.New("versioned clientset package cannot be empty")
	}
	if len(a.ExternalVersionsInformersPackage) == 0 {
		return errors.New("external versions informers package cannot be empty")
	}
	return nil
}

func (a *Args) GetOutputDir() string {
	return filepath.Clean(a.OutputDir)
}

func (a *Args) GetOutputPackagePath() string {
	return filepath.Clean(a.OutputPackagePath)
}
