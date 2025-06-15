package models

import "time"

type ClickhouseEvent struct {
	ID          int       `json:"Id"`
	ProjectID   int       `json:"ProjectId"`
	Name        string    `json:"Name"`
	Description string    `json:"Description"`
	Priority    int       `json:"Priority"`
	Removed     bool      `json:"Removed"`
	EventTime   time.Time `json:"EventTime"`
}

func NewClickhouseEvent(good *Good) *ClickhouseEvent {
	return &ClickhouseEvent{
		ID:          good.ID,
		ProjectID:   good.ProjectID,
		Name:        good.Name,
		Description: good.Description,
		Priority:    good.Priority,
		Removed:     good.Removed,
		EventTime:   time.Now(),
	}
}
