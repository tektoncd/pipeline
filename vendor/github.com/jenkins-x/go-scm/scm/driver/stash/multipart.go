package stash

import "mime/multipart"

type MultipartWriter struct {
	Writer *multipart.Writer
	Error  error
}

func (mw *MultipartWriter) Write(f, v string) {
	if mw.Error != nil {
		return
	}
	if v == "" {
		return
	}
	mw.Error = mw.Writer.WriteField(f, v)
}

func (mw *MultipartWriter) Close() {
	mw.Writer.Close()
}

func (mw *MultipartWriter) FormDataContentType() string {
	return mw.Writer.FormDataContentType()
}
