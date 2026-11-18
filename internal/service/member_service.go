package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"smartcoop/internal/domain"
)

const (
	memberMaxNIM      = 20
	memberMaxName     = 120
	memberMaxUnit     = 100
	summaryWindowDays = 30
)

var (
	// NIM/NIP: huruf, angka, titik, garis bawah, atau strip; tanpa spasi.
	nimPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	// Telepon: 8-15 digit, boleh diawali +. Spasi, strip, titik, dan kurung diabaikan saat validasi.
	phonePattern = regexp.MustCompile(`^\+?[0-9]{8,15}$`)
	phoneStrip   = strings.NewReplacer(" ", "", "-", "", ".", "", "(", "", ")", "")
)

// Service hanya butuh method berikut dari repo anggota (interface sempit,
// sama seperti pola di order_service.go).
type memberStore interface {
	FindAll(ctx context.Context, filter domain.MemberFilter) ([]domain.Member, int64, error)
	FindByNIM(ctx context.Context, nim string) (*domain.Member, error)
	Create(ctx context.Context, m *domain.Member) error
	Update(ctx context.Context, nim string, m *domain.Member) error
	Deactivate(ctx context.Context, nim string) error
	Activate(ctx context.Context, nim string) error
	DeletePermanent(ctx context.Context, nim string) error
	Summary(ctx context.Context, buyersSince time.Time) (*domain.MemberSummary, error)
}

type memberService struct {
	store memberStore
}

var _ domain.MemberService = (*memberService)(nil)

func NewMemberService(store memberStore) domain.MemberService {
	return &memberService{store: store}
}

// ------------------------------------------------------------------ List

func (s *memberService) List(ctx context.Context, f domain.MemberFilter) ([]domain.Member, int64, error) {
	f.Query = strings.TrimSpace(f.Query)
	f.Status = strings.ToLower(strings.TrimSpace(f.Status))

	switch f.Status {
	case "all":
		f.Status = ""
	case "", domain.MemberStatusActive, domain.MemberStatusInactive:
	default:
		return nil, 0, domain.NewValidationError("Parameter filter tidak valid",
			domain.ValidationErrorDetail{Field: "status", Message: "harus active atau inactive"})
	}

	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = defaultLimit
	}
	if f.Limit > maxLimit {
		f.Limit = maxLimit
	}

	return s.store.FindAll(ctx, f)
}

func (s *memberService) GetByNIM(ctx context.Context, nim string) (*domain.Member, error) {
	nim = strings.TrimSpace(nim)
	if nim == "" {
		return nil, domain.NewValidationError("NIM wajib diisi")
	}
	return s.store.FindByNIM(ctx, nim)
}

// ---------------------------------------------------------------- Create

func (s *memberService) Create(ctx context.Context, req *domain.CreateMemberRequest) (*domain.Member, error) {
	if req == nil {
		return nil, domain.NewValidationError("Body request tidak boleh kosong")
	}

	req.NIM = strings.TrimSpace(req.NIM)
	req.Name = strings.TrimSpace(req.Name)
	req.Unit = strings.TrimSpace(req.Unit)
	req.Phone = strings.TrimSpace(req.Phone)

	details := validateNIM(req.NIM)
	details = append(details, validateMemberFields(req.Name, req.Unit, req.Phone)...)

	if len(details) > 0 {
		return nil, domain.NewValidationError("Validasi gagal", details...)
	}

	// 1. Cek duplikasi NIM terlebih dahulu
	existing, err := s.store.FindByNIM(ctx, req.NIM)
	if err == nil && existing != nil {
		return nil, domain.NewDuplicateError(
			fmt.Sprintf("Anggota dengan NIM %s sudah terdaftar", req.NIM),
		)
	}
	if err != nil && !domain.IsNotFound(err) {
		return nil, err
	}

	now := time.Now().Truncate(time.Millisecond)
	joined := now
	if req.JoinedAt != nil && !req.JoinedAt.IsZero() {
		joined = *req.JoinedAt
	}

	m := &domain.Member{
		NIM:       req.NIM,
		Name:      req.Name,
		Email:     strings.TrimSpace(req.Email),
		Unit:      req.Unit,
		Phone:     req.Phone,
		IsActive:  true,
		JoinedAt:  joined,
		CreatedAt: now,
	}

	if err := s.store.Create(ctx, m); err != nil {
		if isDuplicateKey(err) {
			errMsg := err.Error()
			if strings.Contains(errMsg, "email") {
				return nil, domain.NewDuplicateError(
					fmt.Sprintf("Anggota dengan email %s sudah terdaftar", m.Email),
				)
			}
			return nil, domain.NewDuplicateError(
				fmt.Sprintf("Anggota dengan NIM %s sudah terdaftar", m.NIM),
			)
		}
		return nil, err
	}
	return m, nil
}

// ---------------------------------------------------------------- Update

