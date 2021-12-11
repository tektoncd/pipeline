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
