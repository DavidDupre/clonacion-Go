package resolution

import "time"

// Resolution represents a DIAN resolution for invoice numbering ranges.
type Resolution struct {
	ResolutionNumber string    `json:"resolutionNumber"`
	ResolutionDate   time.Time `json:"resolutionDate"`
	Prefix           string    `json:"prefix"`
	FromNumber       int64     `json:"fromNumber"`
	ToNumber         int64     `json:"toNumber"`
	ValidDateFrom    time.Time `json:"validDateFrom"`
	ValidDateTo      time.Time `json:"validDateTo"`
}
