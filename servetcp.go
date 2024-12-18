package modbusserver

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"strings"

	reuse "github.com/libp2p/go-reuseport"
)

func (s *Server) accept(listen net.Listener) error {
	log.Print("Start acception connections")
	isFirstClient := true
	for {
		conn, err := listen.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			log.Printf("Unable to accept connections: %#v\n", err)
			return err
		}
		log.Printf("New connection: type - %s, address - %s", conn.RemoteAddr().Network(), conn.RemoteAddr().String())
		if isFirstClient {
			s.ConnectionChanel <- &conn
			isFirstClient = false
		}
		go func(conn net.Conn) {
			defer conn.Close()

			for {
				packet := make([]byte, 512)
				bytesRead, err := conn.Read(packet)
				if err != nil {
					if err != io.EOF {
						log.Printf("read error %v\n", err)
					}
					return
				}
				// Set the length of the packet to the number of read bytes.
				packet = packet[:bytesRead]

				frame, err := NewTCPFrame(packet)
				if err != nil {
					log.Printf("bad packet error %v\n", err)
					return
				}
				log.Printf("Current slave ID: %d", frame.GetSlaveId())
				if _, ok := s.Slaves[frame.GetSlaveId()]; ok {
					request := &Request{conn, frame}
					s.requestChan <- request
				}
			}
		}(conn)
	}
}

// ListenTCP starts the Modbus server listening on "address:port".
func (s *Server) ListenTCP(addressPort string) (err error) {
	listen, err := reuse.Listen("tcp", addressPort)
	if err != nil {
		log.Printf("Failed to Listen: %v\n", err)
		return err
	}
	s.listeners = append(s.listeners, listen)
	go s.accept(listen)
	return err
}

// ListenTLS starts the Modbus server listening on "address:port".
func (s *Server) ListenTLS(addressPort string, config *tls.Config) (err error) {
	listen, err := tls.Listen("tcp", addressPort, config)
	if err != nil {
		log.Printf("Failed to Listen on TLS: %v\n", err)
		return err
	}
	s.listeners = append(s.listeners, listen)
	go s.accept(listen)
	return err
}
