package forecast

import (
	"math"
	"reflect"
	"testing"
	"time"
)

func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func TestWeightedMovingAverage(t *testing.T) {
	got := WeightedMovingAverage([]float64{100, 110, 125, 130, 150, 160}, 3)
	if !almostEqual(got, 151.6666666667, 1e-8) {
		t.Fatalf("WMA = %.10f, want 151.6666666667", got)
	}
}

func TestWeightedMovingAverageBoundaries(t *testing.T) {
	cases := []struct {
		name string
		data []float64
		k    int
		want float64
	}{
		{name: "empty", data: nil, k: 3, want: 0},
		{name: "zero window", data: []float64{10, 20}, k: 0, want: 0},
		{name: "window larger than data", data: []float64{10, 20}, k: 3, want: 50.0 / 3.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WeightedMovingAverage(tc.data, tc.k); !almostEqual(got, tc.want, 1e-9) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLinearRegression(t *testing.T) {
	slope, intercept, r2 := LinearRegression([]float64{100, 110, 125, 130, 150, 160})
	if !almostEqual(slope, 12.1428571429, 1e-8) {
		t.Errorf("slope = %v", slope)
	}
	if !almostEqual(intercept, 86.6666666667, 1e-8) {
		t.Errorf("intercept = %v", intercept)
	}
	if r2 < 0.98 || r2 > 1 {
		t.Errorf("R² = %v, want a value close to 1", r2)
	}
}

func TestPredictInsufficientData(t *testing.T) {
	got := Predict([]float64{10, 20})
	if got.Sufficient {
		t.Fatal("series with two points must be insufficient")
	}
	if got.Predicted != 15 || got.DataPoints != 2 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestPredictPRDExample(t *testing.T) {
	got := Predict([]float64{100, 110, 125, 130, 150, 160})
	if !got.Sufficient {
		t.Fatal("expected sufficient data")
	}
	if got.Predicted != 166 {
		t.Fatalf("predicted = %d, want 166 (result: %+v)", got.Predicted, got)
	}
}

func TestPredictConstantSeries(t *testing.T) {
	got := Predict([]float64{10, 10, 10, 10})
	if got.Predicted != 10 || got.R2 != 1 {
		t.Fatalf("unexpected result for constant series: %+v", got)
	}
}

func TestPredictRejectsInvalidValues(t *testing.T) {
	got := Predict([]float64{10, -1, 12})
	if got.Predicted != 0 || got.Sufficient {
		t.Fatalf("invalid data should not produce a forecast: %+v", got)
	}
}

func TestBuildSeriesFillsMissingMonthsAndProjectsCurrentMonth(t *testing.T) {
	now := time.Date(2026, time.November, 16, 10, 0, 0, 0, time.FixedZone("WIB", 7*60*60))
	sales := map[MonthKey]int64{
		{Year: 2026, Month: 8}:  80,
		{Year: 2026, Month: 10}: 100,
		{Year: 2026, Month: 11}: 60,
	}
	got := BuildSeries(sales, now, 12)
	want := []float64{80, 0, 100, 60 * 30.0 / 16.0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("series = %#v, want %#v", got, want)
	}
}

func TestBuildSeriesSkipsCurrentMonthFirstThreeDays(t *testing.T) {
	now := time.Date(2026, time.November, 3, 12, 0, 0, 0, time.UTC)
	sales := map[MonthKey]int64{
		{Year: 2026, Month: 10}: 100,
		{Year: 2026, Month: 11}: 10,
	}
	got := BuildSeries(sales, now, 12)
	want := []float64{100}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("series = %#v, want %#v", got, want)
	}
}

func TestBuildSeriesKeepsLatestMonths(t *testing.T) {
	sales := map[MonthKey]int64{}
	for month := 1; month <= 8; month++ {
		sales[MonthKey{Year: 2026, Month: month}] = int64(month * 10)
	}
	now := time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC)
	got := BuildSeries(sales, now, 3)
	want := []float64{60, 70, 80}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("series = %#v, want %#v", got, want)
	}
}

func TestMAPEExcludesZeroActual(t *testing.T) {
	got, excluded, err := MAPE([]float64{100, 0, 200}, []float64{110, 20, 180})
	if err != nil {
		t.Fatal(err)
	}
	if excluded != 1 || !almostEqual(got, 10, 1e-9) {
		t.Fatalf("MAPE = %v, excluded = %d; want 10 and 1", got, excluded)
	}
}

func TestBacktestUsesOnlyPastObservations(t *testing.T) {
	got := Backtest([]float64{10, 12, 14, 16, 18, 20})
	if got.Evaluated != 3 {
		t.Fatalf("evaluated = %d, want 3", got.Evaluated)
	}
	if got.WMA <= 0 || got.Linear < 0 || got.Ensemble <= 0 {
		t.Fatalf("expected positive MAPE values: %+v", got)
	}
}

func TestBacktestExcludesZeroActualMonths(t *testing.T) {
	got := Backtest([]float64{10, 12, 14, 0, 18})
	if got.ExcludedZeroReal != 1 || got.Evaluated != 1 {
		t.Fatalf("unexpected backtest counts: %+v", got)
	}
}
