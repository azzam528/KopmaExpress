package service

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"

	"smartcoop/internal/domain"
	"smartcoop/internal/security"
)

type AuthService struct {
	userRepo   domain.UserRepository
	memberRepo domain.MemberRepository
	jwtSecret  string
	jwtTTL     time.Duration
}

func NewAuthService(userRepo domain.UserRepository, memberRepo domain.MemberRepository, jwtSecret string, jwtTTL time.Duration) *AuthService {
	if jwtTTL <= 0 {
		jwtTTL = 24 * time.Hour
	}
	return &AuthService{
		userRepo:   userRepo,
		memberRepo: memberRepo,
		jwtSecret:  jwtSecret,
		jwtTTL:     jwtTTL,
	}
}

// Login authenticates a user using NIM and password against the "users" collection.
func (s *AuthService) Login(ctx context.Context, req *domain.LoginRequest) (*domain.User, *domain.Member, string, error) {
	if req == nil {
		return nil, nil, "", domain.NewValidationError("Data login tidak boleh kosong")
	}

	nim := strings.TrimSpace(req.NIM)
	if nim == "" {
		nim = strings.TrimSpace(req.Email) // Fallback for backward compatibility
	}
	password := req.Password

	if nim == "" || password == "" {
		return nil, nil, "", domain.NewValidationError("NIM dan kata sandi wajib diisi")
	}

	user, err := s.userRepo.FindByNIM(ctx, nim)
	if err != nil || user == nil {
		return nil, nil, "", domain.NewUnauthorizedError("NIM atau kata sandi salah")
	}

	if user.Status != domain.UserStatusActive {
		return nil, nil, "", domain.NewUnauthorizedError("Akun tidak aktif, silakan hubungi pengurus KOPMA")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, "", domain.NewUnauthorizedError("NIM atau kata sandi salah")
	}

	var member *domain.Member
	if user.MemberID != nil && s.memberRepo != nil {
		m, err := s.memberRepo.FindByID(ctx, *user.MemberID)
		if err == nil && m != nil {
			member = m
		} else {
			m2, err2 := s.memberRepo.FindByNIM(ctx, user.NIM)
			if err2 == nil && m2 != nil {
				member = m2
			}
		}
	} else if user.Role == domain.RoleMember && s.memberRepo != nil {
		m, err := s.memberRepo.FindByNIM(ctx, user.NIM)
		if err == nil && m != nil {
			member = m
		}
	}

	claims := security.JWTClaims{
		UserID: user.ID.Hex(),
		NIM:    user.NIM,
		Name:   user.Name,
		Role:   user.Role,
	}

	token, err := security.GenerateToken(s.jwtSecret, claims, s.jwtTTL)
	if err != nil {
		return nil, nil, "", domain.NewInternalError("Gagal membuat token autentikasi")
	}

	return user, member, token, nil
}

// GetProfile retrieves the authenticated user's profile and linked member details.
func (s *AuthService) GetProfile(ctx context.Context, userID primitive.ObjectID, nim string) (*domain.UserProfileResponse, error) {
	var user *domain.User
	var err error

	if !userID.IsZero() {
		user, err = s.userRepo.FindByID(ctx, userID)
	}
	if (err != nil || user == nil) && nim != "" {
		user, err = s.userRepo.FindByNIM(ctx, nim)
	}
	if err != nil || user == nil {
		return nil, domain.NewNotFoundError("User tidak ditemukan")
	}

	profile := &domain.UserProfileResponse{
		UserID: user.ID.Hex(),
		NIM:    user.NIM,
		Name:   user.Name,
		Role:   user.Role,
		Status: user.Status,
	}

	if user.MemberID != nil {
		mIDStr := user.MemberID.Hex()
		profile.MemberID = &mIDStr

		if s.memberRepo != nil {
			m, mErr := s.memberRepo.FindByID(ctx, *user.MemberID)
			if mErr == nil && m != nil {
				profile.Member = &domain.MemberInfo{
					ID:       m.ID.Hex(),
					NIM:      m.NIM,
					Name:     m.Name,
					Unit:     m.Unit,
					Phone:    m.Phone,
					Email:    m.Email,
					IsActive: m.IsActive,
				}
			}
		}
	} else if s.memberRepo != nil {
		m, mErr := s.memberRepo.FindByNIM(ctx, user.NIM)
		if mErr == nil && m != nil {
			mIDStr := m.ID.Hex()
			profile.MemberID = &mIDStr
			profile.Member = &domain.MemberInfo{
				ID:       m.ID.Hex(),
				NIM:      m.NIM,
				Name:     m.Name,
				Unit:     m.Unit,
				Phone:    m.Phone,
				Email:    m.Email,
				IsActive: m.IsActive,
			}
		}
	}

	return profile, nil
}

