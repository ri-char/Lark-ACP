package acp

import (
	"io"
	"log"
)

type TeeReader struct {
	r io.Reader
}
func NewTeeReader(r io.Reader) *TeeReader {
	return &TeeReader{r: r}
}
func (r *TeeReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	log.Printf("=== Read: %v", string(p[:n]))
	return
}

type TeeWriter struct {
	r io.Writer
}
func NewTeeWriter(r io.Writer) *TeeWriter {
	return &TeeWriter{r: r}
}
func (r *TeeWriter) Write(p []byte) (n int, err error) {
	log.Printf("=== Write: %v", string(p))
	n, err = r.r.Write(p)
	return
}

var _ io.Reader = (*TeeReader)(nil)
var _ io.Writer = (*TeeWriter)(nil)
