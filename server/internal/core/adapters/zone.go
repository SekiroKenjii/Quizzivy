package adapters

import "context"

// DefaultZone supplies the calendar zone until profile preferences replace it.
type DefaultZone struct{}

func (DefaultZone) ZoneOf(context.Context, string) (string, error) { return "Asia/Ho_Chi_Minh", nil }
