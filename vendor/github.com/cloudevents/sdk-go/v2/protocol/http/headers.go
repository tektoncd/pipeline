package http

import (
	"net/textproto"
	"strings"
	"unicode"

	"github.com/cloudevents/sdk-go/v2/binding/spec"
)

var attributeHeadersMapping map[string]string

func init() {
	attributeHeadersMapping = make(map[string]string)
	for _, v := range specs.Versions() {
		for _, a := range v.Attributes() {
			if a.Kind() == spec.DataContentType {
				attributeHeadersMapping[a.Name()] = ContentType
			} else {
				attributeHeadersMapping[a.Name()] = textproto.CanonicalMIMEHeaderKey(prefix + a.Name())
			}
		}
	}
}

func extNameToHeaderName(name string) string {
	var b strings.Builder
	b.Grow(len(name) + len(prefix))
	b.WriteString(prefix)
	b.WriteRune(unicode.ToUpper(rune(name[0])))
	b.WriteString(name[1:])
	return b.String()
}
