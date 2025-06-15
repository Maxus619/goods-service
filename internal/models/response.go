package models

type PriorityResponse struct {
	Priorities []PriorityItem `json:"priorities"`
}

type PriorityItem struct {
	ID        int `json:"id"`
	Priority  int `json:"priority"`
	ProjectID int `json:"-"` // Не включён в JSON, используется для логов в NATS
}
