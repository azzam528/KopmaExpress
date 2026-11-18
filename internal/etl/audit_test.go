package etl_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"smartcoop/internal/etl"
	mongorepo "smartcoop/internal/repository/mongo"
)

func TestImportPipelineIntegration(t *testing.T) {
	_ = godotenv.Load("../../.env")
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongodriver.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("Connect err: %v", err)
	}
	defer client.Disconnect(ctx)

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "smartcoop"
	}
	db := client.Database(dbName)

	productRepo := mongorepo.NewProductRepo(db)
	historyRepo := mongorepo.NewHistoryRepo(db)
	importJobRepo := mongorepo.NewImportJobRepo(db)

	svc := etl.NewService(productRepo, historyRepo, importJobRepo)

	path := filepath.Join("..", "..", "testdata", "data_koperasi_lama_dummy.xlsx")
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile err: %v", err)
	}

	job, err := svc.StartImport(ctx, "test_data_koperasi_dummy.xlsx", fileBytes, "replace")
	if err != nil {
		t.Fatalf("StartImport err: %v", err)
	}

	t.Logf("Job started: %s", job.ID.Hex())

	// Poll until job completes or fails
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)
		curJob, err := svc.GetJob(ctx, job.ID)
		if err != nil {
			t.Fatalf("GetJob err: %v", err)
		}
		t.Logf("Job status: %s (Processed: %d, Inserted: %d, Updated: %d, Skipped: %d, Failed: %d)",
			curJob.Status, curJob.ProcessedRows, curJob.Inserted, curJob.Updated, curJob.Skipped, curJob.Failed)
		if curJob.Status == "DONE" {
			t.Logf("SUCCESS! Total rows: %d, Inserted: %d, Updated: %d", curJob.TotalRows, curJob.Inserted, curJob.Updated)
			return
		}
		if curJob.Status == "FAILED" {
			t.Fatalf("Job FAILED! Errors: %+v", curJob.Errors)
		}
	}
	t.Fatalf("Timed out waiting for job to complete")
}
