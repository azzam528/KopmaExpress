package mongo

import (
	"context"
	"fmt"
	"sort"
	"time"

	"smartcoop/internal/domain"
	"smartcoop/internal/timeutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AnalyticsRepo implements the descriptive analytics queries required by PRD §3.4.
type AnalyticsRepo struct {
	db *mongo.Database
}

func NewAnalyticsRepo(db *mongo.Database) *AnalyticsRepo {
	return &AnalyticsRepo{db: db}
}

func (r *AnalyticsRepo) GetTopSelling(
	ctx context.Context,
	fromYear, fromMonth, toYear, toMonth, limit int,
) ([]domain.TopSellingProduct, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	match := monthRangeMatch(fromYear, fromMonth, toYear, toMonth)
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: match}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$product_code"},
			{Key: "total_qty", Value: bson.D{{Key: "$sum", Value: "$qty_sold"}}},
			{Key: "total_omzet", Value: bson.D{{Key: "$sum", Value: "$revenue"}}},
		}}},
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "products"},
			{Key: "localField", Value: "_id"},
			{Key: "foreignField", Value: "kode_barang"},
			{Key: "as", Value: "product"},
		}}},
		bson.D{{Key: "$unwind", Value: bson.D{
			{Key: "path", Value: "$product"},
			{Key: "preserveNullAndEmptyArrays", Value: true},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "total_qty", Value: -1}, {Key: "_id", Value: 1}}}},
		bson.D{{Key: "$limit", Value: int64(limit)}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "product_code", Value: "$_id"},
			{Key: "product_name", Value: "$product.name"},
			{Key: "category", Value: "$product.category"},
			{Key: "total_qty", Value: 1},
			{Key: "total_omzet", Value: 1},
		}}},
	}

	cursor, err := r.db.Collection("monthly_sales_history").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate top selling products: %w", err)
	}
	return decodeTopSelling(ctx, cursor)
}

// salesTotal adalah akumulasi penjualan satu barang dari beberapa sumber.
type salesTotal struct {
	code, name string
	qty, omzet int64
}

// GetTopSellingByDate menghitung barang terlaris pada rentang [from, to) dengan
// menggabungkan pesanan COMPLETED (live) dan riwayat harian hasil import Excel.
// Pesanan CANCELLED tidak dihitung.
func (r *AnalyticsRepo) GetTopSellingByDate(
	ctx context.Context,
	from, to time.Time,
	limit int,
) ([]domain.TopSellingProduct, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	totals := make(map[string]*salesTotal)
	if err := r.addOrderTotals(ctx, from, to, totals); err != nil {
		return nil, err
	}
	if err := r.addDailyHistoryTotals(ctx, from, to, totals); err != nil {
		return nil, err
	}

	ranked := make([]*salesTotal, 0, len(totals))
	for _, t := range totals {
		ranked = append(ranked, t)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].qty != ranked[j].qty {
			return ranked[i].qty > ranked[j].qty
		}
		return ranked[i].code < ranked[j].code
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	codes := make([]string, 0, len(ranked))
	for _, t := range ranked {
		codes = append(codes, t.code)
	}
	products, err := r.productsByCode(ctx, codes)
	if err != nil {
		return nil, err
	}

	results := make([]domain.TopSellingProduct, 0, len(ranked))
	for i, t := range ranked {
		item := domain.TopSellingProduct{
			Rank:        i + 1,
			ProductCode: t.code,
			ProductName: t.name,
			TotalQty:    t.qty,
			TotalOmzet:  t.omzet,
		}
		if p, ok := products[t.code]; ok {
			if p.Name != "" {
				item.ProductName = p.Name
			}
			item.Category = p.Category
		}
		results = append(results, item)
	}
	return results, nil
}

func (r *AnalyticsRepo) addOrderTotals(ctx context.Context, from, to time.Time, totals map[string]*salesTotal) error {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{
			"status":     domain.OrderCompleted,
			"created_at": bson.M{"$gte": from, "$lt": to},
		}}},
		bson.D{{Key: "$unwind", Value: "$items"}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$items.product_code"},
			{Key: "name", Value: bson.D{{Key: "$first", Value: "$items.product_name"}}},
			{Key: "total_qty", Value: bson.D{{Key: "$sum", Value: "$items.qty"}}},
			{Key: "total_omzet", Value: bson.D{{Key: "$sum", Value: "$items.subtotal"}}},
		}}},
	}
	cursor, err := r.db.Collection("orders").Aggregate(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("aggregate order sales by date: %w", err)
	}
	defer cursor.Close(ctx)

	var rows []struct {
		Code  string `bson:"_id"`
		Name  string `bson:"name"`
		Qty   int64  `bson:"total_qty"`
		Omzet int64  `bson:"total_omzet"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return fmt.Errorf("decode order sales by date: %w", err)
	}
	for _, row := range rows {
		addSalesTotal(totals, row.Code, row.Name, row.Qty, row.Omzet)
	}
	return nil
}

func (r *AnalyticsRepo) addDailyHistoryTotals(ctx context.Context, from, to time.Time, totals map[string]*salesTotal) error {
	loc := timeutil.Location()
	const layout = "2006-01-02"
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{
			"date": bson.M{"$gte": from.In(loc).Format(layout), "$lt": to.In(loc).Format(layout)},
		}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$product_code"},
			{Key: "total_qty", Value: bson.D{{Key: "$sum", Value: "$qty_sold"}}},
			{Key: "total_omzet", Value: bson.D{{Key: "$sum", Value: "$revenue"}}},
		}}},
	}
	cursor, err := r.db.Collection("daily_sales_history").Aggregate(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("aggregate daily sales history: %w", err)
	}
	defer cursor.Close(ctx)

	var rows []struct {
		Code  string `bson:"_id"`
		Qty   int64  `bson:"total_qty"`
		Omzet int64  `bson:"total_omzet"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return fmt.Errorf("decode daily sales history: %w", err)
	}
	for _, row := range rows {
		addSalesTotal(totals, row.Code, "", row.Qty, row.Omzet)
	}
	return nil
}

