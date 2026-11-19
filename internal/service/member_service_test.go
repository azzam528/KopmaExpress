package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"smartcoop/internal/domain"
)

// ------------------------------------------------------------------ fake

type fakeMemberStore struct {
	items      map[string]*domain.Member
	createErr  error
	lastFilter domain.MemberFilter
	lastSince  time.Time
	deleteErr  error
	renamed    []string
}

func newFakeMemberStore() *fakeMemberStore {
	return &fakeMemberStore{items: map[string]*domain.Member{
		"2210101001": {NIM: "2210101001", Name: "Andi Pratama", Unit: "TI 2022", IsActive: true},
	}}
}

func (f *fakeMemberStore) FindAll(_ context.Context, flt domain.MemberFilter) ([]domain.Member, int64, error) {
	f.lastFilter = flt
	return []domain.Member{}, 0, nil
}

func (f *fakeMemberStore) FindByNIM(_ context.Context, nim string) (*domain.Member, error) {
	if m, ok := f.items[nim]; ok {
		cp := *m
		return &cp, nil
	}
	return nil, domain.NewNotFoundError("Anggota tidak ditemukan")
}

func (f *fakeMemberStore) Create(_ context.Context, m *domain.Member) error {
	if f.createErr != nil {
		return f.createErr
	}
	if _, dup := f.items[m.NIM]; dup {
		return errors.New("E11000 duplicate key error collection: members index: idx_members_nim_unique")
	}
	cp := *m
	f.items[m.NIM] = &cp
	return nil
}

func (f *fakeMemberStore) Update(_ context.Context, nim string, m *domain.Member) error {
	cur, ok := f.items[nim]
	if !ok {
		return domain.NewNotFoundError("Anggota tidak ditemukan")
	}
	if m.NIM != "" && m.NIM != nim {
		if _, dup := f.items[m.NIM]; dup {
			return errors.New("E11000 duplicate key error collection: members index: idx_members_nim_unique")
		}
		delete(f.items, nim)
		cur.NIM = m.NIM
		f.items[m.NIM] = cur
		f.renamed = append(f.renamed, nim+"->"+m.NIM)
	}
	cur.Name, cur.Unit, cur.Phone = m.Name, m.Unit, m.Phone
	return nil
}

func (f *fakeMemberStore) Deactivate(_ context.Context, nim string) error {
	cur, ok := f.items[nim]
	if !ok {
		return domain.NewNotFoundError("Anggota tidak ditemukan")
	}
	cur.IsActive = false
	return nil
}

func (f *fakeMemberStore) Activate(_ context.Context, nim string) error {
	cur, ok := f.items[nim]
	if !ok {
		return domain.NewNotFoundError("Anggota tidak ditemukan")
	}
	cur.IsActive = true
	return nil
}

func (f *fakeMemberStore) DeletePermanent(_ context.Context, nim string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.items[nim]; !ok {
		return domain.NewNotFoundError("Anggota tidak ditemukan")
	}
	delete(f.items, nim)
	return nil
}

func (f *fakeMemberStore) Summary(_ context.Context, since time.Time) (*domain.MemberSummary, error) {
	f.lastSince = since
	return &domain.MemberSummary{Total: 3, Active: 2, Inactive: 1, ActiveBuyers30: 1}, nil
}

// ------------------------------------------------------------------ tests

func TestMemberCreate_Success(t *testing.T) {
	store := newFakeMemberStore()
	svc := NewMemberService(store)

	m, err := svc.Create(context.Background(), &domain.CreateMemberRequest{
		NIM: "  2310101002 ", Name: "  Citra Dewi ", Unit: " SI 2023 ", Phone: " 0812-3456-7890 ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.NIM != "2310101002" || m.Name != "Citra Dewi" || m.Unit != "SI 2023" || m.Phone != "0812-3456-7890" {
		t.Errorf("input tidak di-trim: %+v", m)
	}
	if !m.IsActive {
		t.Error("anggota baru harus aktif")
	}
	if m.JoinedAt.IsZero() || m.CreatedAt.IsZero() {
		t.Error("joined_at/created_at harus terisi")
	}
	if _, ok := store.items["2310101002"]; !ok {
		t.Error("anggota tidak tersimpan di repo")
	}
}

func TestMemberCreate_ValidationErrors(t *testing.T) {
	cases := []struct {
		name  string
		req   domain.CreateMemberRequest
		field string
	}{
		{"nim kosong", domain.CreateMemberRequest{NIM: " ", Name: "A"}, "nim"},
		{"nim mengandung spasi", domain.CreateMemberRequest{NIM: "2310 1010", Name: "A"}, "nim"},
		{"nim terlalu panjang", domain.CreateMemberRequest{NIM: strings.Repeat("1", 21), Name: "A"}, "nim"},
		{"nama kosong", domain.CreateMemberRequest{NIM: "123", Name: "  "}, "nama"},
		{"nama terlalu panjang", domain.CreateMemberRequest{NIM: "123", Name: strings.Repeat("a", 121)}, "nama"},
		{"unit terlalu panjang", domain.CreateMemberRequest{NIM: "123", Name: "A", Unit: strings.Repeat("u", 101)}, "unit"},
		{"telepon bukan angka", domain.CreateMemberRequest{NIM: "123", Name: "A", Phone: "abc12345"}, "telepon"},
		{"telepon terlalu pendek", domain.CreateMemberRequest{NIM: "123", Name: "A", Phone: "12345"}, "telepon"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewMemberService(newFakeMemberStore())
			req := tc.req
			_, err := svc.Create(context.Background(), &req)
			ae := assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)

			details, ok := ae.Details.([]domain.ValidationErrorDetail)
			if !ok {
				t.Fatalf("details bertipe %T", ae.Details)
			}
			found := false
			for _, d := range details {
				if d.Field == tc.field {
					found = true
				}
			}
			if !found {
				t.Errorf("tidak ada detail untuk field %q: %+v", tc.field, details)
			}
		})
	}
}

