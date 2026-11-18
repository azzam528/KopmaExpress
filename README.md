# 🏪 KOPMA Express — Smart Cooperative Management System

**Sistem Informasi dan Analitik Prediktif untuk Koperasi Mahasiswa Berbasis Golang dan MongoDB**

KOPMA Express (SmartCoop) merupakan aplikasi manajemen koperasi mahasiswa yang dirancang untuk membantu pengurus dalam mengelola data barang, anggota, transaksi penjualan, serta persediaan stok secara terintegrasi.

Aplikasi ini dilengkapi dengan teknologi ETL (Extract, Transform, Load) untuk migrasi data dari Excel, analitik penjualan, dan Smart Demand Forecasting untuk memprediksi kebutuhan stok serta memberikan rekomendasi restock berdasarkan riwayat penjualan.

---

## ✨ Fitur Utama

### 1. Authentication & Role Management
- Login menggunakan NIM dan password.
- Autentikasi menggunakan JWT.
- Pengelolaan hak akses berbasis role (RBAC).
- Dua role pengguna: Admin dan Member.
- Proteksi halaman dan API berdasarkan hak akses.
- Logout dan pengelolaan sesi pengguna.

### 2. Manajemen Data Barang
- Menambahkan barang baru.
- Mengubah informasi barang.
- Menghapus barang dengan mekanisme soft delete.
- Pencarian dan pengelolaan data barang.
- Pengelolaan harga dan stok barang.
- Validasi kode barang agar tidak terjadi duplikasi.

### 3. Manajemen Data Anggota
- Mengelola data anggota koperasi.
- Menyimpan informasi NIM, nama, unit/prodi, dan nomor telepon.
- Pencarian anggota berdasarkan NIM atau nama.
- Aktivasi dan deaktivasi anggota.
- Validasi NIM unik.

### 4. Import & Export Excel (ETL)
- Import data koperasi dari file Excel (.xlsx).
- Mendukung sheet Barang, Anggota, dan Riwayat Penjualan.
- Data cleaning dan normalisasi data otomatis.
- Penanganan data duplikat menggunakan upsert.
- Pemrosesan data menggunakan worker pool dan goroutine.
- Import berjalan secara asynchronous.
- Monitoring progres import melalui job ID.
- Menampilkan jumlah data berhasil, diperbarui, dilewati, dan gagal.
- Generator file Excel dummy untuk kebutuhan pengujian.

### 5. Manajemen Transaksi & Pemesanan
- Membuat pesanan dengan banyak item.
- Mendukung metode pembayaran CASH, TRANSFER, QRIS, dan KASBON.
- Perhitungan subtotal dan total transaksi otomatis.
- Pengurangan stok secara atomic.
- Validasi ketersediaan stok sebelum transaksi.
- Riwayat transaksi.
- Pembatalan pesanan dan pengembalian stok.
- Sinkronisasi transaksi dengan riwayat penjualan bulanan.

### 6. Dashboard & Business Analytics
- Ringkasan informasi operasional koperasi.
- Statistik penjualan.
- Top Selling Products.
- Top Stock Products.
- Informasi valuasi persediaan barang.
- Grafik tren penjualan bulanan.
- Ringkasan status persediaan.

### 7. Smart Demand Forecasting
Fitur analitik prediktif untuk membantu pengurus memperkirakan kebutuhan persediaan barang.

Metode yang digunakan:
- Weighted Moving Average (WMA).
- Ordinary Least Squares (OLS) / Regresi Linear.
- Adaptive Ensemble untuk menggabungkan hasil prediksi.
- Run-rate projection untuk memperkirakan penjualan bulan berjalan.
- MAPE dan Rolling-Origin Backtesting untuk evaluasi prediksi.

Sistem menghasilkan rekomendasi status persediaan:
- 🔴 **CRITICAL** — Stok tidak mencukupi kebutuhan prediksi.
- 🟡 **OVERSTOCK WARNING** — Indikasi persediaan berlebih.
- 🟢 **SAFE** — Persediaan dalam kondisi aman.
- ⚪ **INSUFFICIENT DATA** — Data historis belum mencukupi.

Hasil forecasting disimpan ke MongoDB dan dapat dihitung ulang melalui fitur Recalculate.

---

## 🛠️ Teknologi yang Digunakan

| Teknologi | Kegunaan |
|---|---|
| Go (Golang) | Backend dan business logic |
| Go Fiber | REST API dan HTTP server |
| MongoDB | Database NoSQL |
| MongoDB Go Driver | Integrasi database |
| Excelize | Import dan export Excel |
| JWT | Authentication |
| Bcrypt | Password hashing |
| HTML5 | Struktur halaman web |
| CSS3 | Styling antarmuka |
| JavaScript | Interaksi frontend |
| Tailwind CSS | Utility-first styling |
| Chart.js | Visualisasi data dan grafik |
| Docker | Containerization |

---

## 🏗️ Arsitektur Sistem

