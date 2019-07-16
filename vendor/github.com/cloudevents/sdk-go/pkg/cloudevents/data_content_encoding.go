package cloudevents

const (
	Base64 = "base64"
)

// StringOfBase64 returns a string pointer to "Base64"
func StringOfBase64() *string {
	a := Base64
	return &a
}
