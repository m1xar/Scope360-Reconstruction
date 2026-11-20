package ctrader

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	stream "github.com/m1xar/scope360-reconstruction/pkg/transport/stream"
	gproto "google.golang.org/protobuf/proto"

	pb "github.com/m1xar/scope360-reconstruction/pkg/ctrader/connector/ctrader/proto"
)

// fakeOpenAPI emulates the cTrader Open API protocol rules that matter for auth:
// one ApplicationAuth per connection (a repeat gets ALREADY_LOGGED_IN), account list
// by access token, account auth with a token check.
type fakeOpenAPI struct {
	mu          sync.Mutex
	isLive      bool
	login       int64
	validTokens map[string]bool
	dials       map[string]int
	appAuths    map[string]int
	alreadyLogs int
	accAuths    int
}

func newFakeOpenAPI(isLive bool, login int64, validToken string) *fakeOpenAPI {
	return &fakeOpenAPI{
		isLive:      isLive,
		login:       login,
		validTokens: map[string]bool{validToken: true},
		dials:       map[string]int{},
		appAuths:    map[string]int{},
	}
}

func (f *fakeOpenAPI) dial(endpoint string) stream.DialFunc {
	return func(ctx context.Context) (net.Conn, error) {
		client, server := net.Pipe()
		f.mu.Lock()
		f.dials[endpoint]++
		f.mu.Unlock()
		go f.serve(endpoint, server)
		return client, nil
	}
}

func (f *fakeOpenAPI) serve(endpoint string, conn net.Conn) {
	defer conn.Close()
	codec := stream.BigEndianFrameCodec{}
	appAuthed := false
	for {
		frame, err := codec.ReadFrame(conn)
		if err != nil {
			return
		}
		var msg pb.ProtoMessage
		if err := gproto.Unmarshal(frame, &msg); err != nil {
			return
		}
		if msg.GetClientMsgId() == "" {
			continue // heartbeat
		}
		var (
			resType pb.ProtoOAPayloadType
			res     gproto.Message
		)
		switch pb.ProtoOAPayloadType(msg.GetPayloadType()) {
		case pb.ProtoOAPayloadType_PROTO_OA_APPLICATION_AUTH_REQ:
			f.mu.Lock()
			f.appAuths[endpoint]++
			if appAuthed {
				f.alreadyLogs++
			}
			f.mu.Unlock()
			if appAuthed {
				resType, res = errorRes(AlreadyLoggedInCode, "Open API application is already authorized")
			} else {
				appAuthed = true
				resType, res = pb.ProtoOAPayloadType_PROTO_OA_APPLICATION_AUTH_RES, &pb.ProtoOAApplicationAuthRes{}
			}
		case pb.ProtoOAPayloadType_PROTO_OA_GET_ACCOUNTS_BY_ACCESS_TOKEN_REQ:
			var req pb.ProtoOAGetAccountListByAccessTokenReq
			_ = gproto.Unmarshal(msg.GetPayload(), &req)
			if !f.tokenValid(req.GetAccessToken()) {
				resType, res = errorRes("CH_ACCESS_TOKEN_INVALID", "Invalid access token")
				break
			}
			id, live, login, token := uint64(777), f.isLive, f.login, req.GetAccessToken()
			resType, res = pb.ProtoOAPayloadType_PROTO_OA_GET_ACCOUNTS_BY_ACCESS_TOKEN_RES, &pb.ProtoOAGetAccountListByAccessTokenRes{
				AccessToken:       &token,
				CtidTraderAccount: []*pb.ProtoOACtidTraderAccount{{CtidTraderAccountId: &id, IsLive: &live, TraderLogin: &login}},
			}
		case pb.ProtoOAPayloadType_PROTO_OA_ACCOUNT_AUTH_REQ:
			var req pb.ProtoOAAccountAuthReq
			_ = gproto.Unmarshal(msg.GetPayload(), &req)
			f.mu.Lock()
			f.accAuths++
			f.mu.Unlock()
			if !appAuthed {
				resType, res = errorRes("CH_CLIENT_NOT_AUTHENTICATED", "Not authenticated")
				break
			}
			if !f.tokenValid(req.GetAccessToken()) {
				resType, res = errorRes("CH_ACCESS_TOKEN_INVALID", "Invalid access token")
				break
			}
			id := req.GetCtidTraderAccountId()
			resType, res = pb.ProtoOAPayloadType_PROTO_OA_ACCOUNT_AUTH_RES, &pb.ProtoOAAccountAuthRes{CtidTraderAccountId: &id}
		default:
			resType, res = errorRes("UNSUPPORTED_MESSAGE", "not emulated")
		}
		payload, _ := gproto.Marshal(res)
		pt := uint32(resType)
		id := msg.GetClientMsgId()
		out, _ := gproto.Marshal(&pb.ProtoMessage{PayloadType: &pt, Payload: payload, ClientMsgId: &id})
		if err := codec.WriteFrame(conn, out); err != nil {
			return
		}
	}
}

func (f *fakeOpenAPI) tokenValid(token string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.validTokens[token]
}

func errorRes(code, description string) (pb.ProtoOAPayloadType, gproto.Message) {
	return pb.ProtoOAPayloadType_PROTO_OA_ERROR_RES, &pb.ProtoOAErrorRes{ErrorCode: &code, Description: &description}
}

func useFake(t *testing.T, f *fakeOpenAPI) {
	t.Helper()
	prev := dialEndpoint
	dialEndpoint = f.dial
	prevRegistry := refreshLocks
	refreshLocks = &refreshRegistry{locks: map[[32]byte]*sync.Mutex{}, sets: map[[32]byte]rotatedSet{}}
	t.Cleanup(func() { dialEndpoint, refreshLocks = prev, prevRegistry })
}

// tokenServer counts refresh calls; any call fails the "no refresh" tests.
func tokenServer(t *testing.T, body string) (*int, *resty.Client) {
	t.Helper()
	calls := 0
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	prev := tokenURL
	tokenURL = srv.URL
	t.Cleanup(func() { tokenURL = prev })
	return &calls, resty.New()
}

func testCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestAuthSessionLiveKeepsConnectionAndSkipsSecondApplicationAuth(t *testing.T) {
	fake := newFakeOpenAPI(true, 5010782, "access-1")
	useFake(t, fake)
	calls, httpClient := tokenServer(t, `{"accessToken":"rotated","refreshToken":"rotated-r","expiresIn":2628000}`)
	refreshed := 0
	client := NewClient(Config{
		Credentials:    Credentials{ClientID: "id", ClientSecret: "secret", AccessToken: "access-1", RefreshToken: "refresh-1", AccountLogin: 5010782},
		HTTPClient:     httpClient,
		OnTokenRefresh: func(TokenSet) { refreshed++ },
	})
	defer client.Disconnect()

	session, err := client.AuthSession(testCtx(t))
	if err != nil {
		t.Fatalf("AuthSession(live) error = %v", err)
	}
	if !session.IsLive || session.CtidTraderAccountID != 777 {
		t.Fatalf("unexpected session %+v", session)
	}
	if fake.dials[LiveEndpoint] != 1 || fake.dials[DemoEndpoint] != 0 {
		t.Fatalf("dials = %v, want one live connection", fake.dials)
	}
	if fake.appAuths[LiveEndpoint] != 1 || fake.alreadyLogs != 0 {
		t.Fatalf("application auths = %v (already logged in %d), want exactly one", fake.appAuths, fake.alreadyLogs)
	}
	if fake.accAuths != 1 {
		t.Fatalf("account auths = %d, want 1", fake.accAuths)
	}
	if *calls != 0 || refreshed != 0 {
		t.Fatalf("token refresh called %d times (callback %d), want 0", *calls, refreshed)
	}
	if client.Credentials().AccessToken != "access-1" || client.Credentials().RefreshToken != "refresh-1" {
		t.Fatal("credentials changed without a refresh")
	}
}

func TestAuthSessionDemoSwitchesEndpointAndAuthorizesApplicationAgain(t *testing.T) {
	fake := newFakeOpenAPI(false, 2065426, "access-1")
	useFake(t, fake)
	calls, httpClient := tokenServer(t, `{"accessToken":"rotated"}`)
	client := NewClient(Config{
		Credentials: Credentials{ClientID: "id", ClientSecret: "secret", AccessToken: "access-1", RefreshToken: "refresh-1", AccountLogin: 2065426},
		HTTPClient:  httpClient,
	})
	defer client.Disconnect()

	session, err := client.AuthSession(testCtx(t))
	if err != nil {
		t.Fatalf("AuthSession(demo) error = %v", err)
	}
	if session.IsLive {
		t.Fatal("demo session reported live")
	}
	if fake.dials[LiveEndpoint] != 1 || fake.dials[DemoEndpoint] != 1 {
		t.Fatalf("dials = %v, want live then demo", fake.dials)
	}
	if fake.appAuths[LiveEndpoint] != 1 || fake.appAuths[DemoEndpoint] != 1 || fake.alreadyLogs != 0 {
		t.Fatalf("application auths = %v (already logged in %d), want one per endpoint", fake.appAuths, fake.alreadyLogs)
	}
	if *calls != 0 {
		t.Fatalf("token refresh called %d times, want 0", *calls)
	}
}

func TestAlreadyLoggedInIsSuccessAndNeverRefreshesToken(t *testing.T) {
	fake := newFakeOpenAPI(true, 5010782, "access-1")
	useFake(t, fake)
	calls, httpClient := tokenServer(t, `{"accessToken":"rotated","refreshToken":"rotated-r"}`)
	refreshed := 0
	client := NewClient(Config{
		Credentials:    Credentials{ClientID: "id", ClientSecret: "secret", AccessToken: "access-1", RefreshToken: "refresh-1", AccountLogin: 5010782},
		HTTPClient:     httpClient,
		OnTokenRefresh: func(TokenSet) { refreshed++ },
	})
	defer client.Disconnect()
	ctx := testCtx(t)
	if _, err := client.AuthSession(ctx); err != nil {
		t.Fatalf("AuthSession error = %v", err)
	}
	// Repeated application and account auth on the same connection (what v0.9.0 did
	// for every live account) must succeed without a token refresh.
	if err := client.applicationAuth(ctx); err != nil {
		t.Fatalf("repeated applicationAuth error = %v, want nil for ALREADY_LOGGED_IN", err)
	}
	if fake.alreadyLogs != 1 {
		t.Fatalf("fake answered ALREADY_LOGGED_IN %d times, want 1", fake.alreadyLogs)
	}
	if *calls != 0 || refreshed != 0 {
		t.Fatalf("token refresh called %d times (callback %d) on ALREADY_LOGGED_IN, want 0", *calls, refreshed)
	}
	if client.Credentials().AccessToken != "access-1" {
		t.Fatal("access token rotated on ALREADY_LOGGED_IN")
	}
}

func TestIsAuthErrorClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		auth bool
	}{
		{"already logged in", APIError{Code: "ALREADY_LOGGED_IN", Description: "Open API application is already authorized"}, false},
		{"already authorized text only", APIError{Code: "SOMETHING", Description: "Account is already authorized"}, false},
		{"wrapped already logged in", errors.Join(errors.New("ctx"), APIError{Code: "ALREADY_LOGGED_IN"}), false},
		{"invalid token", APIError{Code: "CH_ACCESS_TOKEN_INVALID", Description: "Invalid access token"}, true},
		{"expired token", APIError{Code: "OA_AUTH_TOKEN_EXPIRED", Description: "Access token expired"}, true},
		{"other error", APIError{Code: "INVALID_REQUEST", Description: "bad"}, false},
		{"not an api error", errors.New("token"), false},
	}
	for _, tc := range cases {
		if got := isAuthError(tc.err); got != tc.auth {
			t.Errorf("%s: isAuthError = %v, want %v", tc.name, got, tc.auth)
		}
	}
	if !IsAlreadyAuthorized(APIError{Code: "ALREADY_LOGGED_IN"}) {
		t.Error("IsAlreadyAuthorized(ALREADY_LOGGED_IN) = false")
	}
}

func TestInvalidTokenRefreshesOnceAndReportsRotatedTokens(t *testing.T) {
	fake := newFakeOpenAPI(true, 5010782, "access-2")
	useFake(t, fake)
	calls, httpClient := tokenServer(t, `{"accessToken":"access-2","refreshToken":"refresh-2","expiresIn":2628000,"tokenType":"bearer"}`)
	var got []TokenSet
	client := NewClient(Config{
		Credentials:    Credentials{ClientID: "id", ClientSecret: "secret", AccessToken: "access-1", RefreshToken: "refresh-1", AccountLogin: 5010782},
		HTTPClient:     httpClient,
		OnTokenRefresh: func(set TokenSet) { got = append(got, set) },
	})
	defer client.Disconnect()
	if _, err := client.AuthSession(testCtx(t)); err != nil {
		t.Fatalf("AuthSession error = %v", err)
	}
	if *calls != 1 || len(got) != 1 {
		t.Fatalf("refresh calls = %d, callbacks = %d, want 1/1", *calls, len(got))
	}
	if got[0].AccessToken != "access-2" || got[0].RefreshToken != "refresh-2" {
		t.Fatal("callback did not receive the rotated token pair")
	}
	if fake.alreadyLogs != 0 {
		t.Fatalf("ALREADY_LOGGED_IN answered %d times, want 0", fake.alreadyLogs)
	}
}

func TestRejectedRefreshIsTyped(t *testing.T) {
	fake := newFakeOpenAPI(true, 5010782, "never")
	useFake(t, fake)
	_, httpClient := tokenServer(t, `{"errorCode":"ACCESS_DENIED","description":"refresh token revoked"}`)
	client := NewClient(Config{
		Credentials: Credentials{ClientID: "id", ClientSecret: "secret", AccessToken: "access-1", RefreshToken: "refresh-1", AccountLogin: 5010782},
		HTTPClient:  httpClient,
	})
	defer client.Disconnect()
	_, err := client.AuthSession(testCtx(t))
	if err == nil {
		t.Fatal("AuthSession succeeded with a revoked grant")
	}
	if !errors.Is(err, ErrTokenRefreshRejected) {
		t.Fatalf("error %v does not wrap ErrTokenRefreshRejected", err)
	}
	var apiErr APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "CH_ACCESS_TOKEN_INVALID" {
		t.Fatalf("error %v lost the provider code", err)
	}
}

func TestParallelClientsOfOneGrantShareOneRefresh(t *testing.T) {
	fake := newFakeOpenAPI(true, 5010782, "access-2")
	useFake(t, fake)
	calls, httpClient := tokenServer(t, `{"accessToken":"access-2","refreshToken":"refresh-2"}`)
	var (
		mu  sync.Mutex
		got []TokenSet
		wg  sync.WaitGroup
	)
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			client := NewClient(Config{
				Credentials: Credentials{ClientID: "id", ClientSecret: "secret", AccessToken: "access-1", RefreshToken: "refresh-1", AccountLogin: 5010782},
				HTTPClient:  httpClient,
				OnTokenRefresh: func(set TokenSet) {
					mu.Lock()
					got = append(got, set)
					mu.Unlock()
				},
			})
			defer client.Disconnect()
			_, errs[i] = client.AuthSession(testCtx(t))
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("client %d: AuthSession error = %v", i, err)
		}
	}
	if *calls != 1 {
		t.Fatalf("token endpoint called %d times for one grant, want 1", *calls)
	}
	if len(got) != len(errs) {
		t.Fatalf("callbacks = %d, want one per client", len(got))
	}
	for _, set := range got {
		if set.AccessToken != "access-2" || set.RefreshToken != "refresh-2" {
			t.Fatal("a client got a different pair than the single rotation")
		}
	}
}
