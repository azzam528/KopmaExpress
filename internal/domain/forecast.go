package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RestockStatus string

const (
	StatusCritical         RestockStatus = "CRITICAL"
	StatusOverstock        RestockStatus = "OVERSTOCK_WARNING"
	StatusSafe             RestockStatus = "SAFE"
	StatusInsufficientData RestockStatus = "INSUFFICIENT_DATA"
)

// ForecastResult represents smart demand forecasting output per product (§4.2)
type ForecastResult struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"    json:"id"`
	ProductCode     string             `bson:"product_code"     json:"kode_barang"`
	ProductName     string             `bson:"product_name"     json:"nama_barang"`
	TargetMonth     string             `bson:"target_month"     json:"bulan_target"` // e.g. "2026-12"
	CurrentStock    int64              `bson:"current_stock"    json:"stok_saat_ini"`
	WMA             float64            `bson:"wma"              json:"wma"`
	LinearTrend     float64            `bson:"linear_trend"     json:"regresi_linear"`
	Slope           float64            `bson:"slope"            json:"slope"`
	R2              float64            `bson:"r2"               json:"r_squared"`
	PredictedDemand int64              `bson:"predicted_demand" json:"prediksi_bulan_depan"`
	RestockQty      int64              `bson:"restock_qty"      json:"rekomendasi_restock"`
	Status          RestockStatus      `bson:"status"           json:"status"`
	DataPoints      int                `bson:"data_points"      json:"jumlah_data_bulan"`
	CalculatedAt    time.Time          `bson:"calculated_at"    json:"calculated_at"`
}

// ForecastDetail combines the latest forecast with the historical monthly
// observations used as its input, so the client can render the trend chart.
type ForecastDetail struct {
	ForecastResult
	History []MonthlySalesHistory `json:"riwayat_bulanan"`
}

type ForecastFilter struct {
	Status RestockStatus
	Query  string
	Page   int
	Limit  int
	Sort   string
}

type ProductSalesData struct {
	Product Product
	Sales   map[string]int64 // key: "YYYY-MM"
}

type ForecastRepository interface {
	UpsertForecasts(ctx context.Context, results []ForecastResult) error
	FindAll(ctx context.Context, filter ForecastFilter) ([]ForecastResult, int64, error)
	FindByProductCode(ctx context.Context, code string) (*ForecastResult, error)
	FindSalesHistory(ctx context.Context, code string) ([]MonthlySalesHistory, error)
	GetStatusSummary(ctx context.Context) (map[RestockStatus]int64, error)
	LoadProductSales(ctx context.Context) ([]ProductSalesData, error)
}

type ForecastService interface {
	Recalculate(ctx context.Context) error
	List(ctx context.Context, filter ForecastFilter) ([]ForecastResult, int64, error)
	GetByProductCode(ctx context.Context, code string) (*ForecastDetail, error)
	GetSummary(ctx context.Context) (map[RestockStatus]int64, error)
}
