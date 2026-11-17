package forecast

import "time"

// MonthKey identifies one calendar month.
type MonthKey struct {
	Year  int
	Month int
}

func (m MonthKey) Time() time.Time {
	return time.Date(m.Year, time.Month(m.Month), 1, 0, 0, 0, 0, time.UTC)
}

// BuildSeries creates a continuous monthly series from the earliest historical
// month through the previous month. Missing months become zero. When the
// current day is at least 4, the current month is appended as a run-rate
// projection. Only the latest maxMonths values are returned.
func BuildSeries(sales map[MonthKey]int64, now time.Time, maxMonths int) []float64 {
	if len(sales) == 0 {
		return nil
	}
	if maxMonths <= 0 {
		maxMonths = 12
	}

	currentStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	first := currentStart
	validSales := make(map[MonthKey]int64, len(sales))

	for key, quantity := range sales {
		if key.Year < 1 || key.Month < 1 || key.Month > 12 || quantity < 0 {
			continue
		}
		month := key.Time()
		if month.After(currentStart) {
			continue // ignore future-dated records
		}
		validSales[key] = quantity
		if month.Before(first) {
			first = month
		}
	}
	if len(validSales) == 0 {
		return nil
	}

	// No more than maxMonths values can survive the final window. Starting
	// earlier would only generate months that are immediately discarded.
	windowStart := currentStart.AddDate(0, -maxMonths, 0)
	if first.Before(windowStart) {
		first = windowStart
	}

	series := make([]float64, 0, maxMonths+1)
	for month := first; month.Before(currentStart); month = month.AddDate(0, 1, 0) {
		key := MonthKey{Year: month.Year(), Month: int(month.Month())}
		series = append(series, float64(validSales[key]))
	}

	if now.Day() >= 4 {
		currentKey := MonthKey{Year: now.Year(), Month: int(now.Month())}
		quantitySoFar := float64(validSales[currentKey])
		daysInMonth := float64(currentStart.AddDate(0, 1, -1).Day())
		series = append(series, quantitySoFar*daysInMonth/float64(now.Day()))
	}

	if len(series) > maxMonths {
		series = series[len(series)-maxMonths:]
	}
	return series
}
