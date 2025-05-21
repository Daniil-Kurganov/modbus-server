package modbusserver

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"slices"
	"strings"
	"time"

	reuse "github.com/libp2p/go-reuseport"
)

func (s *Server) accept(listen net.Listener) error {
	s.logger.Debug(fmt.Sprintf("Server %s: start accepting connections", listen.Addr().String()))
	isFirstClient := true
	for {
		conn, err := listen.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			s.logger.Info(fmt.Sprintf("Server %s: unable to accept connections: %s", s.listeners[0].Addr().String(), err.Error()))
			return err
		}
		s.logger.Debug(fmt.Sprintf("Server %s: new connection: type - %s, address - %s",
			conn.LocalAddr().String(), conn.RemoteAddr().Network(), conn.RemoteAddr().String()))
		if isFirstClient {
			s.logger.Debug(fmt.Sprintf("Server %s: connection now is first client", conn.LocalAddr().String()))
			if s.ConnectionChanel != nil {
				select {
				case <-time.After(50 * time.Millisecond):
				case s.ConnectionChanel <- true:
				}
			}
			isFirstClient = false
			s.logger.Debug(fmt.Sprintf("Server %s: connection now isn't first client", conn.LocalAddr().String()))
		}
		reader := bufio.NewReader(conn)
		for {
			packet := make([]byte, 512)
			s.logger.Debug(fmt.Sprintf("Server %s: current packet reading", conn.LocalAddr().String()))
			bytesRead, err := reader.Read(packet)
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") || err == io.EOF {
					s.logger.Error(fmt.Sprintf("Server %s: current packet reading  error: %s; breaking connection", conn.LocalAddr().String(), err.Error()))
					break
				}
				s.logger.Error(fmt.Sprintf("Server %s: current packet reading error: %s", conn.LocalAddr().String(), err.Error()))
				continue
			}
			s.logger.Debug(fmt.Sprintf("Server %s: current packet successfully received", conn.LocalAddr().String()))
			packet = packet[:bytesRead]
			s.logger.Debug(fmt.Sprintf("Server %s: current packet preparing", conn.LocalAddr().String()))
			frame, err := NewTCPFrame(packet)
			if err != nil {
				s.logger.Error(fmt.Sprintf("Server %s: current packet preparing error: %s", conn.LocalAddr().String(), err.Error()))
				continue
			}
			slaveID := frame.GetSlaveId()
			s.logger.Debug(fmt.Sprintf("Server %s: current packet successfully prepared: slave Id = %d", conn.LocalAddr().String(), slaveID))
			if _, ok := s.Slaves[slaveID]; ok && !slices.Contains(s.SlavesStoppedResponse, slaveID) {
				request := &Request{conn, frame}
				s.logger.Debug(fmt.Sprintf("Server %s: current request successfully starts procesing", conn.LocalAddr().String()))
				s.requestChan <- request
			} else {
				s.logger.Warn(fmt.Sprintf("Server %s: invalid slave Id: requested slave Id doesn't initialized or disabled", conn.LocalAddr().String()))
			}
		}
		conn.Close()
		s.logger.Info(fmt.Sprintf("Server %s: close connection %s", conn.LocalAddr().String(), conn.RemoteAddr().String()))
	}
}

// ListenTCP starts the Modbus server listening on "address:port".
func (s *Server) ListenTCP(addressPort string) (err error) {
	listen, err := reuse.Listen("tcp", addressPort)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Server %s: Failed to listen: %s", s.listeners[0].Addr().String(), err.Error()))
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
