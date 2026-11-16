# KOPMA Express - Implementasi Orang 4

Dokumen ini mencatat modul Analytics dan Smart Demand Forecasting yang ditambahkan ke snapshot repository.

## Ruang lingkup

1. Analitik deskriptif: Top Selling, Top Stock, Dashboard Summary, dan Monthly Trend.
2. Analitik prediktif: pembentukan deret bulanan, run-rate bulan berjalan, WMA, OLS, adaptive ensemble, status restock, serta penyimpanan hasil forecast.
3. Evaluasi: MAPE dan rolling-origin backtesting untuk WMA, OLS, dan ensemble.

## File utama

- `internal/forecast/forecast.go`: WMA, OLS, ensemble, MAPE, dan backtesting. Tidak memiliki dependensi database atau HTTP.
- `internal/forecast/series.go`: pengisian bulan kosong, proyeksi run-rate, dan pembatasan 12 bulan terakhir.
- `internal/forecast/forecast_test.go`: unit test algoritma dan pembentukan deret.
- `internal/service/analytics_service.go`: validasi periode, rentang bulan, dan batas Top-N.
- `internal/service/forecast_service.go`: worker pool, prediksi, klasifikasi restock, dan orkestrasi penyimpanan.
- `internal/service/forecast_service_test.go`: unit test prioritas status dan jumlah restock.
- `internal/repository/mongo/analytics_repo.go`: MongoDB aggregation untuk analitik deskriptif.
- `internal/repository/mongo/forecast_repo.go`: load riwayat penjualan, upsert hasil prediksi, filter, dan ringkasan status.
- `internal/handler/analytics_handler.go` dan `internal/handler/forecast_handler.go`: endpoint Fiber.
- `internal/timeutil/time.go`: waktu bisnis WIB (Asia/Jakarta), dengan fallback UTC+7 untuk container minimal.
- `cmd/server/main.go`: dependency injection dan registrasi rute Orang 4.

## Endpoint

Semua endpoint memakai prefix `/api/v1`.

| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/analytics/top-selling?limit=10&period=6m` | Barang terlaris |
| GET | `/analytics/top-selling?from=2026-01&to=2026-06` | Barang terlaris pada rentang bulan inklusif |
| GET | `/analytics/top-stock?limit=10` | Stok terbanyak dan valuasi gudang |
| GET | `/analytics/summary` | KPI dashboard |
| GET | `/analytics/monthly-trend?code=BRG-0001` | Riwayat bulanan produk |
| POST | `/forecast/recalculate` | Hitung dan simpan forecast bulan berikutnya |
| GET | `/forecast` | Daftar forecast terbaru dengan filter dan pagination |
| GET | `/forecast/:code` | Forecast terbaru beserta riwayat bulanan |
| GET | `/forecast/summary` | Jumlah produk per status untuk target bulan berikutnya |

Filter forecast:

- `status`: `CRITICAL`, `OVERSTOCK_WARNING`, `SAFE`, atau `INSUFFICIENT_DATA`.
- `q`: pencarian kode/nama barang.
- `page`: nomor halaman, mulai dari 1.
- `limit`: jumlah data per halaman, maksimum 100.
- `sort`: `restock_qty` (default), `restock_qty_asc`, `predicted_demand`, `current_stock`, atau `product_code`.

## Kontrak data lintas anggota

Modul ini bergantung pada struktur berikut:

### `products`

- `kode_barang`: kode produk.
- `name`: nama produk.
- `category`: kategori.
- `price`: harga satuan integer Rupiah.
- `stock`: stok saat ini.
- `is_active`: hanya produk aktif yang dihitung.

### `monthly_sales_history`

Satu dokumen per produk per bulan:

```json
{
  "product_code": "BRG-0001",
  "year": 2026,
  "month": 11,
  "qty_sold": 150,
  "revenue": 525000,
  "source": "mixed",
  "updated_at": "2026-11-16T10:00:00+07:00"
}
```

PENTING untuk Orang 2 dan Orang 3: proses import Excel dan transaksi pesanan selesai harus memperbarui koleksi `monthly_sales_history` dengan kunci unik `(product_code, year, month)`. Gunakan upsert/increment, bukan insert dokumen bulanan berulang. Forecast membaca `product_code`, `year`, `month`, dan `qty_sold` dengan nama field tersebut.

### `orders`

Dashboard Summary membaca `status`, `created_at`, dan `total_amount`. Omzet bulan berjalan dihitung dari pesanan berstatus `COMPLETED`.

## Catatan algoritma

- Bulan kosong di antara titik data diisi 0.
- Perhitungan periode, batas bulan, dan target forecast menggunakan zona waktu bisnis WIB (Asia/Jakarta).
- Bulan berjalan diproyeksikan hanya mulai hari ke-4 dengan `qty_so_far * days_in_month / current_day`.
- Maksimum 12 titik terbaru dipakai.
- Kurang dari 3 titik menghasilkan status `INSUFFICIENT_DATA` dan prediksi rata-rata sederhana.
- WMA memakai bobot 1, 2, 3 pada tiga bulan terakhir.
- OLS memakai x = 1..n; regresi bulan berikutnya dibatasi pada 0 sampai 2 kali penjualan maksimum historis.
- Bobot regresi adalah clamp(R², 0.2, 0.7); bobot WMA adalah sisanya.
- Urutan status: `INSUFFICIENT_DATA`, `CRITICAL`, `OVERSTOCK_WARNING`, lalu `SAFE`.
- Top-N stok dan median prediksi dihitung setelah semua prediksi selesai.
- Backtesting memprediksi satu bulan dari data sebelum bulan tersebut. Bulan dengan aktual 0 dikecualikan dari MAPE.

### Koreksi angka contoh PRD §7.9

Untuk seri `[100, 110, 125, 130, 150, 160]`, rumus OLS yang tertulis di PRD menghasilkan:

- WMA = 151.6667
- slope = 12.142857
- intercept = 86.666667
- prediksi regresi bulan berikutnya = 171.666667
- R² ≈ 0.984556
- ensemble = ceil(0.3 × 151.6667 + 0.7 × 171.6667) = **166**

Angka `slope ≈ 11.43`, `intercept ≈ 87.33`, dan prediksi akhir `163` di contoh PRD tidak konsisten dengan rumus OLS tersebut. Implementasi mengikuti rumus, bukan angka contoh yang tidak konsisten.

## Pengujian

Package algoritma murni dapat diuji tanpa koneksi MongoDB:

```bash
go test ./internal/forecast
```

Setelah dependensi Go tersedia, jalankan semua test:

```bash
go test ./...
go test -race ./...
```

Untuk mencoba API, jalankan MongoDB replica set sesuai `docker-compose.yml`, siapkan `.env` lokal dari `.env.example`, jalankan server, lalu panggil endpoint analytics dan forecast di atas. Endpoint forecast akan menghasilkan data jika koleksi `products` dan `monthly_sales_history` sudah berisi data yang sesuai.

## Batas integrasi yang perlu diverifikasi di branch tim

ZIP sumber yang diterima tidak berisi folder `.git`, sehingga nama branch, commit, dan status merge tidak dapat diperiksa dari snapshot ini. Sebelum merge, cocokkan kembali file yang mungkin sudah dibuat anggota lain, terutama `cmd/server/main.go`, `internal/domain/history.go`, `internal/domain/forecast.go`, dan `internal/repository/mongo/client.go`.
