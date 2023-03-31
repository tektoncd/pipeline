/*
Copyright 2023 The Tekton Authors

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

package pod

// TODO(#3972): Use text/template templating instead of %s based templating
const (
	debugContinueScriptTemplate = `
numberOfSteps=%d
debugInfo=%s
tektonRun=%s

postFile="$(ls ${debugInfo} | grep -E '[0-9]+' | tail -1)"
stepNumber="$(echo ${postFile} | sed 's/[^0-9]*//g')"

if [ $stepNumber -lt $numberOfSteps ]; then
	touch ${tektonRun}/${stepNumber}/out # Mark step as success
	echo "0" > ${tektonRun}/${stepNumber}/out.breakpointexit
	echo "Executing step $stepNumber..."
else
	echo "Last step (no. $stepNumber) has already been executed, breakpoint exiting !"
	exit 0
fi`
	debugFailScriptTemplate = `
numberOfSteps=%d
debugInfo=%s
tektonRun=%s

postFile="$(ls ${debugInfo} | grep -E '[0-9]+' | tail -1)"
stepNumber="$(echo ${postFile} | sed 's/[^0-9]*//g')"

if [ $stepNumber -lt $numberOfSteps ]; then
	touch ${tektonRun}/${stepNumber}/out.err # Mark step as a failure
	echo "1" > ${tektonRun}/${stepNumber}/out.breakpointexit
	echo "Executing step $stepNumber..."
else
	echo "Last step (no. $stepNumber) has already been executed, breakpoint exiting !"
	exit 0
fi`
	initScriptDirective = `tmpfile="%s"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << '%s'
%s
%s
`
)
