package isurus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

type InitializeRequest struct {
	RootPath string `json:"rootPath"`
}

func InitializeHandler(ctx context.Context, conn *jsonrpc2.Conn, message *json.RawMessage) (any, error) {
	var params InitializeRequest
	if err := json.Unmarshal(*message, &params); err != nil {
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: fmt.Errorf("failed to unmarshal params: %w", err).Error(),
		}
	}

	codeStore.Store(NewStore(params.RootPath))

	return "ok", nil
}
