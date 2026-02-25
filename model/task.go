package model

type Task struct {
	ID          string `json:"id"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Status      string `json:"status"`
	ActiveForm  string `json:"activeForm"`
}
