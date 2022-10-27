#!/bin/sh
# This script file is used to exit before step breakpoint and process current step
set -e
numberOfSteps=%d
debugInfo=%s
tektonRun=%s

postFile="$(ls ${debugInfo} | grep -E '[0-9]+' | tail -1)"
stepNumber="$(echo ${postFile} | sed 's/[^0-9]*//g')"

if [ $stepNumber -lt $numberOfSteps ]; then
	echo "0" > ${tektonRun}/${stepNumber}/out.beforestepexit
	echo "Executing step $stepNumber..."
else
	echo "Last step (no. $stepNumber) has already been executed, before step breakpoint exiting !"
	exit 0
fi