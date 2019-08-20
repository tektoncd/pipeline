package completion

var zshCompletion = `
# Custom function for Completions
function __tkn_get_object() {
    res=($(tkn ${1} ls -o template --template="{{ range .items  }}{{ .metadata.name }} {{ end }}" 2>/dev/null))
    compadd $res
}

function __kubectl_get_object() {
    res=($(kubectl get ${1} -o template --template="{{ range .items  }}{{ .metadata.name }} {{ end }}" 2>/dev/null))
    compadd $res
}

function __kubectl_get_namespace() { __kubectl_get_object namespace }
function __tkn_get_pipeline() { __tkn_get_object pipeline; }
function __tkn_get_pipelinerun() { __tkn_get_object pipelinerun; }
function __tkn_get_taskrun() { __tkn_get_object taskrun; }
function __tkn_get_pipelineresource() { __tkn_get_object pipelineresource; }
`
