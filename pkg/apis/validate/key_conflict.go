package validate

import (
	"fmt"
	"reflect"

	"knative.dev/pkg/apis"
)

// KeyConflict returns an error if any of the elements in things has the same
// value for the field named fieldName.
//
// It also returns an error if things is not a slice, if any element in things
// is not a struct, or if any field named fieldName in the struct is not a
// string or is not defined. Things can be a slice of pointers to structs, and
// fieldName can be a field of type *string.
func KeyConflict(things interface{}, fieldName, fieldPath string) *apis.FieldError {
	tsv := reflect.ValueOf(things)
	if tsv.Kind() != reflect.Slice {
		return &apis.FieldError{
			Message: fmt.Sprintf("value is not a slice (%T)", things),
			Paths:   []string{fieldPath},
		}
	}

	seen := map[string]int{}
	for i := 0; i < tsv.Len(); i++ {
		tv := tsv.Index(i)
		if tv.Kind() == reflect.Ptr {
			tv = tv.Elem()
		}
		if tv.Kind() != reflect.Struct {
			return &apis.FieldError{
				Message: fmt.Sprintf("item %d is not a struct", i),
				Paths:   []string{fieldPath},
			}
		}
		fv := tv.FieldByName(fieldName)
		fvk := fv.Kind()
		if fvk == reflect.Ptr {
			fv = fv.Elem()
			fvk = fv.Kind()
		}
		if fvk == reflect.Invalid {
			return &apis.FieldError{
				Message: fmt.Sprintf("item %d value for field %q is not defined", i, fieldName),
				Paths:   []string{fieldPath},
			}
		} else if fvk != reflect.String {
			return &apis.FieldError{
				Message: fmt.Sprintf("item %d value for field %q is not a string (%s)", i, fieldName, fvk),
				Paths:   []string{fieldPath},
			}
		}
		sv := fv.String()
		if idx, found := seen[sv]; found {
			return &apis.FieldError{
				Message: fmt.Sprintf("item %d value for field %q conflicts with value for item %d", i, fieldName, idx),
				Paths:   []string{fieldPath},
			}
		}
		seen[sv] = i
	}
	return nil
}
