package utils

import (
	"bytes"
	"compress/gzip"
)

// Compress gzips the given input.
func Compress(b []byte) (*bytes.Buffer, error) {
	var buf bytes.Buffer

	w := gzip.NewWriter(&buf)
	_, err := w.Write(b)
	if err != nil {
		return nil, err
	}
	_ = w.Close()

	return &buf, nil
}
