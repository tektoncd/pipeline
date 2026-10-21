{{- define "gvDetails" -}}
{{- $gv := . -}}
[id="{{ asciidocGroupVersionID $gv | asciidocRenderAnchorID }}"]
=== {{ $gv.GroupVersionString }}

{{ $gv.Doc }}

{{- if $gv.Kinds  }}
.Resource Types
{{- range $gv.SortedKinds }}
- {{ $gv.TypeForKind . | asciidocRenderTypeLink }}
{{- end }}
{{ end }}

{{ range $gv.SortedTypes }}
{{ template "type" . }}
{{ end }}

{{- end -}}
