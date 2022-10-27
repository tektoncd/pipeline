package pod

import (
	_ "embed"
)

// TODO(#3972): Use text/template templating instead of %s based templating
//
//go:embed scripts/debug-continue.sh
var debugContinueScriptTemplate string

//go:embed scripts/debug-fail-continue.sh
var debugFailScriptTemplate string

//go:embed scripts/debug-beforestep-continue.sh
var debugBeforeContinueScriptTemplate string

//go:embed scripts/debug-beforestep-fail.sh
var debugBeforeFailScriptTemplate string

//go:embed scripts/debug-afterstep-continue.sh
var debugAfterContinueScriptTemplate string

//go:embed scripts/debug-afterstep-fail.sh
var debugAfterFailScriptTemplate string

var initScriptDirective = `tmpfile="%s"
touch ${tmpfile} && chmod +x ${tmpfile}
cat > ${tmpfile} << '%s'
%s
%s
`
