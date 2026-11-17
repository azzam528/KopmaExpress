
package mongo

import (
	"context"
	"errors"
	"time"

	"smartcoop/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MemberRepo struct {
	collection *mongo.Collection
}

func NewMemberRepo(db *mongo.Database) *MemberRepo {
	return &MemberRepo{
		collection: db.Collection("members"),
	}
}

func (r *MemberRepo) FindByNIM(ctx context.Context, nim string) (*domain.Member, error) {
	var member domain.Member

	err := r.collection.FindOne(ctx, bson.M{
		"nim": nim,
	}).Decode(&member)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.NewNotFoundError("Anggota tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	return &member, nil
}

func (r *MemberRepo) FindByEmail(ctx context.Context, email string) (*domain.Member, error) {
	var member domain.Member

	err := r.collection.FindOne(ctx, bson.M{
		"email": email,
	}).Decode(&member)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.NewNotFoundError("Email tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	return &member, nil
}

func (r *MemberRepo) Create(ctx context.Context, member *domain.Member) error {
	result, err := r.collection.InsertOne(ctx, member)
	if err != nil {
		return err
	}

	if id, ok := result.InsertedID.(primitive.ObjectID); ok {
		member.ID = id
	}

	return nil
}

func (r *MemberRepo) FindAll(ctx context.Context, filter domain.MemberFilter) ([]domain.Member, int64, error) {
	query := bson.M{}

	if filter.Query != "" {
		query["$or"] = []bson.M{
			{"nim": bson.M{"$regex": filter.Query, "$options": "i"}},
			{"name": bson.M{"$regex": filter.Query, "$options": "i"}},
			{"email": bson.M{"$regex": filter.Query, "$options": "i"}},
		}
	}

	page := filter.Page
	limit := filter.Limit

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	total, err := r.collection.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	members := make([]domain.Member, 0)

	if err := cursor.All(ctx, &members); err != nil {
		return nil, 0, err
	}

	return members, total, nil
}

func (r *MemberRepo) Update(ctx context.Context, nim string, member *domain.Member) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"nim": nim},
		bson.M{
			"$set": bson.M{
				"name":  member.Name,
				"unit":  member.Unit,
				"phone": member.Phone,
			},
		},
	)

	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return domain.NewNotFoundError("Anggota tidak ditemukan")
	}

	return nil
}

func (r *MemberRepo) Deactivate(ctx context.Context, nim string) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"nim": nim},
		bson.M{
			"$set": bson.M{
				"is_active": false,
			},
		},
	)

	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return domain.NewNotFoundError("Anggota tidak ditemukan")
	}

	return nil
}

func (r *MemberRepo) BulkUpsert(ctx context.Context, members []domain.Member) (int64, int64, error) {
	if len(members) == 0 {
		return 0, 0, nil
	}

	models := make([]mongo.WriteModel, 0, len(members))

	for _, member := range members {
		model := mongo.NewUpdateOneModel().
			SetFilter(bson.M{"nim": member.NIM}).
			SetUpdate(bson.M{
				"$set": bson.M{
					"name":      member.Name,
					"unit":      member.Unit,
					"phone":     member.Phone,
					"is_active": member.IsActive,
					"joined_at": member.JoinedAt,
				},
				"$setOnInsert": bson.M{
					"nim":        member.NIM,
					"created_at": time.Now(),
				},
			}).
			SetUpsert(true)

		models = append(models, model)
	}

	result, err := r.collection.BulkWrite(ctx, models)
	if err != nil {
		return 0, 0, err
	}

	return result.UpsertedCount, result.ModifiedCount, nil
}
