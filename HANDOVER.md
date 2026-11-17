# DOKUMEN HANDOVER IMPLEMENTASI AUTENTIKASI, USER MANAGEMENT & OTORISASI
## Proyek: KOPMA Express (SmartCoop)
**Tanggal Handover:** 18 November 2026  
**Branch Kerja:** `azzam`  
**Status Workspace:** Perubahan tersimpan lokal (uncommitted), siap dilanjutkan pada sesi berikutnya.

---

> [!WARNING]
> **CATATAN REGRESI PENTING:**  
> Seluruh perubahan autentikasi dan integrasi ini **BELUM BOLEH DIANGGAP SELESAI SECARA FINAL** sebelum seluruh *regression test* (mencakup modul Order, ETL Import, Catalog, Analytics, dan Forecast) berhasil dijalankan secara komprehensif pada environment pengujian penuh dengan MongoDB Atlas live.

---

## 1. Tujuan Utama Pekerjaan

Membangun dan mengintegrasikan sistem autentikasi, manajemen pengguna (*user management*), otorisasi berbasis peran (*role-based access control / RBAC*), dan integrasi halaman belanja (*shop*) pada sistem KopmaExpress tanpa merusak modul bisnis yang sudah berjalan:
1. Memisahkan kredensial akun/login (`users`) dari data keanggotaan koperasi (`members`).
2. Menerapkan login terpusat berbasis **NIM** dan **Kata Sandi** (bcrypt hash) dengan JWT token yang disimpan pada cookie aman (`HttpOnly`, `SameSite=Lax`).
3. Menerapkan tepat dua peran (*roles*):
   - `admin`: untuk Administrator dan Kasir (mengakses seluruh manajemen, inventaris, anggota, import ETL, forecast, dan transaksi kasir).
   - `member`: untuk Anggota Koperasi (mengakses portal belanja `/belanja` dan membuat pesanan hanya untuk akun/NIM sendiri).
4. Menghilangkan fitur registrasi publik (tidak ada pendaftaran akun mandiri untuk sementara; akun dikelola admin/seed).
5. Menyediakan mekanisme seeding akun testing secara idempoten:
   - Admin: NIM `000000001`, Nama `Admin Testing`, role `admin`, `member_id: nil`.
   - Member: NIM `000000002`, Nama `Member Testing`, role `member`, terhubung ke relasi `members`.
6. Melindungi seluruh route backend dan frontend web:
   - Route Web Admin dilindungi dan mengalihkan (*redirect*) tamu ke `/login`, serta mengalihkan role `member` ke `/belanja`.
   - Route API Mutasi (POST/PUT/DELETE) produk, import ETL, manajemen anggota, forecast, dan analitik dilindungi untuk role `admin` saja (HTTP 403 Forbidden untuk role `member`).
7. Menghubungkan identitas dinamis pada seluruh header/navbar admin dan portal belanja melalui endpoint `/api/v1/auth/me`, menghapus data hardcoded ("Nur Sabrina Hanifa" / "Administrator").
8. Menghilangkan mekanisme login lawas tanpa password pada shop (`/masuk` dan `localStorage.getItem('kopma_member')`), menggantikannya dengan autentikasi berbasis sesi terpusat.

---

## 2. Arsitektur Aplikasi

