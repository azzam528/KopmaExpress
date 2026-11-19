// Package forecast contains the pure forecasting algorithms used by KOPMA Express.
// It deliberately has no database, HTTP, or application-service dependencies.
package forecast

import "math"

// Result is the forecasting output for a single monthly sales series.
type Result struct {
	WMA        float64
	Linear     float64
	Slope      float64
	Intercept  float64
	R2         float64
	Predicted  int64
	DataPoints int
	Sufficient bool
}

// WeightedMovingAverage computes a linearly weighted moving average over the
// latest k observations. The newest observation receives weight k.
func WeightedMovingAverage(y []float64, k int) float64 {
	if len(y) == 0 || k <= 0 {
		return 0
	}
	if k > len(y) {
		k = len(y)
	}

	var weightedSum, weightSum float64
	start := len(y) - k
	for i := 0; i < k; i++ {
		weight := float64(i + 1)
		value := y[start+i]
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		weightedSum += weight * value
		weightSum += weight
	}
	if weightSum == 0 {
		return 0
	}
	return weightedSum / weightSum
}

// LinearRegression performs Ordinary Least Squares with x = 1..n.
// It returns the slope, intercept, and coefficient of determination (R²).
func LinearRegression(y []float64) (slope, intercept, r2 float64) {
	if len(y) < 2 {
		return 0, 0, 0
	}

	n := float64(len(y))
	var sumX, sumY, sumXY, sumXX float64
	for i, value := range y {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, 0, 0
		}
		x := float64(i + 1)
		sumX += x
		sumY += value
		sumXY += x * value
		sumXX += x * x
	}

	denominator := n*sumXX - sumX*sumX
	if denominator == 0 {
		return 0, sumY / n, 0
	}

	slope = (n*sumXY - sumX*sumY) / denominator
	intercept = (sumY - slope*sumX) / n

	mean := sumY / n
	var ssTotal, ssResidual float64
	for i, actual := range y {
		predicted := intercept + slope*float64(i+1)
		residual := actual - predicted
		deviation := actual - mean
		ssResidual += residual * residual
		ssTotal += deviation * deviation
	}

	// A constant series is perfectly represented by its mean.
	if ssTotal == 0 {
		r2 = 1
	} else {
		r2 = 1 - ssResidual/ssTotal
	}
	return slope, intercept, r2
}

func clamp(value, lower, upper float64) float64 {
	return math.Max(lower, math.Min(upper, value))
}

func maxOf(values []float64) float64 {
	maximum := values[0]
	for _, value := range values[1:] {
		if value > maximum {
			maximum = value
		}
	}
	return maximum
}

func mean(values []float64) float64 {
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

// Predict blends WMA and linear-regression forecasts. Fewer than three
// observations use a simple average and are marked as insufficient.
func Predict(y []float64) Result {
	result := Result{DataPoints: len(y)}
	if len(y) == 0 {
		return result
	}

	for _, value := range y {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return result
		}
	}

	if len(y) < 3 {
		average := mean(y)
		result.WMA = average
		result.Linear = average
		result.Predicted = int64(math.Ceil(average))
		return result
	}

	result.Sufficient = true
	result.WMA = WeightedMovingAverage(y, 3)
	result.Slope, result.Intercept, result.R2 = LinearRegression(y)

	nextValue := result.Intercept + result.Slope*float64(len(y)+1)
	result.Linear = clamp(nextValue, 0, 2*maxOf(y))

	regressionWeight := clamp(result.R2, 0.2, 0.7)
	wmaWeight := 1 - regressionWeight
	blended := wmaWeight*result.WMA + regressionWeight*result.Linear
	result.Predicted = int64(math.Ceil(math.Max(0, blended)))
	return result
}

// MAPE returns mean absolute percentage error in percent. Observations with
// an actual value of zero are excluded because percentage error is undefined.
// The second return value is the number of excluded zero-actual observations.
func MAPE(actual, predicted []float64) (mape float64, excluded int, err error) {
	if len(actual) != len(predicted) {
		return 0, 0, ErrMismatchedSeriesLength
	}

	var total float64
	var count int
	for i, value := range actual {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) || predicted[i] < 0 || math.IsNaN(predicted[i]) || math.IsInf(predicted[i], 0) {
			return 0, excluded, ErrInvalidSeriesValue
		}
		if value == 0 {
			excluded++
			continue
		}
		total += math.Abs(value-predicted[i]) / value
		count++
	}
	if count == 0 {
		return 0, excluded, nil
	}
	return total / float64(count) * 100, excluded, nil
}

// BacktestResult compares one-step-ahead MAPE for WMA, OLS regression, and
// the adaptive ensemble. The first evaluated point has three prior months.
type BacktestResult struct {
	WMA              float64
	Linear           float64
	Ensemble         float64
	Evaluated        int
	ExcludedZeroReal int
}

// Backtest performs rolling-origin evaluation: to predict observation i, only
// observations before i are used. Actual-zero months are excluded from MAPE.
func Backtest(y []float64) BacktestResult {
	result := BacktestResult{}
	var wmaError, linearError, ensembleError float64
	for _, value := range y {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return result
		}
	}

	for i := 3; i < len(y); i++ {
		actual := y[i]
		training := y[:i]
		if actual < 0 || math.IsNaN(actual) || math.IsInf(actual, 0) {
			continue
		}
		if actual == 0 {
			result.ExcludedZeroReal++
			continue
		}

		wmaPrediction := WeightedMovingAverage(training, 3)
		slope, intercept, _ := LinearRegression(training)
		linearPrediction := clamp(intercept+slope*float64(len(training)+1), 0, 2*maxOf(training))
		ensemblePrediction := float64(Predict(training).Predicted)

		wmaError += math.Abs(actual-wmaPrediction) / actual
		linearError += math.Abs(actual-linearPrediction) / actual
		ensembleError += math.Abs(actual-ensemblePrediction) / actual
		result.Evaluated++
	}

	if result.Evaluated > 0 {
		denominator := float64(result.Evaluated)
		result.WMA = wmaError / denominator * 100
		result.Linear = linearError / denominator * 100
		result.Ensemble = ensembleError / denominator * 100
	}
	return result
}