// SeedInitialUsers ensures that the Admin Testing and Member Testing accounts exist idempotently.
func (s *AuthService) SeedInitialUsers(ctx context.Context, adminNIM, adminPassword, memberNIM, memberPassword string) error {
	now := time.Now()

	// 1. Seed Admin Testing Account
	if adminNIM != "" && adminPassword != "" {
		existingAdmin, err := s.userRepo.FindByNIM(ctx, adminNIM)
		if err != nil && isNotFound(err) {
			hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			adminUser := &domain.User{
				NIM:          adminNIM,
				Name:         "Admin Testing",
				PasswordHash: string(hash),
				Role:         domain.RoleAdmin,
				Status:       domain.UserStatusActive,
				MemberID:     nil,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err := s.userRepo.Create(ctx, adminUser); err != nil {
				return err
			}
		} else if err == nil && existingAdmin != nil {
			// Ensure role is admin and status is active
			if existingAdmin.Role != domain.RoleAdmin || existingAdmin.Status != domain.UserStatusActive {
				existingAdmin.Role = domain.RoleAdmin
				existingAdmin.Status = domain.UserStatusActive
				_ = s.userRepo.Update(ctx, existingAdmin.ID, existingAdmin)
			}
		}
	}

	// 2. Seed Member Testing Account & Member Document
	if memberNIM != "" && memberPassword != "" {
		var memberDoc *domain.Member
		if s.memberRepo != nil {
			m, err := s.memberRepo.FindByNIM(ctx, memberNIM)
			if err != nil && isNotFound(err) {
				newM := &domain.Member{
					NIM:       memberNIM,
					Name:      "Member Testing",
					Unit:      "KOPMA",
					Phone:     "081200000002",
					IsActive:  true,
					JoinedAt:  now,
					CreatedAt: now,
				}
				if createErr := s.memberRepo.Create(ctx, newM); createErr == nil {
					memberDoc = newM
				}
			} else if err == nil && m != nil {
				memberDoc = m
			}
		}

		existingMemberUser, err := s.userRepo.FindByNIM(ctx, memberNIM)
		if err != nil && isNotFound(err) {
			hash, err := bcrypt.GenerateFromPassword([]byte(memberPassword), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			var memberID *primitive.ObjectID
			if memberDoc != nil && !memberDoc.ID.IsZero() {
				memberID = &memberDoc.ID
			}

			memberUser := &domain.User{
				NIM:          memberNIM,
				Name:         "Member Testing",
				PasswordHash: string(hash),
				Role:         domain.RoleMember,
				Status:       domain.UserStatusActive,
				MemberID:     memberID,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err := s.userRepo.Create(ctx, memberUser); err != nil {
				return err
			}
		} else if err == nil && existingMemberUser != nil {
			needsUpdate := false
			if existingMemberUser.Role != domain.RoleMember {
				existingMemberUser.Role = domain.RoleMember
				needsUpdate = true
			}
			if existingMemberUser.Status != domain.UserStatusActive {
				existingMemberUser.Status = domain.UserStatusActive
				needsUpdate = true
			}
			if existingMemberUser.MemberID == nil && memberDoc != nil && !memberDoc.ID.IsZero() {
				existingMemberUser.MemberID = &memberDoc.ID
				needsUpdate = true
			}
			if needsUpdate {
				_ = s.userRepo.Update(ctx, existingMemberUser.ID, existingMemberUser)
			}
		}
	}

	return nil
}
