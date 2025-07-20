package main

import (
	"flag"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/mdlayher/packet"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"
)

func init() {
	flag.Usage = func() {
		fmt.Printf("syntax: %s [flags] IFNAME timout-in-seconds\n", os.Args[0])
		flag.PrintDefaults()
	}
	//flag.Var(&options, "option", "custom DHCP option for the request (code,value)")
	//flag.Var(&requestParams, "request", "Additional value for the DHCP Request List Option 55 (code)")
}

func main() {
	err := start()
	if err != nil {
		log.Fatal(err)
	}
}

func start() (err error) {
	flag.Parse()

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(1)
	}

	timeoutSeconds, err := strconv.Atoi(flag.Arg(1))
	if err != nil {
		return err
	}
	timeout := time.Duration(timeoutSeconds) * time.Second

	ifname := flag.Arg(0)
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		return
	}

	conn, err := packet.Listen(iface, packet.Raw, int(layers.EthernetTypeIPv4), nil)
	if err != nil {
		return
	}

	xid := rand.Uint32()

	toSend := newPacket(layers.DHCPMsgTypeDiscover, iface.HardwareAddr, xid)

	log.Printf("sending packet")
	err = sendMulticast(conn, toSend, iface.HardwareAddr)
	if err != nil {
		return
	}

	received, err := waitForResponse(conn, xid, timeout)
	if err != nil {
		return
	}

	if received != nil {
		log.Printf("received packet client: %s, server: %s", received.YourClientIP.String(), received.NextServerIP.String())
	}

	defer func() {
		err = conn.Close()
	}()
	return
}

func newPacket(msgType layers.DHCPMsgType, addr net.HardwareAddr, xid uint32) *layers.DHCPv4 {
	pack := layers.DHCPv4{
		Operation:    layers.DHCPOpRequest,
		HardwareType: layers.LinkTypeEthernet,
		ClientHWAddr: addr,
		Xid:          xid,
	}

	pack.Options = append(pack.Options, layers.DHCPOption{
		Type:   layers.DHCPOptMessageType,
		Data:   []byte{byte(msgType)},
		Length: 1,
	})

	return &pack
}

func sendMulticast(conn *packet.Conn, dhcp *layers.DHCPv4, addr net.HardwareAddr) error {
	eth := layers.Ethernet{
		EthernetType: layers.EthernetTypeIPv4,
		SrcMAC:       addr,
		DstMAC:       layers.EthernetBroadcast,
	}
	ip := layers.IPv4{
		Version:  4,
		TTL:      64,
		SrcIP:    []byte{0, 0, 0, 0},
		DstIP:    []byte{255, 255, 255, 255},
		Protocol: layers.IPProtocolUDP,
	}
	udp := layers.UDP{
		SrcPort: 68,
		DstPort: 67,
	}

	// Serialize packet
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
	err := udp.SetNetworkLayerForChecksum(&ip)
	if err != nil {
		return err
	}
	err = gopacket.SerializeLayers(buf, opts, &eth, &ip, &udp, dhcp)
	if err != nil {
		return err
	}

	// Send packet
	_, err = conn.WriteTo(buf.Bytes(), &packet.Addr{HardwareAddr: eth.DstMAC})
	return err
}

// waitForResponse waits for a DHCP packet with matching transaction ID and the given message type
func waitForResponse(conn *packet.Conn, xid uint32, timeout time.Duration) (*layers.DHCPv4, error) {
	err := conn.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		return nil, err
	}
	recvBuf := make([]byte, 1500)
	for {
		_, _, err = conn.ReadFrom(recvBuf)

		if err != nil {
			return nil, err
		}

		pack := parsePacket(recvBuf)
		if pack == nil {
			continue
		}

		if pack.Xid == xid && pack.Operation == layers.DHCPOpReply {
			return pack, nil
		}
	}
}

// parsePacket decodes a DHCPv4 packet
func parsePacket(data []byte) *layers.DHCPv4 {
	pack := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.Default)
	dhcpLayer := pack.Layer(layers.LayerTypeDHCPv4)

	if dhcpLayer == nil {
		// received packet is not DHCP
		return nil
	}
	return dhcpLayer.(*layers.DHCPv4)
}
