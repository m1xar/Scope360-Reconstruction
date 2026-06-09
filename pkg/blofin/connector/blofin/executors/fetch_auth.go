package executors

import (
	"github.com/go-resty/resty/v2"
)

func FetchAuthStatus(client *resty.Client) error {
	_, err := FetchBalance(client)
	return err
}
