package cards

import "time"

type Card struct {
	ID         string      `json:"id"`
	Source     string      `json:"source"`
	Type       string      `json:"type"`
	Priority   int         `json:"priority"`
	Icon       string      `json:"icon"`
	Title      string      `json:"title"`
	Subtitle   string      `json:"subtitle,omitempty"`
	Status     string      `json:"status,omitempty"`
	Color      string      `json:"color,omitempty"`
	Body       string      `json:"body,omitempty"`
	Metrics    []Metric    `json:"metrics,omitempty"`
	Items      []ListItem  `json:"items,omitempty"`
	AlertLevel string      `json:"alert_level,omitempty"`
	ExpiresAt  *time.Time  `json:"expires_at,omitempty"`
	TimeWindow *TimeWindow `json:"time_window,omitempty"`
	CreatedBy  string      `json:"created_by,omitempty"`
	CreatedVia string      `json:"created_via,omitempty"`
	Persistent bool        `json:"-"`
}

type Metric struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type ListItem struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

type TimeWindow struct {
	ActiveFrom  string `json:"active_from,omitempty"`
	ActiveUntil string `json:"active_until,omitempty"`
	AllDay      bool   `json:"all_day,omitempty"`
}
