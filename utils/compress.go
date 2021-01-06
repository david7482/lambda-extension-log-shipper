package utils

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io/ioutil"
)

// Compress gzips the given input.
func Compress(d []byte) (*bytes.Buffer, error) {
	if len(d) == 0 {
		return nil, errors.New("utils: empty data")
	}

	var buf bytes.Buffer

	w := gzip.NewWriter(&buf)
	_, err := w.Write(d)
	if err != nil {
		return nil, err
	}
	_ = w.Close()

	return &buf, nil
}

// Decompress gzips the given input.
func Decompress(b *bytes.Buffer) ([]byte, error) {
	r, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	d, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return d, nil
}
