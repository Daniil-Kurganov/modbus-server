package modbusserver

import (
	"io"
	"log"
	"net"
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
			log.Print("First client")
			if _, ok := <-s.ConnectionChanel; ok {
				log.Print("Channel is valid")
				s.ConnectionChanel <- &conn
			}
			log.Print("Channel isn't valid")
			isFirstClient = false
		}
		go func(conn net.Conn) {
			defer conn.Close()
			for {
				packet := make([]byte, 512)
				bytesRead, err := conn.Read(packet)
				log.Printf("Current bytes read: %d", bytesRead)
				if err != nil {
					if err != io.EOF {
						log.Printf("read error %v\n", err)
					}
					return
				}
				packet = packet[:bytesRead]
				frame, err := NewRTUFrame(packet)
				if err != nil {
					log.Printf("bad packet error %v\n", err)
					return
				}
				if _, ok := s.Slaves[frame.GetSlaveId()]; ok {
					request := &Request{conn, frame}
					s.requestChan <- request
				}
			}
		}(conn)
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
