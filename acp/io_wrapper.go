package acp

import (
	"encoding/json"
	"io"
	"log"
)

// JSONRPCReader wraps an io.Reader to handle multi-line JSON-RPC messages.
// It reads line by line and accumulates lines until a complete JSON message
// is formed, then outputs it as a single line. Any remaining partial JSON
// is preserved for the next Read call.
type JSONRPCReader struct {
	reader *json.Decoder
	output []byte // compacted JSON ready to output
	done   bool
}

// NewJSONRPCReader creates a new reader that converts potentially
// multi-line JSON-RPC messages to single-line format.
func NewJSONRPCReader(r io.Reader) *JSONRPCReader {
	reader := json.NewDecoder(r)
	return &JSONRPCReader{reader: reader}
}

// Read implements io.Reader. It reads from the underlying reader and
// returns single-line JSON-RPC messages.
func (r *JSONRPCReader) Read(p []byte) (n int, err error) {
	// If we have leftover output ready, return it
	if len(r.output) > 0 {
		n = copy(p, r.output)
		r.output = r.output[n:]
		return n, nil
	}

	if r.done {
		return 0, io.EOF
	}

	// Read one complete JSON object at a time
	var raw map[string]interface{}
	if err := r.reader.Decode(&raw); err != nil {
		if err == io.EOF {
			r.done = true
			return 0, io.EOF
		}
		log.Printf("parse json error: %v", err)
		return 0, err
	}

	// Marshal to compact single-line format
	compacted, err := json.Marshal(raw)
	if err != nil {
		return 0, err
	}

	// Add newline delimiter
	r.output = append(compacted, '\n')
	n = copy(p, r.output)
	r.output = r.output[n:]
	return n, nil
}

// Ensure JSONRPCReader implements io.Reader
var _ io.Reader = (*JSONRPCReader)(nil)
