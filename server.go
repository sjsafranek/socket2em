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
)

type TcpServer struct {
	Host             string
	Port             int
	ConnType         string
	ActiveTcpClients int
	LoggingHandler   func(string)
	MethodHandlers   map[string]func(TcpMessage, net.Conn)
}

func (self *TcpServer) RegisterMethod(method string, function func(TcpMessage, net.Conn)) error {
	if "help" == method {
		return fmt.Errorf("Method not allowed")
	}
	self.MethodHandlers[method] = function
	return nil
}

func (self *TcpServer) Log(message ...string) {
	msg := strings.Join(message, " ")
	msg = "[TCP] " + msg
	if nil == self.LoggingHandler {
		log.Println(msg)
		return
	}
	self.LoggingHandler(msg)
}

func (self *TcpServer) getHost() string {
	if self.Host == "" {
		self.Host = TCP_DEFAULT_CONN_HOST
		return TCP_DEFAULT_CONN_HOST
	}
	return self.Host
}

func (self *TcpServer) getPort() int {
	if self.Port == 0 {
		self.Port = TCP_DEFAULT_CONN_PORT
		return TCP_DEFAULT_CONN_PORT
	}
	return self.Port
}

func (self *TcpServer) getConnType() string {
	if self.ConnType == "" {
		self.ConnType = TCP_DEFAULT_CONN_TYPE
		return TCP_DEFAULT_CONN_TYPE
	}
	return self.ConnType
}

func (self *TcpServer) Start() {

	self.MethodHandlers = make(map[string]func(TcpMessage, net.Conn))

	self.ActiveTcpClients = 0
	go func() {
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

			// check for local connection
			// if strings.Contains(conn.RemoteAddr().String(), "127.0.0.1") {
			// Handle connections in a new goroutine.
			go self.tcpClientHandler(conn)
			// } else {
			// 	// don't accept not local connections
			// 	conn.Close()
			// }

		}
	}()
}

func (self *TcpServer) GetNumClients() int {
	return self.ActiveTcpClients
}

// close tcp client
func (self *TcpServer) closeClient(conn net.Conn) {
	self.ActiveTcpClients--
	conn.Close()
}

// Handles incoming requests.
func (self *TcpServer) tcpClientHandler(conn net.Conn) {

	self.ActiveTcpClients++
	defer self.closeClient(conn)

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

		self.Log(conn.RemoteAddr().String(), "Message Received:", string([]byte(message)))

		// json parse message
		req := TcpMessage{}
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
			methods := []string{"help"}
			for i := range self.MethodHandlers {
				methods = append(methods, i)
			}
			response := fmt.Sprintf(`["%v"]`, strings.Join(methods, `", "`))
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

func (self TcpServer) HandleError(err error, conn net.Conn) {
	conn.Write([]byte("{\"status\": \"error\", \"error\": \"" + err.Error() + "\"}\n"))
}

func (self TcpServer) HandleSuccess(data string, conn net.Conn) {
	conn.Write([]byte("{\"status\": \"ok\", \"data\": " + data + "}\n"))
}

func (self TcpServer) missingParams(conn net.Conn) {
	err := errors.New("Missing required parameters")
	self.HandleError(err, conn)
}

func (self TcpServer) SendResponseFromStruct(data interface{}, conn net.Conn) {
	js, err := json.Marshal(data)
	if err != nil {
		self.HandleError(err, conn)
		return
	}
	self.HandleSuccess(string(js), conn)
}
