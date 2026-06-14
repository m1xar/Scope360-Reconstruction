package stream

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

var ErrClientClosed = errors.New("stream client closed")

type Config struct {
	Dial             DialFunc
	Codec            Codec
	MatchID          func([]byte) (string, error)
	EventHandler     func([]byte)
	Heartbeat        func() ([]byte, error)
	HeartbeatEvery   time.Duration
	RequestTimeout   time.Duration
	WriterBufferSize int
}

type Client struct {
	cfg Config

	mu      sync.Mutex
	conn    net.Conn
	writeCh chan []byte
	pending map[string]chan response
	closed  bool
	done    chan struct{}
}

type response struct {
	payload []byte
	err     error
}

func NewClient(cfg Config) *Client {
	if cfg.Codec == nil {
		cfg.Codec = LittleEndianFrameCodec{}
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 30 * time.Second
	}
	if cfg.HeartbeatEvery == 0 {
		cfg.HeartbeatEvery = 10 * time.Second
	}
	if cfg.WriterBufferSize == 0 {
		cfg.WriterBufferSize = 32
	}
	return &Client{cfg: cfg, pending: make(map[string]chan response), done: make(chan struct{})}
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil && !c.closed {
		return nil
	}
	if c.cfg.Dial == nil {
		return errors.New("stream dialer is required")
	}
	conn, err := c.cfg.Dial(ctx)
	if err != nil {
		return err
	}
	c.conn = conn
	c.writeCh = make(chan []byte, c.cfg.WriterBufferSize)
	c.pending = make(map[string]chan response)
	c.closed = false
	c.done = make(chan struct{})
	go c.writerLoop(conn, c.writeCh, c.done)
	go c.readerLoop(conn, c.done)
	if c.cfg.Heartbeat != nil {
		go c.heartbeatLoop(c.writeCh, c.done)
	}
	return nil
}

func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeLocked(nil)
}

func (c *Client) Request(ctx context.Context, id string, payload []byte) ([]byte, error) {
	if id == "" {
		return nil, errors.New("request id is required")
	}
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	ch := make(chan response, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClientClosed
	}
	if _, exists := c.pending[id]; exists {
		c.mu.Unlock()
		return nil, fmt.Errorf("duplicate request id: %s", id)
	}
	c.pending[id] = ch
	writeCh := c.writeCh
	c.mu.Unlock()

	defer c.removePending(id)

	select {
	case writeCh <- payload:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, ErrClientClosed
	}

	requestCtx := ctx
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok && c.cfg.RequestTimeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, c.cfg.RequestTimeout)
		defer cancel()
	}

	select {
	case res := <-ch:
		return res.payload, res.err
	case <-requestCtx.Done():
		return nil, requestCtx.Err()
	case <-c.done:
		return nil, ErrClientClosed
	}
}

func (c *Client) removePending(id string) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *Client) writerLoop(conn net.Conn, writeCh <-chan []byte, done <-chan struct{}) {
	for {
		select {
		case payload := <-writeCh:
			if err := c.cfg.Codec.WriteFrame(conn, payload); err != nil {
				c.closeWithError(err)
				return
			}
		case <-done:
			return
		}
	}
}

func (c *Client) readerLoop(conn net.Conn, done <-chan struct{}) {
	for {
		payload, err := c.cfg.Codec.ReadFrame(conn)
		if err != nil {
			c.closeWithError(err)
			return
		}
		id := ""
		if c.cfg.MatchID != nil {
			matched, matchErr := c.cfg.MatchID(payload)
			if matchErr != nil {
				continue
			}
			id = matched
		}
		if id == "" {
			if c.cfg.EventHandler != nil {
				c.cfg.EventHandler(payload)
			}
			continue
		}
		c.mu.Lock()
		ch := c.pending[id]
		c.mu.Unlock()
		if ch != nil {
			ch <- response{payload: payload}
		}
		select {
		case <-done:
			return
		default:
		}
	}
}

func (c *Client) heartbeatLoop(writeCh chan<- []byte, done <-chan struct{}) {
	ticker := time.NewTicker(c.cfg.HeartbeatEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			payload, err := c.cfg.Heartbeat()
			if err != nil || len(payload) == 0 {
				continue
			}
			select {
			case writeCh <- payload:
			case <-done:
				return
			}
		case <-done:
			return
		}
	}
}

func (c *Client) closeWithError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.closeLocked(err)
}

func (c *Client) closeLocked(err error) error {
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.done)
	if c.conn != nil {
		_ = c.conn.Close()
	}
	if err == nil {
		err = ErrClientClosed
	}
	for id, ch := range c.pending {
		ch <- response{err: err}
		delete(c.pending, id)
	}
	return nil
}
