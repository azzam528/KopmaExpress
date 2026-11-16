package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	ImportStatusPending = "PENDING"
	ImportStatusRunning = "RUNNING"
	ImportStatusDone    = "DONE"
	ImportStatusFailed  = "FAILED"
)

// RowError captures line-level validation/cleaning issues during ETL import (§4.2)
type RowError struct {
	Sheet  string `bson:"sheet"  json:"sheet"`
	Row    int    `bson:"row"    json:"row"`
	Reason string `bson:"reason" json:"reason"`
}

// ImportJob tracks asynchronous Excel migration job progress (§4.2)
type ImportJob struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"job_id"`
	FileName      string             `bson:"file_name"      json:"file_name"`
	Status        string             `bson:"status"         json:"status"` // PENDING|RUNNING|DONE|FAILED
	TotalRows     int64              `bson:"total_rows"     json:"total_rows"`
	ProcessedRows int64              `bson:"processed_rows" json:"processed_rows"`
	Inserted      int64              `bson:"inserted"       json:"inserted"`
	Updated       int64              `bson:"updated"        json:"updated"`
	Skipped       int64              `bson:"skipped"        json:"skipped"`
	Failed        int64              `bson:"failed"         json:"failed"`
	Errors        []RowError         `bson:"errors"         json:"errors"`
	StartedAt     time.Time          `bson:"started_at"     json:"started_at"`
	FinishedAt    *time.Time         `bson:"finished_at"    json:"finished_at,omitempty"`
}

type ImportJobRepository interface {
	Create(ctx context.Context, job *ImportJob) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*ImportJob, error)
	FindAll(ctx context.Context, page, limit int) ([]ImportJob, int64, error)
	UpdateProgress(ctx context.Context, id primitive.ObjectID, processed, inserted, updated, skipped, failed int64, errors []RowError) error
	MarkCompleted(ctx context.Context, id primitive.ObjectID, totalRows int64) error
	MarkFailed(ctx context.Context, id primitive.ObjectID, reason string) error
}

type ETLService interface {
	StartImport(ctx context.Context, fileName string, fileBytes []byte, stockMode string) (*ImportJob, error)
	GetJob(ctx context.Context, id primitive.ObjectID) (*ImportJob, error)
	ListJobs(ctx context.Context, page, limit int) ([]ImportJob, int64, error)
	GenerateDummyExcel(items, months int, dirty bool, seed int64) ([]byte, error)
}

