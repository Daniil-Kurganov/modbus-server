package modbusserver

import (
	"io"
	"log"
	"net"
	"strings"
)

func (s *Server) acceptRTUOverTCP(listen net.Listener) error {
	for {
		conn, err := listen.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			log.Printf("Unable to accept connections: %#v\n", err)
			return err
		}
		log.Print("Current connection successfully done")
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
				log.Printf("Current frame: %v", frame)
				conn.Write([]byte{1, 0, 1})
				return
				request := &Request{conn, frame}
				s.requestChan <- request
			}
		}(conn)
	}
}

func (s *Server) ListenRTUOverTCP(addressPort string) (err error) {
	listen, err := net.Listen("tcp", addressPort)
	if err != nil {
		log.Printf("Failed to Listen: %v\n", err)
		return err
	}
	s.listeners = append(s.listeners, listen)
	log.Print("Start listening")
	go s.acceptRTUOverTCP(listen)
	return err
}
