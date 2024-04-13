package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/joho/godotenv"
	cn "github.com/saeidalz13/vpnsimulation/server/config"
	"github.com/saeidalz13/vpnsimulation/server/db"
	"github.com/saeidalz13/vpnsimulation/server/db/sqlc"
)

const ctxTimeoutDuration = time.Second * 5

type TCPServer struct {
	Listener net.Listener
	AesKey   string
	Db       *sql.DB
}

func deleteConnFromDb(db *sql.DB, remoteAddr string) error {
	q := sqlc.New(db)
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeoutDuration)
	defer cancel()

	if err := q.DeleteConnection(ctx, remoteAddr); err != nil {
		return err
	}
	return nil
}

func insertConnToDb(db *sql.DB, remoteAddr string) error {
	q := sqlc.New(db)
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeoutDuration)
	defer cancel()

	if err := q.InsertConnection(ctx, remoteAddr); err != nil {
		return err
	}
	return nil
}

func handlePacketRouting(conn net.Conn, ip *layers.IPv4, tcp *layers.TCP, payload []byte) error {
	dstAddr := fmt.Sprintf("%s:%d", ip.DstIP, tcp.DstPort)
	dialer := net.Dialer{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	connReq, err := dialer.DialContext(ctx, "tcp", dstAddr)
	if err != nil {
		log.Println(err)
		return err
	}
	defer connReq.Close()

	// sending the payload to the destination
	log.Println("writing data to packet destination connection")
	_, err = connReq.Write(payload)
	if err != nil {
		log.Printf("Failed to write payload to connection %s: %v", dstAddr, err)
		return err
	}

	// handling the response
	respBuff := make([]byte, 4096)
	n, err := connReq.Read(respBuff)
	if err != nil {
		log.Printf("Failed to get the response from the connection %s: %v", dstAddr, err)
		return err
	}

	// writing response to VPN client
	_, err = conn.Write(respBuff[:n])
	if err != nil {
		log.Printf("Failed to send response back to client: %v", err)
		return err
	}
	return nil
}

func (t *TCPServer) AcceptConnection() {
	for {
		// accept a connection
		conn, err := t.Listener.Accept()
		if err != nil {
			log.Println("connection failed", err)
			continue
		}
		log.Println("new connection:", conn.RemoteAddr().String())

		if err := insertConnToDb(t.Db, conn.RemoteAddr().String()); err != nil {
			// if database is not working, i don't want to allow to proceed any further
			log.Println("failed to add connection to db", err)
			continue
		}
		log.Println("connection added to database", conn.RemoteAddr().String())

		// initialize GCM for user data encryption
		// key := []byte(t.AesKey)
		// gcm, err := InitGcm(key)
		// if err != nil {
		// 	log.Println(err)
		// 	continue
		// }
		go t.ManageConn(conn)
	}
}

func (t *TCPServer) ManageConn(conn net.Conn) {
	defer func() {
		log.Println("connection terminated:", conn.RemoteAddr().String())
		conn.Close()
		if err := deleteConnFromDb(t.Db, conn.RemoteAddr().String()); err != nil {
			log.Println("failed to delete connection from database: ", conn.RemoteAddr().String(), err)
			return
		}
		log.Println("connection deleted from database")
	}()

	conn.Write([]byte("hey vpn client!"))
	for {
		packet := make([]byte, 4096)
		n, err := conn.Read(packet)
		if err != nil {
			// if TLS handshake not executed
			if strings.Contains(err.Error(), "first record does not look like a TLS handshake") {
				log.Println(err)
				break
			}
			// if the connection got terminated from the client
			if err == io.EOF {
				log.Println(err)
				break
			}
			// other errors
			log.Println(err)
			continue
		}

		packetData := gopacket.NewPacket(packet[:n], layers.LayerTypeIPv4, gopacket.Default)
		ipLayer := packetData.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}

		packetTcp := gopacket.NewPacket(packet[:n], layers.LayerTypeTCP, gopacket.Default)
		tcpLayer := packetTcp.Layer(layers.LayerTypeTCP)
		if tcpLayer == nil {
			continue
		}

		// logging the incoming packets
		ip, _ := ipLayer.(*layers.IPv4)
		tcp, _ := tcpLayer.(*layers.TCP)
		if err := handlePacketRouting(conn, ip, tcp, tcp.Payload); err != nil {
			// error or not, after this line the loop should continue
			conn.Write([]byte("Failed to send packet to destination"))
		}

		// log.Printf("Source IP: %s | IP Dest: %s", ip.SrcIP, ip.DstIP)
		// log.Printf("TCP Source Port: %s | TCP Dest PORT: %s", tcp.SrcPort, tcp.DstPort)

		// TODO: Encryption
		// encData, err := EncryptData(gcm, srcBuff)
		// if err != nil {
		// 	client.Conn.Write([]byte("failed to encrypt data. traffic did not transmitted"))
		// 	continue
		// }
		// log.Println(string(encData))
	}
}

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
	tcpServerOption, err := cn.NewTCPServer(cn.WithAesKey(aesKey))
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
	tcpServer := &TCPServer{
		Listener: tcpListener,
		AesKey:   tcpServerOption.AesKey,
		Db:       db,
	}
	tcpServer.AcceptConnection()
}