Project menggunakan pendekatan Clean Architecture ringan dengan pemisahan tanggung jawab menjadi beberapa layer.

```text
KOPMA EXPRESS
│
├── Frontend
│   ├── HTML
│   ├── CSS
│   ├── JavaScript
│   └── Chart.js
│
├── Backend (Go Fiber)
│   │
│   ├── Handler Layer
│   │   └── HTTP Request & Response
│   │
│   ├── Service Layer
│   │   └── Business Logic
│   │
│   ├── Domain Layer
│   │   └── Entity & Interface
│   │
│   ├── Repository Layer
│   │   └── MongoDB Operations
│   │
│   ├── ETL Module
│   │   └── Excel Processing
│   │
│   └── Forecast Module
│       └── Demand Prediction
│
└── Database
    └── MongoDB
```

---

## 📂 Struktur Project

```text
KopmaExpress/
│
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── config/
│   ├── domain/
│   ├── etl/
│   │   └── excelgen/
│   ├── forecast/
│   ├── handler/
│   ├── httpx/
│   ├── middleware/
│   ├── repository/
│   │   └── mongo/
│   ├── security/
│   ├── service/
│   └── timeutil/
│
├── web/
│   ├── static/
│   │   ├── css/
│   │   ├── js/
│   │   └── images/
│   │
│   ├── templates/
│   │   ├── dashboard/
│   │   ├── forecast/
│   │   ├── import/
│   │   ├── login/
│   │   ├── members/
│   │   ├── orders/
│   │   ├── products/
│   │   └── shop/
│   │
│   └── stitch_screens/
│
├── testdata/
│   └── data_koperasi_lama_dummy.xlsx
│
├── .env.example
├── .gitignore
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── go.mod
├── go.sum
├── HANDOVER.md
├── ORANG4_IMPLEMENTATION.md
└── PRD_KopmaExpress.md
```

---

## 🚀 Instalasi dan Menjalankan Project

### 1. Persyaratan

Pastikan perangkat sudah memiliki:

- Go 1.26 atau versi yang sesuai dengan `go.mod`.
- MongoDB 7 atau kompatibel.
- Git.
- Visual Studio Code (opsional).

### 2. Clone Repository

```bash
git clone <URL_REPOSITORY>
```

Masuk ke direktori project:

```bash
cd KopmaExpress
```

### 3. Install Dependencies

```bash
go mod download
```

### 4. Konfigurasi Environment

Salin file `.env.example` menjadi `.env`.

Windows PowerShell:

```powershell
Copy-Item .env.example .env
```

Sesuaikan konfigurasi `.env`:

```env
PORT=3000
APP_ENV=development

MONGO_URI=mongodb://127.0.0.1:27017/?replicaSet=rs0
DB_NAME=smartcoop

MAX_UPLOAD_MB=50
WORKERS=4

JWT_SECRET=ganti_dengan_secret_yang_aman
JWT_TTL_HOURS=24
```

Pastikan menggunakan JWT secret yang kuat dan jangan mengunggah file `.env` ke GitHub.

### 5. Menjalankan MongoDB

Project menggunakan MongoDB replica set karena beberapa operasi transaksi membutuhkan dukungan database transaction.

Jika menggunakan Docker Compose, jalankan:

```bash
docker compose up -d mongo
```

Inisialisasi replica set:

```bash
docker exec smartcoop-mongo mongosh --eval "rs.initiate()"
```

Tunggu hingga MongoDB siap menerima koneksi.

### 6. Menjalankan Backend

Jalankan server:

```bash
go run ./cmd/server
```

Jika berhasil, server berjalan pada:

```text
http://localhost:3000
```

### 7. Mengakses Aplikasi

| Halaman | URL |
|---|---|
| Login | http://localhost:3000/login |
| Dashboard Admin | http://localhost:3000/ |
| Data Barang | http://localhost:3000/products |
| Data Anggota | http://localhost:3000/members |
| Import Excel | http://localhost:3000/import |
| Form Pemesanan | http://localhost:3000/orders/new |
| Forecast & Restock | http://localhost:3000/forecast |
| Portal Belanja Member | http://localhost:3000/belanja |

Halaman admin dan member membutuhkan autentikasi sesuai role masing-masing.

---

## 🔐 Authentication

Sistem menggunakan autentikasi berbasis JWT dengan cookie `auth_token`.

| Role | Hak Akses |
|---|---|
| Admin | Dashboard, barang, anggota, import Excel, transaksi, analytics, dan forecasting |
| Member | Portal belanja dan transaksi milik sendiri |

Akun awal dapat dibuat melalui mekanisme seeding otomatis ketika aplikasi terhubung ke MongoDB.

Password awal dapat dikonfigurasi melalui:

```env
SEED_ADMIN_PASSWORD=your_admin_password
SEED_MEMBER_PASSWORD=your_member_password
```

Gunakan password yang berbeda dan aman untuk lingkungan production.

---

## 🔌 REST API

Base URL:

```text
/api/v1
```

### Authentication

