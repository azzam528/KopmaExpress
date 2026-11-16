package excelgen

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

const (
	SheetProducts = "Barang"
	SheetMembers  = "Anggota"
	SheetHistory  = "Riwayat_Penjualan"
)

type Options struct {
	Items   int
	Months  int
	Members int
	Dirty   bool
	Seed    int64
	// Now fixes the reference date so output is reproducible; zero means time.Now().
	Now time.Time
}

type categorySpec struct {
	name     string
	minPrice int64
	maxPrice int64
	minBase  int
	maxBase  int
	items    []string
}

var categories = []categorySpec{
	{"ATK", 1000, 25000, 10, 200, []string{"Buku Tulis 38 Lembar", "Pulpen Standard", "Pensil 2B", "Penghapus", "Penggaris 30 cm", "Spidol Whiteboard", "Stabilo", "Map Plastik"}},
	{"Makanan", 2000, 30000, 10, 200, []string{"Mie Instan Goreng", "Roti Tawar", "Biskuit Kelapa", "Keripik Singkong", "Cokelat Batang", "Wafer Krim", "Kacang Atom", "Permen Mint"}},
	{"Minuman", 2000, 25000, 10, 200, []string{"Air Mineral 600 ml", "Teh Botol", "Kopi Sachet", "Susu UHT", "Jus Jeruk", "Minuman Isotonik", "Teh Kotak", "Kopi Susu Botol"}},
	{"Seragam", 50000, 250000, 3, 40, []string{"Kemeja Putih", "Almamater", "Kaos Olahraga", "Celana Bahan", "Rok Panjang", "Dasi", "Topi Almamater", "Kaus Kaki"}},
	{"Buku", 10000, 120000, 5, 60, []string{"Buku Pemrograman Dasar", "Kamus Inggris", "Modul Praktikum", "Buku Statistika", "Novel Remaja", "Buku Basis Data", "Buku Algoritma", "Atlas"}},
	{"Elektronik", 25000, 250000, 3, 40, []string{"Kabel USB-C", "Charger 20W", "Earphone", "Flashdisk 32 GB", "Mouse Optik", "Powerbank 10000 mAh", "Baterai AA", "Colokan T"}},
	{"Kebersihan", 3000, 60000, 10, 200, []string{"Sabun Cuci Tangan", "Tisu Gulung", "Sapu Lantai", "Pel Lantai", "Pembersih Lantai", "Masker", "Hand Sanitizer", "Kantong Sampah"}},
	{"Lainnya", 1000, 100000, 10, 200, []string{"Payung", "Tas Serut", "Gantungan Kunci", "Stiker", "Lanyard", "Botol Minum", "Kotak Pensil", "Gembok"}},
}

var (
	variants   = []string{"", "Besar", "Kecil", "Premium", "Hemat", "Jumbo", "Mini", "Reguler"}
	firstNames = []string{"Andi", "Budi", "Citra", "Dewi", "Eko", "Fajar", "Gita", "Hana", "Indra", "Joko", "Kiki", "Lestari", "Maya", "Nando", "Putri"}
	lastNames  = []string{"Pratama", "Santoso", "Wijaya", "Lestari", "Hidayat", "Saputra", "Nugroho", "Rahma", "Kusuma", "Permana"}
	units      = []string{"Teknik Informatika", "Sistem Informasi", "Manajemen", "Akuntansi", "Teknik Elektro", "Hukum", "Psikologi", "Kedokteran"}
	monthNames = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
)

type pattern int

const (
	patRising pattern = iota
	patFalling
	patStable
	patSeasonal
)

type product struct {
	code     string
	name     string
	category string
	price    int64
	stock    int64
}

