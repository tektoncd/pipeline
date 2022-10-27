#!/bin/sh
# This script file is used to exit after step breakpoint and continue subsequent steps.
set -e
numberOfSteps=%d
debugInfo=%s
tektonRun=%s

postFile="$(ls ${debugInfo} | grep -E '[0-9]+' | tail -1)"
stepNumber="$(echo ${postFile} | sed 's/[^0-9]*//g')"

if [ $stepNumber -lt $numberOfSteps ]; then
	echo "0" > ${tektonRun}/${stepNumber}/out.afterstepexit
	echo "Executing step $stepNumber..."
else
	echo "Last step (no. $stepNumber) has already been executed, after step breakpoint exiting !"
	exit 0
fi