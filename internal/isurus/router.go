package isurus

import (
	"context"
	"encoding/json"
	"io"
	"log"

	"github.com/sourcegraph/jsonrpc2"
)

type Router struct {
	stream io.ReadWriteCloser
}

type readWriteCloser struct {
	io.Reader
	io.Writer
}

func (rw readWriteCloser) Close() error {
	return nil
}

func NewRouter(r io.Reader, w io.Writer) *Router {
	return &Router{
		stream: readWriteCloser{r, w},
	}
}

func (r *Router) Run() error {
	objectStream := jsonrpc2.NewPlainObjectStream(r.stream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := jsonrpc2.NewConn(ctx, objectStream, createHandler(map[string]methodHandler{
		"initialize": InitializeHandler,
	}), jsonrpc2.LogMessages(log.Default()))
	<-conn.DisconnectNotify()

	return nil
}

type methodHandler func(context.Context, *jsonrpc2.Conn, *json.RawMessage) (any, error)

func createHandler(handlerMap map[string]methodHandler) jsonrpc2.Handler {
	return jsonrpc2.AsyncHandler(jsonrpc2.HandlerWithError(func(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (any, error) {
		handler, ok := handlerMap[req.Method]
		if !ok {
			return nil, &jsonrpc2.Error{
				Code:    jsonrpc2.CodeMethodNotFound,
				Message: "method not found",
			}
		}

		return handler(ctx, conn, req.Params)
	}))
}
