package mongo

import (
	"context"
	"time"

	"smartcoop/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ImportJobRepo struct {
	collection *mongo.Collection
}

func NewImportJobRepo(db *mongo.Database) *ImportJobRepo {
	return &ImportJobRepo{
		collection: db.Collection("import_jobs"),
	}
}

func (r *ImportJobRepo) Create(ctx context.Context, job *domain.ImportJob) error {
	_, err := r.collection.InsertOne(ctx, job)
	return err
}

func (r *ImportJobRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*domain.ImportJob, error) {
	var job domain.ImportJob
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&job)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *ImportJobRepo) FindAll(ctx context.Context, page, limit int) ([]domain.ImportJob, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	skip := int64((page - 1) * limit)

	total, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSkip(skip).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "started_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var jobs []domain.ImportJob
	if err := cursor.All(ctx, &jobs); err != nil {
		return nil, 0, err
	}
	return jobs, total, nil
}

// UpdateProgress mencatat progres import yang sedang berjalan.
func (r *ImportJobRepo) UpdateProgress(
	ctx context.Context,
	id primitive.ObjectID,
	processed, inserted, updated, skipped, failed int64,
	errors []domain.RowError,
) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"processed_rows": processed,
			"inserted":       inserted,
			"updated":        updated,
			"skipped":        skipped,
			"failed":         failed,
			"errors":         errors,
		}},
	)
	return err
}

// MarkCompleted menandai job selesai dengan status DONE.
func (r *ImportJobRepo) MarkCompleted(ctx context.Context, id primitive.ObjectID, totalRows int64) error {
	now := time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"status":      domain.ImportStatusDone,
			"total_rows":  totalRows,
			"finished_at": now,
		}},
	)
	return err
}

// MarkFailed menandai job gagal dengan pesan error.
func (r *ImportJobRepo) MarkFailed(ctx context.Context, id primitive.ObjectID, reason string) error {
	now := time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"status":      domain.ImportStatusFailed,
			"finished_at": now,
			"errors": []domain.RowError{
				{Sheet: "SYSTEM", Row: 0, Reason: reason},
			},
		}},
	)
	return err
}

