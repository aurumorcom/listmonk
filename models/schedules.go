package models

import "time"

// TimeBlock represents a single start/end time window on a given day (HH:MM format).
type TimeBlock struct {
	Start string `json:"start"` // e.g. "08:00"
	End   string `json:"end"`   // e.g. "17:00"
}

// DailyWindows maps day names (mon, tue, wed, thu, fri, sat, sun) to a slice of TimeBlock.
type DailyWindows map[string][]TimeBlock

// Schedule represents a reusable Apollo-style sequence sending schedule.
type Schedule struct {
	Base
	UUID               string    `db:"uuid" json:"uuid"`
	Name               string    `db:"name" json:"name"`
	Timezone           string    `db:"timezone" json:"timezone"`
	UseContactTimezone bool      `db:"use_contact_timezone" json:"use_contact_timezone"`
	SkipHolidays       bool      `db:"skip_holidays" json:"skip_holidays"`
	SendingWindows     JSON      `db:"sending_windows" json:"sending_windows"`
	IsDefault          bool      `db:"is_default" json:"is_default"`
	CreatedAt          time.Time `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time `db:"updated_at" json:"updated_at"`
}

// Schedules represents a slice of Schedule.
type Schedules []Schedule
