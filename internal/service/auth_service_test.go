package service

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"

	"smartcoop/internal/domain"
)

// inMemoryUserRepo implements domain.UserRepository for unit testing.
type inMemoryUserRepo struct {
	mu    sync.RWMutex
	users map[string]*domain.User // keyed by NIM
}

func newInMemoryUserRepo() *inMemoryUserRepo {
	return &inMemoryUserRepo{
		users: make(map[string]*domain.User),
	}
}

func (r *inMemoryUserRepo) FindByNIM(_ context.Context, nim string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[strings.TrimSpace(nim)]
	if !ok {
		return nil, domain.NewNotFoundError("user")
	}
	cp := *u
	return &cp, nil
}

func (r *inMemoryUserRepo) FindByID(_ context.Context, id primitive.ObjectID) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if u.ID == id {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.NewNotFoundError("user")
}

func (r *inMemoryUserRepo) FindByMemberID(_ context.Context, memberID primitive.ObjectID) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if u.MemberID != nil && *u.MemberID == memberID {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.NewNotFoundError("user")
}

func (r *inMemoryUserRepo) Create(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.users[u.NIM]; exists {
		return domain.NewDuplicateError("user dengan NIM tersebut sudah ada")
	}
	if u.ID.IsZero() {
		u.ID = primitive.NewObjectID()
	}
	cp := *u
	r.users[u.NIM] = &cp
	return nil
}

func (r *inMemoryUserRepo) Update(_ context.Context, id primitive.ObjectID, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, existing := range r.users {
		if existing.ID == id {
			cp := *u
			cp.ID = id
			r.users[k] = &cp
			return nil
		}
	}
	return domain.NewNotFoundError("user")
}

func (r *inMemoryUserRepo) Count(_ context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return int64(len(r.users)), nil
}

// inMemoryMemberRepo implements domain.MemberRepository for unit testing.
type inMemoryMemberRepo struct {
	mu      sync.RWMutex
	members map[string]*domain.Member // keyed by NIM
}

func newInMemoryMemberRepo() *inMemoryMemberRepo {
	return &inMemoryMemberRepo{
		members: make(map[string]*domain.Member),
	}
}

func (r *inMemoryMemberRepo) FindAll(_ context.Context, _ domain.MemberFilter) ([]domain.Member, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]domain.Member, 0, len(r.members))
	for _, m := range r.members {
		res = append(res, *m)
	}
	return res, int64(len(res)), nil
}

func (r *inMemoryMemberRepo) FindByNIM(_ context.Context, nim string) (*domain.Member, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.members[strings.TrimSpace(nim)]
	if !ok {
		return nil, domain.NewNotFoundError("member")
	}
	cp := *m
	return &cp, nil
}

func (r *inMemoryMemberRepo) FindByID(_ context.Context, id primitive.ObjectID) (*domain.Member, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.members {
		if m.ID == id {
			cp := *m
			return &cp, nil
		}
	}
	return nil, domain.NewNotFoundError("member")
}

func (r *inMemoryMemberRepo) FindByEmail(_ context.Context, email string) (*domain.Member, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.members {
		if strings.EqualFold(m.Email, strings.TrimSpace(email)) {
			cp := *m
			return &cp, nil
		}
	}
	return nil, domain.NewNotFoundError("member")
}

func (r *inMemoryMemberRepo) Create(_ context.Context, m *domain.Member) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.members[m.NIM]; exists {
		return domain.NewDuplicateError("member dengan NIM tersebut sudah ada")
	}
	if m.ID.IsZero() {
		m.ID = primitive.NewObjectID()
	}
	cp := *m
	r.members[m.NIM] = &cp
	return nil
}

func (r *inMemoryMemberRepo) Update(_ context.Context, nim string, m *domain.Member) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.members[nim]; !exists {
		return domain.NewNotFoundError("member")
	}
	cp := *m
	r.members[nim] = &cp
	return nil
}

func (r *inMemoryMemberRepo) Deactivate(_ context.Context, nim string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, exists := r.members[nim]; exists {
		m.IsActive = false
		return nil
	}
	return domain.NewNotFoundError("member")
}