func (s *memberService) Update(ctx context.Context, nim string, req *domain.UpdateMemberRequest) (*domain.Member, error) {
	nim = strings.TrimSpace(nim)
	if nim == "" {
		return nil, domain.NewValidationError("NIM wajib diisi")
	}
	if req == nil {
		return nil, domain.NewValidationError("Body request tidak boleh kosong")
	}

	req.NIM = strings.TrimSpace(req.NIM)
	req.Name = strings.TrimSpace(req.Name)
	req.Unit = strings.TrimSpace(req.Unit)
	req.Phone = strings.TrimSpace(req.Phone)

	// NIM kosong = tidak diganti. Hanya NIM yang benar-benar berbeda yang divalidasi.
	newNIM := req.NIM
	if newNIM == "" {
		newNIM = nim
	}

	var details []domain.ValidationErrorDetail
	if newNIM != nim {
		details = validateNIM(newNIM)
		existing, err := s.store.FindByNIM(ctx, newNIM)
		if err == nil && existing != nil {
			return nil, domain.NewDuplicateError(
				fmt.Sprintf("Anggota dengan NIM %s sudah terdaftar", newNIM),
			)
		}
	}
	details = append(details, validateMemberFields(req.Name, req.Unit, req.Phone)...)
	if len(details) > 0 {
		return nil, domain.NewValidationError("Validasi gagal", details...)
	}

	if err := s.store.Update(ctx, nim, &domain.Member{
		NIM:   newNIM,
		Name:  req.Name,
		Unit:  req.Unit,
		Phone: req.Phone,
	}); err != nil {
		if isDuplicateKey(err) {
			errMsg := err.Error()
			if strings.Contains(errMsg, "email") {
				return nil, domain.NewDuplicateError("Email sudah terdaftar")
			}
			return nil, domain.NewDuplicateError(
				fmt.Sprintf("Anggota dengan NIM %s sudah terdaftar", newNIM),
			)
		}
		return nil, err
	}
	return s.store.FindByNIM(ctx, newNIM)
}

// ------------------------------------------- Nonaktifkan / aktifkan / ringkasan

func (s *memberService) Delete(ctx context.Context, nim string) error {
	nim = strings.TrimSpace(nim)
	if nim == "" {
		return domain.NewValidationError("NIM wajib diisi")
	}
	return s.store.Deactivate(ctx, nim)
}

func (s *memberService) Activate(ctx context.Context, nim string) error {
	nim = strings.TrimSpace(nim)
	if nim == "" {
		return domain.NewValidationError("NIM wajib diisi")
	}
	return s.store.Activate(ctx, nim)
}

// DeletePermanent: aturan "nonaktif dan tanpa pesanan" ditegakkan di repo agar
// pengecekan dan penghapusan berada di satu tempat.
func (s *memberService) DeletePermanent(ctx context.Context, nim string) error {
	nim = strings.TrimSpace(nim)
	if nim == "" {
		return domain.NewValidationError("NIM wajib diisi")
	}
	return s.store.DeletePermanent(ctx, nim)
}

func (s *memberService) Summary(ctx context.Context) (*domain.MemberSummary, error) {
	return s.store.Summary(ctx, time.Now().AddDate(0, 0, -summaryWindowDays))
}

// --------------------------------------------------------------- Helpers

// validateNIM memeriksa format NIM (input sudah di-trim).
func validateNIM(nim string) []domain.ValidationErrorDetail {
	switch {
	case nim == "":
		return []domain.ValidationErrorDetail{{Field: "nim", Message: "wajib diisi"}}
	case utf8.RuneCountInString(nim) > memberMaxNIM:
		return []domain.ValidationErrorDetail{{
			Field: "nim", Message: fmt.Sprintf("maksimal %d karakter", memberMaxNIM),
		}}
	case !nimPattern.MatchString(nim):
		return []domain.ValidationErrorDetail{{
			Field: "nim", Message: "hanya huruf, angka, titik, garis bawah, dan strip; tanpa spasi",
		}}
	}
	return nil
}

// validateMemberFields memeriksa nama, unit, dan telepon (input sudah di-trim).
func validateMemberFields(name, unit, phone string) []domain.ValidationErrorDetail {
	var d []domain.ValidationErrorDetail

	if name == "" {
		d = append(d, domain.ValidationErrorDetail{Field: "nama", Message: "wajib diisi"})
	} else if utf8.RuneCountInString(name) > memberMaxName {
		d = append(d, domain.ValidationErrorDetail{
			Field: "nama", Message: fmt.Sprintf("maksimal %d karakter", memberMaxName),
		})
	}

	if utf8.RuneCountInString(unit) > memberMaxUnit {
		d = append(d, domain.ValidationErrorDetail{
			Field: "unit", Message: fmt.Sprintf("maksimal %d karakter", memberMaxUnit),
		})
	}

	if phone != "" && !phonePattern.MatchString(phoneStrip.Replace(phone)) {
		d = append(d, domain.ValidationErrorDetail{
			Field: "telepon", Message: "harus 8-15 digit, boleh diawali +",
		})
	}

	return d
}
