package config

import (
	"fmt"
)

type OnError string

var (
	OnErrorPanic OnError = "panic"
	OnErrorFatal OnError = "fatal"
)

func (o OnError) String() string {
	return string(o)
}

func NewOnError(s string) (OnError, error) {
	for _, enum := range []OnError{OnErrorPanic, OnErrorFatal} {
		if s == enum.String() {
			return OnError(s), nil
		}
	}
	return OnError(s), fmt.Errorf("unknown value %q", s)
}
