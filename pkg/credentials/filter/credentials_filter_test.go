package filter_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/tektoncd/pipeline/pkg/credentials/filter"
)

// Implements WriteCloser on a buffer.
type BufferCloser struct {
	bytes.Buffer
}

func (b *BufferCloser) Close() error {
	// Noop
	return nil
}

func TestFilterSecrets(t *testing.T) {
	detectedSecrets := []*filter.DetectedSecret{
		{Name: "secret1", Value: []byte("secretvalue1")},
		{Name: "secret2", Value: []byte("sv2")},
		{Name: "secret3", Value: []byte("someverysecretvalue")},
	}

	textWithSecrets := `
This is a multiline text that contains secret values
We use secretvalue1 and some more secrets
we hope that sv2 is redacted too
And someverysecretvalue especially
`

	textWithRedactedSecrets := `
This is a multiline text that contains secret values
We use [REDACTED:secret1] and some more secrets
we hope that [REDACTED:secret2] is redacted too
And [REDACTED:secret3] especially
`

	var out *BufferCloser = &BufferCloser{}
	writer := filter.NewFilteredCredentialsWriter(detectedSecrets, out)

	io.WriteString(writer, textWithSecrets)
	writer.Close()

	if out.String() != textWithRedactedSecrets {
		t.Errorf("Filtered string not as expected.\r\nexpected: %s\r\nactual: %s", textWithRedactedSecrets, out.String())
	}
}

func TestFilterSecretsOnClose(t *testing.T) {
	detectedSecrets := []*filter.DetectedSecret{
		{Name: "secret1", Value: []byte("secretvalue1")},
		{Name: "secret2", Value: []byte("sv2")},
		{Name: "secret3", Value: []byte("someverysecretvalue")},
	}

	textWithSecrets := `
This is a multiline text that contains secret values
We use secretvalue1 and some more secrets
we hope that sv2 is redacted too
And someverysecretvalue especially
`

	// this will be printed separately at the end
	// it will not be filtered because it will not overflow the buffer
	// because this secret is much shorter
	// in this way it will stay in the buffer until close is called
	textEndWithSecret := "E sv2"

	textWithRedactedSecrets := `
This is a multiline text that contains secret values
We use [REDACTED:secret1] and some more secrets
we hope that [REDACTED:secret2] is redacted too
And [REDACTED:secret3] especially
E [REDACTED:secret2]`

	var out *BufferCloser = &BufferCloser{}
	writer := filter.NewFilteredCredentialsWriter(detectedSecrets, out)

	io.WriteString(writer, textWithSecrets)
	io.WriteString(writer, textEndWithSecret)
	writer.Close()

	if out.String() != textWithRedactedSecrets {
		t.Errorf("Filtered string not as expected.\r\nexpected: %s\r\nactual: %s", textWithRedactedSecrets, out.String())
	}
}

func TestFilterNoSecrets(t *testing.T) {
	detectedSecrets := []*filter.DetectedSecret{}

	textWithSecrets := `
This is a multiline text that contains secret values
We use secretvalue1 and some more secrets
we hope that sv2 is redacted too
And someverysecretvalue especially
`

	textWithRedactedSecrets := `
This is a multiline text that contains secret values
We use secretvalue1 and some more secrets
we hope that sv2 is redacted too
And someverysecretvalue especially
`

	var out *BufferCloser = &BufferCloser{}
	writer := filter.NewFilteredCredentialsWriter(detectedSecrets, out)

	io.WriteString(writer, textWithSecrets)
	writer.Close()

	if out.String() != textWithRedactedSecrets {
		t.Errorf("Filtered string not as expected.\r\nexpected: %s\r\nactual: %s", textWithRedactedSecrets, out.String())
	}
}
