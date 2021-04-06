package main

import (
	"fmt"
	"github.com/pion/stun"
	"github.com/pkg/errors"
	"log"
	"net"
	"strings"
)

// Server is RFC 5389 basic server implementation.
//
// Current implementation is UDP only and not utilizes FINGERPRINT mechanism,
// nor ALTERNATE-SERVER, nor credentials mechanisms. It does not support
// backwards compatibility with RFC 3489.
type Server struct{}

var (
	software          = stun.NewSoftware("webix-hub/chat-go")
	errNotSTUNMessage = errors.New("not stun message")
)

func basicProcess(addr net.Addr, b []byte, req, res *stun.Message) error {
	if !stun.IsMessage(b) {
		return errNotSTUNMessage
	}
	if _, err := req.Write(b); err != nil {
		return errors.Wrap(err, "failed to read message")
	}
	var (
		ip   net.IP
		port int
	)
	switch a := addr.(type) {
	case *net.UDPAddr:
		ip = a.IP
		port = a.Port
	default:
		panic(fmt.Sprintf("unknown addr: %v", addr))
	}
	return res.Build(req,
		stun.BindingSuccess,
		software,
		&stun.XORMappedAddress{
			IP:   ip,
			Port: port,
		},
		stun.Fingerprint,
	)
}

func (s *Server) serveConn(c net.PacketConn, res, req *stun.Message) error {
	if c == nil {
		return nil
	}
	buf := make([]byte, 1024)
	n, addr, err := c.ReadFrom(buf)
	if err != nil {
		log.Printf("ReadFrom: %v", err)
		return nil
	}
	// s.log().Printf("read %d bytes from %s", n, addr)
	if _, err = req.Write(buf[:n]); err != nil {
		log.Printf("Write: %v", err)
		return err
	}
	if err = basicProcess(addr, buf[:n], req, res); err != nil {
		if err == errNotSTUNMessage {
			return nil
		}
		log.Printf("basicProcess: %v", err)
		return nil
	}
	_, err = c.WriteTo(res.Raw, addr)
	if err != nil {
		log.Printf("WriteTo: %v", err)
	}
	return err
}

// Serve reads packets from connections and responds to BINDING requests.
func (s *Server) Serve(c net.PacketConn) error {
	var (
		res = new(stun.Message)
		req = new(stun.Message)
	)
	for {
		if err := s.serveConn(c, res, req); err != nil {
			log.Printf("serve: %v", err)
			return err
		}
		res.Reset()
		req.Reset()
	}
}

func normalize(address string) string {
	if len(address) == 0 {
		address = "0.0.0.0"
	}
	if !strings.Contains(address, ":") {
		address = fmt.Sprintf("%s:%d", address, stun.DefaultPort)
	}
	return address
}

// ListenUDPAndServe listens on laddr and process incoming packets.
func startStunServer(serverNet, laddr string) error {
	normalized := normalize(laddr)
	fmt.Println("gortc/stund listening on", normalized, "via", serverNet)
	c, err := net.ListenPacket(serverNet, normalized)
	if err != nil {
		return err
	}
	s := &Server{}
	return s.Serve(c)
}