func TestMemberCreate_PhoneVariantsAccepted(t *testing.T) {
	for _, phone := range []string{"", "081234567890", "+62 812-3456-7890", "(021) 5551234"} {
		svc := NewMemberService(newFakeMemberStore())
		_, err := svc.Create(context.Background(), &domain.CreateMemberRequest{NIM: "999", Name: "Z", Phone: phone})
		if err != nil {
			t.Errorf("telepon %q seharusnya valid: %v", phone, err)
		}
	}
}

func TestMemberCreate_DuplicateNIM(t *testing.T) {
	svc := NewMemberService(newFakeMemberStore())
	_, err := svc.Create(context.Background(), &domain.CreateMemberRequest{NIM: "2210101001", Name: "Kembar"})
	assertAppError(t, err, domain.CodeDuplicate, http.StatusConflict)
}

func TestMemberCreate_NilAndRepoError(t *testing.T) {
	svc := NewMemberService(newFakeMemberStore())
	_, err := svc.Create(context.Background(), nil)
	assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)

	store := newFakeMemberStore()
	store.createErr = errors.New("koneksi putus")
	_, err = NewMemberService(store).Create(context.Background(), &domain.CreateMemberRequest{NIM: "1", Name: "A"})
	if err == nil || err.Error() != "koneksi putus" {
		t.Errorf("error repo harus diteruskan apa adanya, got %v", err)
	}
}

