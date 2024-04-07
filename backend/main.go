package main

import (
	"crypto/tls"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/saeidalz13/zapvpn/db"
	"github.com/saeidalz13/zapvpn/server"
)

func main() {
	// set environment vars
	if os.Getenv("DEV_STAGE") != "prod" {
		if err := godotenv.Load(); err != nil {
			panic("failed to fetch .env vars")
		}
	}
	aesKey := os.Getenv("AES_ENCR_KEY")
	psqlUrl := os.Getenv("PSQL_URL")

	// prepare tcp server options
	tcpServerOption, err := server.NewTCPServer(server.WithAesKey(aesKey))
	if err != nil {
		log.Panicln("failed to init server struct", err)
	}

	// connect to psql db
	db := db.MustConnectToPsql(psqlUrl)

	// Load server's certificate and private key
	cer, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Fatal(err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	// start tcp server with TLS
	tcpListener, err := tls.Listen("tcp", tcpServerOption.ConnString(), config)
	if err != nil {
		panic(err)
	}
	log.Printf("listening to port %d ...", *tcpServerOption.Port)

	// accepting connections
	tcpServer := &server.TCPServer{
		Listener: tcpListener,
		AesKey:   tcpServerOption.AesKey,
		Db:       db,
	}
	tcpServer.AcceptConnection()
}