func (o *Options) normalize() error {
	if o.Items <= 0 {
		o.Items = 300
	}
	if o.Months <= 0 {
		o.Months = 6
	}
	if o.Members <= 0 {
		o.Members = 30
	}
	if o.Items > 100000 {
		return errors.New("items maksimal 100000")
	}
	if o.Months > 36 {
		return errors.New("months maksimal 36")
	}
	if o.Members > 10000 {
		return errors.New("members maksimal 10000")
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	return nil
}

func Generate(opts Options) (*excelize.File, error) {
	if err := opts.normalize(); err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewSource(opts.Seed))

	f := excelize.NewFile()
	style, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"047857"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}

	products, err := newSheet(f, SheetProducts, true)
	if err != nil {
		return nil, err
	}
	members, err := newSheet(f, SheetMembers, false)
	if err != nil {
		return nil, err
	}
	history, err := newSheet(f, SheetHistory, false)
	if err != nil {
		return nil, err
	}

	items, err := writeProducts(products, rng, opts)
	if err != nil {
		return nil, err
	}
	if err := writeMembers(members, rng, opts); err != nil {
		return nil, err
	}
	if err := writeHistory(history, rng, opts, items); err != nil {
		return nil, err
	}

	for _, s := range []*sheet{products, members, history} {
		if err := s.finish(style); err != nil {
			return nil, err
		}
	}
	return f, nil
}

func WriteFile(path string, opts Options) error {
	f, err := Generate(opts)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.SaveAs(path)
}

func writeProducts(s *sheet, rng *rand.Rand, opts Options) ([]product, error) {
	if err := s.put("Kode Barang", "Nama Barang", "Kategori", "Harga", "Stok"); err != nil {
		return nil, err
	}

	items := make([]product, 0, opts.Items)
	for i := 0; i < opts.Items; i++ {
		p := newProduct(rng, i)
		items = append(items, p)

		if opts.Dirty && rng.Float64() < 0.02 {
			s.skip()
		}
		if err := s.put(productRow(rng, p, opts.Dirty)...); err != nil {
			return nil, err
		}
		if opts.Dirty && rng.Float64() < 0.01 {
			dup := p
			dup.price += 500
			if err := s.put(productRow(rng, dup, opts.Dirty)...); err != nil {
				return nil, err
			}
		}
	}
	return items, nil
}

func newProduct(rng *rand.Rand, i int) product {
	cat := categories[i%len(categories)]
	k := i / len(categories)
	base := cat.items[k%len(cat.items)]
	round := k / len(cat.items)

	name := base
	if v := variants[round%len(variants)]; v != "" {
		name += " " + v
	}
	if round >= len(variants) {
		name += fmt.Sprintf(" #%d", round)
	}

	price := cat.minPrice + rng.Int63n(cat.maxPrice-cat.minPrice+1)
	price = price / 500 * 500
	if price < cat.minPrice {
		price = cat.minPrice
	}

	stock := rng.Int63n(301)
	if rng.Float64() < 0.15 {
		stock = rng.Int63n(16)
	}

	return product{
		code:     fmt.Sprintf("BRG-%04d", i+1),
		name:     name,
		category: cat.name,
		price:    price,
		stock:    stock,
	}
}

func productRow(rng *rand.Rand, p product, dirty bool) []any {
	var price, stock any = p.price, p.stock
	if dirty && rng.Float64() < 0.03 {
		price = "Rp " + formatRupiah(p.price)
	}
	if dirty && rng.Float64() < 0.03 {
		stock = fmt.Sprintf("%d pcs", p.stock)
	}
	return []any{p.code, p.name, p.category, price, stock}
}

func writeMembers(s *sheet, rng *rand.Rand, opts Options) error {
	if err := s.put("NIM", "Nama", "Unit/Prodi", "Telepon", "Tanggal Bergabung"); err != nil {
		return err
	}
	for i := 0; i < opts.Members; i++ {
		name := firstNames[rng.Intn(len(firstNames))] + " " + lastNames[rng.Intn(len(lastNames))]
		phone := fmt.Sprintf("08%02d%08d", 11+rng.Intn(80), rng.Intn(100000000))
		joined := time.Date(2020+rng.Intn(5), time.Month(1+rng.Intn(12)), 1+rng.Intn(28), 0, 0, 0, 0, time.UTC)

		layout := "2006-01-02"
		if opts.Dirty && rng.Float64() < 0.3 {
			layout = "02/01/2006"
		}
		if err := s.put(fmt.Sprintf("22101%05d", i+1), name, units[rng.Intn(len(units))], phone, joined.Format(layout)); err != nil {
			return err
		}
	}
	return nil
}

