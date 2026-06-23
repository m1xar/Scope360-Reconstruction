package ctrader

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	stream "github.com/m1xar/scope360-reconstruction/pkg/transport/stream"
	gproto "google.golang.org/protobuf/proto"

	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

const (
	LiveEndpoint = "live.ctraderapi.com:5035"
	DemoEndpoint = "demo.ctraderapi.com:5035"
)

type Config struct {
	Credentials    Credentials
	StreamClient   *stream.Client
	HTTPClient     *resty.Client
	OnTokenRefresh func(TokenSet)
}

type Client struct {
	creds          Credentials
	streamClient   *stream.Client
	httpClient     *resty.Client
	onTokenRefresh func(TokenSet)
	session        *Session
	endpoint       string
	eventMu        sync.Mutex
	spotWaiters    map[int64][]chan *pb.ProtoOASpotEvent
}

type APIError struct {
	Code        string
	Description string
	RetryAfter  uint64
}

func (e APIError) Error() string {
	if e.Description == "" {
		return e.Code
	}
	return e.Code + ": " + e.Description
}

func NewClient(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = resty.New()
	}
	return &Client{
		creds:          cfg.Credentials,
		streamClient:   cfg.StreamClient,
		httpClient:     httpClient,
		onTokenRefresh: cfg.OnTokenRefresh,
		endpoint:       LiveEndpoint,
		spotWaiters:    make(map[int64][]chan *pb.ProtoOASpotEvent),
	}
}

func (c *Client) Session() *Session        { return c.session }
func (c *Client) Credentials() Credentials { return c.creds }

func (c *Client) Connect(ctx context.Context, endpoint string) error {
	if endpoint == "" {
		endpoint = LiveEndpoint
	}
	if c.streamClient != nil && c.endpoint == endpoint {
		return c.streamClient.Connect(ctx)
	}
	if c.streamClient != nil {
		_ = c.streamClient.Disconnect()
	}
	c.endpoint = endpoint
	c.streamClient = stream.NewClient(stream.Config{
		Dial:           stream.TLSDialer(endpoint, &tls.Config{ServerName: strings.Split(endpoint, ":")[0], MinVersion: tls.VersionTLS12}),
		Codec:          stream.BigEndianFrameCodec{},
		MatchID:        matchClientMsgID,
		EventHandler:   c.handleEvent,
		Heartbeat:      heartbeatFrame,
		HeartbeatEvery: 10 * time.Second,
		RequestTimeout: 30 * time.Second,
	})
	return c.streamClient.Connect(ctx)
}

func (c *Client) Disconnect() error {
	if c.streamClient == nil {
		return nil
	}
	return c.streamClient.Disconnect()
}

func (c *Client) Do(ctx context.Context, payloadType pb.ProtoOAPayloadType, req gproto.Message, res gproto.Message) error {
	err := c.doOnce(ctx, payloadType, req, res)
	if !isAuthError(err) {
		return err
	}
	if _, refreshErr := c.RefreshToken(); refreshErr != nil {
		return fmt.Errorf("%w; token refresh failed: %v", err, refreshErr)
	}
	if c.session != nil {
		if authErr := c.accountAuth(ctx, false); authErr != nil {
			return fmt.Errorf("%w; account re-auth failed after token refresh: %v", err, authErr)
		}
	}
	return c.doOnce(ctx, payloadType, req, res)
}

func (c *Client) doOnce(ctx context.Context, payloadType pb.ProtoOAPayloadType, req gproto.Message, res gproto.Message) error {
	if c.streamClient == nil {
		if err := c.Connect(ctx, c.endpoint); err != nil {
			return err
		}
	}
	payload, err := gproto.Marshal(req)
	if err != nil {
		return err
	}
	id := uuid.NewString()
	pt := uint32(payloadType)
	envelope := &pb.ProtoMessage{PayloadType: &pt, Payload: payload, ClientMsgId: &id}
	frame, err := gproto.Marshal(envelope)
	if err != nil {
		return err
	}
	raw, err := c.streamClient.Request(ctx, id, frame)
	if err != nil {
		return err
	}
	var reply pb.ProtoMessage
	if err := gproto.Unmarshal(raw, &reply); err != nil {
		return err
	}
	if reply.GetPayloadType() == uint32(pb.ProtoOAPayloadType_PROTO_OA_ERROR_RES) {
		var apiErr pb.ProtoOAErrorRes
		if unmarshalErr := gproto.Unmarshal(reply.GetPayload(), &apiErr); unmarshalErr != nil {
			return unmarshalErr
		}
		return APIError{
			Code:        apiErr.GetErrorCode(),
			Description: apiErr.GetDescription(),
			RetryAfter:  apiErr.GetRetryAfter(),
		}
	}
	return gproto.Unmarshal(reply.GetPayload(), res)
}

