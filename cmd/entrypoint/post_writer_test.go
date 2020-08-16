package main

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestRealPostWriterWrite(t *testing.T) {
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	//defer os.RemoveAll(tmp)
	postfile := filepath.Join(tmp, "file")
	for _, c := range []struct {
		desc         string
		rpw          realPostWriter
		err          error
		expectedFile string
	}{{
		desc:         "post regular file",
		rpw:          realPostWriter{},
		err:          nil,
		expectedFile: postfile,
	}, {
		desc:         "post error file",
		rpw:          realPostWriter{},
		err:          errors.New("test error"),
		expectedFile: postfile + ".err",
	}, {
		desc:         "discard error",
		rpw:          realPostWriter{ignoreError: true},
		err:          errors.New("test error"),
		expectedFile: postfile,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			rpw := c.rpw
			rpw.Write(postfile, c.err)
			if _, err := os.Stat(c.expectedFile); os.IsNotExist(err) {
				t.Errorf("file %q not found", c.expectedFile)
			} else if err != nil {
				t.Fatalf("error stating %q: %v", postfile, err)
			}
		})
	}
}
