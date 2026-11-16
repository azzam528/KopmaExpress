# PRD & Spesifikasi Teknis
# KOPMA Express — Sistem Informasi & Analitik Prediktif Koperasi Mahasiswa Berbasis High-Performance Go

| Atribut | Keterangan |
|---|---|
| Mata Kuliah | Big Data 2 |
| Versi Dokumen | 1.0 |
| Bahasa / Runtime | Go 1.22+ |
| Framework | Go Fiber v2 |
| Database | MongoDB (replica set) |
| Status | Siap dieksekusi |

---

## Daftar Isi
1. [Ringkasan Eksekutif & Keunggulan Go](#1-ringkasan-eksekutif--keunggulan-go)
2. [Arsitektur Sistem & Alur Konkurensi](#2-arsitektur-sistem--alur-konkurensi)
3. [Spesifikasi Fungsional (User Stories & Acceptance Criteria)](#3-spesifikasi-fungsional)
4. [Struct Golang & Skema Database](#4-struct-golang--skema-database)
5. [Format Kolom Excel "Sistem Lama"](#5-format-kolom-excel-sistem-lama)
6. [Spesifikasi Endpoint REST API](#6-spesifikasi-endpoint-rest-api)
7. [Algoritma & Kode Smart Demand Forecasting](#7-algoritma--kode-smart-demand-forecasting)
8. [Struktur Folder & Rancangan UI Dashboard](#8-struktur-folder--rancangan-ui-dashboard)
9. [Non-Functional Requirements, Pengujian & Rencana Eksekusi](#9-non-functional-requirements-pengujian--rencana-eksekusi)

---

## 1. Ringkasan Eksekutif & Keunggulan Go

### 1.1 Ringkasan
Koperasi saat ini mencatat inventaris dan transaksi pada file Excel yang terpisah-pisah. Hal ini menyebabkan: (a) data tidak terintegrasi, (b) stok sulit dipantau, (c) tidak ada analitik penjualan, dan (d) keputusan restock bersifat intuitif tanpa dasar data.

**KOPMA Express** adalah aplikasi web yang:
1. **Memigrasikan** data Excel lama ke database melalui *ETL pipeline* konkuren (high-throughput).
2. **Menyediakan** pemesanan mandiri bagi anggota dengan validasi stok atomik.
3. **Menyajikan** dashboard analitik deskriptif: barang terlaris dan stok terbanyak.
4. **Memprediksi** kebutuhan stok bulan depan (*Smart Demand Forecasting*) dengan kombinasi **Weighted Moving Average (WMA)** dan **Simple Linear Regression (OLS)**, lalu memberi rekomendasi restock otomatis.

### 1.2 Tujuan Terukur (Success Metrics)
| Metrik | Target |
|---|---|
| Waktu import Excel 50.000 baris | < 10 detik (laptop dev standar) |
| Latensi endpoint order (p95) | < 100 ms pada 200 request konkuren |
| Overselling (stok negatif) | 0 kasus pada uji konkurensi |
| Waktu kalkulasi forecast 1.000 produk | < 500 ms |
| Akurasi forecast (MAPE, backtest) | < 25% pada data dummy ber-tren |

### 1.3 Keunggulan Golang untuk Kasus Big Data Ini
| Aspek | Keunggulan Go | Dampak pada Proyek |
|---|---|---|
| **Konkurensi** | Goroutine ringan (~2 KB stack) + channel | Parsing, validasi, dan insert Excel berjalan paralel tanpa kompleksitas thread |
| **Performa HTTP** | Fiber berbasis `fasthttp`, alokasi memori minimal | Throughput tinggi untuk endpoint order & dashboard |
| **Memori efisien** | Streaming reader `excelize.Rows()` | File Excel besar tidak dimuat penuh ke RAM |
| **Komputasi numerik** | Tipe statis, kompilasi native | Forecast ribuan produk dihitung paralel dalam milidetik |
| **Deployment** | Single static binary | Mudah didemokan di mata kuliah, tanpa runtime tambahan |
| **Keamanan konkurensi** | `sync`, `context`, race detector (`go test -race`) | Bug race condition dapat dideteksi saat pengujian |

---

## 2. Arsitektur Sistem & Alur Konkurensi

### 2.1 Diagram Arsitektur

```
┌──────────────────────────── Browser (Tailwind + Alpine.js + Chart.js) ───────────────────────────┐
│  Dashboard │ Master Data │ Import Excel │ Form Pemesanan │ Forecast & Restock                    │
└───────────────────────────────────────────┬──────────────────────────────────────────────────────┘
                                            │ HTTP/JSON (Fetch API)
┌───────────────────────────────────────────▼──────────────────────────────────────────────────────┐
│                                   Go Fiber (fasthttp)                                            │
│  Middleware: Logger, Recover, CORS, RequestID, RateLimiter, BodyLimit                            │
│  ┌──────────────┐ ┌───────────────┐ ┌──────────────┐ ┌─────────────┐ ┌─────────────────────────┐ │
│  │ Handler      │ │ Handler       │ │ Handler      │ │ Handler     │ │ Handler Forecast        │ │
│  │ Product/Mem. │ │ Import/Export │ │ Order        │ │ Analytics   │ │                         │ │
│  └──────┬───────┘ └──────┬────────┘ └──────┬───────┘ └──────┬──────┘ └───────────┬─────────────┘ │
│         ▼                ▼                 ▼                ▼                    ▼               │
│  ┌──────────────────────────────  Service Layer (business logic)  ───────────────────────────┐   │
│  │ ProductSvc │ ETLService (Worker Pool) │ OrderSvc (atomic) │ AnalyticsSvc │ ForecastSvc   │   │
│  └─────────────────────────────────────────┬──────────────────────────────────────────────┘   │
│                                            ▼                                                    │
│  ┌──────────────────────────────  Repository Layer (Mongo driver)  ──────────────────────────┐ │
└──┴────────────────────────────────────────┬─────────────────────────────────────────────────┴─┘
                                             ▼
                        MongoDB (replica set: products, members, orders,
                                 monthly_sales_history, forecast_results, import_jobs)
```

### 2.2 Alur Konkurensi: Pipeline ETL Import Excel

Pola: **Producer → Parser/Validator Workers (fan-out) → Batch Writer (fan-in)**.

```
 upload .xlsx ──► Producer (1 goroutine)
                   │  excelize.Rows() streaming, kelompokkan 500 baris = 1 chunk
                   ▼
              jobs chan []RawRow  (buffered)
        ┌──────────┼──────────┐
        ▼          ▼          ▼
    Worker 1   Worker 2 ... Worker N   (N = runtime.NumCPU())
    clean + validate + normalize → []ProductDoc / []HistoryDoc
        └──────────┼──────────┘
                   ▼
             results chan Batch
                   ▼
        Writer goroutine(s): BulkWrite (Upsert, ordered=false)
                   ▼
        Aggregator: hitung inserted / updated / skipped / errors → ImportJob
```

**Aturan teknis:**
- Hanya **satu** goroutine yang membaca file (excelize tidak thread-safe untuk pembacaan paralel pada satu `File`).
- `context.Context` dengan timeout dan *cancel* disebarkan ke semua goroutine; error fatal membatalkan pipeline.
- Channel *buffered* untuk memberi *back-pressure* (mencegah lonjakan memori).
- `sync.WaitGroup` memastikan semua worker selesai sebelum channel `results` ditutup.
- Error per baris dikumpulkan (maks. 1.000 entri) dan tidak menghentikan proses.
- Import berjalan **asynchronous**: endpoint langsung mengembalikan `job_id` (HTTP 202), status dipantau via polling.

### 2.3 Alur Konkurensi: Pemesanan Atomik
- Pengurangan stok memakai **conditional atomic update** satu dokumen:
  `UpdateOne({_id, stock: {$gte: qty}}, {$inc: {stock: -qty}})`.
  Jika `MatchedCount == 0` maka stok tidak cukup (tanpa race condition, karena operasi atomik per dokumen).
- Pesanan multi-item dibungkus **MongoDB multi-document transaction** (`session.WithTransaction`) sehingga jika satu item gagal, seluruh pesanan di-*rollback*.
- Agregasi `monthly_sales_history` diperbarui dalam transaksi yang sama menggunakan `$inc` + upsert.

### 2.4 Alur Konkurensi: Forecast
- Semua produk dibaca satu kali, lalu dibagi ke **worker pool** (fan-out). Setiap worker menghitung WMA + regresi untuk produk miliknya (fungsi murni, tanpa shared state).
- Hasil ditulis ke `forecast_results` dengan `BulkWrite` upsert (key: `product_id + target_month`).

### 2.5 Keputusan Pemilihan Database: MongoDB
| Pertimbangan | Alasan |
|---|---|
| Aggregation Pipeline | `$group`, `$sort`, `$limit`, `$lookup` cocok untuk Top Selling & riwayat bulanan |
| BulkWrite + Upsert | Native, efisien untuk migrasi ribuan baris dengan duplikasi |
| Skema fleksibel | Item pesanan ditanam (*embedded*) dalam dokumen order |
| Transaksi | Didukung pada replica set (cukup 1 node replica set untuk dev) |

> **Catatan dev:** jalankan MongoDB sebagai single-node replica set agar transaksi aktif:
> `mongod --replSet rs0` lalu `rs.initiate()`; atau gunakan Docker Compose (lihat §9.4).

---

## 3. Spesifikasi Fungsional

### 3.1 Aktor
| Aktor | Deskripsi |
|---|---|
| **Admin/Pengurus** | Mengelola barang, anggota, import Excel, melihat analitik & forecast |
| **Anggota** | Membuat pesanan mandiri |
| **Sistem** | Menjalankan ETL, kalkulasi forecast, alert |

> Autentikasi tidak menjadi fokus tugas. Opsional: *API key* sederhana atau role statis via header `X-Role`. Dokumen ini mengasumsikan **tanpa login** pada MVP, dengan titik ekstensi middleware auth.

### 3.2 Modul A — Manajemen Data & ETL

**US-A1** *Sebagai admin, saya ingin menambah/mengedit/menghapus barang secara manual agar data selalu terbarui.*
- AC1: Form memuat Kode Barang, Nama, Kategori, Harga, Stok Saat Ini.
- AC2: `kode_barang` unik (case-insensitive, otomatis `UPPERCASE` + `TrimSpace`). Duplikat → HTTP 409.
- AC3: Harga ≥ 0 dan stok ≥ 0 (integer). Validasi gagal → HTTP 422 dengan daftar error per field.
- AC4: Hapus barang memakai *soft delete* (`is_active=false`) agar riwayat tetap konsisten.

**US-A2** *Sebagai admin, saya ingin mengelola data anggota (NIM, nama, prodi/unit, kontak).*
- AC1: `nim` unik. Duplikat → HTTP 409.
- AC2: Pencarian anggota by NIM/nama (prefix, case-insensitive) untuk *autocomplete* di form pesanan.

**US-A3** *Sebagai admin, saya ingin mengunggah Excel sistem lama agar data historis termigrasi otomatis.*
- AC1: Hanya menerima `.xlsx`, ukuran maks. 50 MB (konfigurabel). Selain itu → HTTP 413/415.
- AC2: Sistem membaca sheet `Barang`, `Anggota` (opsional), dan `Riwayat_Penjualan` sesuai §5.
- AC3: **Data cleaning**: hapus baris kosong total; `TrimSpace`; kode barang di-`UPPERCASE`; harga seperti `"Rp 12.500,00"`, `"12,500"`, `"12500.0"` dikonversi ke `float64`/`int64`; stok non-numerik → baris ditolak dan dicatat sebagai error.
- AC4: **Duplikasi**: baris dengan `kode_barang` yang sama (dalam file atau sudah ada di DB) di-*upsert* (update harga/nama/kategori; stok mengikuti kebijakan `stock_mode`: `replace` atau `skip`).
- AC5: Riwayat penjualan di-upsert dengan key `(kode_barang, tahun, bulan)`; mengimpor ulang file yang sama **tidak** menggandakan data (idempoten).
- AC6: Respons awal HTTP 202 + `job_id`. Endpoint status menampilkan progres (`processed_rows`, `total_rows`), `inserted`, `updated`, `skipped`, `failed`, dan daftar error (nomor baris + alasan).
- AC7: Import 50.000 baris selesai < 10 detik; memori puncak < 300 MB.

**US-A4** *Sebagai admin, saya ingin mengunduh contoh Excel "Sistem Lama" agar bisa dipakai untuk demo.*
- AC1: Endpoint menghasilkan file `.xlsx` dengan ≥ 300 barang, ≥ 20 anggota, dan riwayat penjualan 6 bulan per barang (jumlah baris dapat diatur via query `items`, `months`).
- AC2: Data memuat pola variatif (naik, turun, stabil, musiman) agar forecast terlihat bermakna; seed random dapat ditentukan.
- AC3: Sengaja menyisipkan sebagian kecil "kotoran" (baris kosong, harga berformat string, duplikat) untuk mendemokan data cleaning (parameter `dirty=true`).

### 3.3 Modul B — Transaksi & Pemesanan

**US-B1** *Sebagai anggota, saya ingin memesan beberapa barang sekaligus dengan metode pembayaran pilihan.*
- AC1: Form: pilih anggota (NIM), tambah satu/lebih baris item (barang + qty ≥ 1), pilih metode pembayaran (`CASH`, `TRANSFER`, `QRIS`, `KASBON`).
- AC2: Sistem menghitung subtotal & total menggunakan **harga saat transaksi** (snapshot harga disimpan di item order).
- AC3: Jika stok salah satu barang kurang → seluruh pesanan ditolak (HTTP 409) dengan rincian barang dan stok tersedia.
- AC4: Stok berkurang secara atomik; 200 request konkuren pada stok = 100 menghasilkan **tepat 100 qty** terjual, tanpa stok negatif.
- AC5: Nomor pesanan unik berformat `ORD-YYYYMMDD-XXXXXX`.
- AC6: Riwayat pesanan dapat dilihat (filter: anggota, rentang tanggal, status), diurutkan terbaru.
- AC7: Pembatalan pesanan (status `CANCELLED`) mengembalikan stok dan mengurangi agregasi riwayat bulanan (dalam satu transaksi).

### 3.4 Modul C — Analitik Deskriptif

**US-C1** *Barang Terlaris.*
- AC1: Menampilkan Top-N (default 10, maks. 50) berdasarkan total qty terjual.
- AC2: Sumber data = `monthly_sales_history` (migrasi Excel **+** pesanan baru, karena order baru ikut di-`$inc` ke koleksi ini).
- AC3: Filter periode: `all`, `3m`, `6m`, atau rentang `from`–`to` (format `YYYY-MM`).
- AC4: Tampil sebagai *horizontal bar chart* dan tabel (peringkat, kode, nama, kategori, total qty, total omzet).

**US-C2** *Stok Terbanyak.*
- AC1: Top-N barang dengan `stock` terbesar saat ini.
- AC2: Kolom valuasi aset = `stock × price`; ditampilkan juga total valuasi seluruh gudang.
- AC3: Grafik bar + tabel.

### 3.5 Modul D — Analitik Prediktif

**US-D1** *Sebagai admin, saya ingin melihat prediksi kebutuhan stok bulan depan per barang.*
- AC1: Sistem menghitung prediksi dari riwayat bulanan (minimal 3 bulan data; < 3 bulan → status `INSUFFICIENT_DATA` dan prediksi memakai rata-rata sederhana).
- AC2: Bulan berjalan yang belum selesai **diproyeksikan** (*run-rate*: `qty_sejauh_ini / hari_berjalan × hari_dalam_bulan`) sebelum masuk deret waktu, agar tidak menjatuhkan tren.
- AC3: Hasil prediksi = kombinasi berbobot WMA dan regresi linear (lihat §7), dibulatkan ke atas (`ceil`) dan tidak negatif.
- AC4: Tersedia tombol *Recalculate*; hasil juga disimpan di `forecast_results` untuk dibandingkan antar waktu.

**US-D2** *Sebagai admin, saya ingin rekomendasi restock & alert otomatis.*
- AC1: Status ditentukan dengan aturan berikut (urutan evaluasi penting):

| Prioritas | Kondisi | Status | Rekomendasi |
|---|---|---|---|
| 1 | `stok < prediksi` | 🔴 **CRITICAL / WAJIB RESTOCK** | `restock_qty = prediksi − stok` |
| 2 | `stok` termasuk Top-N Stok Terbanyak **dan** `prediksi` termasuk kuantil bawah (`prediksi < 50%` median prediksi semua barang, atau `stok ≥ 3 × prediksi`) | 🟡 **OVERSTOCK WARNING** | Tahan pembelian; pertimbangkan promo |
| 3 | Selain itu | 🟢 **AMAN** | – |

- AC2: Ringkasan dashboard menampilkan jumlah barang per status; daftar CRITICAL diurutkan dari `restock_qty` terbesar.
- AC3: Ada badge alert pada navbar berisi jumlah barang CRITICAL.
- AC4: Ekspor daftar restock ke `.xlsx` (opsional, nilai tambah).

---

## 4. Struct Golang & Skema Database

### 4.1 Koleksi & Indeks

| Koleksi | Indeks |
|---|---|
| `products` | `{kode_barang:1}` **unique**; `{stock:-1}`; `{category:1}`; `{is_active:1}` |
| `members` | `{nim:1}` **unique**; text/prefix index `{name:1}` |
| `orders` | `{order_no:1}` **unique**; `{member_nim:1, created_at:-1}`; `{created_at:-1}`; `{status:1}` |
| `monthly_sales_history` | `{product_code:1, year:1, month:1}` **unique**; `{year:1, month:1}` |
| `forecast_results` | `{product_code:1, target_month:1}` **unique**; `{status:1}` |
| `import_jobs` | `{created_at:-1}` |

### 4.2 Struct (`internal/domain`)

```go
package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ===== Product =====
type Product struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"  json:"id"`
	Code      string             `bson:"kode_barang"    json:"kode_barang" validate:"required,max=30"`
	Name      string             `bson:"name"           json:"nama_barang" validate:"required,max=150"`
	Category  string             `bson:"category"       json:"kategori"    validate:"required,max=60"`
	Price     int64              `bson:"price"          json:"harga"       validate:"gte=0"` // Rupiah (integer)
	Stock     int64              `bson:"stock"          json:"stok"        validate:"gte=0"`
	IsActive  bool               `bson:"is_active"      json:"is_active"`
	CreatedAt time.Time          `bson:"created_at"     json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"     json:"updated_at"`
}

// ===== Member =====
type Member struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	NIM       string             `bson:"nim"           json:"nim"   validate:"required,max=20"`
	Name      string             `bson:"name"          json:"nama"  validate:"required,max=120"`
	Unit      string             `bson:"unit"          json:"unit"` // prodi/unit/angkatan
	Phone     string             `bson:"phone"         json:"telepon"`
	IsActive  bool               `bson:"is_active"     json:"is_active"`
	JoinedAt  time.Time          `bson:"joined_at"     json:"joined_at"`
	CreatedAt time.Time          `bson:"created_at"    json:"created_at"`
}

// ===== Order =====
type PaymentMethod string

const (
	PayCash     PaymentMethod = "CASH"
	PayTransfer PaymentMethod = "TRANSFER"
	PayQRIS     PaymentMethod = "QRIS"
	PayKasbon   PaymentMethod = "KASBON"
)

type OrderStatus string

const (
	OrderCompleted OrderStatus = "COMPLETED"
	OrderCancelled OrderStatus = "CANCELLED"
)

type OrderItem struct {
	ProductCode string `bson:"product_code" json:"kode_barang"`
	ProductName string `bson:"product_name" json:"nama_barang"` // snapshot
	Qty         int64  `bson:"qty"          json:"qty"`
	UnitPrice   int64  `bson:"unit_price"   json:"harga_satuan"` // snapshot harga saat transaksi
	Subtotal    int64  `bson:"subtotal"     json:"subtotal"`
}

type Order struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OrderNo       string             `bson:"order_no"       json:"order_no"`
	MemberNIM     string             `bson:"member_nim"     json:"nim"`
	MemberName    string             `bson:"member_name"    json:"nama_anggota"`
	Items         []OrderItem        `bson:"items"          json:"items"`
	TotalAmount   int64              `bson:"total_amount"   json:"total"`
	Payment       PaymentMethod      `bson:"payment_method" json:"metode_pembayaran"`
	Status        OrderStatus        `bson:"status"         json:"status"`
	CreatedAt     time.Time          `bson:"created_at"     json:"created_at"`
}

// ===== MonthlySalesHistory =====
// Satu dokumen = penjualan 1 barang pada 1 bulan.
// Sumber: migrasi Excel (Source="excel") + akumulasi order baru (Source="live"/"mixed").
type MonthlySalesHistory struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProductCode string             `bson:"product_code"  json:"kode_barang"`
	Year        int                `bson:"year"          json:"tahun"`
	Month       int                `bson:"month"         json:"bulan"` // 1-12
	QtySold     int64              `bson:"qty_sold"      json:"qty_terjual"`
	Revenue     int64              `bson:"revenue"       json:"omzet"`
	Source      string             `bson:"source"        json:"sumber"` // excel | live | mixed
	UpdatedAt   time.Time          `bson:"updated_at"    json:"updated_at"`
}

// ===== ForecastResult =====
type RestockStatus string

const (
	StatusCritical         RestockStatus = "CRITICAL"
	StatusOverstock        RestockStatus = "OVERSTOCK_WARNING"
	StatusSafe             RestockStatus = "SAFE"
	StatusInsufficientData RestockStatus = "INSUFFICIENT_DATA"
)

type ForecastResult struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"    json:"id"`
	ProductCode     string             `bson:"product_code"     json:"kode_barang"`
	ProductName     string             `bson:"product_name"     json:"nama_barang"`
	TargetMonth     string             `bson:"target_month"     json:"bulan_target"` // "2026-12"
	CurrentStock    int64              `bson:"current_stock"    json:"stok_saat_ini"`
	WMA             float64            `bson:"wma"              json:"wma"`
	LinearTrend     float64            `bson:"linear_trend"     json:"regresi_linear"`
	Slope           float64            `bson:"slope"            json:"slope"`
	R2              float64            `bson:"r2"               json:"r_squared"`
	PredictedDemand int64              `bson:"predicted_demand" json:"prediksi_bulan_depan"`
	RestockQty      int64              `bson:"restock_qty"      json:"rekomendasi_restock"`
	Status          RestockStatus      `bson:"status"           json:"status"`
	DataPoints      int                `bson:"data_points"      json:"jumlah_data_bulan"`
	CalculatedAt    time.Time          `bson:"calculated_at"    json:"calculated_at"`
}

// ===== ImportJob =====
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

type RowError struct {
	Sheet  string `bson:"sheet"  json:"sheet"`
	Row    int    `bson:"row"    json:"row"`
	Reason string `bson:"reason" json:"reason"`
}
```

---

## 5. Format Kolom Excel "Sistem Lama"

File: `sistem_lama_koperasi.xlsx` — terdiri dari **3 sheet**.

### 5.1 Sheet `Barang` (header di baris 1)
| Kolom | Header | Tipe di Excel | Contoh | Aturan Cleaning |
|---|---|---|---|---|
| A | `Kode Barang` | Teks | `BRG-0001` | Trim, UPPERCASE, wajib |
| B | `Nama Barang` | Teks | `Buku Tulis Sidu 38 Lembar` | Trim, wajib |
| C | `Kategori` | Teks | `ATK` | Trim, default `Lainnya` |
| D | `Harga` | Teks/Angka campuran | `Rp 3.500`, `3500`, `3,500.00` | Hilangkan `Rp`, spasi, pemisah ribuan → int64 |
| E | `Stok` | Angka/Teks | `120`, `"120 pcs"` | Ambil digit; non-numerik → error |

### 5.2 Sheet `Anggota` (header di baris 1) — opsional
| Kolom | Header | Contoh |
|---|---|---|
| A | `NIM` | `2210101001` |
| B | `Nama` | `Andi Pratama` |
| C | `Unit/Prodi` | `Teknik Informatika` |
| D | `Telepon` | `081234567890` |
| E | `Tanggal Bergabung` | `2022-08-15` (atau `15/08/2022`) |

### 5.3 Sheet `Riwayat_Penjualan` (header di baris 1)
Format agregat bulanan (efisien untuk forecasting):

| Kolom | Header | Contoh | Catatan |
|---|---|---|---|
| A | `Kode Barang` | `BRG-0001` | Harus ada di sheet Barang/DB; jika tidak → error baris |
| B | `Tahun` | `2026` | Integer 2000–2100 |
| C | `Bulan` | `5` | Integer 1–12 (juga menerima `Mei`/`May`) |
| D | `Qty Terjual` | `87` | Integer ≥ 0 |
| E | `Total Omzet` | `304500` | Opsional; default `qty × harga` |

> **Alternatif (varian transaksi):** jika riwayat berupa transaksi harian (`Tanggal`, `Kode Barang`, `Qty`), service meng-*group* per `(kode, tahun, bulan)` saat parsing. Deteksi varian lewat header; implementasikan varian bulanan terlebih dahulu.

### 5.4 Spesifikasi Generator Dummy (`excelize`)
- `Barang`: N barang (default 300) dari 8 kategori (ATK, Makanan, Minuman, Seragam, Buku, Elektronik, Kebersihan, Lainnya) dengan nama realistis dan harga Rp 1.000–Rp 250.000.
- `Anggota`: 30 anggota dummy.
- `Riwayat_Penjualan`: untuk setiap barang, M bulan terakhir (default 6) dengan pola acak: `naik`, `turun`, `stabil`, `musiman`, ditambah *noise* ±15%.
- Opsi `dirty=true`: ~2% baris kosong, ~3% harga berformat `Rp x.xxx`, ~1% duplikat kode barang.
- Gaya: header tebal berwarna, lebar kolom otomatis, *freeze pane* baris 1.

---

## 6. Spesifikasi Endpoint REST API

**Base URL:** `/api/v1` — **Content-Type:** `application/json` (kecuali upload/download).

**Format respons standar:**
```json
{ "success": true,  "data": { }, "meta": { }, "error": null }
{ "success": false, "data": null, "error": { "code": "VALIDATION_ERROR", "message": "…", "details": [ { "field": "harga", "message": "harus ≥ 0" } ] } }
```

**Kode error:** `VALIDATION_ERROR` (422), `NOT_FOUND` (404), `DUPLICATE` (409), `INSUFFICIENT_STOCK` (409), `UNSUPPORTED_FILE` (415), `FILE_TOO_LARGE` (413), `INTERNAL` (500).

### 6.1 Produk
| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/products?q=&category=&page=1&limit=20&sort=name` | Daftar + pencarian + paginasi |
| GET | `/products/:code` | Detail barang |
| POST | `/products` | Tambah barang |
| PUT | `/products/:code` | Edit barang |
| DELETE | `/products/:code` | Soft delete |

**POST `/products` — Request**
```json
{ "kode_barang": "BRG-0001", "nama_barang": "Buku Tulis Sidu", "kategori": "ATK", "harga": 3500, "stok": 120 }
```
**Response 201**
```json
{ "success": true, "data": { "id": "665f…", "kode_barang": "BRG-0001", "nama_barang": "Buku Tulis Sidu", "kategori": "ATK", "harga": 3500, "stok": 120, "is_active": true, "created_at": "2026-11-16T08:40:00+07:00" }, "error": null }
```

### 6.2 Anggota
| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/members?q=&page=&limit=` | Daftar/pencarian (autocomplete) |
| GET | `/members/:nim` | Detail |
| POST | `/members` | Tambah |
| PUT | `/members/:nim` | Edit |
| DELETE | `/members/:nim` | Nonaktifkan |

**POST `/members` — Request**
```json
{ "nim": "2210101001", "nama": "Andi Pratama", "unit": "Teknik Informatika", "telepon": "081234567890" }
```

### 6.3 Import / Export Excel
| Method | Endpoint | Fungsi |
|---|---|---|
| POST | `/import/excel` | Upload `.xlsx` (multipart), mulai ETL async |
| GET | `/import/jobs/:id` | Status & hasil import |
| GET | `/import/jobs` | Riwayat import |
| GET | `/export/dummy-excel?items=300&months=6&dirty=true&seed=42` | Unduh Excel dummy |
| GET | `/export/restock-excel` | (Opsional) Unduh daftar restock |

**POST `/import/excel`** — `multipart/form-data`: field `file` (wajib), `stock_mode` (`replace`|`skip`, default `replace`).

**Response 202**
```json
{ "success": true, "data": { "job_id": "6660…", "status": "RUNNING" }, "error": null }
```
**GET `/import/jobs/:id` — Response 200**
```json
{
  "success": true,
  "data": {
    "job_id": "6660…", "file_name": "sistem_lama_koperasi.xlsx", "status": "DONE",
    "total_rows": 2103, "processed_rows": 2103,
    "inserted": 295, "updated": 5, "skipped": 0, "failed": 3,
    "errors": [ { "sheet": "Barang", "row": 42, "reason": "stok tidak valid: 'N/A'" } ],
    "started_at": "2026-11-16T08:41:00+07:00", "finished_at": "2026-11-16T08:41:03+07:00"
  },
  "error": null
}
```

### 6.4 Pesanan
| Method | Endpoint | Fungsi |
|---|---|---|
| POST | `/orders` | Buat pesanan (atomik) |
| GET | `/orders?nim=&from=&to=&status=&page=&limit=` | Riwayat pesanan |
| GET | `/orders/:order_no` | Detail pesanan |
| POST | `/orders/:order_no/cancel` | Batalkan & kembalikan stok |

**POST `/orders` — Request**
```json
{
  "nim": "2210101001",
  "metode_pembayaran": "QRIS",
  "items": [
    { "kode_barang": "BRG-0001", "qty": 3 },
    { "kode_barang": "BRG-0007", "qty": 1 }
  ]
}
```
**Response 201**
```json
{
  "success": true,
  "data": {
    "order_no": "ORD-20261116-000123", "nim": "2210101001", "nama_anggota": "Andi Pratama",
    "items": [
      { "kode_barang": "BRG-0001", "nama_barang": "Buku Tulis Sidu", "qty": 3, "harga_satuan": 3500, "subtotal": 10500 },
      { "kode_barang": "BRG-0007", "nama_barang": "Pulpen Pilot", "qty": 1, "harga_satuan": 5000, "subtotal": 5000 }
    ],
    "total": 15500, "metode_pembayaran": "QRIS", "status": "COMPLETED", "created_at": "2026-11-16T08:45:10+07:00"
  },
  "error": null
}
```
**Response 409 (stok kurang)**
```json
{
  "success": false, "data": null,
  "error": {
    "code": "INSUFFICIENT_STOCK", "message": "Stok tidak mencukupi",
    "details": [ { "kode_barang": "BRG-0007", "diminta": 5, "tersedia": 2 } ]
  }
}
```

### 6.5 Analitik Deskriptif
| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/analytics/top-selling?limit=10&period=6m` | Barang terlaris |
| GET | `/analytics/top-stock?limit=10` | Stok terbanyak + valuasi |
| GET | `/analytics/summary` | KPI ringkasan dashboard |
| GET | `/analytics/monthly-trend?code=BRG-0001` | Tren bulanan 1 barang (untuk grafik) |

**GET `/analytics/top-selling` — Response**
```json
{
  "success": true,
  "data": [
    { "rank": 1, "kode_barang": "BRG-0012", "nama_barang": "Air Mineral 600ml", "kategori": "Minuman", "total_qty": 1840, "total_omzet": 5520000 }
  ],
  "meta": { "period": "6m", "limit": 10 }, "error": null
}
```
**GET `/analytics/top-stock` — Response**
```json
{
  "success": true,
  "data": [
    { "rank": 1, "kode_barang": "BRG-0105", "nama_barang": "Kertas HVS A4", "kategori": "ATK", "stok": 900, "harga": 48000, "valuasi_aset": 43200000 }
  ],
  "meta": { "total_valuasi_gudang": 912450000 }, "error": null
}
```
**GET `/analytics/summary` — Response**
```json
{ "success": true, "data": { "total_produk": 300, "total_anggota": 30, "total_pesanan_bulan_ini": 87, "omzet_bulan_ini": 15400000, "total_valuasi_gudang": 912450000, "critical_count": 18 }, "error": null }
```

### 6.6 Forecast
| Method | Endpoint | Fungsi |
|---|---|---|
| POST | `/forecast/recalculate` | Hitung ulang semua barang (sinkron; cepat) |
| GET | `/forecast?status=&q=&page=&limit=&sort=restock_qty` | Daftar hasil forecast + filter status |
| GET | `/forecast/:code` | Detail 1 barang (deret historis + garis tren + prediksi) |
| GET | `/forecast/summary` | Jumlah per status |

**GET `/forecast` — Response**
```json
{
  "success": true,
  "data": [
    {
      "kode_barang": "BRG-0012", "nama_barang": "Air Mineral 600ml", "bulan_target": "2026-12",
      "stok_saat_ini": 150, "wma": 298.4, "regresi_linear": 312.0, "r_squared": 0.84,
      "prediksi_bulan_depan": 304, "rekomendasi_restock": 154, "status": "CRITICAL",
      "jumlah_data_bulan": 6
    }
  ],
  "meta": { "page": 1, "limit": 20, "total": 300, "target_month": "2026-12", "calculated_at": "2026-11-16T08:50:00+07:00" },
  "error": null
}
```
**GET `/forecast/summary` — Response**
```json
{ "success": true, "data": { "CRITICAL": 18, "OVERSTOCK_WARNING": 9, "SAFE": 266, "INSUFFICIENT_DATA": 7 }, "error": null }
```

### 6.7 Rute Halaman (HTML statis/template)
`/` (dashboard), `/products`, `/members`, `/import`, `/orders/new`, `/orders`, `/forecast`.

---

## 7. Algoritma & Kode Smart Demand Forecasting

### 7.1 Persiapan Deret Waktu
1. Ambil `monthly_sales_history` per produk, urut `(year, month)`.
2. **Isi bulan kosong dengan 0** (bulan tanpa penjualan tetap menjadi titik data) pada rentang bulan pertama → bulan lalu.
3. **Bulan berjalan** diproyeksikan dengan *run-rate*: `proyeksi = qty_sejauh_ini × hari_dalam_bulan / hari_berjalan`. Pada hari ke-1–3 bulan, bulan berjalan **diabaikan** (data terlalu sedikit).
4. Gunakan maksimal **12 bulan terakhir**.
5. Jika `n < 3` → status `INSUFFICIENT_DATA`, prediksi = rata-rata.

### 7.2 Weighted Moving Average (WMA)
Bobot linear: data terbaru berbobot paling besar. Untuk window `k` data terakhir `y_{n-k+1}…y_n`:

```
WMA = Σ (w_i · y_i) / Σ w_i,   w_i = 1, 2, …, k   (k = min(n, 3))
```
Dengan `k=3`: `WMA = (1·y_{n-2} + 2·y_{n-1} + 3·y_n) / 6`.

### 7.3 Simple Linear Regression (Least Squares)
Dengan `x_i = 1..n` dan `y_i` = qty bulanan:

```
slope  b = (n·Σxy − Σx·Σy) / (n·Σx² − (Σx)²)
intercept a = (Σy − b·Σx) / n
ŷ(n+1) = a + b·(n+1)
R² = 1 − SS_res / SS_tot
```

### 7.4 Kombinasi (Ensemble Adaptif)
Bobot regresi disesuaikan dengan kualitas fit (`R²`) agar tren lemah tidak mendominasi:

```
w_reg = clamp(R², 0.2, 0.7)
w_wma = 1 − w_reg
Prediksi = max(0, ceil( w_wma · WMA + w_reg · ŷ(n+1) ))
```
Untuk mencegah ekstrapolasi liar, `ŷ` dibatasi: `0 ≤ ŷ ≤ 2 × max(y)`.

### 7.5 Aturan Status (lihat §3.5 AC D2)
Dievaluasi setelah prediksi tersedia; `OVERSTOCK` memerlukan konteks global (Top-N stok & median prediksi), sehingga dihitung pada tahap kedua setelah seluruh prediksi selesai.

### 7.6 Kode Inti (`internal/forecast/forecast.go`)

```go
package forecast

import (
	"math"
)

// Result adalah keluaran forecast murni untuk satu deret waktu.
type Result struct {
	WMA        float64
	Linear     float64
	Slope      float64
	Intercept  float64
	R2         float64
	Predicted  int64
	DataPoints int
	Sufficient bool
}

// WeightedMovingAverage menghitung WMA dengan bobot linear (1..k) pada k data terakhir.
func WeightedMovingAverage(y []float64, k int) float64 {
	n := len(y)
	if n == 0 {
		return 0
	}
	if k > n {
		k = n
	}
	var num, den float64
	for i := 0; i < k; i++ {
		w := float64(i + 1)            // bobot 1..k, terbaru = k
		num += w * y[n-k+i]
		den += w
	}
	return num / den
}

// LinearRegression melakukan Ordinary Least Squares dengan x = 1..n.
// Mengembalikan slope, intercept, dan R².
func LinearRegression(y []float64) (slope, intercept, r2 float64) {
	n := float64(len(y))
	if n < 2 {
		return 0, 0, 0
	}
	var sumX, sumY, sumXY, sumXX float64
	for i, v := range y {
		x := float64(i + 1)
		sumX += x
		sumY += v
		sumXY += x * v
		sumXX += x * x
	}
	denom := n*sumXX - sumX*sumX
	if denom == 0 {
		return 0, sumY / n, 0
	}
	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n

	// R²
	mean := sumY / n
	var ssTot, ssRes float64
	for i, v := range y {
		pred := intercept + slope*float64(i+1)
		ssRes += (v - pred) * (v - pred)
		ssTot += (v - mean) * (v - mean)
	}
	if ssTot == 0 {
		r2 = 1 // deret konstan: fit sempurna
	} else {
		r2 = 1 - ssRes/ssTot
	}
	return slope, intercept, r2
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

func maxOf(y []float64) float64 {
	m := y[0]
	for _, v := range y[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func mean(y []float64) float64 {
	var s float64
	for _, v := range y {
		s += v
	}
	return s / float64(len(y))
}

// Predict menggabungkan WMA dan regresi linear menjadi prediksi bulan depan.
func Predict(y []float64) Result {
	n := len(y)
	res := Result{DataPoints: n}

	if n == 0 {
		return res
	}
	if n < 3 { // data tidak cukup -> rata-rata sederhana
		avg := mean(y)
		res.WMA, res.Linear = avg, avg
		res.Predicted = int64(math.Ceil(avg))
		return res
	}
	res.Sufficient = true

	res.WMA = WeightedMovingAverage(y, 3)
	slope, intercept, r2 := LinearRegression(y)
	res.Slope, res.Intercept, res.R2 = slope, intercept, r2

	next := intercept + slope*float64(n+1)
	res.Linear = clamp(next, 0, 2*maxOf(y)) // batasi ekstrapolasi

	wReg := clamp(r2, 0.2, 0.7)
	wWma := 1 - wReg
	blended := wWma*res.WMA + wReg*res.Linear

	res.Predicted = int64(math.Ceil(math.Max(0, blended)))
	return res
}
```

### 7.7 Penyiapan Deret Waktu (`internal/forecast/series.go`)

```go
package forecast

import "time"

type MonthKey struct{ Year, Month int }

func (m MonthKey) Time() time.Time {
	return time.Date(m.Year, time.Month(m.Month), 1, 0, 0, 0, 0, time.UTC)
}

// BuildSeries membentuk deret bulanan kontinu (bulan kosong = 0) hingga bulan lalu,
// lalu menambahkan proyeksi run-rate bulan berjalan bila hari berjalan >= 4.
func BuildSeries(sales map[MonthKey]int64, now time.Time, maxMonths int) []float64 {
	if len(sales) == 0 {
		return nil
	}
	// cari bulan paling awal
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for k := range sales {
		if t := k.Time(); t.Before(first) {
			first = t
		}
	}
	currentStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	var series []float64
	for t := first; t.Before(currentStart); t = t.AddDate(0, 1, 0) {
		series = append(series, float64(sales[MonthKey{t.Year(), int(t.Month())}]))
	}

	// bulan berjalan: proyeksi run-rate
	if now.Day() >= 4 {
		sofar := float64(sales[MonthKey{now.Year(), int(now.Month())}])
		daysInMonth := float64(currentStart.AddDate(0, 1, -1).Day())
		series = append(series, sofar*daysInMonth/float64(now.Day()))
	}

	if len(series) > maxMonths {
		series = series[len(series)-maxMonths:]
	}
	return series
}
```

### 7.8 Worker Pool Forecast & Penentuan Status (`internal/service/forecast_service.go`)

```go
package service

import (
	"context"
	"runtime"
	"sort"
	"sync"
	"time"

	"smartcoop/internal/domain"
	"smartcoop/internal/forecast"
)

type productSales struct {
	Product domain.Product
	Sales   map[forecast.MonthKey]int64
}

func (s *ForecastService) Recalculate(ctx context.Context) error {
	now := time.Now()
	target := now.AddDate(0, 1, 0).Format("2006-01")

	data, err := s.repo.LoadProductSales(ctx) // []productSales (1 aggregation pipeline)
	if err != nil {
		return err
	}

	in := make(chan productSales, 256)
	out := make(chan domain.ForecastResult, 256)

	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ps := range in {
				series := forecast.BuildSeries(ps.Sales, now, 12)
				r := forecast.Predict(series)
				out <- domain.ForecastResult{
					ProductCode: ps.Product.Code, ProductName: ps.Product.Name,
					TargetMonth: target, CurrentStock: ps.Product.Stock,
					WMA: r.WMA, LinearTrend: r.Linear, Slope: r.Slope, R2: r.R2,
					PredictedDemand: r.Predicted, DataPoints: r.DataPoints,
					Status: domain.StatusSafe, CalculatedAt: now,
				}
			}
		}()
	}
	go func() { // producer
		defer close(in)
		for _, ps := range data {
			select {
			case in <- ps:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { wg.Wait(); close(out) }()

	var results []domain.ForecastResult
	for r := range out {
		results = append(results, r)
	}

	assignStatus(results, s.topStockSize) // tahap 2: butuh konteks global
	return s.repo.UpsertForecasts(ctx, results)
}

func assignStatus(rs []domain.ForecastResult, topN int) {
	// median prediksi
	preds := make([]int64, 0, len(rs))
	for _, r := range rs {
		preds = append(preds, r.PredictedDemand)
	}
	sort.Slice(preds, func(i, j int) bool { return preds[i] < preds[j] })
	var median int64
	if len(preds) > 0 {
		median = preds[len(preds)/2]
	}

	// top-N stok terbanyak
	idx := make([]int, len(rs))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return rs[idx[a]].CurrentStock > rs[idx[b]].CurrentStock })
	topSet := map[int]bool{}
	for i := 0; i < topN && i < len(idx); i++ {
		topSet[idx[i]] = true
	}

	for i := range rs {
		r := &rs[i]
		switch {
		case r.DataPoints < 3:
			r.Status = domain.StatusInsufficientData
		case r.CurrentStock < r.PredictedDemand: // prioritas 1
			r.Status = domain.StatusCritical
			r.RestockQty = r.PredictedDemand - r.CurrentStock
		case topSet[i] && (float64(r.PredictedDemand) < 0.5*float64(median) ||
			r.CurrentStock >= 3*max64(r.PredictedDemand, 1)): // prioritas 2
			r.Status = domain.StatusOverstock
		default:
			r.Status = domain.StatusSafe
		}
	}
}

func max64(a, b int64) int64 { if a > b { return a }; return b }
```

> Opsional: `gonum/stat` (`stat.LinearRegression`) dapat dipakai sebagai pembanding/validasi hasil regresi manual dalam unit test.

### 7.9 Contoh Perhitungan (untuk Laporan)
Riwayat 6 bulan: `y = [100, 110, 125, 130, 150, 160]`, stok = 140.
- WMA (k=3): `(1·130 + 2·150 + 3·160)/6 = 151.67`
- Regresi: `slope ≈ 11.43`, `intercept ≈ 87.33`, `ŷ(7) ≈ 167.3`, `R² ≈ 0.99`
- `w_reg = 0.7`, `w_wma = 0.3` → `0.3·151.67 + 0.7·167.3 ≈ 162.6` → **Prediksi = 163**
- Stok 140 < 163 → 🔴 **CRITICAL**, restock = **23 unit**.

### 7.10 Metode Evaluasi (Backtesting)
Untuk setiap produk, prediksi bulan ke-`n` memakai data `1..n−1`, lalu hitung **MAPE** = `mean(|aktual − prediksi| / aktual)`. Laporkan MAPE rata-rata WMA saja, regresi saja, dan ensemble sebagai bukti analisis di laporan tugas.

---

## 8. Struktur Folder & Rancangan UI Dashboard

### 8.1 Struktur Folder (Standard Go Layout + Clean Architecture Ringan)

```
smartcoop/
├── cmd/
│   └── server/
│       └── main.go                 # bootstrap: config, DB, DI, routes, graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go               # env: PORT, MONGO_URI, MAX_UPLOAD_MB, WORKERS
│   ├── domain/                     # struct & konstanta (§4.2), interface repository
│   │   ├── product.go
│   │   ├── member.go
│   │   ├── order.go
│   │   ├── history.go
│   │   ├── forecast.go
│   │   └── import_job.go
│   ├── repository/
│   │   └── mongo/
│   │       ├── client.go           # koneksi + EnsureIndexes()
│   │       ├── product_repo.go
│   │       ├── member_repo.go
│   │       ├── order_repo.go       # transaksi atomik
│   │       ├── history_repo.go
│   │       ├── forecast_repo.go
│   │       └── analytics_repo.go   # aggregation pipeline
│   ├── service/
│   │   ├── product_service.go
│   │   ├── member_service.go
│   │   ├── order_service.go
│   │   ├── analytics_service.go
│   │   ├── forecast_service.go
│   │   └── etl_service.go          # worker pool import Excel
│   ├── etl/
│   │   ├── reader.go               # streaming excelize.Rows
│   │   ├── cleaner.go              # parse harga/stok, normalisasi
│   │   ├── pipeline.go             # producer-worker-writer
│   │   └── dummy_generator.go      # generator Excel dummy
│   ├── forecast/
│   │   ├── forecast.go             # WMA, OLS, Predict (§7.6)
│   │   ├── series.go               # BuildSeries (§7.7)
│   │   └── forecast_test.go
│   ├── handler/                    # Fiber handlers (parse → service → respons)
│   │   ├── product_handler.go
│   │   ├── member_handler.go
│   │   ├── import_handler.go
│   │   ├── order_handler.go
│   │   ├── analytics_handler.go
│   │   ├── forecast_handler.go
│   │   └── page_handler.go
│   ├── middleware/
│   │   └── error_handler.go
│   └── httpx/
│       └── response.go             # envelope respons standar
├── web/
│   ├── templates/                  # HTML (layout.html, dashboard.html, …)
│   └── static/
│       ├── js/                     # api.js, dashboard.js, order.js, forecast.js
│       └── css/                    # output Tailwind (atau CDN)
├── docker-compose.yml              # mongo (replica set) + app
├── Dockerfile
├── Makefile                        # run, test, test-race, bench, seed
├── .env.example
├── go.mod
└── README.md
```

**Aturan dependensi:** `handler → service → repository(interface di domain)`. `forecast` dan `etl/cleaner` adalah paket murni tanpa dependensi DB sehingga mudah diuji.

### 8.2 Kerangka `main.go`

```go
func main() {
	cfg := config.Load()
	client := mongorepo.Connect(cfg.MongoURI)
	db := client.Database("smartcoop")
	mongorepo.EnsureIndexes(db)

	// repositories & services (dependency injection manual)
	productRepo := mongorepo.NewProductRepo(db)
	// … member, order, history, forecast, analytics, import_jobs
	forecastSvc := service.NewForecastService(/*…*/)
	etlSvc := service.NewETLService(/*…*/, cfg.Workers)

	app := fiber.New(fiber.Config{
		BodyLimit:    cfg.MaxUploadMB * 1024 * 1024,
		ErrorHandler: middleware.ErrorHandler,
	})
	app.Use(recover.New(), logger.New(), cors.New())
	app.Static("/static", "./web/static")

	api := app.Group("/api/v1")
	handler.RegisterRoutes(api /*, handlers…*/)
	handler.RegisterPages(app)

	go func() { _ = app.Listen(":" + cfg.Port) }()
	// graceful shutdown: tangkap SIGINT/SIGTERM → app.ShutdownWithTimeout(10s) → client.Disconnect
}
```

### 8.3 Contoh Kode Kritis: Pemesanan Atomik (`order_repo.go`)

```go
func (r *OrderRepo) Create(ctx context.Context, o *domain.Order) error {
	sess, err := r.client.StartSession()
	if err != nil { return err }
	defer sess.EndSession(ctx)

	_, err = sess.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		for i, it := range o.Items {
			// atomik: hanya berhasil jika stok mencukupi
			res, err := r.products.UpdateOne(sc,
				bson.M{"kode_barang": it.ProductCode, "is_active": true, "stock": bson.M{"$gte": it.Qty}},
				bson.M{"$inc": bson.M{"stock": -it.Qty}, "$set": bson.M{"updated_at": time.Now()}},
			)
			if err != nil { return nil, err }
			if res.MatchedCount == 0 {
				return nil, &InsufficientStockError{Code: it.ProductCode, Requested: it.Qty}
			}
			_ = i
			// akumulasi riwayat bulanan (upsert + $inc)
			_, err = r.history.UpdateOne(sc,
				bson.M{"product_code": it.ProductCode, "year": o.CreatedAt.Year(), "month": int(o.CreatedAt.Month())},
				bson.M{
					"$inc": bson.M{"qty_sold": it.Qty, "revenue": it.Subtotal},
					"$set": bson.M{"updated_at": time.Now()},
					"$setOnInsert": bson.M{"source": "live"},
				},
				options.Update().SetUpsert(true),
			)
			if err != nil { return nil, err }
		}
		_, err := r.orders.InsertOne(sc, o)
		return nil, err
	})
	return err
}
```

### 8.4 Contoh Kode Kritis: Writer Batch ETL

```go
func (w *Writer) FlushProducts(ctx context.Context, docs []domain.Product, mode string) (ins, upd int64, err error) {
	models := make([]mongo.WriteModel, 0, len(docs))
	for _, d := range docs {
		set := bson.M{"name": d.Name, "category": d.Category, "price": d.Price, "is_active": true, "updated_at": time.Now()}
		if mode == "replace" {
			set["stock"] = d.Stock
		}
		update := bson.M{"$set": set, "$setOnInsert": bson.M{"kode_barang": d.Code, "created_at": time.Now()}}
		if mode != "replace" {
			update["$setOnInsert"].(bson.M)["stock"] = d.Stock
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"kode_barang": d.Code}).SetUpdate(update).SetUpsert(true))
	}
	res, err := w.col.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err != nil { return 0, 0, err }
	return res.UpsertedCount, res.ModifiedCount, nil
}
```

### 8.5 Contoh Aggregation Pipeline: Barang Terlaris

```go
pipeline := mongo.Pipeline{
	{{Key: "$match", Value: bson.M{"$expr": bson.M{"$gte": bson.A{
		bson.M{"$add": bson.A{bson.M{"$multiply": bson.A{"$year", 12}}, "$month"}}, fromIndex}}}}},
	{{Key: "$group", Value: bson.M{"_id": "$product_code", "total_qty": bson.M{"$sum": "$qty_sold"}, "total_omzet": bson.M{"$sum": "$revenue"}}}},
	{{Key: "$sort", Value: bson.M{"total_qty": -1}}},
	{{Key: "$limit", Value: limit}},
	{{Key: "$lookup", Value: bson.M{"from": "products", "localField": "_id", "foreignField": "kode_barang", "as": "p"}}},
	{{Key: "$unwind", Value: "$p"}},
}
// fromIndex = tahun*12 + bulan batas awal periode
```

### 8.6 Rancangan UI Dashboard

**Layout global:** Navbar atas (logo "KOPMA Express", menu: Dashboard, Barang, Anggota, Import, Pesanan, Forecast; badge merah jumlah CRITICAL). Konten dalam kontainer `max-w-7xl`, kartu putih `rounded-xl shadow`, palet utama Tailwind `emerald/slate`.

#### Halaman 1 — Dashboard (`/`)
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ [Total Barang] [Total Anggota] [Pesanan Bulan Ini] [Omzet] [Valuasi Aset]   │  ← 5 KPI card
├──────────────────────────────────┬──────────────────────────────────────────┤
│ 🏆 Barang Terlaris (Bar chart H) │ 📦 Stok Terbanyak (Bar chart H)         │
│  + filter periode [3m|6m|all]    │  + total valuasi gudang                  │
├──────────────────────────────────┼──────────────────────────────────────────┤
│ Tabel Top 10 Terlaris            │ Tabel Top 10 Stok + valuasi              │
├──────────────────────────────────┴──────────────────────────────────────────┤
│ 🔮 Ringkasan Forecast: Donut (CRITICAL/OVERSTOCK/SAFE) + 5 barang paling    │
│    kritis (kode, stok, prediksi, restock)   [Lihat semua →]                 │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Halaman 2 — Master Barang & Anggota
Tabel dengan pencarian, filter kategori, paginasi; tombol **Tambah** membuka *modal form* (Alpine.js `x-data`); validasi sisi klien + tampilkan pesan error dari API.

#### Halaman 3 — Import Excel (`/import`)
- Kartu 1: **Unduh Excel Dummy** (input jumlah barang, jumlah bulan, centang "sertakan data kotor") → tombol *Download*.
- Kartu 2: **Upload `.xlsx`** (drag & drop, pilih `stock_mode`) → tombol *Mulai Import*.
- Kartu 3: **Progress** — progress bar dari polling `/import/jobs/:id` tiap 1 detik; ringkasan inserted/updated/failed; tabel error (sheet, baris, alasan) yang dapat di-*scroll*.

#### Halaman 4 — Form Pemesanan (`/orders/new`)
- Input NIM dengan *autocomplete* (`/members?q=`).
- Tabel item dinamis: pilih barang (dropdown pencarian; tampil stok tersedia & harga), input qty, tombol hapus baris, tombol "+ Tambah Barang".
- Panel ringkasan: total otomatis, pilihan metode pembayaran (radio), tombol **Buat Pesanan**.
- Respons sukses → *toast* hijau + nomor pesanan; stok kurang → *toast* merah dengan rincian.

#### Halaman 5 — Forecast & Restock (`/forecast`)
- Header: tombol **Recalculate** + waktu hitung terakhir + bulan target.
- 3 kartu status: 🔴 CRITICAL (n), 🟡 OVERSTOCK (n), 🟢 AMAN (n) — klik untuk memfilter.
- Tabel: Kode, Nama, Stok, WMA, Regresi, **Prediksi**, **Restock**, Status (badge berwarna).
- Klik baris → panel detail dengan *line chart* Chart.js: riwayat bulanan (garis solid), garis tren regresi (putus-putus), titik prediksi bulan depan (merah).

#### Panduan Frontend
- Tailwind via CDN (cukup untuk tugas), Alpine.js via CDN, Chart.js via CDN.
- `web/static/js/api.js`: pembungkus `fetch` yang memeriksa `success`, melempar error terstruktur, dan memicu toast.
- Format angka/mata uang dengan `Intl.NumberFormat('id-ID')`.

---

## 9. Non-Functional Requirements, Pengujian & Rencana Eksekusi

### 9.1 Non-Functional Requirements
| Kategori | Persyaratan |
|---|---|
| Performa | Lihat §1.2; aggregation pipeline wajib didukung indeks |
| Keandalan | Graceful shutdown; recover middleware; timeout konteks pada semua query (default 10 detik) |
| Keamanan | Batas ukuran upload; validasi MIME/ekstensi + signature ZIP (`PK`) untuk `.xlsx`; sanitasi input; rate limit pada endpoint upload & order; tidak ada *secret* dalam kode (gunakan `.env`) |
| Observabilitas | Log terstruktur (method, path, status, durasi, request-id) |
| Portabilitas | Docker Compose satu perintah: `docker compose up` |
| Kualitas kode | `gofmt`, `go vet`, `golangci-lint` (opsional) |

### 9.2 Strategi Pengujian
| Jenis | Cakupan | Perintah |
|---|---|---|
| Unit | `forecast` (WMA, OLS, Predict, BuildSeries), `etl/cleaner` (format harga/stok), `assignStatus` | `go test ./...` |
| Race | Seluruh paket | `go test -race ./...` |
| Konkurensi order | 200 goroutine memesan stok=100 → tepat 100 sukses, stok akhir = 0 | test integrasi (Mongo test container) |
| Idempotensi ETL | Import file yang sama 2× → jumlah dokumen tetap | test integrasi |
| Benchmark | Import 50.000 baris: 1 worker vs N worker; forecast 1.000 produk | `go test -bench .` |
| Beban API | Endpoint order & top-selling | `hey` / `k6` / `vegeta` |

**Test case kunci cleaning harga:** `"Rp 12.500,00"→12500`, `"12,500"→12500`, `"12500.0"→12500`, `"  3500 "→3500`, `"abc"→error`, `""→error`, `"-100"→error`.

### 9.3 Bahan Laporan Big Data 2 (Saran Analisis)
1. Benchmark waktu import: sekuensial vs worker pool (grafik speedup vs jumlah worker).
2. Perbandingan akurasi: WMA vs OLS vs Ensemble (MAPE).
3. Uji race condition: tanpa vs dengan conditional atomic update (tunjukkan overselling pada pendekatan *read-then-write*).
4. Analisis indeks MongoDB: `explain()` sebelum/sesudah indeks pada pipeline top-selling.

### 9.4 Docker Compose (Ringkas)
```yaml
services:
  mongo:
    image: mongo:7
    command: ["--replSet", "rs0", "--bind_ip_all"]
    ports: ["27017:27017"]
    healthcheck:
      test: ["CMD", "mongosh", "--quiet", "--eval", "try{rs.status().ok}catch(e){rs.initiate().ok}"]
      interval: 5s
      retries: 10
  app:
    build: .
    environment:
      PORT: "3000"
      MONGO_URI: "mongodb://mongo:27017/?replicaSet=rs0"
      MAX_UPLOAD_MB: "50"
    ports: ["3000:3000"]
    depends_on:
      mongo:
        condition: service_healthy
```

### 9.5 Rencana Eksekusi (Milestone)
| # | Milestone | Keluaran | Estimasi |
|---|---|---|---|
| M1 | Setup proyek, config, koneksi Mongo, indeks, envelope respons | Server berjalan, `/health` | 0,5 hari |
| M2 | CRUD Produk & Anggota + UI master | US-A1, A2 | 1 hari |
| M3 | Generator Excel dummy | US-A4 | 0,5 hari |
| M4 | ETL pipeline + job status + UI import | US-A3 | 1,5 hari |
| M5 | Order atomik + UI pemesanan + test race | US-B1 | 1 hari |
| M6 | Analitik deskriptif + dashboard chart | US-C1, C2 | 1 hari |
| M7 | Forecast engine + status restock + UI | US-D1, D2 | 1,5 hari |
| M8 | Pengujian, benchmark, dokumentasi, laporan | §9.2–9.3 | 1 hari |

### 9.6 Definition of Done
- Seluruh *acceptance criteria* §3 terpenuhi dan diverifikasi.
- `go test -race ./...` lulus tanpa temuan.
- Demo end-to-end: unduh dummy → upload → dashboard terisi → buat pesanan → recalculate forecast → daftar restock tampil.
- README berisi cara menjalankan (`docker compose up`) dan contoh `curl` setiap endpoint utama.

### 9.7 Risiko & Mitigasi
| Risiko | Mitigasi |
|---|---|
| Transaksi Mongo gagal pada instance non-replica | Dokumentasikan setup replica set; sediakan Docker Compose |
| Data historis pendek → forecast tidak akurat | Status `INSUFFICIENT_DATA` + fallback rata-rata |
| Format Excel lama bervariasi | Cleaner toleran + laporan error per baris; deteksi header fleksibel (case-insensitive, alias kolom) |
| Memori tinggi pada file besar | Streaming `Rows()`, chunk 500 baris, channel ber-buffer |
| Ekstrapolasi regresi liar | Clamp `[0, 2×max]` dan bobot adaptif `R²` |

---
*Akhir dokumen — KOPMA Express PRD v1.0*