func matchClientMsgID(frame []byte) (string, error) {
	var msg pb.ProtoMessage
	if err := gproto.Unmarshal(frame, &msg); err != nil {
		return "", err
	}
	return msg.GetClientMsgId(), nil
}

func (c *Client) handleEvent(frame []byte) {
	var msg pb.ProtoMessage
	if err := gproto.Unmarshal(frame, &msg); err != nil {
		return
	}
	if msg.GetPayloadType() != uint32(pb.ProtoOAPayloadType_PROTO_OA_SPOT_EVENT) {
		return
	}
	var event pb.ProtoOASpotEvent
	if err := gproto.Unmarshal(msg.GetPayload(), &event); err != nil {
		return
	}
	c.deliverSpotEvent(&event)
}

func (c *Client) deliverSpotEvent(event *pb.ProtoOASpotEvent) {
	if event == nil {
		return
	}
	symbolID := event.GetSymbolId()
	c.eventMu.Lock()
	waiters := c.spotWaiters[symbolID]
	delete(c.spotWaiters, symbolID)
	c.eventMu.Unlock()
	for _, waiter := range waiters {
		select {
		case waiter <- event:
		default:
		}
	}
}

func (c *Client) registerSpotWaiter(symbolID int64) chan *pb.ProtoOASpotEvent {
	waiter := make(chan *pb.ProtoOASpotEvent, 1)
	c.eventMu.Lock()
	c.spotWaiters[symbolID] = append(c.spotWaiters[symbolID], waiter)
	c.eventMu.Unlock()
	return waiter
}

func (c *Client) unregisterSpotWaiter(symbolID int64, waiter chan *pb.ProtoOASpotEvent) {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()
	waiters := c.spotWaiters[symbolID]
	for i, item := range waiters {
		if item == waiter {
			waiters = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	if len(waiters) == 0 {
		delete(c.spotWaiters, symbolID)
		return
	}
	c.spotWaiters[symbolID] = waiters
}

func (c *Client) FetchSpot(ctx context.Context, symbolID int64) (*pb.ProtoOASpotEvent, error) {
	session, err := c.EnsureSession(ctx)
	if err != nil {
		return nil, err
	}
	waiter := c.registerSpotWaiter(symbolID)
	defer c.unregisterSpotWaiter(symbolID, waiter)

	accountID := session.CtidTraderAccountID
	symbolIDs := []int64{symbolID}
	req := &pb.ProtoOASubscribeSpotsReq{CtidTraderAccountId: &accountID, SymbolId: symbolIDs}
	var res pb.ProtoOASubscribeSpotsRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_SUBSCRIBE_SPOTS_REQ, req, &res); err != nil {
		return nil, err
	}

	waitCtx := ctx
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		waitCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	select {
	case event := <-waiter:
		return event, nil
	case <-waitCtx.Done():
		return nil, waitCtx.Err()
	}
}

func heartbeatFrame() ([]byte, error) {
	pt := uint32(pb.ProtoPayloadType_HEARTBEAT_EVENT)
	msg := &pb.ProtoMessage{PayloadType: &pt}
	return gproto.Marshal(msg)
}

func isAuthError(err error) bool {
	var apiErr APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	code := strings.ToUpper(apiErr.Code)
	desc := strings.ToUpper(apiErr.Description)
	return strings.Contains(code, "TOKEN") || strings.Contains(code, "AUTH") || strings.Contains(desc, "TOKEN") || strings.Contains(desc, "AUTH") || strings.Contains(desc, "EXPIRED")
}

func (c *Client) FetchTrader(ctx context.Context) (*pb.ProtoOATrader, error) {
	if c.session == nil {
		return nil, fmt.Errorf("ctrader session is not resolved")
	}
	accountID := c.session.CtidTraderAccountID
	req := &pb.ProtoOATraderReq{CtidTraderAccountId: &accountID}
	var res pb.ProtoOATraderRes
	if err := c.Do(ctx, pb.ProtoOAPayloadType_PROTO_OA_TRADER_REQ, req, &res); err != nil {
		return nil, err
	}
	return res.GetTrader(), nil
}