func (r *inMemoryMemberRepo) Activate(_ context.Context, nim string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, exists := r.members[nim]; exists {
		m.IsActive = true
		return nil
	}
	return domain.NewNotFoundError("member")
}

func (r *inMemoryMemberRepo) DeletePermanent(_ context.Context, nim string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.members, nim)
	return nil
}

func (r *inMemoryMemberRepo) Summary(_ context.Context, _ time.Time) (*domain.MemberSummary, error) {
	return &domain.MemberSummary{}, nil
}

func (r *inMemoryMemberRepo) BulkUpsert(_ context.Context, members []domain.Member) (int64, int64, error) {
	return 0, 0, nil
}

// Tests
func TestAuthService_Login(t *testing.T) {
	ctx := context.Background()
	userRepo := newInMemoryUserRepo()
	memberRepo := newInMemoryMemberRepo()
	authSvc := NewAuthService(userRepo, memberRepo, "test_jwt_secret_key_1234567890", 24*time.Hour)

	// Hash password
	pwHash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt hash failed: %v", err)
	}

	// Setup Admin User
	adminUser := &domain.User{
		ID:           primitive.NewObjectID(),
		NIM:          "000000001",
		Name:         "Admin Testing",
		PasswordHash: string(pwHash),
		Role:         domain.RoleAdmin,
		Status:       domain.UserStatusActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := userRepo.Create(ctx, adminUser); err != nil {
		t.Fatalf("failed to seed admin: %v", err)
	}

	// Setup Member User & Member Record
	memObjID := primitive.NewObjectID()
	memberRecord := &domain.Member{
		ID:        memObjID,
		NIM:       "000000002",
		Name:      "Member Testing",
		Email:     "member.test@kampus.ac.id",
		IsActive:  true,
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
	}
	if err := memberRepo.Create(ctx, memberRecord); err != nil {
		t.Fatalf("failed to seed member record: %v", err)
	}

	memberUser := &domain.User{
		ID:           primitive.NewObjectID(),
		NIM:          "000000002",
		Name:         "Member Testing",
		PasswordHash: string(pwHash),
		Role:         domain.RoleMember,
		Status:       domain.UserStatusActive,
		MemberID:     &memObjID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := userRepo.Create(ctx, memberUser); err != nil {
		t.Fatalf("failed to seed member user: %v", err)
	}

	t.Run("Admin login success", func(t *testing.T) {
		u, _, token, err := authSvc.Login(ctx, &domain.LoginRequest{NIM: "000000001", Password: "admin123"})
		if err != nil {
			t.Fatalf("expected login success, got err: %v", err)
		}
		if u.Role != domain.RoleAdmin {
			t.Errorf("expected role admin, got %s", u.Role)
		}
		if u.NIM != "000000001" {
			t.Errorf("expected NIM 000000001, got %s", u.NIM)
		}
		if token == "" {
			t.Error("expected non-empty token")
		}
	})

	t.Run("Member login success", func(t *testing.T) {
		u, mem, token, err := authSvc.Login(ctx, &domain.LoginRequest{NIM: "000000002", Password: "admin123"})
		if err != nil {
			t.Fatalf("expected login success, got err: %v", err)
		}
		if u.Role != domain.RoleMember {
			t.Errorf("expected role member, got %s", u.Role)
		}
		if mem == nil || mem.NIM != "000000002" {
			t.Errorf("expected linked member record, got %+v", mem)
		}
		if token == "" {
			t.Error("expected non-empty token")
		}
	})

	t.Run("Wrong password rejected", func(t *testing.T) {
		_, _, _, err := authSvc.Login(ctx, &domain.LoginRequest{NIM: "000000001", Password: "wrongpassword"})
		if err == nil {
			t.Fatal("expected error for wrong password, got nil")
		}
		if _, ok := err.(*domain.AppError); !ok {
			t.Errorf("expected AppError, got %T: %v", err, err)
		}
	})

	t.Run("Non-existent NIM rejected", func(t *testing.T) {
		_, _, _, err := authSvc.Login(ctx, &domain.LoginRequest{NIM: "999999999", Password: "admin123"})
		if err == nil {
			t.Fatal("expected error for non-existent NIM, got nil")
		}
	})

	t.Run("Empty password rejected", func(t *testing.T) {
		_, _, _, err := authSvc.Login(ctx, &domain.LoginRequest{NIM: "000000001", Password: ""})
		if err == nil {
			t.Fatal("expected error for empty password, got nil")
		}
	})

	t.Run("Inactive user rejected", func(t *testing.T) {
		inactiveUser := &domain.User{
			NIM:          "000000003",
			Name:         "Inactive User",
			PasswordHash: string(pwHash),
			Role:         domain.RoleMember,
			Status:       domain.UserStatusInactive,
		}
		_ = userRepo.Create(ctx, inactiveUser)

		_, _, _, err := authSvc.Login(ctx, &domain.LoginRequest{NIM: "000000003", Password: "admin123"})
		if err == nil {
			t.Fatal("expected error for inactive user, got nil")
		}
	})
}

func TestAuthService_GetProfile(t *testing.T) {
	ctx := context.Background()
	userRepo := newInMemoryUserRepo()
	memberRepo := newInMemoryMemberRepo()
	authSvc := NewAuthService(userRepo, memberRepo, "test_jwt_secret_key_1234567890", 24*time.Hour)

	memID := primitive.NewObjectID()
	_ = memberRepo.Create(ctx, &domain.Member{
		ID:        memID,
		NIM:       "000000002",
		Name:      "Budi Santoso",
		Email:     "budi@kampus.ac.id",
		IsActive:  true,
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
	})

	userID := primitive.NewObjectID()
	user := &domain.User{
		ID:        userID,
		NIM:       "000000002",
		Name:      "Budi Santoso",
		Role:      domain.RoleMember,
		Status:    domain.UserStatusActive,
		MemberID:  &memID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = userRepo.Create(ctx, user)

	profile, err := authSvc.GetProfile(ctx, userID, "000000002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Name != "Budi Santoso" {
		t.Errorf("expected name Budi Santoso, got %s", profile.Name)
	}
	if profile.Member == nil || profile.Member.Email != "budi@kampus.ac.id" {
		t.Errorf("expected linked member with email budi@kampus.ac.id, got %+v", profile.Member)
	}
}

func TestAuthService_SeedInitialUsers_Idempotent(t *testing.T) {
	ctx := context.Background()
	userRepo := newInMemoryUserRepo()
	memberRepo := newInMemoryMemberRepo()
	authSvc := NewAuthService(userRepo, memberRepo, "test_jwt_secret_key_1234567890", 24*time.Hour)

	// First run
	if err := authSvc.SeedInitialUsers(ctx, "000000001", "admin123", "000000002", "member123"); err != nil {
		t.Fatalf("first seed failed: %v", err)
	}

	count, err := userRepo.Count(ctx)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 users seeded, got %d", count)
	}

	// Verify Admin
	admin, err := userRepo.FindByNIM(ctx, "000000001")
	if err != nil || admin.Role != domain.RoleAdmin {
		t.Fatalf("expected admin user 000000001 with role admin, got %+v (err: %v)", admin, err)
	}

	// Verify Member
	member, err := userRepo.FindByNIM(ctx, "000000002")
	if err != nil || member.Role != domain.RoleMember {
		t.Fatalf("expected member user 000000002 with role member, got %+v (err: %v)", member, err)
	}
	if member.MemberID == nil {
		t.Fatal("expected member user to have linked MemberID")
	}

	// Second run (Idempotency test)
	if err := authSvc.SeedInitialUsers(ctx, "000000001", "admin123", "000000002", "member123"); err != nil {
		t.Fatalf("second seed failed: %v", err)
	}

	count2, err := userRepo.Count(ctx)
	if err != nil {
		t.Fatalf("count2 failed: %v", err)
	}
	if count2 != 2 {
		t.Errorf("expected count to remain 2 after re-seeding, got %d", count2)
	}
}
