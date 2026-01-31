package health

import "time"

// Status captures the state of the service at a moment in time.
type Status struct {
	Service      string    `json:"service"`
	Version      string    `json:"version"`
	Environment  string    `json:"environment"`
	Status       string    `json:"status"`
	StartedAt    time.Time `json:"startedAt"`
	Uptime       string    `json:"uptime"`
	UptimeSecs   int64     `json:"uptimeSeconds"`
	Dependencies []string  `json:"dependencies,omitempty"`
}
