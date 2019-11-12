package entrypoint

// SkipError is returned when an entrypoint Waiter detects that a prior
// step failed and therefore this step should also be skipped.
type SkipError string

// Error returns the string message of the SkipError.
func (e SkipError) Error() string {
	return string(e)
}
