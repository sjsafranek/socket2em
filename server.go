package socket2em

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/textproto"
	"strings"
	"sync"
)

type Server struct {
	Host           string
	Port           int
	ConnType       string
	NumClients     int
	LoggingHandler func(string)
	OnDisconnect   func(string)
	MethodHandlers map[string]func(Message, net.Conn)
	Clients        map[int]net.Conn
	guard          sync.RWMutex
}

func (self *Server) RegisterMethod(method string, function func(Message, net.Conn)) error {
	self.init()
	if "help" == method {
		return fmt.Errorf("Method not allowed")
	}
	self.MethodHandlers[method] = function
	return nil
}

func (self *Server) Log(message ...string) {
	msg := strings.Join(message, " ")
	msg = "[TCP] " + msg
	if nil == self.LoggingHandler {
		log.Println(msg)
		return
	}
	self.LoggingHandler(msg)
}

func (self *Server) getHost() string {
	if self.Host == "" {
		self.Host = TCP_DEFAULT_CONN_HOST
		return TCP_DEFAULT_CONN_HOST
	}
	return self.Host
}

func (self *Server) getPort() int {
	if self.Port == 0 {
		self.Port = TCP_DEFAULT_CONN_PORT
		return TCP_DEFAULT_CONN_PORT
	}
	return self.Port
}

func (self *Server) getConnType() string {
	if self.ConnType == "" {
		self.ConnType = TCP_DEFAULT_CONN_TYPE
		return TCP_DEFAULT_CONN_TYPE
	}
	return self.ConnType
}

func (self *Server) init() {
	if nil == self.MethodHandlers {
		self.MethodHandlers = make(map[string]func(Message, net.Conn))
	}
	if nil == self.Clients {
		self.Clients = make(map[int]net.Conn)
	}
}

func (self *Server) Start() {

	self.init()

	counter := 0

	self.NumClients = 0

	// Check settings and apply defaults
	serv := fmt.Sprintf("%v:%v", self.getHost(), self.getPort())

	// Listen for incoming connections.
	l, err := net.Listen(self.getConnType(), serv)
	if err != nil {
		self.Log("Error listening:", err.Error())
		panic(err)
	}
	self.Log("Tcp Listening on " + serv)

	// Close the listener when the application closes.
	defer l.Close()
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			self.Log("Error accepting connection: ", err.Error())
			return
		}

		self.Log(conn.RemoteAddr().String(), "Connection open")

		self.guard.Lock()
		counter++
		self.Clients[counter] = conn
		self.guard.Unlock()

		// Handle connections in a new goroutine.
		go self.tcpClientHandler(conn, counter)

	}
}

func (self *Server) GetNumClients() int {
	return self.NumClients
}

// close tcp client
func (self *Server) closeClient(conn net.Conn, idx int) {
	self.NumClients--
	conn.Close()
	self.guard.Lock()
	delete(self.Clients, idx)
	self.guard.Unlock()
	if nil != self.OnDisconnect {
		self.OnDisconnect(conn.RemoteAddr().String())
	}
}

// Handles incoming requests.
func (self *Server) tcpClientHandler(conn net.Conn, idx int) {

	self.NumClients++
	defer self.closeClient(conn, idx)

	reader := bufio.NewReader(conn)
	tp := textproto.NewReader(reader)

	for {
		// will listen for message to process ending in newline (\n)
		message, err := tp.ReadLine()
		if io.EOF == err {
			self.Log(conn.RemoteAddr().String(), "Connection closed")
			return
		}

		// No message was sent
		if "" == message {
			continue
		}

		// Command
		exitFlag := false
		switch {
		case strings.HasPrefix(message, "help"):
			response := self.Help()
			self.HandleSuccess(response, conn)
			continue
		case strings.HasPrefix(message, "quit"):
			fallthrough
		case strings.HasPrefix(message, "bye"):
			fallthrough
		case strings.HasPrefix(message, "exit"):
			exitFlag = true
		}
		if exitFlag {
			break
		}
		//.end

		self.Log(conn.RemoteAddr().String(), "Message Received:", string([]byte(message)))

		// json parse message
		req := Message{}
		err = json.Unmarshal([]byte(message), &req)
		if err != nil {
			// invalid message
			// close connection
			// '\x04' end of transmittion character
			self.Log(conn.RemoteAddr().String(), err.Error())
			resp := `{"status": "error", "error": "` + fmt.Sprintf("%v", err) + `",""}`
			conn.Write([]byte(resp + "\n"))
			continue
		}

		switch {
		case req.Method == "":
			// No method provided
			continue
		case req.Method == "help":
			// {"method": "help"}
			response := self.Help()
			self.HandleSuccess(response, conn)
		default:
			// Run registered method
			if function, ok := self.MethodHandlers[req.Method]; ok {
				function(req, conn)
			} else {
				err := errors.New("Method not found")
				self.HandleError(err, conn)
			}
		}
	}
}

func (self *Server) Help() string {
	methods := []string{"help"}
	for i := range self.MethodHandlers {
		methods = append(methods, i)
	}
	response := fmt.Sprintf(`{"methods":["%v"]}`, strings.Join(methods, `", "`))
	return response
}

func (self Server) HandleError(err error, conn net.Conn) {
	conn.Write([]byte("{\"status\": \"error\", \"error\": \"" + err.Error() + "\"}\n"))
}

func (self Server) HandleSuccess(data string, conn net.Conn) {
	conn.Write([]byte("{\"status\": \"ok\", \"data\": " + data + "}\n"))
}

func (self Server) missingParams(conn net.Conn) {
	err := errors.New("Missing required parameters")
	self.HandleError(err, conn)
}

func (self Server) SendResponseFromStruct(data interface{}, conn net.Conn) {
	js, err := json.Marshal(data)
	if err != nil {
		self.HandleError(err, conn)
		return
	}
	self.HandleSuccess(string(js), conn)
}

func (self *Server) Broadcast(message string) {
	for _, conn := range self.Clients {
		conn.Write([]byte(fmt.Sprintf("%v\n", message)))
	}
}
