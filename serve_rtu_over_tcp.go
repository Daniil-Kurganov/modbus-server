package modbusserver

import (
	"bufio"
	"io"
	"log"
	"net"
	"slices"
	"strings"

	reuse "github.com/libp2p/go-reuseport"
)

func (s *Server) acceptRTUOverTCP(listen net.Listener) error {
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
			if s.ConnectionChanel != nil {
				s.ConnectionChanel <- true
			}
			isFirstClient = false
		}
		reader := bufio.NewReader(conn)
		for {
			packet := make([]byte, 512)
			bytesRead, err := reader.Read(packet)
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					break
				} else if err != io.EOF {
					log.Printf("read error %v\n", err)
				}
				continue
			}
			packet = packet[:bytesRead]
			frame, err := NewRTUFrame(packet)
			if err != nil {
				log.Printf("bad packet error %v\n", err)
				continue
			}
			slaveID := frame.GetSlaveId()
			if _, ok := s.Slaves[slaveID]; ok && !slices.Contains(s.SlavesStoppedResponse, slaveID) {
				request := &Request{conn, frame}
				s.requestChan <- request
			} else {
				log.Print("invalid slave Id: requested slave Id doesn't initialized or disabled")
			}
		}
		conn.Close()
	}
}

func (s *Server) ListenRTUOverTCP(addressPort string) (err error) {
	listen, err := reuse.Listen("tcp", addressPort)
	if err != nil {
		log.Printf("Failed to Listen: %v\n", err)
		return err
	}
	s.listeners = append(s.listeners, listen)
	go s.acceptRTUOverTCP(listen)
	return err
}
