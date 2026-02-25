package model

type StepStatus int

const (
	StepPending StepStatus = iota
	StepWIP
	StepDone
)

type PlanStep struct {
	Num    int        `json:"num"`
	Text   string     `json:"text"`
	Status StepStatus `json:"status"`
}

type Plan struct {
	Title string     `json:"title"`
	Steps []PlanStep `json:"steps"`
}
