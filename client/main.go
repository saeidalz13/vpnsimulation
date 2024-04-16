package main

import (
	"crypto/tls"
	"fmt"
	"log"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/songgao/water"
)

const mtu = 1500

/*
1. Run this applicatoin to have a utun13 interface available
2. Run command `sudo ifconfig utun13 inet 10.0.0.1 10.0.0.2 up`
  - 10.0.0.1 is your local machine
  - 10.0.0.2 is the IP of this application, and set the flag up so it's up and running

3. Run command `sudo route add 0.0.0.0/0 -interface utun13`
  - we're saying whatever data from or received from your machine would directly go into this Go app
  - basically, you have rerouted the traffic of your machine to go through this app
*/
func main() {
	config := &tls.Config{
		// accepting any certificate from the server
		// since this is a simulation and server is known, it's ok to set it to true
		InsecureSkipVerify: true,
	}

	// connecting to the server
	conn, err := tls.Dial("tcp", "localhost:8000", config)
	if err != nil {
		log.Panicln("failed to dial tcp server", err)
	}

	// Setting a 8 second deadline for read operation
	// if err := conn.SetReadDeadline(time.Now().Add(time.Second*8)); err != nil {
	// 	log.Panicln("failed to set read deadlin:", err)
	// }

	// make a buffer and read the first hello from the server to make sure the connection works
	buff := make([]byte, 2048)
	n, err := conn.Read(buff)
	if err != nil {
		log.Panicln("failed to read the response message from tcp server", err)
	}
	fmt.Println(string(buff[:n]))

	// configuring TUN (network tunnel using water package)
	configWater := water.Config{
		DeviceType: water.TUN,
	}
	configWater.Name = "utun13"

	ifce, err := water.New(configWater)
	if err != nil {
		log.Panicln("failed to create a new water interface", err)
	}
	log.Println("virtual network interface craeted:", ifce.Name())

	// start sending the packets to VPN server
	// For now, sending over all the traffic to the VPN server with no filter
	// TODO: filter the traffic sent to VPN server
	for {
		packet := make([]byte, mtu)
		n, err := ifce.Read(packet)
		if err != nil {
			log.Panicln(err)
		}
		log.Println(packet[:n])

		_, err = conn.Write(packet[:n])
		if err != nil {
			packetData := gopacket.NewPacket(packet[:n], layers.LayerTypeIPv4, gopacket.Default)
			ipLayer := packetData.Layer(layers.LayerTypeIPv4)
			if ipLayer != nil {
				ip, _ := ipLayer.(*layers.IPv4)
				log.Printf("failed to write to TCP conn -> Source IP: %s | Dest IP: %s", ip.SrcIP, ip.DstIP)
			}
			continue
		}

		respBuff := make([]byte, 4096)
		n, err = conn.Read(respBuff)
		if err != nil {
			log.Println("failed to fetch response from vpn server")
			continue
		}

		log.Println(respBuff[:n])
	}
}