```
                   +---------------------------------------------+
                   |           Browser Client                    |
                   |  (Admin Pages / Cashier / Member Shop)      |
                   +---------------------------------------------+
                                       |
                                       | Cookie: auth_token (HttpOnly, SameSite=Lax)
                                       v
                   +---------------------------------------------+
                   |                 Go Fiber                    |
                   +---------------------------------------------+
                     /                         \
         Web Middleware                         API Middleware
       - WebRequireAuth (Redirect)            - RequireAuth (401 JSON)
       - WebRequireGuest (Redirect)           - RequireRole (403 JSON)
                     \                         /
                   +---------------------------------------------+
                   |              Handlers Layer                 |
                   | - AuthHandler   - ProductHandler            |
                   | - OrderHandler  - MemberHandler             |
                   | - ImportHandler - ForecastHandler           |
                   | - AnalyticsHandler                          |
                   +---------------------------------------------+
                                       |
                   +---------------------------------------------+
                   |              Services Layer                 |
                   | - AuthService   - ProductService            |
                   | - OrderService  - MemberService             |
                   | - ETLService    - ForecastService           |
                   +---------------------------------------------+
                                       |
                   +---------------------------------------------+
                   |             Repositories Layer              |
                   | - UserRepo      - ProductRepo               |
                   | - MemberRepo    - OrderRepo                 |
                   | - ImportJobRepo - StockHistoryRepo          |
                   +---------------------------------------------+
                                       |
                                       v
                   +---------------------------------------------+
                   |             MongoDB Collections             |
                   | - users (Kredensial, Role, NIM, Status)     |
                   | - members (Data Anggota, Unit, Telepon)     |
                   | - products, orders, import_jobs, dll.       |
                   +---------------------------------------------+
```

---

## 3. Requirement Lengkap Sistem User dan Member

| Aspek | Koleksi `users` | Koleksi `members` |
|---|---|---|
| **Tanggung Jawab** | Kredensial autentikasi, status login, hak akses (RBAC). | Data profil keanggotaan koperasi (organisasi, data sosial, keaktifan). |
| **Identifier Utama** | `nim` (unik, string indeks) & `_id` (ObjectID). | `nim` (unik, string indeks) & `_id` (ObjectID). |
| **Field Kunci** | `nim`, `password_hash`, `role` (`admin`/`member`), `status` (`active`/`inactive`), `member_id` (nullable ObjectID). | `nim`, `name`, `email`, `unit`, `phone`, `is_active`, `joined_at`, `created_at`. |
| **Relasi** | Menyimpan pointer `member_id` ke dokumen `members` (untuk role `member`; `nil` untuk admin sistem). | Tidak menyimpan `user_id` agar modular dan tidak merusak skema import anggota lama. |
| **Indeks MongoDB** | 1. `nim` (Unique)<br>2. `role`<br>3. `member_id` (Unique, Sparse)<br>4. `status` | 1. `nim` (Unique)<br>2. `is_active`<br>3. `email` (Sparse) |

---

## 4. Fitur yang Sudah Berhasil Diimplementasikan

