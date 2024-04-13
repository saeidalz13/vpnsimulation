package config

import (
	"errors"
	"fmt"
)

var defaultHostname = "127.0.0.1"
var defaultPort int16 = 8000

type TCPServerOption struct {
	Hostname *string
	Port     *int16
	AesKey   string
}

func (t *TCPServerOption) ConnString() string {
	return fmt.Sprintf("%s:%d", *t.Hostname, *t.Port)
}

type OptionFunc func(tcpSever *TCPServerOption) error

func NewTCPServer(opts ...OptionFunc) (*TCPServerOption, error) {
	var options TCPServerOption
	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	if options.Port == nil {
		options.Port = &defaultPort
	}
	if options.Hostname == nil {
		options.Hostname = &defaultHostname
	}

	server := &TCPServerOption{
		Hostname: options.Hostname,
		Port:     options.Port,
		AesKey:   options.AesKey,
	}
	return server, nil
}

func WithHostname(hostname string) OptionFunc {
	return func(tcpSever *TCPServerOption) error {
		tcpSever.Hostname = &hostname
		return nil
	}
}

func WithPort(port int16) OptionFunc {
	return func(tcpSever *TCPServerOption) error {
		tcpSever.Port = &port
		return nil
	}
}

func WithAesKey(aesKey string) OptionFunc {
	return func(tcpSever *TCPServerOption) error {
		if len(aesKey) != 32 {
			return errors.New("AES key must have length of 32")
		}

		tcpSever.AesKey = aesKey
		return nil
	}
}
