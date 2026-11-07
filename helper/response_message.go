package helper

import (
	"encoding/json"
	"os"
)

type Message struct {
	Code int    `json:"code"`
	ID   string `json:"id"`
	EN   string `json:"en"`
}

type Messages map[string]Message

func GetResponseMessageJSON() (Messages, error) {
	byteValue, err := os.ReadFile("response_message.json")
	if err != nil {
		return nil, err
	}

	var messages Messages
	err = json.Unmarshal(byteValue, &messages)
	if err != nil {
		return nil, err
	}

	return messages, nil
}