1. **Domain & Entity Modeling:**
   - Dibuat domain `User` dan interface `UserRepository` di [internal/domain/user.go](file:///d:/AZZAMCODING/KopmaExpress/internal/domain/user.go).
   - Diperbarui `MemberRepository` di [internal/domain/member.go](file:///d:/AZZAMCODING/KopmaExpress/internal/domain/member.go) agar interface mencakup `FindByID` dan `FindByEmail`.
2. **MongoDB User Repository & Indexing:**
   - Dibuat `UserRepo` di [internal/repository/mongo/user_repo.go](file:///d:/AZZAMCODING/KopmaExpress/internal/repository/mongo/user_repo.go) dengan implementasi `FindByNIM`, `FindByID`, `FindByMemberID`, `Create`, `Update`, `Count`.
   - Diperbarui [internal/repository/mongo/member_repo.go](file:///d:/AZZAMCODING/KopmaExpress/internal/repository/mongo/member_repo.go) dengan `FindByID`, `FindByEmail`, dan `BulkUpsert`.
   - Ditambahkan pembuatan indeks otomatis koleksi `users` pada `EnsureIndexes` di [internal/repository/mongo/client.go](file:///d:/AZZAMCODING/KopmaExpress/internal/repository/mongo/client.go).
3. **Security & Authentication Service:**
   - Ditambahkan field `UserID` dan `Role` pada `JWTClaims` di [internal/security/jwt.go](file:///d:/AZZAMCODING/KopmaExpress/internal/security/jwt.go).
   - Dibuat implementasi lengkap `AuthService` di [internal/service/auth_service.go](file:///d:/AZZAMCODING/KopmaExpress/internal/service/auth_service.go) yang memvalidasi NIM + kata sandi (bcrypt), menerbitkan JWT, mengambil profil pengguna gabungan (*user + member*), dan melakukan seeding akun testing secara idempoten.
4. **Auth Handler & Routing API:**
   - Dibuat `AuthHandler` di [internal/handler/auth_handler.go](file:///d:/AZZAMCODING/KopmaExpress/internal/handler/auth_handler.go):
     - `POST /api/v1/auth/login`: validasi NIM & password, set cookie `auth_token` (`HttpOnly`, `SameSite=Lax`, TTL dinamis), mengembalikan redirect URL sesuai role (`/` untuk admin, `/belanja` untuk member).
     - `POST /api/v1/auth/logout`: menghapus cookie `auth_token`.
     - `GET /api/v1/auth/me`: mengembalikan profil pengguna terautentikasi berserta informasi profil anggota terkait.
5. **Middleware Autentikasi & Otorisasi:**
   - [internal/middleware/auth.go](file:///d:/AZZAMCODING/KopmaExpress/internal/middleware/auth.go):
     - `RequireAuth`: memvalidasi token JWT dari Cookie `auth_token` atau header `Authorization: Bearer <token>`.
     - `RequireRole(allowedRoles...)`: membatasi akses API JSON (mengembalikan HTTP 403 Forbidden jika role tidak sesuai).
     - `WebRequireAuth(jwtSecret, allowedRoles...)`: penjaga halaman HTML (mengalihkan tamu ke `/login`, mengalihkan role `member` ke `/belanja` jika mencoba mengakses admin).
     - `WebRequireGuest(jwtSecret)`: mengalihkan pengguna yang sudah login dari `/login` ke halaman utama sesuai role.
6. **Proteksi Route Admin Backend:**
   - [cmd/server/main.go](file:///d:/AZZAMCODING/KopmaExpress/cmd/server/main.go): seluruh halaman admin (`/`, `/products`, `/members`, `/import`, `/orders/new`, `/forecast`) dilindungi middleware `adminWebAuth`.
   - Mutasi produk (`POST`, `PUT`, `DELETE` di `/api/v1/products`) dilindungi role `admin`.
   - Manajemen anggota (`/api/v1/members/*`) dilindungi role `admin`.
   - Pipeline import ETL (`/api/v1/import/*`) dilindungi role `admin`.
   - Forecast (`/api/v1/forecast/*`) dan Analitik (`/api/v1/analytics/*`) dilindungi role `admin`.
7. **Keamanan Kepemilikan Pesanan (Order Security):**
   - [internal/handler/order_handler.go](file:///d:/AZZAMCODING/KopmaExpress/internal/handler/order_handler.go): Pengguna role `member` dicegah membuat atau melihat pesanan atas nama NIM anggota lain (mengembalikan HTTP 403 Forbidden).
8. **Frontend Login & Navbar Dinamis:**
   - [web/templates/login/index.html](file:///d:/AZZAMCODING/KopmaExpress/web/templates/login/index.html) & [web/templates/login.html](file:///d:/AZZAMCODING/KopmaExpress/web/templates/login.html): Label diubah menjadi "NIM", form pendaftaran mandiri dihapus, submit handler mengarah ke `/api/v1/auth/login` dengan redirect otomatis.
   - [web/templates/partials/navbar.html](file:///d:/AZZAMCODING/KopmaExpress/web/templates/partials/navbar.html) dan seluruh template admin (`dashboard`, `products`, `members`, `import`, `orders`, `forecast`): Menghapus nama statis "Nur Sabrina Hanifa", digantikan elemen profil dinamis `#userProfileName` dan `#userProfileRole`, serta tombol logout `#logoutBtn`.
   - [web/static/js/main.js](file:///d:/AZZAMCODING/KopmaExpress/web/static/js/main.js): Menghubungkan pemanggilan `/api/v1/auth/me` otomatis untuk merender identitas nama dan role pengguna, serta binding handler logout ke `POST /api/v1/auth/logout`.
9. **Integrasi Halaman Belanja (Shop):**
   - [web/static/js/pages/shop.js](file:///d:/AZZAMCODING/KopmaExpress/web/static/js/pages/shop.js): Menghilangkan ketergantungan `localStorage.getItem('kopma_member')`, mengambil profil login dari `/api/v1/auth/me`, memvalidasi sesi cookie, dan menggunakan NIM pengguna yang login untuk pembuatan pesanan serta riwayat pesanan.

---

## 5. Fitur yang Masih Belum Selesai / Perlu Tindak Lanjut

1. **Uji Coba End-to-End dengan MongoDB Live:**
   - Memastikan saat server dijalankan dengan `MONGO_URI` aktif, proses pembuatan index `users` dan seeding akun `000000001` & `000000002` berjalan tanpa benturan data existing di collection `members`.
2. **Pembersihan File Template Backup:**
   - Meninjau file legacy seperti `web/templates/products/index_backup.html` dan template statis legacy di root `web/templates/*.html` agar tidak membingungkan anggota tim lainnya.
3. **Penyelarasan Role Kasir Khusus (Opsional di masa depan):**
   - Saat ini role `admin` mencakup Admin sekaligus Kasir sesuai instruksi. Jika di masa depan dibutuhkan hak akses terpisah antara Admin Operasional dan Kasir Penjualan, sistem middleware `RequireRole` sudah siap menerima multiple roles.

---

## 6. Daftar File yang Dibuat dan Diubah

### File Baru Dibuat (Untracked):
1. `internal/domain/user.go` - Definisi domain entity User, role constant, dan interface UserRepository.
2. `internal/repository/mongo/user_repo.go` - Implementasi MongoDB repository untuk koleksi `users`.
3. `internal/service/auth_service_test.go` - Unit testing untuk AuthService (login, hash, profile, seeding idempotent).
4. `internal/handler/auth_handler_test.go` - Integration testing untuk AuthHandler, middleware RBAC, cookie, dan proteksi order ownership.
5. `HANDOVER.md` - Dokumen handover teknis ini.

### File Diubah (Modified):
1. `cmd/server/main.go` - Wiring UserRepo, AuthService, Initial User Seeding, route Auth API, dan Web Route Protection.
2. `internal/domain/member.go` - Penyesuaian LoginRequest & MemberRepository interface (`FindByID`, `FindByEmail`).
3. `internal/handler/analytics_handler.go` - Penambahan variadic middleware (adminOnly) pada rute analitik.
4. `internal/handler/auth_handler.go` - Implementasi handler login, logout, me.
5. `internal/handler/forecast_handler.go` - Penambahan variadic middleware (adminOnly) pada rute forecast.
6. `internal/handler/import_handler.go` - Penambahan variadic middleware (adminOnly) pada rute import ETL.
7. `internal/handler/member_handler.go` - Penambahan variadic middleware (adminOnly) pada rute member management.
8. `internal/handler/order_handler.go` - Enforce validasi NIM kepemilikan pesanan untuk role member.
9. `internal/handler/product_handler.go` - Proteksi rute mutasi produk (POST/PUT/DELETE) dengan adminOnly.
10. `internal/middleware/auth.go` - Implementasi RequireAuth, RequireRole, WebRequireAuth, WebRequireGuest.
11. `internal/repository/mongo/client.go` - Registrasi index MongoDB untuk koleksi `users`.
12. `internal/repository/mongo/member_repo.go` - Implementasi `FindByID`, `FindByEmail`, `BulkUpsert`.
13. `internal/security/jwt.go` - Payload JWT Claims dengan UserID dan Role.
14. `internal/service/auth_service.go` - Implementasi bisnis logic login, profile, dan seed.
15. `web/static/js/main.js` - Sinkronisasi identitas navbar dengan `/api/v1/auth/me` dan fungsi logout.
16. `web/static/js/pages/shop.js` - Menggunakan session auth cookie untuk identitas belanja dan order.
17. `web/templates/dashboard.html` & `web/templates/dashboard/index.html` - Profil dinamis & tombol logout.
18. `web/templates/forecast.html` & `web/templates/forecast/index.html` - Profil dinamis & tombol logout.
19. `web/templates/import.html` & `web/templates/import/index.html` - Profil dinamis & tombol logout.
20. `web/templates/index.html` - Profil dinamis & tombol logout.
21. `web/templates/login.html` & `web/templates/login/index.html` - Form login NIM, hapus link register.
22. `web/templates/members/index.html` - Profil dinamis & tombol logout.
23. `web/templates/orders/new.html` - Profil dinamis & tombol logout.
24. `web/templates/partials/navbar.html` - Profil dinamis & tombol logout.
25. `web/templates/products.html`, `web/templates/products/index.html`, `web/templates/products/index_backup.html` - Profil dinamis & tombol logout.

---

## 7. Perubahan Database dan Status Migrasi

- **Koleksi `users` Baru:**
  Dokumen kredensial disimpan terpisah dari anggota:
  ```json
  {
    "_id": { "$oid": "..." },
    "nim": "000000001",
    "name": "Admin Testing",
    "password_hash": "$2a$10$...",
    "role": "admin",
    "status": "active",
    "member_id": null,
    "created_at": { "$date": "..." },
    "updated_at": { "$date": "..." }
  }
  ```
- **Koleksi `members` Existing:**
  - Tidak ada field yang dihapus dari `members`.
  - Data existing (13 dokumen anggota di database) tetap utuh tanpa modifikasi destruktif.
- **Indeks yang Ditambahkan:**
  - `idx_users_nim_unique`: `{"nim": 1}` (Unique)
  - `idx_users_role`: `{"role": 1}`
  - `idx_users_member_id_unique_sparse`: `{"member_id": 1}` (Unique, Sparse)
  - `idx_users_status`: `{"status": 1}`

---

## 8. Endpoint yang Tersedia

### Autentikasi (Publik & Sesi)
- `POST /api/v1/auth/login` - Menerima `{ "nim": "...", "password": "..." }`, mengembalikan JWT cookie & redirect URL.
- `POST /api/v1/auth/logout` - Menghapus cookie `auth_token`.
- `GET /api/v1/auth/me` - Mengambil data profil user yang sedang login beserta data keanggotaan (jika role member).

### Halaman Web (HTML)
- `GET /login` - Halaman login terpusat (diarahkan ke `/` atau `/belanja` jika sudah login).
- `GET /masuk` - Redirect otomatis ke `/login`.
- `GET /belanja` - Halaman portal belanja online (terproteksi: `member`, `admin`).
- `GET /` & `/dashboard` - Dashboard analitik (terproteksi: `admin`).
- `GET /products` - Master Barang (terproteksi: `admin`).
- `GET /members` - Master Anggota (terproteksi: `admin`).
- `GET /import` - Import Excel ETL (terproteksi: `admin`).
- `GET /orders/new` - Form Pesanan Kasir (terproteksi: `admin`).
- `GET /forecast` - Forecast & Restock Analitik (terproteksi: `admin`).

---

## 9. Status Integrasi Frontend dan Backend

- [x] Form login HTML mengirim format JSON `{ "nim": ..., "password": ... }`.
- [x] Respon login membaca `redirect_url` dari backend secara otomatis.
- [x] Navbar admin membaca profil dari `GET /api/v1/auth/me` dan menampilkan nama serta role sebenarnya.
- [x] Tombol logout pada navbar memicu `POST /api/v1/auth/logout` dan meredirect browser ke `/login`.
- [x] Halaman `/belanja` memvalidasi cookie sesi dan mengisi nama serta NIM anggota dari sesi aktif.
- [x] Pesanan online dari `/belanja` terkunci ke NIM milik anggota yang sedang login.

---

## 10. Error / Kendala yang Telah Diidentifikasi dan Diselesaikan

1. **Konflik Method Signature `MemberRepository`:**
   Interface awal belum memiliki `FindByID` dan `FindByEmail`. Telah diselaraskan pada domain dan implementasi MongoDB repository.
2. **Duplikasi Helper `isNotFound`:**
   Ditemukan fungsi `isNotFound` yang dideklarasikan ulang di package service. Telah diperbaiki dengan memakai helper domain.
3. **Encoding Glitch pada PowerShell:**
   Karakter unicode section symbol (`§`) sempat menyebabkan mismatch pada pencarian string Windows. Telah diselesaikan dengan byte-safe matching.
4. **Validasi Struct Config:**
   Unit test handler sempat menggunakan `JWTTTL` bertipe `time.Duration`, sementara struct `config.Config` menggunakan `JWTTTLHours int`. Telah diselaraskan ke `JWTTTLHours: 24`.

---

## 11. Hasil Testing Aktual

### A. Verifikasi Kompilasi & Linter
```bash
go vet ./...
# Result: Exit code 0 (bersih tanpa error dan tanpa peringatan)
```

### B. Unit Test AuthService (`internal/service/auth_service_test.go`)
```text
=== RUN   TestAuthService_Login
=== RUN   TestAuthService_Login/Admin_login_success
=== RUN   TestAuthService_Login/Member_login_success
=== RUN   TestAuthService_Login/Wrong_password_rejected
=== RUN   TestAuthService_Login/Non-existent_NIM_rejected
=== RUN   TestAuthService_Login/Empty_password_rejected
=== RUN   TestAuthService_Login/Inactive_user_rejected
--- PASS: TestAuthService_Login (0.23s)
=== RUN   TestAuthService_GetProfile
--- PASS: TestAuthService_GetProfile (0.00s)
=== RUN   TestAuthService_SeedInitialUsers_Idempotent
--- PASS: TestAuthService_SeedInitialUsers_Idempotent (0.12s)
PASS
ok      smartcoop/internal/service      1.689s
```

### C. Integration Test AuthHandler & Authorization (`internal/handler/auth_handler_test.go`)
```text
=== RUN   TestAuthHandler_Login
=== RUN   TestAuthHandler_Login/Admin_login_success_returns_200_and_redirect_/
=== RUN   TestAuthHandler_Login/Member_login_success_returns_200_and_redirect_/belanja
=== RUN   TestAuthHandler_Login/Wrong_password_returns_401_Unauthorized
=== RUN   TestAuthHandler_Login/Missing_credentials_returns_422
--- PASS: TestAuthHandler_Login (0.23s)
=== RUN   TestAuthHandler_Logout
--- PASS: TestAuthHandler_Logout (0.06s)
=== RUN   TestAuthHandler_Me
=== RUN   TestAuthHandler_Me/Unauthenticated_GET_/me_returns_401
=== RUN   TestAuthHandler_Me/Authenticated_GET_/me_returns_200_and_profile
--- PASS: TestAuthHandler_Me (0.06s)
=== RUN   TestRoleBasedAuthorization
=== RUN   TestRoleBasedAuthorization/Member_accessing_admin_API_returns_403_Forbidden
=== RUN   TestRoleBasedAuthorization/Admin_accessing_admin_API_returns_200_OK
=== RUN   TestRoleBasedAuthorization/Unauthenticated_user_accessing_admin_web_page_redirects_to_/login
=== RUN   TestRoleBasedAuthorization/Member_accessing_admin_web_page_redirects_to_/belanja
=== RUN   TestRoleBasedAuthorization/Logged-in_admin_visiting_/login_redirects_to_/
=== RUN   TestRoleBasedAuthorization/Logged-in_member_visiting_/login_redirects_to_/belanja
--- PASS: TestRoleBasedAuthorization (0.06s)
=== RUN   TestMemberOrderOwnershipSecurity
=== RUN   TestMemberOrderOwnershipSecurity/Member_cannot_create_order_for_another_member_NIM_(403)
=== RUN   TestMemberOrderOwnershipSecurity/Member_ordering_with_own_NIM_succeeds
--- PASS: TestMemberOrderOwnershipSecurity (0.00s)
PASS
ok      smartcoop/internal/handler      0.588s
```

---

## 12. Environment Variable yang Diperlukan

Variabel berikut harus tersedia pada `.env` (tanpa membocorkan isi credential):

```env
# Server
PORT=3000
APP_ENV=development

# Database MongoDB
MONGO_URI=mongodb+srv://<username>:<password>@<cluster>.mongodb.net/?retryWrites=true&w=majority
DB_NAME=smartcoop

# Keamanan JWT
JWT_SECRET=super_secret_jwt_key_at_least_32_characters_long
JWT_TTL_HOURS=24

# Initial Seed Credentials (Opsional, memiliki fallback bawaan jika tidak diset)
SEED_ADMIN_PASSWORD=admin123
SEED_MEMBER_PASSWORD=member123
```

---

## 13. Langkah Konkret untuk Melanjutkan Pekerjaan

Saat melanjutkan di sesi berikutnya, ikuti langkah berikut:

1. **Cek Status Workspace:**
   ```bash
   git status
   ```
   Pastikan berada pada branch `azzam` dan tidak ada perubahan yang hilang.
2. **Jalankan Uji Coba Kompilasi:**
   ```bash
   go vet ./...
   go test -v -run "TestAuth" ./internal/service
   go test -v -run "TestAuth|TestRole" ./internal/handler
   ```
3. **Jalankan Aplikasi secara Lokal / Test Run:**
   ```bash
   go run cmd/server/main.go
   ```
4. **Verifikasi Browser Manual:**
   - Kunjungi `http://localhost:3000/login`.
   - Login dengan Akun Admin: NIM `000000001`, password `admin123`. Pastikan masuk ke dashboard `/` dan navbar menampilkan "Admin Testing".
   - Klik tombol logout. Pastikan kembali ke `/login`.
   - Login dengan Akun Member: NIM `000000002`, password `member123`. Pastikan diarahkan ke `/belanja` dan header shop menampilkan "Member Testing".
   - Coba buka URL `http://localhost:3000/products` saat login sebagai member. Pastikan otomatis dialihkan kembali ke `/belanja`.
5. **Jalankan Regression Test Seluruh Modul:**
   ```bash
   go test ./...
   ```
   Pastikan tidak ada modul lama (ETL, Forecast, Products, Orders) yang mengalami regresi.

---

## 14. Urutan Prioritas Pengerjaan Berikutnya

1. **Prioritas 1 (Kritis):** Jalankan server live dan pastikan inisialisasi indeks serta seeding `000000001` & `000000002` sukses masuk ke MongoDB Atlas tanpa benturan data.
2. **Prioritas 2 (Tinggi):** Validasi seluruh flow transaksi kasir (`/orders/new`) dan pesanan online (`/belanja`) memastikan status stock dan ledger keuangan tetap konsisten.
3. **Prioritas 3 (Sedang):** Pembersihan file template duplikat di root `web/templates/*.html` setelah memastikan seluruh rute Fiber hanya mengacu pada subfolder modular `web/templates/<modul>/index.html`.
4. **Prioritas 4 (Finalisasi):** Melakukan commit terstruktur dan dokumentasi akhir sebelum penggabungan branch.
