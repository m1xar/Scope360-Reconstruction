package stream

import (
	"context"
	"crypto/tls"
	"net"
)

type DialFunc func(context.Context) (net.Conn, error)

func TLSDialer(addr string, cfg *tls.Config) DialFunc {
	return func(ctx context.Context) (net.Conn, error) {
		dialer := &tls.Dialer{Config: cfg}
		return dialer.DialContext(ctx, "tcp", addr)
	}
}
