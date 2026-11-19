package forecast

import "errors"

var (
	ErrMismatchedSeriesLength = errors.New("actual and predicted series must have the same length")
	ErrInvalidSeriesValue     = errors.New("series values must be finite and non-negative")
)
