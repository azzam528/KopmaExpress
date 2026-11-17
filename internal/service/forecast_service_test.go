package service

import (
	"testing"

	"smartcoop/internal/domain"
)

func TestAssignStatusPriorityAndRestockQty(t *testing.T) {
	results := []domain.ForecastResult{
		{ProductCode: "A", CurrentStock: 0, PredictedDemand: 200, DataPoints: 2},
		{ProductCode: "B", CurrentStock: 10, PredictedDemand: 40, DataPoints: 6},
		{ProductCode: "C", CurrentStock: 1000, PredictedDemand: 10, DataPoints: 6},
		{ProductCode: "D", CurrentStock: 70, PredictedDemand: 50, DataPoints: 6},
	}

	assignStatus(results, 1)

	if results[0].Status != domain.StatusInsufficientData {
		t.Errorf("A status = %s, want INSUFFICIENT_DATA", results[0].Status)
	}
	if results[1].Status != domain.StatusCritical || results[1].RestockQty != 30 {
		t.Errorf("B status/restock = %s/%d, want CRITICAL/30", results[1].Status, results[1].RestockQty)
	}
	if results[2].Status != domain.StatusOverstock {
		t.Errorf("C status = %s, want OVERSTOCK_WARNING", results[2].Status)
	}
	if results[3].Status != domain.StatusSafe || results[3].RestockQty != 0 {
		t.Errorf("D status/restock = %s/%d, want SAFE/0", results[3].Status, results[3].RestockQty)
	}
}

func TestAssignStatusCriticalHasPriorityOverOverstock(t *testing.T) {
	results := []domain.ForecastResult{
		{ProductCode: "A", CurrentStock: 5, PredictedDemand: 30, DataPoints: 6},
		{ProductCode: "B", CurrentStock: 500, PredictedDemand: 100, DataPoints: 6},
	}
	assignStatus(results, 1)
	if results[0].Status != domain.StatusCritical || results[0].RestockQty != 25 {
		t.Fatalf("critical product got %+v", results[0])
	}
}
