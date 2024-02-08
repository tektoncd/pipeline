package artifactref

import "regexp"

const stepArtifactUsagePattern = `\$\(steps\..*\.outputs\..*\)`

var StepArtifactOutputRegex = regexp.MustCompile(stepArtifactUsagePattern)

