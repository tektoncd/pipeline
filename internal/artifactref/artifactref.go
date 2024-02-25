package artifactref

import "regexp"

// case 1: steps.<step-name>.inputs
// case 2: steps.<step-name>.outputs
// case 3: steps.<step-name>.inputs.<artifact-category-name>
// case 4: steps.<step-name>.outputs.<artifact-category-name>
const stepArtifactUsagePattern = `\$\(steps\.([^.]+)\.(?:inputs|outputs)(?:\.([^.^\)]+))?\)`

var StepArtifactRegex = regexp.MustCompile(stepArtifactUsagePattern)