func addSalesTotal(totals map[string]*salesTotal, code, name string, qty, omzet int64) {
	t, ok := totals[code]
	if !ok {
		t = &salesTotal{code: code}
		totals[code] = t
	}
	if t.name == "" {
		t.name = name
	}
	t.qty += qty
	t.omzet += omzet
}

func (r *AnalyticsRepo) productsByCode(ctx context.Context, codes []string) (map[string]domain.Product, error) {
	result := make(map[string]domain.Product, len(codes))
	if len(codes) == 0 {
		return result, nil
	}
	cursor, err := r.db.Collection("products").Find(ctx, bson.M{"kode_barang": bson.M{"$in": codes}})
	if err != nil {
		return nil, fmt.Errorf("find products for top selling: %w", err)
	}
	defer cursor.Close(ctx)

	var products []domain.Product
	if err := cursor.All(ctx, &products); err != nil {
		return nil, fmt.Errorf("decode products for top selling: %w", err)
	}
	for _, p := range products {
		result[p.Code] = p
	}
	return result, nil
}

func decodeTopSelling(ctx context.Context, cursor *mongo.Cursor) ([]domain.TopSellingProduct, error) {
	defer cursor.Close(ctx)

	var rows []struct {
		ProductCode string `bson:"product_code"`
		ProductName string `bson:"product_name"`
		Category    string `bson:"category"`
		TotalQty    int64  `bson:"total_qty"`
		TotalOmzet  int64  `bson:"total_omzet"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("decode top selling products: %w", err)
	}

	results := make([]domain.TopSellingProduct, 0, len(rows))
	for i, row := range rows {
		results = append(results, domain.TopSellingProduct{
			Rank:        i + 1,
			ProductCode: row.ProductCode,
			ProductName: row.ProductName,
			Category:    row.Category,
			TotalQty:    row.TotalQty,
			TotalOmzet:  row.TotalOmzet,
		})
	}
	return results, nil
}

func (r *AnalyticsRepo) GetTopStock(ctx context.Context, limit int) ([]domain.TopStockProduct, int64, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	findOptions := options.Find().
		SetSort(bson.D{{Key: "stock", Value: -1}, {Key: "kode_barang", Value: 1}}).
		SetLimit(int64(limit))
	cursor, err := r.db.Collection("products").Find(ctx, bson.M{"is_active": true}, findOptions)
	if err != nil {
		return nil, 0, fmt.Errorf("find top stock products: %w", err)
	}
	defer cursor.Close(ctx)

	var products []domain.Product
	if err := cursor.All(ctx, &products); err != nil {
		return nil, 0, fmt.Errorf("decode top stock products: %w", err)
	}

	results := make([]domain.TopStockProduct, 0, len(products))
	for i, product := range products {
		results = append(results, domain.TopStockProduct{
			Rank:        i + 1,
			ProductCode: product.Code,
			ProductName: product.Name,
			Category:    product.Category,
			Stock:       product.Stock,
			Price:       product.Price,
			ValuasiAset: product.Stock * product.Price,
		})
	}

	valuation, err := r.totalWarehouseValuation(ctx)
	if err != nil {
		return nil, 0, err
	}
	return results, valuation, nil
}

func (r *AnalyticsRepo) GetSummary(ctx context.Context) (*domain.DashboardSummary, error) {
	summary := &domain.DashboardSummary{}
	products := r.db.Collection("products")
	members := r.db.Collection("members")
	orders := r.db.Collection("orders")
	forecasts := r.db.Collection("forecast_results")

	var err error
	if summary.TotalProduk, err = products.CountDocuments(ctx, bson.M{"is_active": true}); err != nil {
		return nil, fmt.Errorf("count active products: %w", err)
	}
	if summary.TotalAnggota, err = members.CountDocuments(ctx, bson.M{"is_active": true}); err != nil {
		return nil, fmt.Errorf("count active members: %w", err)
	}

	now := timeutil.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	nextMonth := monthStart.AddDate(0, 1, 0)
	orderPipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{
			"status":     domain.OrderCompleted,
			"created_at": bson.M{"$gte": monthStart, "$lt": nextMonth},
		}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total_orders", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "total_revenue", Value: bson.D{{Key: "$sum", Value: "$total_amount"}}},
		}}},
	}
	orderCursor, err := orders.Aggregate(ctx, orderPipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate current-month orders: %w", err)
	}
	var orderTotals []struct {
		TotalOrders  int64 `bson:"total_orders"`
		TotalRevenue int64 `bson:"total_revenue"`
	}
	if err := orderCursor.All(ctx, &orderTotals); err != nil {
		_ = orderCursor.Close(ctx)
		return nil, fmt.Errorf("decode current-month order totals: %w", err)
	}
	_ = orderCursor.Close(ctx)
	if len(orderTotals) > 0 {
		summary.TotalPesananBulanIni = orderTotals[0].TotalOrders
		summary.OmzetBulanIni = orderTotals[0].TotalRevenue
	}

	if summary.TotalValuasiGudang, err = r.totalWarehouseValuation(ctx); err != nil {
		return nil, err
	}
	targetMonth := now.AddDate(0, 1, 0).Format("2006-01")
	if summary.CriticalCount, err = forecasts.CountDocuments(ctx, bson.M{
		"target_month": targetMonth,
		"status":       domain.StatusCritical,
	}); err != nil {
		return nil, fmt.Errorf("count critical forecasts: %w", err)
	}
	return summary, nil
}

func (r *AnalyticsRepo) GetMonthlyTrend(ctx context.Context, productCode string) ([]domain.MonthlyTrendItem, error) {
	findOptions := options.Find().SetSort(bson.D{{Key: "year", Value: 1}, {Key: "month", Value: 1}})
	cursor, err := r.db.Collection("monthly_sales_history").Find(ctx, bson.M{"product_code": productCode}, findOptions)
	if err != nil {
		return nil, fmt.Errorf("find monthly trend for %s: %w", productCode, err)
	}
	defer cursor.Close(ctx)

	var histories []domain.MonthlySalesHistory
	if err := cursor.All(ctx, &histories); err != nil {
		return nil, fmt.Errorf("decode monthly trend for %s: %w", productCode, err)
	}
	if len(histories) == 0 {
		return []domain.MonthlyTrendItem{}, nil
	}

	byMonth := make(map[int]domain.MonthlyTrendItem, len(histories))
	first := time.Date(histories[0].Year, time.Month(histories[0].Month), 1, 0, 0, 0, 0, time.UTC)
	last := first
	for _, history := range histories {
		month := time.Date(history.Year, time.Month(history.Month), 1, 0, 0, 0, 0, time.UTC)
		if month.Before(first) {
			first = month
		}
		if month.After(last) {
			last = month
		}
		key := history.Year*100 + history.Month
		byMonth[key] = domain.MonthlyTrendItem{
			Year:    history.Year,
			Month:   history.Month,
			QtySold: history.QtySold,
			Revenue: history.Revenue,
		}
	}

	results := make([]domain.MonthlyTrendItem, 0, int((last.Year()-first.Year())*12)+int(last.Month()-first.Month())+1)
	for month := first; !month.After(last); month = month.AddDate(0, 1, 0) {
		key := month.Year()*100 + int(month.Month())
		item, exists := byMonth[key]
		if !exists {
			item = domain.MonthlyTrendItem{Year: month.Year(), Month: int(month.Month())}
		}
		results = append(results, item)
	}
	return results, nil
}

func (r *AnalyticsRepo) totalWarehouseValuation(ctx context.Context) (int64, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"is_active": true}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$multiply", Value: bson.A{"$stock", "$price"}}}}}},
		}}},
	}
	cursor, err := r.db.Collection("products").Aggregate(ctx, pipeline)
	if err != nil {
		return 0, fmt.Errorf("calculate warehouse valuation: %w", err)
	}
	defer cursor.Close(ctx)

	var rows []struct {
		Total int64 `bson:"total"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return 0, fmt.Errorf("decode warehouse valuation: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].Total, nil
}

func monthRangeMatch(fromYear, fromMonth, toYear, toMonth int) bson.M {
	conditions := make(bson.A, 0, 2)
	if fromYear > 0 && fromMonth > 0 {
		conditions = append(conditions, bson.M{"$or": bson.A{
			bson.M{"year": bson.M{"$gt": fromYear}},
			bson.M{"year": fromYear, "month": bson.M{"$gte": fromMonth}},
		}})
	}
	if toYear > 0 && toMonth > 0 {
		conditions = append(conditions, bson.M{"$or": bson.A{
			bson.M{"year": bson.M{"$lt": toYear}},
			bson.M{"year": toYear, "month": bson.M{"$lte": toMonth}},
		}})
	}
	if len(conditions) == 0 {
		return bson.M{}
	}
	if len(conditions) == 1 {
		return conditions[0].(bson.M)
	}
	return bson.M{"$and": conditions}
}
