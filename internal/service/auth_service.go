package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"smartcoop/internal/domain"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	memberRepo domain.MemberRepository
}

func NewAuthService(memberRepo domain.MemberRepository) *AuthService {
	return &AuthService{
		memberRepo: memberRepo,
	}
}

func isAuthNotFound(err error) bool {
	var appErr *domain.AppError
	return errors.As(err, &appErr) && appErr.Code == domain.CodeNotFound
}

func isValidEmail(email string) bool {
	if email == "" || len(email) > 254 {
		return false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return false
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	domainPart := parts[1]
	if !strings.Contains(domainPart, ".") || strings.HasPrefix(domainPart, ".") || strings.HasSuffix(domainPart, ".") {
		return false
	}
	return true
}

func (s *AuthService) Register(ctx context.Context, req *domain.CreateMemberRequest) (*domain.Member, error) {
	if req == nil {
		return nil, domain.NewValidationError("Data pendaftaran tidak boleh kosong")
	}

	req.NIM = strings.TrimSpace(req.NIM)
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Unit = strings.TrimSpace(req.Unit)
	req.Phone = strings.TrimSpace(req.Phone)

	var validationDetails []domain.ValidationErrorDetail

	if req.NIM == "" {
		validationDetails = append(validationDetails, domain.ValidationErrorDetail{Field: "nim", Message: "NIM wajib diisi"})
	} else if len(req.NIM) > 20 {
		validationDetails = append(validationDetails, domain.ValidationErrorDetail{Field: "nim", Message: "NIM maksimal 20 karakter"})
	}

	if req.Name == "" {
		validationDetails = append(validationDetails, domain.ValidationErrorDetail{Field: "nama", Message: "Nama lengkap wajib diisi"})
	} else if len(req.Name) > 120 {
		validationDetails = append(validationDetails, domain.ValidationErrorDetail{Field: "nama", Message: "Nama lengkap maksimal 120 karakter"})
	}

	if req.Email == "" {
		validationDetails = append(validationDetails, domain.ValidationErrorDetail{Field: "email", Message: "Email wajib diisi"})
	} else if !isValidEmail(req.Email) {
		validationDetails = append(validationDetails, domain.ValidationErrorDetail{Field: "email", Message: "Format email tidak valid"})
	}

	passBytes := []byte(req.Password)
	if len(passBytes) < 8 || len(passBytes) > 72 {
		validationDetails = append(validationDetails, domain.ValidationErrorDetail{Field: "password", Message: "Password harus 8-72 karakter"})
	}

	if len(validationDetails) > 0 {
		return nil, domain.NewValidationError("Validasi pendaftaran gagal", validationDetails...)
	}

	existingNIM, err := s.memberRepo.FindByNIM(ctx, req.NIM)
	if err == nil && existingNIM != nil {
		return nil, domain.NewDuplicateError("NIM sudah terdaftar")
	}
	if err != nil && !isAuthNotFound(err) {
		return nil, err
	}

	existingEmail, err := s.memberRepo.FindByEmail(ctx, req.Email)
	if err == nil && existingEmail != nil {
		return nil, domain.NewDuplicateError("Email sudah terdaftar")
	}
	if err != nil && !isAuthNotFound(err) {
		return nil, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(passBytes, bcrypt.DefaultCost)
	if err != nil {
		return nil, domain.NewInternalError("Gagal memproses password")
	}

	now := time.Now()

	member := &domain.Member{
		NIM:          req.NIM,
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Unit:         req.Unit,
		Phone:        req.Phone,
		IsActive:     true,
		CreatedAt:    now,
		JoinedAt:     now,
	}

	if req.JoinedAt != nil {
		member.JoinedAt = *req.JoinedAt
	}

	if err := s.memberRepo.Create(ctx, member); err != nil {
		return nil, err
	}

	return member, nil
}

func (s *AuthService) Login(ctx context.Context, req *domain.LoginRequest) (*domain.Member, error) {
	if req == nil {
		return nil, domain.NewValidationError("Data login tidak boleh kosong")
	}

	identifier := strings.ToLower(strings.TrimSpace(req.Email))
	password := req.Password

	if identifier == "" || password == "" {
		return nil, domain.NewValidationError("Email dan password wajib diisi")
	}

	var member *domain.Member
	var err error

	// Cari berdasarkan email; jika bukan format email atau tidak ditemukan, coba juga berdasarkan NIM
	if strings.Contains(identifier, "@") {
		member, err = s.memberRepo.FindByEmail(ctx, identifier)
	} else {
		member, err = s.memberRepo.FindByNIM(ctx, identifier)
		if err != nil || member == nil {
			member, err = s.memberRepo.FindByEmail(ctx, identifier)
		}
	}

	if err != nil || member == nil {
		return nil, domain.NewUnauthorizedError("Email atau password salah")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(member.PasswordHash),
		[]byte(password),
	); err != nil {
		return nil, domain.NewUnauthorizedError("Email atau password salah")
	}

	if !member.IsActive {
		return nil, domain.NewUnauthorizedError("Akun tidak aktif, silakan hubungi pengurus KOPMA")
	}

	return member, nil
}
