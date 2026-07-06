package server

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
)

// ServeStdio runs the MCP server over stdio, reading newline-delimited JSON-RPC
// messages from stdin and writing responses to stdout. It returns when stdin
// reaches EOF or ctx is cancelled.
func (s *Server) ServeStdio(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line, readErr := reader.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			resp, isNotification := s.handleMessage(ctx, line)
			if !isNotification && resp != nil {
				if err := writeLine(writer, resp); err != nil {
					return err
				}
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

// writeLine writes a response followed by a newline and flushes it.
func writeLine(writer *bufio.Writer, resp []byte) error {
	if _, err := writer.Write(resp); err != nil {
		return err
	}
	if err := writer.WriteByte('\n'); err != nil {
		return err
	}
	return writer.Flush()
}
