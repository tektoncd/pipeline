package subcommands

// InitCommand is the name of main initialization command
const InitCommand = "init"

// init copies the entrypoint to the right place and sets up /tekton/steps directory for the pod.
// This expects  the list of steps (in order matching the Task spec).
func entrypointInit(src, dst string, steps []string) error {
	if err := cp(src, dst); err != nil {
		return err
	}
	return stepInit(steps)
}
