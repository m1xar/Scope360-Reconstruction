package blofin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
)

func AttachAuth(client *resty.Client, creds Credentials) {
	client.SetPreRequestHook(func(_ *resty.Client, req *http.Request) error {
		ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
		nonce := uuid.NewString()
		body, err := requestBody(req)
		if err != nil {
			return err
		}

		sign := signRequest(creds.Secret, req.URL.RequestURI(), req.Method, ts, nonce, body)

		req.Header.Set("ACCESS-KEY", creds.APIKey)
		req.Header.Set("ACCESS-SIGN", sign)
		req.Header.Set("ACCESS-TIMESTAMP", ts)
		req.Header.Set("ACCESS-NONCE", nonce)
		req.Header.Set("ACCESS-PASSPHRASE", creds.Passphrase)
		req.Header.Set("Content-Type", "application/json")
		return nil
	})
}

func signRequest(secret, requestPath, method, timestamp, nonce, body string) string {
	payload := requestPath + strings.ToUpper(method) + timestamp + nonce + body
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	hexDigest := hex.EncodeToString(h.Sum(nil))
	return base64.StdEncoding.EncodeToString([]byte(hexDigest))
}

func requestBody(req *http.Request) (string, error) {
	if req.Body == nil {
		return "", nil
	}
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return "", err
	}
	req.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	return string(bodyBytes), nil
}
