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

func handlePacketRouting(conn net.Conn, innerPacket []byte) error {
	packetData := gopacket.NewPacket(innerPacket, layers.LayerTypeIPv4, gopacket.Default)
	ipLayer := packetData.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return fmt.Errorf("no ip layer found in the inner packet")
	}
	ipv4, _ := ipLayer.(*layers.IPv4)

	packetTcp := gopacket.NewPacket(innerPacket, layers.LayerTypeTCP, gopacket.Default)
	if tcpLayer := packetTcp.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)
		dstAddr := fmt.Sprintf("%s:%d", ipv4.DstIP, tcp.DstPort)

		dialer := net.Dialer{}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()
		log.Println("TCP packet received; dialing destination PORT:IP...")

		// TODO: Attempt a tcp connection every time, Overhead is an issue here; connection pool should be made
		connReq, err := dialer.DialContext(ctx, "tcp", dstAddr)
		if err != nil {
			return err
		}
		defer connReq.Close()

		// sending the payload to the destination
		log.Println("writing data to packet destination connection")
		_, err = connReq.Write(tcp.Payload)
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

		_, err = conn.Write(respBuff[:n])
		if err != nil {
			log.Printf("Failed to send response back to client: %v", err)
			return err
		}
		return nil
	}

	packetUdp := gopacket.NewPacket(innerPacket, layers.LayerTypeUDP, gopacket.Default)
	if udpLayer := packetUdp.Layer(layers.LayerTypeUDP); udpLayer != nil {
		log.Println("UDP packet received; sending datagrams to destination PORT:IP...")
		udp, _ := udpLayer.(*layers.UDP)
		dstAddr := fmt.Sprintf("%s:%d", ipv4.DstIP, udp.DstPort)

		dialer := net.Dialer{}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()

		udpConn, err := dialer.DialContext(ctx, "udp", dstAddr)
		if err != nil {
			return err
		}
		_, err = udpConn.Write(udp.Payload)
		if err != nil {
			return err
		}

		respBuff := make([]byte, 4096)
		n, err := udpConn.Read(respBuff)
		if err != nil {
			return err
		}

		_, err = conn.Write(respBuff[:n])
		if err != nil {
			return err
		}
		return nil
	}

	_, err := conn.Write([]byte("Unsupported protocol"))
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

		packetTcp := gopacket.NewPacket(packet[:n], layers.LayerTypeTCP, gopacket.Default)
		tcpLayer := packetTcp.Layer(layers.LayerTypeTCP)
		if tcpLayer == nil {
			continue
		}
		tcp, _ := tcpLayer.(*layers.TCP)
		innerPacket := tcp.Payload

		if err := handlePacketRouting(conn, innerPacket); err != nil {
			// error or not, after this line the loop should continue
			log.Println(err)
			conn.Write([]byte("Failed to send packet to destination"))
		}

		// log.Printf("Source IP: %s | IP Dest: %s", ip.SrcIP, ip.DstIP)
		// log.Printf("TCP Source Port: %s | TCP Dest PORT: %s", tcp.SrcPort, tcp.DstPort)

		// packetData := gopacket.NewPacket(packet[:n], layers.LayerTypeIPv4, gopacket.Default)
		// ipLayer := packetData.Layer(layers.LayerTypeIPv4)
		// if ipLayer == nil {
		// 	continue
		// }
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
	log.Printf("listening to port %d...", *tcpServerOption.Port)

	// accepting connections
	tcpServer := &TCPServer{
		Listener: tcpListener,
		AesKey:   tcpServerOption.AesKey,
		Db:       db,
	}
	tcpServer.AcceptConnection()
}
