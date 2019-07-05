package completion

var zsh_completion = `
#compdef _tkn tkn
local -a common_options common_describe_options common_logs_options

common_options=(
    '(-k --kubeconfig)'{-k,--kubeconfig}'[kubectl config file (default: $HOME/.kube/config)]:files:_files' \
    '(-n --namespace)'{-n,--namespace}'[namespace to use (default: from $KUBECONFIG)]:namespace:__kubectl_get_namespace'
)

common_describe_options=(
    "$common_options[@]" \
    '--allow-missing-template-keys[If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats.]' \
    '(-o --output)'{-o,--output}'[Output format. One of: json|yaml|name|template|go-template|go-template-file|templatefile|jsonpath|jsonpath-file]:outputs:("json" "yaml" "name" "template" "go-template" "go-template-file" "jsonpath" "jsonpath-file")' \
    '--template[Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates \[http://golang.org/pkg/text/template/#pkg-overview\].]:filename:_files'
)

common_logs_options=(
    "$common_options[@]" \
    '(-a --all)'{-a,--all}'[show all logs including init steps injected by tekton]' \
    '(-f --follow)'{-f,--follow}'[stream live logs]'
)

function __tkn_get_object() {
    res=($(tkn ${1} ls -o template --template="{{ range .items  }}{{ .metadata.name }} {{ end }}" 2>/dev/null))
    compadd $res
}

function __kubectl_get_object() {
    res=($(kubectl get ${1} -o template --template="{{ range .items  }}{{ .metadata.name }} {{ end }}" 2>/dev/null))
    compadd $res
}
function __kubectl_get_namespace() { __kubectl_get_object namespace }

function _tkn {
  local -a commands

  _arguments -C \
    "1: :->cmnds" \
    "*::arg:->args"

  case $state in
  cmnds)
    commands=(
      "help:Help about any command"
      "pipeline:Manage pipelines"
      "pipelinerun:Manage pipelineruns"
      "resource:Manage pipeline resources"
      "task:Manage tasks"
      "taskrun:Manage taskruns"
      "version:Prints version information"
    )
    _describe "command" commands
    ;;
  esac

  case "$words[1]" in
  help)
    _tkn_help
    ;;
  p|pipeline|pipelines)
    _tkn_pipeline
    ;;
  pr|pipelinerun|pipelineruns)
    _tkn_pipelinerun
    ;;
  res|resource|resources)
    _tkn_resource
    ;;
  t|task|tasks)
    _tkn_task
    ;;
  tr|taskrun|taskruns)
    _tkn_taskrun
    ;;
  version)
    _tkn_version
    ;;
  esac
}

function _tkn_help {
  _arguments
}


function __tkn_get_pipeline() { __tkn_get_object pipeline; }

function _tkn_pipeline {
  local -a commands

  _arguments -C \
    "$common_options[@]" \
    "1: :->cmnds" \
    "*::arg:->args"

  case $state in
  cmnds)
    commands=(
      "describe:Describes a pipeline in a namespace"
      "list:Lists pipelines in a namespace"
      "start:Start pipelines"
    )
    _describe "command" commands
    ;;
  esac

  case "$words[1]" in
  describe|desc)
    _tkn_pipeline_describe
    ;;
  list|ls)
    _tkn_pipeline_list
    ;;
  start|trigger)
    _tkn_pipeline_start
    ;;
  esac
}

function _tkn_pipeline_describe {
  _arguments \
    "$common_describe_options[@]" \
    '1: :__tkn_get_pipeline'
}

function _tkn_pipeline_list {
  _arguments \
    "$common_describe_options[@]" \
}

function _tkn_pipeline_start {
  _arguments \
    "$common_options[@]" \
    '(*-p *--param)'{\*-p,\*--param}'[pass the param]:' \
    '(*-r *--resource)'{\*-r,\*--resource}'[pass the resource name and ref]:' \
    '(-s --serviceaccount)'{-s,--serviceaccount}'[pass the serviceaccount name]:' \
    '1: :__tkn_get_pipeline'
}


function __tkn_get_pipelinerun() { __tkn_get_object pipelinerun; }
function _tkn_pipelinerun {
  local -a commands

  _arguments -C \
    "$common_options[@]" \
    "1: :->cmnds" \
    "*::arg:->args"

  case $state in
  cmnds)
    commands=(
      "describe:Describe a pipelinerun in a namespace"
      "list:Lists pipelineruns in a namespace"
      "logs:Show the logs of PipelineRun"
    )
    _describe "command" commands
    ;;
  esac

  case "$words[1]" in
  describe|desc)
    _tkn_pipelinerun_describe
    ;;
  list|ls)
    _tkn_pipelinerun_list
    ;;
  logs)
    _tkn_pipelinerun_logs
    ;;
  esac
}

function _tkn_pipelinerun_describe {
  _arguments \
    "$common_describe_options[@]" \
    '1: :__tkn_get_pipelinerun'
}

function _tkn_pipelinerun_list {
  _arguments \
    "$common_describe_options[@]"
}

function _tkn_pipelinerun_logs {
  _arguments \
    "$common_logs_options[@]" \
    '(*-t *--only-tasks)'{\*-t,\*--only-tasks}'[show logs for mentioned tasks only]:' \
    '1: :__tkn_get_pipelinerun'
}


function _tkn_resource {
  local -a commands

  _arguments -C \
    "$common_options[@]" \
    "1: :->cmnds" \
    "*::arg:->args"

  case $state in
  cmnds)
    commands=(
      "list:Lists pipeline resources in a namespace"
    )
    _describe "command" commands
    ;;
  esac

  case "$words[1]" in
  list|ls)
    _tkn_resource_list
    ;;
  esac
}

function _tkn_resource_list {
  _arguments \
    "$common_describe_options[@]"
}


function _tkn_task {
  local -a commands

  _arguments -C \
    "$common_options[@]" \
    "1: :->cmnds" \
    "*::arg:->args"

  case $state in
  cmnds)
    commands=(
      "list:Lists tasks in a namespace"
    )
    _describe "command" commands
    ;;
  esac

  case "$words[1]" in
  list|ls)
    _tkn_task_list
    ;;
  esac
}

function _tkn_task_list {
  _arguments \
    "$common_describe_options[@]"
}


function __tkn_get_taskrun() { __tkn_get_object taskrun; }

function _tkn_taskrun {
  local -a commands

  _arguments -C \
    "$common_options[@]" \
    "1: :->cmnds" \
    "*::arg:->args"

  case $state in
  cmnds)
    commands=(
      "list:Lists taskruns in a namespace"
      "logs:Show taskruns logs"
    )
    _describe "command" commands
    ;;
  esac

  case "$words[1]" in
  list|ls)
    _tkn_taskrun_list
    ;;
  logs)
    _tkn_taskrun_logs
    ;;
  esac
}

function _tkn_taskrun_list {
  _arguments \
    "$common_describe_options[@]"
}

function _tkn_taskrun_logs {
  _arguments \
    "$common_logs_options[@]" \
    '1: :__tkn_get_taskrun'
}
`
