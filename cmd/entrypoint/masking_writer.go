//go:build !windows

/*
Copyright 2026 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"io"
	"sort"
)

const maskReplacement = "***"

type maskingWriter struct {
	underlying    io.Writer
	secrets       [][]byte
	maxSecretLen  int
	carry         []byte
	maskTokenByte []byte
}

func newMaskingWriter(w io.Writer, secrets []string) *maskingWriter {
	mw := &maskingWriter{
		underlying:    w,
		maskTokenByte: []byte(maskReplacement),
	}

	secretBytes := make([][]byte, 0, len(secrets))
	for _, s := range secrets {
		if len(s) >= 3 {
			secret := []byte(s)
			secretBytes = append(secretBytes, secret)
			if len(secret) > mw.maxSecretLen {
				mw.maxSecretLen = len(secret)
			}
		}
	}

	sort.Slice(secretBytes, func(i, j int) bool {
		if len(secretBytes[i]) == len(secretBytes[j]) {
			return bytes.Compare(secretBytes[i], secretBytes[j]) < 0
		}
		return len(secretBytes[i]) > len(secretBytes[j])
	})
	mw.secrets = secretBytes

	if mw.maxSecretLen > 1 {
		mw.carry = make([]byte, 0, mw.maxSecretLen-1)
	}
	return mw
}

func (m *maskingWriter) MaxSecretLen() int {
	return m.maxSecretLen
}

func (m *maskingWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	if m.maxSecretLen == 0 {
		_, err = m.underlying.Write(p)
		return len(p), err
	}

	m.carry = append(m.carry, p...)

	hold := m.maxSecretLen - 1
	safeEmitUntil := len(m.carry) - hold
	if safeEmitUntil <= 0 {
		return len(p), nil
	}

	var out bytes.Buffer
	for {
		safeEmitUntil = len(m.carry) - hold
		if safeEmitUntil <= 0 {
			break
		}

		matchStart, matchLen, ok := m.findEarliestMatch(m.carry)
		if !ok || matchStart >= safeEmitUntil {
			out.Write(m.carry[:safeEmitUntil])
			m.carry = append(m.carry[:0], m.carry[safeEmitUntil:]...)
			break
		}

		out.Write(m.carry[:matchStart])
		out.Write(m.maskTokenByte)
		m.carry = append(m.carry[:0], m.carry[matchStart+matchLen:]...)
	}

	if out.Len() > 0 {
		_, err = m.underlying.Write(out.Bytes())
	}
	return len(p), err
}

func (m *maskingWriter) Flush() error {
	if len(m.carry) == 0 {
		return nil
	}
	var out bytes.Buffer
	for len(m.carry) > 0 {
		matchStart, matchLen, ok := m.findEarliestMatch(m.carry)
		if !ok {
			out.Write(m.carry)
			m.carry = m.carry[:0]
			break
		}
		out.Write(m.carry[:matchStart])
		out.Write(m.maskTokenByte)
		m.carry = append(m.carry[:0], m.carry[matchStart+matchLen:]...)
	}
	_, err := m.underlying.Write(out.Bytes())
	m.carry = m.carry[:0]
	return err
}

func (m *maskingWriter) findEarliestMatch(data []byte) (start int, matchLen int, found bool) {
	bestStart := -1
	bestLen := 0
	for _, secret := range m.secrets {
		idx := bytes.Index(data, secret)
		if idx == -1 {
			continue
		}
		if bestStart == -1 || idx < bestStart || (idx == bestStart && len(secret) > bestLen) {
			bestStart = idx
			bestLen = len(secret)
		}
		if bestStart == 0 && bestLen == m.maxSecretLen {
			break
		}
	}
	if bestStart == -1 {
		return 0, 0, false
	}
	return bestStart, bestLen, true
}
