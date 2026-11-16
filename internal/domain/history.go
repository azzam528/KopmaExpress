package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MonthlySalesHistory represents monthly sales aggregation per product (§4.2)
type MonthlySalesHistory struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProductCode string             `bson:"product_code"  json:"kode_barang"`
	Year        int                `bson:"year"          json:"tahun"`
	Month       int                `bson:"month"         json:"bulan"` // 1-12
	QtySold     int64              `bson:"qty_sold"      json:"qty_terjual"`
	Revenue     int64              `bson:"revenue"       json:"omzet"`
	Source      string             `bson:"source"        json:"sumber"` // excel | live | mixed
	UpdatedAt   time.Time          `bson:"updated_at"    json:"updated_at"`
}

type TopSellingProduct struct {
	Rank        int    `json:"rank"`
	ProductCode string `json:"kode_barang"`
	ProductName string `json:"nama_barang"`
	Category    string `json:"kategori"`
	TotalQty    int64  `json:"total_qty"`
	TotalOmzet  int64  `json:"total_omzet"`
}

type TopStockProduct struct {
	Rank        int    `json:"rank"`
	ProductCode string `json:"kode_barang"`
	ProductName string `json:"nama_barang"`
	Category    string `json:"kategori"`
	Stock       int64  `json:"stok"`
	Price       int64  `json:"harga"`
	ValuasiAset int64  `json:"valuasi_aset"`
}

type DashboardSummary struct {
	TotalProduk          int64 `json:"total_produk"`
	TotalAnggota         int64 `json:"total_anggota"`
	TotalPesananBulanIni int64 `json:"total_pesanan_bulan_ini"`
	OmzetBulanIni        int64 `json:"omzet_bulan_ini"`
	TotalValuasiGudang   int64 `json:"total_valuasi_gudang"`
	CriticalCount        int64 `json:"critical_count"`
}

type MonthlyTrendItem struct {
	Year    int   `json:"tahun"`
	Month   int   `json:"bulan"`
	QtySold int64 `json:"qty_terjual"`
	Revenue int64 `json:"omzet"`
}

type HistoryRepository interface {
	UpsertSales(ctx context.Context, item *MonthlySalesHistory) error
	BulkUpsert(ctx context.Context, records []MonthlySalesHistory) (inserted int64, updated int64, err error)
	FindByProduct(ctx context.Context, productCode string) ([]MonthlySalesHistory, error)
	GetTopSelling(ctx context.Context, fromYear, fromMonth, limit int) ([]TopSellingProduct, error)
	GetMonthlyTrend(ctx context.Context, productCode string) ([]MonthlyTrendItem, error)
}

type AnalyticsRepository interface {
	GetTopSelling(ctx context.Context, fromYear, fromMonth, toYear, toMonth, limit int) ([]TopSellingProduct, error)
	GetTopStock(ctx context.Context, limit int) ([]TopStockProduct, int64, error)
	GetSummary(ctx context.Context) (*DashboardSummary, error)
	GetMonthlyTrend(ctx context.Context, productCode string) ([]MonthlyTrendItem, error)
}

type AnalyticsService interface {
	GetTopSelling(ctx context.Context, period string, limit int) ([]TopSellingProduct, error)
	GetTopSellingRange(ctx context.Context, from, to string, limit int) ([]TopSellingProduct, error)
	GetTopStock(ctx context.Context, limit int) ([]TopStockProduct, int64, error)
	GetSummary(ctx context.Context) (*DashboardSummary, error)
	GetMonthlyTrend(ctx context.Context, productCode string) ([]MonthlyTrendItem, error)
}
