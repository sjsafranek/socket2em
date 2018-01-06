package socket2em

type Response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

type Message struct {
	Method string      `json:"method"`
	Data   interface{} `json:"data"`
}