| Method | Endpoint | Fungsi |
|---|---|---|
| POST | `/auth/login` | Login pengguna |
| POST | `/auth/logout` | Logout pengguna |
| GET | `/auth/me` | Mendapatkan profil pengguna |

### Products

| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/products` | Daftar barang |
| GET | `/products/:code` | Detail barang |
| POST | `/products` | Tambah barang |
| PUT | `/products/:code` | Update barang |
| DELETE | `/products/:code` | Hapus barang |

### Members

| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/members` | Daftar anggota |
| GET | `/members/summary` | Ringkasan anggota |
| GET | `/members/:nim` | Detail anggota |
| POST | `/members` | Tambah anggota |
| PUT | `/members/:nim` | Update anggota |
| DELETE | `/members/:nim` | Nonaktifkan anggota |
| POST | `/members/:nim/activate` | Aktifkan anggota |

### Import & Export

| Method | Endpoint | Fungsi |
|---|---|---|
| POST | `/import/excel` | Import file Excel |
| GET | `/import/jobs` | Daftar pekerjaan import |
| GET | `/import/jobs/:id` | Status pekerjaan import |
| GET | `/export/dummy-excel` | Download Excel dummy |

### Orders

| Method | Endpoint | Fungsi |
|---|---|---|
| POST | `/orders` | Membuat pesanan |
| GET | `/orders` | Daftar pesanan |
| GET | `/orders/:order_no` | Detail pesanan |
| POST | `/orders/:order_no/cancel` | Membatalkan pesanan |

### Analytics

| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/analytics/top-selling` | Barang terlaris |
| GET | `/analytics/top-stock` | Barang dengan stok terbanyak |
| GET | `/analytics/summary` | Ringkasan dashboard |
| GET | `/analytics/monthly-trend` | Tren penjualan bulanan |

### Forecast

| Method | Endpoint | Fungsi |
|---|---|---|
| POST | `/forecast/recalculate` | Menghitung ulang prediksi |
| GET | `/forecast` | Daftar hasil forecasting |
| GET | `/forecast/:code` | Forecast berdasarkan kode barang |
| GET | `/forecast/summary` | Ringkasan status restock |

Sebagian besar endpoint operasional membutuhkan autentikasi. Endpoint administrasi dibatasi untuk role Admin.

---

## 📊 Smart Demand Forecasting

KOPMA Express memiliki modul prediksi kebutuhan stok menggunakan data historis penjualan bulanan.

Alur perhitungan:

1. Mengambil riwayat penjualan setiap barang.
2. Melengkapi periode bulan yang kosong.
3. Memproyeksikan penjualan bulan berjalan menggunakan run-rate.
4. Menghitung Weighted Moving Average.
5. Menghitung regresi linear menggunakan OLS.
6. Menggabungkan hasil melalui Adaptive Ensemble.
7. Membandingkan hasil prediksi dengan stok saat ini.
8. Menghasilkan rekomendasi restock.
9. Menyimpan hasil forecasting ke MongoDB.

Hasil forecasting digunakan untuk membantu pengurus mengambil keputusan terkait pengadaan dan pengendalian persediaan.

---

## 🧪 Testing

Untuk menjalankan seluruh unit test:

```bash
go test ./...
```

Untuk menjalankan test dengan Race Detector:

```bash
go test -race ./...
```

Untuk pengujian khusus modul forecasting:

```bash
go test ./internal/forecast
```

---

## 🐳 Docker

Project menyediakan Dockerfile dan Docker Compose untuk mendukung deployment berbasis container.

Pastikan konfigurasi `MONGO_URI` pada `.env` menggunakan hostname `mongo` ketika aplikasi dijalankan dalam Docker Compose:

```env
MONGO_URI=mongodb://mongo:27017/?replicaSet=rs0
```

Jalankan:

```bash
docker compose up -d --build
```

Untuk menghentikan container:

```bash
docker compose down
```

Catatan: sebelum menjalankan build Docker, sesuaikan versi image Go pada Dockerfile dengan versi Go yang ditentukan di `go.mod`.

---

## 🎯 Tujuan Pengembangan

KOPMA Express dikembangkan untuk:

- Mendigitalisasi proses operasional koperasi mahasiswa.
- Mengintegrasikan pengelolaan barang, anggota, dan transaksi.
- Mempermudah migrasi data historis menggunakan ETL.
- Mengurangi risiko kesalahan pengelolaan stok.
- Memanfaatkan analitik data untuk mendukung pengambilan keputusan.
- Menerapkan forecasting sebagai pendekatan prediktif dalam pengelolaan persediaan.

---

## 👨‍💻 Pengembangan

Project ini dikembangkan secara kolaboratif sebagai bagian dari kegiatan perkuliahan dan implementasi sistem informasi koperasi mahasiswa.

Kontribusi pengembangan mencakup backend, frontend, pengolahan data, integrasi database, analitik, serta pengujian sistem.

---

**KOPMA Express — Smart Data, Smarter Inventory, Better Cooperative Management.**
