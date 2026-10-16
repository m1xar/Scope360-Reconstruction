package matchtrader

import (
	"net/http"
	"time"
)

type Config struct {
	BaseURL    string
	Email      string
	Password   string
	BrokerID   string
	AccountID  string
	HTTPClient *http.Client
	Recorder   Recorder
}

type Exchange struct {
	Name         string
	Method       string
	Path         string
	RequestBody  []byte
	Status       int
	ResponseBody []byte
	Duration     time.Duration
	Err          error
}

type Recorder func(Exchange)
