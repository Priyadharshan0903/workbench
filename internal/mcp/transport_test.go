package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

type shortWriter struct{}

func (shortWriter) Write(contents []byte) (int, error) {
	return len(contents) - 1, nil
}

func TestResponseWriterReportsShortWrites(t *testing.T) {
	reply := successResponse(json.RawMessage("1"), map[string]any{})
	if err := writeResponse(shortWriter{}, reply); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short write silently discarded part of an MCP message", err)
	}
}

func TestOversizedResponseBecomesBoundedToolError(t *testing.T) {
	requestID := json.RawMessage(`"large-response"`)
	reply := successResponse(requestID, map[string]any{
		"contents": strings.Repeat("x", maxResponseBytes),
	})
	var output bytes.Buffer

	if err := writeResponse(&output, reply); err != nil {
		t.Fatal(err)
	}
	if output.Len() > maxResponseBytes || bytes.Count(output.Bytes(), []byte("\n")) != 1 {
		t.Fatal("response is oversized or incorrectly framed", output.Len())
	}

	var decoded struct {
		ID     json.RawMessage `json:"id"`
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.ID, requestID) || !decoded.Result.IsError {
		t.Fatal("bounded error lost the request ID or error state", output.String())
	}
}
