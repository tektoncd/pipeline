package tracestate

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	// ErrInvalidListMember occurs if at least one list member is invalid, e.g., contains an unexpected character.
	ErrInvalidListMember = errors.New("tracecontext: Invalid tracestate list member")
	// ErrDuplicateListMemberKey occurs if at least two list members contain the same vendor-tenant pair.
	ErrDuplicateListMemberKey = errors.New("tracecontext: Duplicate list member key in tracestate")
	// ErrTooManyListMembers occurs if the list contains more than the maximum number of members per the spec, i.e., 32.
	ErrTooManyListMembers = errors.New("tracecontext: Too many list members in tracestate")
)

const (
	maxMembers = 32

	delimiter = ","
)

var (
	re = regexp.MustCompile(`^\s*(?:([a-z0-9_\-*/]{1,241})@([a-z0-9_\-*/]{1,14})|([a-z0-9_\-*/]{1,256}))=([\x20-\x2b\x2d-\x3c\x3e-\x7e]*[\x21-\x2b\x2d-\x3c\x3e-\x7e])\s*$`)
)

// Member contains vendor-specific data that should be propagated across all new spans started within a given trace.
type Member struct {
	// Vendor is a key representing a particular trace vendor.
	Vendor string
	// Tenant is a key used to distinguish between tenants of a multi-tenant trace vendor.
	Tenant string
	// Value is the particular data that the vendor intents to pass to child spans.
	Value string
}

// String encodes a `Member` into a string formatted according to the W3C spec.
// The string may be invalid if any fields are invalid, e.g, the vendor contains a non-compliant character.
func (m Member) String() string {
	if m.Tenant == "" {
		return fmt.Sprintf("%s=%s", m.Vendor, m.Value)
	}
	return fmt.Sprintf("%s@%s=%s", m.Vendor, m.Tenant, m.Value)
}

// TraceState represents a list of `Member`s that should be propagated to new spans started in a trace.
type TraceState []Member

// String encodes all `Member`s of the `TraceState` into a single string, formatted according to the W3C spec.
// The string may be invalid if any `Member`s are invalid, e.g., containing a non-compliant character.
func (ts TraceState) String() string {
	var members []string
	for _, member := range ts {
		members = append(members, member.String())
	}
	return strings.Join(members, ",")
}

// Parse attempts to decode a `TraceState` from a byte array.
// It returns an error if the byte array is invalid, e.g., it contains an incorrectly formatted list member.
func Parse(traceState []byte) (TraceState, error) {
	return parse(string(traceState))
}

// ParseString attempts to decode a `TraceState` from a string.
// It returns an error if the string is invalid, e.g., it contains an incorrectly formatted list member.
func ParseString(traceState string) (TraceState, error) {
	return parse(traceState)
}

func parse(traceState string) (ts TraceState, err error) {
	found := make(map[string]interface{})

	members := strings.Split(traceState, delimiter)

	for _, member := range members {
		if len(member) == 0 {
			continue
		}

		var m Member
		m, err = parseMember(member)
		if err != nil {
			return
		}

		key := fmt.Sprintf("%s%s", m.Vendor, m.Tenant)
		if _, ok := found[key]; ok {
			err = ErrDuplicateListMemberKey
			return
		}
		found[key] = nil

		ts = append(ts, m)

		if len(ts) > maxMembers {
			err = ErrTooManyListMembers
			return
		}
	}

	return
}

func parseMember(s string) (Member, error) {
	matches := re.FindStringSubmatch(s)
	if len(matches) != 5 {
		return Member{}, ErrInvalidListMember
	}

	vendor := matches[1]
	if vendor == "" {
		vendor = matches[3]
	}

	return Member{
		Vendor: vendor,
		Tenant: matches[2],
		Value:  matches[4],
	}, nil
}
