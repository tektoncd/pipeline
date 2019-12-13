{{/* Range and render annotations list */}}
{{define "tekton.helpers.annotations"}}
  {{if .annotations -}}
  annotations:
  {{- range $index, $annotation := .annotations}}
    {{ $index }}: {{$annotation}}
  {{- end}}
  {{- end}}
{{- end}}
