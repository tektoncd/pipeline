package artifactref

import "regexp"

// case 1: steps.<step-name>.inputs.<artifact-category-name>
// case 2: steps.<step-name>.outputs.<artifact-category-name>
const stepArtifactUsagePattern = `\$\(steps\.([^.]+)\.(?:inputs|outputs)\.([^.)]+)\)`

// case 1: tasks.<task-name>.inputs.<artifact-category-name>
// case 2: tasks.<task-name>.outputs.<artifact-category-name>
const taskArtifactUsagePattern = `\$\(tasks\.([^.]+)\.(?:inputs|outputs)\.([^.)]+)\)`

const StepArtifactPathPattern = `step.artifacts.path`

const TaskArtifactPathPattern = `artifacts.path`

var StepArtifactRegex = regexp.MustCompile(stepArtifactUsagePattern)
var TaskArtifactRegex = regexp.MustCompile(taskArtifactUsagePattern)
