package lsp

import (
	"context"
	"io"

	"github.com/sourcegraph/jsonrpc2"
)

type Connection interface {
	Call(ctx context.Context, method string, params interface{}, result interface{}) error
	Notify(ctx context.Context, method string, params interface{}) error
}

type ConnectionImpl struct {
	conn *jsonrpc2.Conn
}

func NewConnection(ctx context.Context, rwc io.ReadWriteCloser, handler jsonrpc2.Handler) Connection {
	// TODO: understand codecs
	stream := jsonrpc2.NewBufferedStream(rwc, jsonrpc2.VSCodeObjectCodec{})
	conn := jsonrpc2.NewConn(ctx, stream, handler)
	return &ConnectionImpl{conn: conn}
}

// Call implements Connection.
func (c *ConnectionImpl) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
	return c.conn.Call(ctx, method, params, result)
}

// Notify implements Connection.
func (c *ConnectionImpl) Notify(ctx context.Context, method string, params interface{}) error {
	return c.conn.Notify(ctx, method, params)
}
