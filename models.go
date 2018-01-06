package socket2em

type TcpData struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

type TcpMessage struct {
	Method string      `json:"method"`
	Data   interface{} `json:"data"`
}
