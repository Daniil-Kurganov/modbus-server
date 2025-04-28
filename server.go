// Package mbserver implments a Modbus server (slave).
package modbusserver

import (
	"fmt"
	"io"
	"log"
	"net"
	"slices"
	"sync"

	"github.com/goburrow/serial"
	"golang.org/x/exp/maps"
)

// Server is a Modbus slave with allocated memory for discrete inputs, coils, etc.
type (
	SlaveData struct {
		Coils            []byte
		DiscreteInputs   []byte
		HoldingRegisters []uint16
		InputRegisters   []uint16
	}
	// Request contains the connection and Modbus frame.
	Request struct {
		conn  io.ReadWriteCloser
		frame Framer
	}
	Server struct {
		// Debug enables more verbose messaging.
		Debug                 bool
		listeners             []net.Listener
		ports                 []serial.Port
		portsWG               sync.WaitGroup
		portsCloseChan        chan struct{}
		requestChan           chan *Request
		ConnectionChanel      chan *net.Conn
		function              [256](func(*Server, Framer) ([]byte, *Exception))
		Slaves                map[uint8]SlaveData
		SlavesStoppedResponse []uint8
	}
)

// NewServer creates a new Modbus server (slave).
func NewServer() *Server {
	s := &Server{}
	s.Slaves = make(map[uint8]SlaveData)

	// Add default functions.
	s.function[1] = ReadCoils
	s.function[2] = ReadDiscreteInputs
	s.function[3] = ReadHoldingRegisters
	s.function[4] = ReadInputRegisters
	s.function[5] = WriteSingleCoil
	s.function[6] = WriteHoldingRegister
	s.function[15] = WriteMultipleCoils
	s.function[16] = WriteHoldingRegisters

	s.requestChan = make(chan *Request)
	s.portsCloseChan = make(chan struct{})
	s.ConnectionChanel = make(chan *net.Conn)

	go s.handler()

	return s
}

// RegisterFunctionHandler override the default behavior for a given Modbus function.
func (s *Server) RegisterFunctionHandler(funcCode uint8, function func(*Server, Framer) ([]byte, *Exception)) {
	s.function[funcCode] = function
}

func (s *Server) handle(request *Request) Framer {
	var exception *Exception
	var data []byte

	response := request.frame.Copy()

	function := request.frame.GetFunction()
	if s.function[function] != nil {
		data, exception = s.function[function](s, request.frame)
		response.SetData(data)
	} else {
		exception = &IllegalFunction
	}

	if exception != &Success {
		response.SetException(exception)
	}
	log.Printf("Current response: %v", response)
	return response
}

// All requests are handled synchronously to prevent modbus memory corruption.
func (s *Server) handler() {
	for {
		request := <-s.requestChan
		log.Printf("Current request: %v", request)
		response := s.handle(request)
		if _, err := request.conn.Write(response.Bytes()); err != nil {
			log.Printf("Error on writting request: %s", err)
		}
	}
}

func (s *Server) InitSlave(id uint8) {
	if _, ok := s.Slaves[id]; ok {
		return
	}
	slave := SlaveData{}
	slave.AllocateMemory()
	s.Slaves[id] = slave
}

func (s *Server) SlaveStopResponse(id uint8) (err error) {
	if _, ok := s.Slaves[id]; !ok {
		err = fmt.Errorf("slave with %d ID didn't implemented on server (must be in %v)", id, maps.Keys(s.Slaves))
		return
	}
	if slices.Contains(s.SlavesStoppedResponse, id) {
		return
	}
	s.SlavesStoppedResponse = append(s.SlavesStoppedResponse, id)
	return
}

func (s *Server) SlaveStartResponse(id uint8) (err error) {
	if _, ok := s.Slaves[id]; !ok {
		err = fmt.Errorf("slave with %d ID didn't implemented on server (must be in %v)", id, maps.Keys(s.Slaves))
		return
	}
	if !slices.Contains(s.SlavesStoppedResponse, id) {
		return
	}
	removeIndex := slices.Index(s.SlavesStoppedResponse, id)
	s.SlavesStoppedResponse = append(s.SlavesStoppedResponse[:removeIndex], s.SlavesStoppedResponse[removeIndex+1:]...)
	return
}

// Close stops listening to TCP/IP ports and closes serial ports.
func (s *Server) Close() {
	for _, listen := range s.listeners {
		listen.Close()
	}
	// close(s.ConnectionChanel)
	close(s.portsCloseChan)
	s.portsWG.Wait()

	for _, port := range s.ports {
		port.Close()
	}
}

func (sD *SlaveData) AllocateMemory() {
	sD.DiscreteInputs = make([]byte, 65536)
	sD.Coils = make([]byte, 65536)
	sD.HoldingRegisters = make([]uint16, 65536)
	sD.InputRegisters = make([]uint16, 65536)
}
