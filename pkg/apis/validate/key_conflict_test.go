package validate

import "testing"

func TestKeyConflict(t *testing.T) {
	type kv struct{ Key, Value string }
	fieldPath := ".field.path"
	aString := "I'm just a string"

	for _, test := range []struct {
		name      string
		things    interface{}
		fieldName string
		want      string
	}{{
		name:      "happy case; one item",
		things:    []kv{{Key: "foo", Value: "bar"}},
		fieldName: "Key",
	}, {
		name: "happy case; two unique items",
		things: []kv{
			{Key: "foo", Value: "bar"},
			{Key: "bar", Value: "baz"},
		},
		fieldName: "Key",
	}, {
		name:      "happy case;  things[i] is a *struct",
		things:    []*kv{{Key: "foo", Value: "bar"}},
		fieldName: "Key",
	}, {
		name:      "things[i].key is a *string",
		things:    []struct{ Key *string }{{Key: &aString}},
		fieldName: "Key",
	}, {
		name: "key conflict",
		things: []kv{
			{Key: "foo", Value: "bar"},
			{Key: "bar", Value: "bar"},
			{Key: "bar", Value: "baz"},
		},
		fieldName: "Key",
		want:      `item 2 value for field "Key" conflicts with value for item 1: .field.path`,
	}, {
		name:      "things is not a slice",
		things:    12345,
		fieldName: "DoesntMatter",
		want:      "value is not a slice (int): .field.path",
	}, {
		name:      "things[i] is not a struct",
		things:    []int{12345},
		fieldName: "DoesntMatter",
		want:      "item 0 is not a struct: .field.path",
	}, {
		name:      "things[i].key is not a string",
		things:    []struct{ NonString int }{{NonString: 12345}},
		fieldName: "NonString",
		want:      `item 0 value for field "NonString" is not a string (int): .field.path`,
	}, {
		name:      "things[i].foo not defined",
		things:    []kv{{Key: "foo", Value: "bar"}},
		fieldName: "DoesntExist",
		want:      `item 0 value for field "DoesntExist" is not defined: .field.path`,
	}} {
		t.Run(test.name, func(t *testing.T) {
			got := KeyConflict(test.things, test.fieldName, fieldPath)
			if got.Error() != test.want {
				t.Errorf("\n got: %v\nwant: %v", got, test.want)
			}
		})
	}
}
