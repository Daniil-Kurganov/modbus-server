package modbusserver

import (
	"bufio"
	"crypto/tls"
	"io"
	"log"
	"net"
	"slices"
	"strings"
	"time"

	reuse "github.com/libp2p/go-reuseport"
)

func (s *Server) accept(listen net.Listener) error {
	log.Printf("--%s (%s): start accepting connections--", listen.Addr().String(), time.Now().String())
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
		log.Printf("--%s (%s): new connection: type - %s, address - %s", conn.LocalAddr().String(), time.Now().String(), conn.RemoteAddr().Network(), conn.RemoteAddr().String())
		if isFirstClient {
			log.Printf("--%s (%s): connection now is first client--", conn.LocalAddr().String(), time.Now().String())
			if s.ConnectionChanel != nil {
				select {
				case <-time.After(50 * time.Millisecond):
				case s.ConnectionChanel <- true:
				}
			}
			isFirstClient = false
			log.Printf("--%s (%s): connection now isn't first client--", conn.LocalAddr().String(), time.Now().String())
		}
		reader := bufio.NewReader(conn)
		for {
			packet := make([]byte, 512)
			log.Printf("--%s (%s): current packet reading--", conn.LocalAddr().String(), time.Now().String())
			bytesRead, err := reader.Read(packet)
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") || err == io.EOF {
					log.Printf("--%s (%s): current packet reading  error: %s; breaking connection--", conn.LocalAddr().String(), time.Now().String(), err.Error())
					break
				}
				log.Printf("--%s (%s): current packet reading  error: %s--", conn.LocalAddr().String(), time.Now().String(), err.Error())
				continue
			}
			log.Printf("--%s (%s): current packet successfully received--", conn.LocalAddr().String(), time.Now().String())
			packet = packet[:bytesRead]
			log.Printf("--%s (%s): current packet preparing--", conn.LocalAddr().String(), time.Now().String())
			frame, err := NewTCPFrame(packet)
			if err != nil {
				log.Printf("--%s (%s): current packet preparing error: %s--", conn.LocalAddr().String(), time.Now().String(), err.Error())
				continue
			}
			slaveID := frame.GetSlaveId()
			log.Printf("--%s (%s): current packet successfully prepared: slave Id = %d--", conn.LocalAddr().String(), time.Now().String(), slaveID)
			if _, ok := s.Slaves[slaveID]; ok && !slices.Contains(s.SlavesStoppedResponse, slaveID) {
				request := &Request{conn, frame}
				log.Printf("--%s (%s): current request successfully starts procesing--", conn.LocalAddr().String(), time.Now().String())
				s.requestChan <- request
			} else {
				log.Printf("--%s (%s): invalid slave Id: requested slave Id doesn't initialized or disabled--", conn.LocalAddr().String(), time.Now().String())
			}
		}
		conn.Close()
		log.Printf("--%s (%s): close connection %s--", conn.LocalAddr().String(), time.Now().String(), conn.RemoteAddr().String())
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
