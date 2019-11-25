package pod

import (
	"fmt"
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

const (
	scriptsVolumeName = "scripts"
	scriptsDir        = "/builder/scripts"
)

var (
	// Volume definition attached to Pods generated from TaskRuns that have
	// steps that specify a Script.
	// TODO(#1605): Generate volumeMount names, to avoid collisions.
	// TODO(#1605): Unexport these vars when Pod conversion is entirely within
	// this package.
	ScriptsVolume = corev1.Volume{
		Name:         scriptsVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
	ScriptsVolumeMount = corev1.VolumeMount{
		Name:      scriptsVolumeName,
		MountPath: scriptsDir,
	}
)

// ConvertScripts converts any steps that specify a Script field into a normal Container.
//
// It does this by prepending a container that writes specified Script bodies
// to executable files in a shared volumeMount, then produces Containers that
// simply run those executable files.
func ConvertScripts(shellImage string, steps []v1alpha1.Step) (*corev1.Container, []corev1.Container) {
	placeScripts := false
	placeScriptsInit := corev1.Container{
		Name:         names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("place-scripts"),
		Image:        shellImage,
		TTY:          true,
		Command:      []string{"sh"},
		Args:         []string{"-c", ""},
		VolumeMounts: []corev1.VolumeMount{ScriptsVolumeMount},
	}

	var containers []corev1.Container
	for i, s := range steps {
		if s.Script == "" {
			// Nothing to convert.
			containers = append(containers, s.Container)
			continue
		}

		// At least one step uses a script, so we should return a
		// non-nil init container.
		placeScripts = true

		// Append to the place-scripts script to place the
		// script file in a known location in the scripts volume.
		tmpFile := filepath.Join(scriptsDir, names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("script-%d", i)))
		// heredoc is the "here document" placeholder string
		// used to cat script contents into the file. Typically
		// this is the string "EOF" but if this value were
		// "EOF" it would prevent users from including the
		// string "EOF" in their own scripts. Instead we
		// randomly generate a string to (hopefully) prevent
		// collisions.
		heredoc := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("script-heredoc-randomly-generated")
		placeScriptsInit.Args[1] += fmt.Sprintf(`tmpfile="%s"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << '%s'
%s
%s
`, tmpFile, heredoc, s.Script, heredoc)

		// Set the command to execute the correct script in the mounted volume.
		steps[i].Command = []string{tmpFile}
		steps[i].VolumeMounts = append(steps[i].VolumeMounts, ScriptsVolumeMount)
		containers = append(containers, steps[i].Container)
	}

	if placeScripts {
		return &placeScriptsInit, containers
	}
	return nil, containers
}
