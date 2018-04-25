package socket2em

import (
	"encoding/json"
)

type Response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

type Message struct {
	Method string      `json:"method"`
	// Data   interface{} `json:"data"`
	Data json.RawMessage `json:data`
}
