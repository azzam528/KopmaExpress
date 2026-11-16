package mongo

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Connect initializes a MongoDB client with a 10s connection timeout and verifies ping.
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(connectCtx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()

	if err := client.Ping(pingCtx, readpref.PrimaryPreferred()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	log.Println("[MONGO] Successfully connected and pinged MongoDB replica set")
	return client, nil
}

// EnsureIndexes creates all required collection indexes as defined in PRD §4.1
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	log.Println("[MONGO] Ensuring database indexes...")

	// 1. Products Indexes
	// {kode_barang:1} UNIQUE; {stock:-1}; {category:1}; {is_active:1}
	productIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "kode_barang", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_products_kode_barang_unique"),
		},
		{
			Keys:    bson.D{{Key: "stock", Value: -1}},
			Options: options.Index().SetName("idx_products_stock_desc"),
		},
		{
			Keys:    bson.D{{Key: "category", Value: 1}},
			Options: options.Index().SetName("idx_products_category"),
		},
		{
			Keys:    bson.D{{Key: "is_active", Value: 1}},
			Options: options.Index().SetName("idx_products_is_active"),
		},
	}
	if _, err := db.Collection("products").Indexes().CreateMany(timeoutCtx, productIndexes); err != nil {
		return fmt.Errorf("error creating indexes for products: %w", err)
	}

	// 2. Members Indexes
	// {nim:1} UNIQUE; {name:1}
	memberIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "nim", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_members_nim_unique"),
		},
		{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index().SetName("idx_members_name"),
		},
	}
	if _, err := db.Collection("members").Indexes().CreateMany(timeoutCtx, memberIndexes); err != nil {
		return fmt.Errorf("error creating indexes for members: %w", err)
	}

	// 3. Orders Indexes
	// {order_no:1} UNIQUE; {member_nim:1, created_at:-1}; {created_at:-1}; {status:1}
	orderIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "order_no", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_orders_order_no_unique"),
		},
		{
			Keys:    bson.D{{Key: "member_nim", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("idx_orders_member_nim_created_at_desc"),
		},
		{
			Keys:    bson.D{{Key: "created_at", Value: -1}},
			Options: options.Index().SetName("idx_orders_created_at_desc"),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}},
			Options: options.Index().SetName("idx_orders_status"),
		},
	}
	if _, err := db.Collection("orders").Indexes().CreateMany(timeoutCtx, orderIndexes); err != nil {
		return fmt.Errorf("error creating indexes for orders: %w", err)
	}

	// 4. MonthlySalesHistory Indexes
	// {product_code:1, year:1, month:1} UNIQUE; {year:1, month:1}
	historyIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "product_code", Value: 1}, {Key: "year", Value: 1}, {Key: "month", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_history_product_year_month_unique"),
		},
		{
			Keys:    bson.D{{Key: "year", Value: 1}, {Key: "month", Value: 1}},
			Options: options.Index().SetName("idx_history_year_month"),
		},
	}
	if _, err := db.Collection("monthly_sales_history").Indexes().CreateMany(timeoutCtx, historyIndexes); err != nil {
		return fmt.Errorf("error creating indexes for monthly_sales_history: %w", err)
	}

	// 5. ForecastResults Indexes
	// {product_code:1, target_month:1} UNIQUE; {status:1}
	forecastIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "product_code", Value: 1}, {Key: "target_month", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_forecast_product_target_month_unique"),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}},
			Options: options.Index().SetName("idx_forecast_status"),
		},
	}
	if _, err := db.Collection("forecast_results").Indexes().CreateMany(timeoutCtx, forecastIndexes); err != nil {
		return fmt.Errorf("error creating indexes for forecast_results: %w", err)
	}

	// 6. ImportJobs Indexes
	// {created_at:-1}
	importJobIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "created_at", Value: -1}},
			Options: options.Index().SetName("idx_import_jobs_created_at_desc"),
		},
	}
	if _, err := db.Collection("import_jobs").Indexes().CreateMany(timeoutCtx, importJobIndexes); err != nil {
		return fmt.Errorf("error creating indexes for import_jobs: %w", err)
	}

	log.Println("[MONGO] All database indexes created/verified successfully")
	return nil
}