func writeHistory(s *sheet, rng *rand.Rand, opts Options, items []product) error {
	if err := s.put("Kode Barang", "Tahun", "Bulan", "Qty Terjual", "Total Omzet"); err != nil {
		return err
	}

	// Last complete month is the newest point; the current month is still running.
	last := time.Date(opts.Now.Year(), opts.Now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)

	for i, p := range items {
		cat := categories[i%len(categories)]
		pat := pattern(rng.Intn(4))
		base := float64(cat.minBase + rng.Intn(cat.maxBase-cat.minBase+1))
		phase := rng.Float64() * 2 * math.Pi

		for m := 0; m < opts.Months; m++ {
			t := last.AddDate(0, -(opts.Months - 1 - m), 0)
			qty := monthlyQty(rng, pat, base, m, phase)

			if opts.Dirty && rng.Float64() < 0.02 {
				s.skip()
			}
			var month any = int(t.Month())
			if opts.Dirty && rng.Float64() < 0.05 {
				month = monthNames[t.Month()-1]
			}
			if err := s.put(p.code, t.Year(), month, qty, qty*p.price); err != nil {
				return err
			}
		}
	}
	return nil
}

func monthlyQty(rng *rand.Rand, pat pattern, base float64, m int, phase float64) int64 {
	factor := 1.0
	switch pat {
	case patRising:
		factor = 1 + 0.10*float64(m)
	case patFalling:
		factor = math.Max(0.25, 1-0.10*float64(m))
	case patSeasonal:
		factor = 1 + 0.4*math.Sin(2*math.Pi*float64(m)/6+phase)
	}
	noise := 1 + (rng.Float64()*0.30 - 0.15)
	return int64(math.Max(0, math.Round(base*factor*noise)))
}

func formatRupiah(n int64) string {
	s := fmt.Sprint(n)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ".")
}

type sheet struct {
	f      *excelize.File
	name   string
	row    int
	widths []int
}

func newSheet(f *excelize.File, name string, first bool) (*sheet, error) {
	if first {
		if err := f.SetSheetName(f.GetSheetName(0), name); err != nil {
			return nil, err
		}
	} else if _, err := f.NewSheet(name); err != nil {
		return nil, err
	}
	return &sheet{f: f, name: name, row: 1}, nil
}

func (s *sheet) put(cells ...any) error {
	cell, err := excelize.CoordinatesToCellName(1, s.row)
	if err != nil {
		return err
	}
	if err := s.f.SetSheetRow(s.name, cell, &cells); err != nil {
		return err
	}
	for i, c := range cells {
		n := utf8.RuneCountInString(fmt.Sprint(c))
		if i >= len(s.widths) {
			s.widths = append(s.widths, n)
		} else if n > s.widths[i] {
			s.widths[i] = n
		}
	}
	s.row++
	return nil
}

func (s *sheet) skip() {
	s.row++
}

func (s *sheet) finish(headerStyle int) error {
	lastCell, err := excelize.CoordinatesToCellName(len(s.widths), 1)
	if err != nil {
		return err
	}
	if err := s.f.SetCellStyle(s.name, "A1", lastCell, headerStyle); err != nil {
		return err
	}
	for i, w := range s.widths {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		if err := s.f.SetColWidth(s.name, col, col, math.Min(float64(w)+3, 60)); err != nil {
			return err
		}
	}
	return s.f.SetPanes(s.name, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
		Selection:   []excelize.Selection{{SQRef: "A2", ActiveCell: "A2", Pane: "bottomLeft"}},
	})
}