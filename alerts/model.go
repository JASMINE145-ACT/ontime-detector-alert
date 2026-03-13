package alerts

import "time"

type Direction string

const (
	DirectionAbove Direction = "above"
	DirectionBelow Direction = "below"
)

type Alert struct {
	ID              string
	Symbol          string
	Direction       Direction
	Threshold       float64
	UserID          string
	Active          bool
	TriggeredAt     *time.Time
	CooldownSeconds int
	LastNotifiedAt  *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

