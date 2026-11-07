package helper

import (
	"strings"

	"github.com/google/uuid"
)

type WebResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

const (
	LayoutDateTime      = `2006-01-02 15:04:05`
	LayoutDateTimeNoSec = `2006-01-02 15:04`
	LayoutDate          = "2006-01-02"
	LayoutTime          = "15:04:05"
	LayoutTimeNoSec     = "15:04"
)

// EmptyObject returns an empty object for responses.
func EmptyObject() interface{} {
	return make(map[string]interface{})
}

// EmptyArray returns an empty array for responses.
func EmptyArray() interface{} {
	return []interface{}{}
}

// GenerateSessionID generates a unique session ID for tracing.
func GenerateSessionID() string {
	randomUUID := uuid.New()
	return strings.Replace(randomUUID.String(), "-", "", -1)
}