func TestMemberUpdate(t *testing.T) {
	store := newFakeMemberStore()
	svc := NewMemberService(store)

	m, err := svc.Update(context.Background(), " 2210101001 ", &domain.UpdateMemberRequest{
		Name: " Andi P. ", Unit: "TI 2022", Phone: "081234567890",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "Andi P." || m.Phone != "081234567890" || m.NIM != "2210101001" {
		t.Errorf("hasil update salah: %+v", m)
	}

	_, err = svc.Update(context.Background(), "tidak-ada", &domain.UpdateMemberRequest{Name: "X"})
	assertAppError(t, err, domain.CodeNotFound, http.StatusNotFound)

	_, err = svc.Update(context.Background(), "2210101001", &domain.UpdateMemberRequest{Name: ""})
	assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)

	_, err = svc.Update(context.Background(), "  ", &domain.UpdateMemberRequest{Name: "X"})
	assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
}

func TestMemberUpdate_ChangeNIM(t *testing.T) {
	store := newFakeMemberStore()
	store.items["2210101002"] = &domain.Member{NIM: "2210101002", Name: "Budi", IsActive: true}
	svc := NewMemberService(store)

	// NIM berubah -> repo menerima NIM baru, hasil dibaca dengan NIM baru
	m, err := svc.Update(context.Background(), "2210101001", &domain.UpdateMemberRequest{
		NIM: " 2210109999 ", Name: "Andi Pratama",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.NIM != "2210109999" {
		t.Errorf("NIM hasil = %q, want 2210109999", m.NIM)
	}
	if len(store.renamed) != 1 || store.renamed[0] != "2210101001->2210109999" {
		t.Errorf("repo tidak menerima penggantian NIM: %v", store.renamed)
	}
	if _, ok := store.items["2210101001"]; ok {
		t.Error("NIM lama seharusnya tidak ada lagi")
	}

	// NIM sama / kosong -> bukan penggantian NIM
	store.renamed = nil
	for _, same := range []string{"", "2210109999"} {
		if _, err := svc.Update(context.Background(), "2210109999", &domain.UpdateMemberRequest{NIM: same, Name: "Andi P."}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if len(store.renamed) != 0 {
		t.Errorf("NIM tidak berubah tapi ada penggantian: %v", store.renamed)
	}

	// NIM baru dipakai anggota lain -> 409 dengan nomor yang dimaksud
	_, err = svc.Update(context.Background(), "2210109999", &domain.UpdateMemberRequest{NIM: "2210101002", Name: "X"})
	ae := assertAppError(t, err, domain.CodeDuplicate, http.StatusConflict)
	if !strings.Contains(ae.Message, "2210101002") {
		t.Errorf("pesan duplikat tidak menyebut NIM: %q", ae.Message)
	}

	// format NIM baru salah -> 422 pada field nim
	_, err = svc.Update(context.Background(), "2210109999", &domain.UpdateMemberRequest{NIM: "ada spasi", Name: "X"})
	ae = assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
	details, _ := ae.Details.([]domain.ValidationErrorDetail)
	if len(details) == 0 || details[0].Field != "nim" {
		t.Errorf("harus ada detail untuk field nim: %+v", ae.Details)
	}

	// NIM lama tidak ada -> 404
	_, err = svc.Update(context.Background(), "tidak-ada", &domain.UpdateMemberRequest{NIM: "BARU1", Name: "X"})
	assertAppError(t, err, domain.CodeNotFound, http.StatusNotFound)
}

func TestMemberDeactivateAndActivate(t *testing.T) {
	store := newFakeMemberStore()
	svc := NewMemberService(store)

	if err := svc.Delete(context.Background(), "2210101001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.items["2210101001"].IsActive {
		t.Error("anggota seharusnya nonaktif")
	}

	if err := svc.Activate(context.Background(), "2210101001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.items["2210101001"].IsActive {
		t.Error("anggota seharusnya aktif kembali")
	}

	assertAppError(t, svc.Delete(context.Background(), "nope"), domain.CodeNotFound, http.StatusNotFound)
	assertAppError(t, svc.Activate(context.Background(), " "), domain.CodeValidationError, http.StatusUnprocessableEntity)
}

func TestMemberDeletePermanent(t *testing.T) {
	store := newFakeMemberStore()
	svc := NewMemberService(store)

	if err := svc.DeletePermanent(context.Background(), " 2210101001 "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.items["2210101001"]; ok {
		t.Error("anggota seharusnya terhapus (NIM di-trim)")
	}

	assertAppError(t, svc.DeletePermanent(context.Background(), "2210101001"), domain.CodeNotFound, http.StatusNotFound)
	assertAppError(t, svc.DeletePermanent(context.Background(), "  "), domain.CodeValidationError, http.StatusUnprocessableEntity)

	// aturan "nonaktif & tanpa pesanan" ada di repo; error 409-nya harus diteruskan apa adanya
	store.deleteErr = domain.NewAppError(http.StatusConflict, domain.CodeMemberHasOrders, "Anggota punya 3 pesanan, nonaktifkan saja", nil)
	ae := assertAppError(t, svc.DeletePermanent(context.Background(), "x"), domain.CodeMemberHasOrders, http.StatusConflict)
	if !strings.Contains(ae.Message, "nonaktifkan saja") {
		t.Errorf("pesan error tidak diteruskan: %q", ae.Message)
	}
}

func TestMemberList_NormalizesFilter(t *testing.T) {
	store := newFakeMemberStore()
	svc := NewMemberService(store)

	_, _, err := svc.List(context.Background(), domain.MemberFilter{Query: "  andi ", Status: " ACTIVE ", Page: 0, Limit: 9999})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := store.lastFilter
	if f.Query != "andi" || f.Status != domain.MemberStatusActive {
		t.Errorf("filter tidak dinormalisasi: %+v", f)
	}
	if f.Page != 1 || f.Limit != maxLimit {
		t.Errorf("paging = %d/%d, want 1/%d", f.Page, f.Limit, maxLimit)
	}

	_, _, _ = svc.List(context.Background(), domain.MemberFilter{Status: "all"})
	if store.lastFilter.Status != "" || store.lastFilter.Limit != defaultLimit {
		t.Errorf("status all harus jadi kosong dan limit default: %+v", store.lastFilter)
	}

	_, _, err = svc.List(context.Background(), domain.MemberFilter{Status: "banned"})
	assertAppError(t, err, domain.CodeValidationError, http.StatusUnprocessableEntity)
}

func TestMemberSummary_Uses30DayWindow(t *testing.T) {
	store := newFakeMemberStore()
	svc := NewMemberService(store)

	before := time.Now()
	s, err := svc.Summary(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Total != 3 || s.Active != 2 {
		t.Errorf("ringkasan salah: %+v", s)
	}

	want := before.AddDate(0, 0, -summaryWindowDays)
	if d := store.lastSince.Sub(want); d < 0 || d > 5*time.Second {
		t.Errorf("batas waktu pembeli = %v, want sekitar %v", store.lastSince, want)
	}
}
