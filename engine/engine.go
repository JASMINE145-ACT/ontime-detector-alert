package engine

import (
	"time"

	"ontime-detector-alert/alerts"
)

func CheckAlert(a alerts.Alert, price float64, now time.Time) bool {
	switch a.Direction {
	case alerts.DirectionAbove:
		if price < a.Threshold {
			return false
		}
	case alerts.DirectionBelow:
		if price > a.Threshold {
			return false
		}
	default:
		return false
	}

	if a.CooldownSeconds > 0 && a.LastNotifiedAt != nil {
		if now.Sub(*a.LastNotifiedAt) < time.Duration(a.CooldownSeconds)*time.Second {
			return false
		}
	}
	return true
}

func EvaluateAlerts(alertsList []alerts.Alert, prices map[string]float64, now time.Time) []alerts.Alert {
	var triggered []alerts.Alert
	for _, a := range alertsList {
		price, ok := prices[a.Symbol]
		if !ok {
			continue
		}
		if CheckAlert(a, price, now) {
			triggered = append(triggered, a)
		}
	}
	return triggered
}

