package isurus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

type AddFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func AddFileHandler(ctx context.Context, conn *jsonrpc2.Conn, message *json.RawMessage) (any, error) {
	var params AddFileRequest
	if err := json.Unmarshal(*message, &params); err != nil {
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: fmt.Errorf("failed to unmarshal params: %w", err).Error(),
		}
	}

	err := codeStore.Load().AddFile(params.Path, params.Content)
	if err != nil {
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: fmt.Errorf("failed to add file: %w", err).Error(),
		}
	}

	return "ok", nil
}
