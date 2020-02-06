package cloudevents

import (
	"regexp"
	"strings"
)

const (
	// DataContentEncodingKey is the key to DeprecatedDataContentEncoding for versions that do not support data content encoding
	// directly.
	DataContentEncodingKey = "datacontentencoding"

	// EventTypeVersionKey is the key to EventTypeVersion for versions that do not support event type version directly.
	EventTypeVersionKey = "eventtypeversion"

	// SubjectKey is the key to Subject for versions that do not support subject directly.
	SubjectKey = "subject"
)

func caseInsensitiveSearch(key string, space map[string]interface{}) (interface{}, bool) {
	lkey := strings.ToLower(key)
	for k, v := range space {
		if strings.EqualFold(lkey, strings.ToLower(k)) {
			return v, true
		}
	}
	return nil, false
}

var IsAlphaNumeric = regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString
