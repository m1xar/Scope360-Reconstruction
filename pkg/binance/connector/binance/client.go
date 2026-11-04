package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/m1xar/scope360-reconstruction/pkg/clientstate"
	"github.com/m1xar/scope360-reconstruction/pkg/clocksync"
)

const (
	BaseURL = "https://fapi.binance.com"

	defaultTimeout    = 20 * time.Second
	defaultRecvWindow = 10000
)

type Credentials struct {
	APIKey string
	Secret string
}

var publicPaths = map[string]struct{}{
	"/fapi/v1/ping":         {},
	"/fapi/v1/time":         {},
	"/fapi/v1/exchangeInfo": {},
	"/fapi/v1/klines":       {},
	"/fapi/v1/ticker/price": {},
	"/fapi/v1/premiumIndex": {},
}

func isPublicPath(path string) bool {
	_, ok := publicPaths[path]
	return ok
}

// attached dedupes AttachAuth per client without keeping the client alive.
var attached clientstate.Map[Credentials]

// clock is shared: the offset belongs to the host clock, not to a client.
var clock = clocksync.New(func() (int64, error) {
	type serverTime struct {
		ServerTime int64 `json:"serverTime"`
	}
	st, err := doGet[serverTime](NewBaseClient(), "/fapi/v1/time", nil, 1)
	return st.ServerTime, err
})

func AttachAuth(client *resty.Client, creds Credentials) {
	if prev, ok := attached.Load(client); ok && prev == creds {
		return
	}
	attached.Store(client, creds)

	client.SetPreRequestHook(func(_ *resty.Client, req *http.Request) error {
		req.Header.Set("X-MBX-APIKEY", creds.APIKey)
		if isPublicPath(req.URL.Path) {
			return nil
		}

		q := req.URL.Query()
		q.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli()+clock.Get(), 10))
		q.Set("recvWindow", strconv.Itoa(defaultRecvWindow))
		raw := q.Encode()

		req.URL.RawQuery = raw + "&signature=" + signRequest(creds.Secret, raw)
		return nil
	})
}

func NewBaseClient() *resty.Client {
	return resty.New().
		SetTimeout(defaultTimeout).
		SetRetryCount(0)
}

func signRequest(secret, payload string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
	Code       int
	Msg        string
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("binance: error %d: %s (%s)", e.Code, e.Msg, e.Status)
	}
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("binance: unexpected status %s", e.Status)
	}
	return fmt.Sprintf("binance: unexpected status %s: %s", e.Status, strings.TrimSpace(e.Body))
}

type apiErrorBody struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// CodeTimestampOutOfWindow is returned for a timestamp outside recvWindow or ahead
// of the server time by more than a second.
const CodeTimestampOutOfWindow = -1021

func DoGet[T any](client *resty.Client, path string, params map[string]string, weight int) (T, error) {
	result, err := doGet[T](client, path, params, weight)
	var httpErr *HTTPError
	if err != nil && errors.As(err, &httpErr) && httpErr.Code == CodeTimestampOutOfWindow {
		clock.Refresh()
		return doGet[T](client, path, params, weight)
	}
	return result, err
}

func doGet[T any](client *resty.Client, path string, params map[string]string, weight int) (T, error) {
	var zero T

	Limiter.Acquire(weight)

	req := client.R()
	for k, v := range params {
		req.SetQueryParam(k, v)
	}

	resp, err := req.Get(BaseURL + path)
	if err != nil {
		return zero, err
	}

	Limiter.Observe(resp.Header().Get("X-MBX-USED-WEIGHT-1M"))

	body := resp.Body()
	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusMultipleChoices {
		httpErr := &HTTPError{
			StatusCode: resp.StatusCode(),
			Status:     resp.Status(),
			Body:       string(body),
		}
		var apiErr apiErrorBody
		if json.Unmarshal(body, &apiErr) == nil {
			httpErr.Code = apiErr.Code
			httpErr.Msg = apiErr.Msg
		}
		if ra := resp.Header().Get("Retry-After"); ra != "" {
			if secs, convErr := strconv.Atoi(ra); convErr == nil {
				httpErr.RetryAfter = time.Duration(secs) * time.Second
			}
		}
		return zero, httpErr
	}

	var result T
	if len(body) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return zero, fmt.Errorf("binance: decode %s: %w", path, err)
	}
	return result, nil
}
