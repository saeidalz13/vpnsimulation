package config

import "net"

type Client struct {
	Conn net.Conn
}

var Clients = make(map[string]*Client)

