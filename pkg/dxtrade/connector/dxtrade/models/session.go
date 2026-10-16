package models

import "encoding/json"

type LoginRequest struct {
	Username string `json:"username"`
	Domain   string `json:"domain"`
	Password string `json:"password"`
}

type SessionToken struct {
	SessionToken string          `json:"sessionToken"`
	Timeout      json.RawMessage `json:"timeout"`
}

type APIError struct {
	ErrorCode   string `json:"errorCode"`
	Description string `json:"description"`
}
