package artifactref

import "regexp"

// case 1: steps.<step-name>.inputs.<artifact-category-name>
// case 2: steps.<step-name>.outputs.<artifact-category-name>
const stepArtifactUsagePattern = `\$\(steps\.([^.]+)\.(?:inputs|outputs)\.([^.)]+)\)`

// case 1: tasks.<task-name>.inputs.<artifact-category-name>
// case 2: tasks.<task-name>.outputs.<artifact-category-name>
// case 3: tasks.<task-name>.outputs.<artifact-category-name>.uri
// case 4: tasks.<task-name>.outputs.<artifact-category-name>.digest
const taskArtifactUsagePattern = `\$\(tasks\.([^.]+)\.(?:inputs|outputs)\.([^.)]+)(?:\.(uri|digest))?\)`

// stepArtifactValuePattern matches $(steps.<step-name>.artifacts.<artifact-name>)
// used in Task artifact output value: fields to reference step-scoped artifacts.
const stepArtifactValuePattern = `\$\(steps\.([^.]+)\.artifacts\.([^.)]+)\)`

const StepArtifactPathPattern = `step.artifacts.path`

const TaskArtifactPathPattern = `artifacts.path`

var StepArtifactRegex = regexp.MustCompile(stepArtifactUsagePattern)
var TaskArtifactRegex = regexp.MustCompile(taskArtifactUsagePattern)
var StepArtifactValueRegex = regexp.MustCompile(stepArtifactValuePattern)
