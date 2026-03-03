package model

type ConvMessage struct {
	Role    string `json:"role"`    // user, assistant
	Content string `json:"content"`
	Time    string `json:"time"`
}
