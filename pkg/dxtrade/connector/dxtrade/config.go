package dxtrade

import (
	"net/http"
	"strings"
)

const (
	apiPrefix = "/dxsca-web"
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36"
)

type Config struct {
	BaseURL    string
	Username   string
	Domain     string
	Password   string
	Account    string
	HTTPClient *http.Client
	Recorder   Recorder
}

func (c Config) host() string {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	base = strings.TrimSuffix(base, apiPrefix)
	return strings.TrimRight(base, "/")
}
