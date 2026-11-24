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

