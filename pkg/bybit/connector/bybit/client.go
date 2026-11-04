package bybit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	BaseURL = "https://api.bybit.com"

	defaultTimeout    = 20 * time.Second
	defaultRecvWindow = "10000"

	publicPathPrefix = "/v5/market/"

	CodeTimestampOutOfWindow = 10002
	CodeRateLimit            = 10006
	CodeServerError          = 10016
	CodeIPRateLimit          = 10018
	CodeUnifiedOnly          = 10028
)

type Credentials struct {
	APIKey string
	Secret string
}

// attached keeps each client's credentials without keeping the client alive; the
// rate limiter keys its per-UID endpoint buckets by the API key.
var attached clientstate.Map[Credentials]

// clock is shared: the offset belongs to the host clock, not to a client.
var clock = clocksync.New(func() (int64, error) {
	st, err := doGet[serverTime](NewBaseClient(), "/v5/market/time", nil)
	if err != nil {
		return 0, err
	}
	if nano, convErr := strconv.ParseInt(st.TimeNano, 10, 64); convErr == nil && nano != 0 {
		return nano / 1_000_000, nil
	}
	sec, err := strconv.ParseInt(st.TimeSecond, 10, 64)
	return sec * 1000, err
})

func isPublicPath(path string) bool {
	return strings.HasPrefix(path, publicPathPrefix)
}

func AttachAuth(client *resty.Client, creds Credentials) {
	if prev, ok := attached.Load(client); ok && prev == creds {
		return
	}
	attached.Store(client, creds)

	client.SetPreRequestHook(func(_ *resty.Client, req *http.Request) error {
		if isPublicPath(req.URL.Path) {
			return nil
		}
		ts := strconv.FormatInt(time.Now().UnixMilli()+clock.Get(), 10)
		req.Header.Set("X-BAPI-API-KEY", creds.APIKey)
		req.Header.Set("X-BAPI-TIMESTAMP", ts)
		req.Header.Set("X-BAPI-RECV-WINDOW", defaultRecvWindow)
		req.Header.Set("X-BAPI-SIGN", Sign(creds.Secret, ts, creds.APIKey, defaultRecvWindow, req.URL.RawQuery))
		return nil
	})
}

func NewBaseClient() *resty.Client {
	return resty.New().
		SetTimeout(defaultTimeout).
		SetRetryCount(0)
}

func Sign(secret, timestamp, apiKey, recvWindow, query string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(timestamp + apiKey + recvWindow + query))
	return hex.EncodeToString(h.Sum(nil))
}

type serverTime struct {
	TimeSecond string `json:"timeSecond"`
	TimeNano   string `json:"timeNano"`
}

type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("bybit: unexpected status %s", e.Status)
	}
	return fmt.Sprintf("bybit: unexpected status %s: %s", e.Status, strings.TrimSpace(e.Body))
}

func (e *HTTPError) IPBanned() bool {
	return e.StatusCode == http.StatusForbidden
}

type APIError struct {
	Code    int
	Msg     string
	Path    string
	ResetAt time.Time
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bybit: error %d: %s (%s)", e.Code, e.Msg, e.Path)
}

type envelope struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
	Time    int64           `json:"time"`
}

func DoGet[T any](client *resty.Client, path string, params map[string]string) (T, error) {
	result, err := doGet[T](client, path, params)
	var apiErr *APIError
	if err != nil && asAPIError(err, &apiErr) && apiErr.Code == CodeTimestampOutOfWindow {
		clock.Refresh()
		return doGet[T](client, path, params)
	}
	return result, err
}

func asAPIError(err error, target **APIError) bool {
	e, ok := err.(*APIError)
	if ok {
		*target = e
	}
	return ok
}

func doGet[T any](client *resty.Client, path string, params map[string]string) (T, error) {
	var zero T

	creds, _ := attached.Load(client)
	Limiter.Acquire(creds.APIKey, path)

	req := client.R()
	for k, v := range params {
		req.SetQueryParam(k, v)
	}

	resp, err := req.Get(BaseURL + path)
	if err != nil {
		return zero, err
	}

	Limiter.Observe(creds.APIKey, path, resp.Header())

	body := resp.Body()
	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusMultipleChoices {
		httpErr := &HTTPError{
			StatusCode: resp.StatusCode(),
			Status:     resp.Status(),
			Body:       string(body),
		}
		if ra := resp.Header().Get("Retry-After"); ra != "" {
			if secs, convErr := strconv.Atoi(ra); convErr == nil {
				httpErr.RetryAfter = time.Duration(secs) * time.Second
			}
		}
		return zero, httpErr
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return zero, fmt.Errorf("bybit: decode %s: %w", path, err)
	}
	if env.RetCode != 0 {
		apiErr := &APIError{Code: env.RetCode, Msg: env.RetMsg, Path: path}
		if env.RetCode == CodeRateLimit {
			apiErr.ResetAt = Limiter.ResetAt(creds.APIKey, path)
		}
		return zero, apiErr
	}

	var result T
	if len(env.Result) == 0 || string(env.Result) == "null" {
		return result, nil
	}
	if err := json.Unmarshal(env.Result, &result); err != nil {
		return zero, fmt.Errorf("bybit: decode %s result: %w", path, err)
	}
	return result, nil
}
