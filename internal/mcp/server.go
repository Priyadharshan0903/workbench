// Package mcp serves local Workbench tools over newline-delimited JSON-RPC on stdio.
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"workbench/internal/catalog"
	"workbench/internal/launch"
)

const (
	maxMessageBytes  = 1 << 20
	maxResponseBytes = 8 << 20
)

var supportedProtocols = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

type Server struct {
	Store       catalog.Store
	Supervisor  *launch.Supervisor
	AllowWrites bool

	initialized bool
	ready       bool
}

func New(store catalog.Store, allowWrites bool) *Server {
	return &Server{
		Store:       store,
		Supervisor:  launch.New(store),
		AllowWrites: allowWrites,
	}
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Serve never opens a listening port or launches a registered project. Stdout
// contains only MCP messages; callers report process errors on stderr.
func (server *Server) Serve(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), maxMessageBytes)

	for scanner.Scan() {
		reply := server.handleMessage(scanner.Bytes())
		if reply == nil {
			continue
		}

		if err := writeResponse(output, reply); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func (server *Server) handleMessage(message []byte) *response {
	if !json.Valid(message) {
		return failureResponse(nil, -32700, "Parse error.")
	}

	var incoming request
	if err := json.Unmarshal(message, &incoming); err != nil {
		return failureResponse(nil, -32600, "Invalid JSON-RPC request.")
	}

	if incoming.JSONRPC != "2.0" || incoming.Method == "" {
		return failureResponse(nil, -32600, "Invalid JSON-RPC request.")
	}

	if len(incoming.ID) > 0 && !validID(incoming.ID) {
		return failureResponse(nil, -32600, "Invalid JSON-RPC request.")
	}

	if len(incoming.ID) == 0 {
		if incoming.Method == "notifications/initialized" && server.initialized {
			server.ready = true
		}
		return nil
	}

	return server.dispatch(incoming)
}

func writeResponse(output io.Writer, reply *response) error {
	encodedResponse, err := json.Marshal(reply)
	if err != nil {
		return err
	}

	if len(encodedResponse) > maxResponseBytes {
		reply = successResponse(reply.ID, toolFailure(errors.New("Result exceeds 8 MB. Request one project instead.")))
		encodedResponse, err = json.Marshal(reply)
		if err != nil {
			return err
		}
	}

	encodedResponse = append(encodedResponse, '\n')
	writtenBytes, err := output.Write(encodedResponse)
	if err == nil && writtenBytes != len(encodedResponse) {
		return io.ErrShortWrite
	}

	return err
}

func validID(rawID json.RawMessage) bool {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(rawID))
	decoder.UseNumber()

	if err := decoder.Decode(&value); err != nil {
		return false
	}

	switch value.(type) {
	case string, json.Number:
		return true
	default:
		return false
	}
}

func successResponse(requestID json.RawMessage, value any) *response {
	return &response{JSONRPC: "2.0", ID: requestID, Result: value}
}

func failureResponse(requestID json.RawMessage, code int, message string) *response {
	if len(requestID) == 0 {
		requestID = json.RawMessage("null")
	}

	return &response{
		JSONRPC: "2.0",
		ID:      requestID,
		Error:   &rpcError{Code: code, Message: message},
	}
}
