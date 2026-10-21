package templates

import "embed"

//go:embed asciidoctor
//go:embed markdown
var Root embed.FS
