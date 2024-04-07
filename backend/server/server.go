package server

import (
	"crypto/cipher"
	"database/sql"
	"io"
	"log"
	"net"
	"strings"

	c "github.com/saeidalz13/zapvpn/client"
)

type TCPServer struct {
	Listener net.Listener
	AesKey   string
	Db       *sql.DB
}

func (t *TCPServer) AcceptConnection() {
	for {
		// TODO: User Authentication

		// accept a connection
		conn, err := t.Listener.Accept()
		if err != nil {
			log.Println("connection failed", err)
			continue
		}
		log.Println("new connection:", conn.RemoteAddr().String())

		// TODO: Add connection to database

		// initialize GCM for user data encryption
		key := []byte(t.AesKey)
		gcm, err := InitGcm(key)
		if err != nil {
			log.Println(err)
			continue
		}

		newClient := &c.Client{Conn: conn}
		c.Clients[conn.RemoteAddr().String()] = newClient

		go t.ManageConn(newClient, gcm)
	}
}

func (t *TCPServer) ManageConn(client *c.Client, gcm cipher.AEAD) {
	defer func() {
		log.Println("connection terminated:", client.Conn.RemoteAddr().String())
		client.Conn.Close()
	}()

	for {
		srcBuff := make([]byte, 2048)

		n, err := client.Conn.Read(srcBuff)
		if err != nil {
			// if TLS handshake not executed
			if strings.Contains(err.Error(), "first record does not look like a TLS handshake") {
				log.Println(err)
				break
			}
			// if the connection got terminated from the client
			if err == io.EOF {
				break
			}
			// other errors
			log.Println(err)
			continue
		}
		encData, err := EncryptData(gcm, srcBuff)
		if err != nil {
			client.Conn.Write([]byte("failed to encrypt data. traffic did not transmitted"))
			continue
		}
		log.Println(string(srcBuff[:n]))
		log.Println(string(encData))
	}
}
