package timeutil

import "time"

// Location is the cooperative's business timezone (WIB). The fixed-offset
// fallback keeps the application usable in minimal containers without tzdata.
func Location() *time.Location {
	location, err := time.LoadLocation("Asia/Jakarta")
	if err == nil {
		return location
	}
	return time.FixedZone("WIB", 7*60*60)
}

// Now returns the current time in the cooperative's business timezone.
func Now() time.Time {
	return time.Now().In(Location())
}
