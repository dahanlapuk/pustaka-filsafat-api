# Pustaka Filsafat - Backend

REST API server untuk sistem manajemen perpustakaan Program Studi Ilmu Filsafat FIB UI.

## Tech Stack

- **Go** 1.21+
- **Fiber** v2 - Web framework
- **PostgreSQL** - Database
- **lib/pq** - PostgreSQL driver

## Setup

### 1. Prerequisites

- Go 1.21 atau lebih baru
- PostgreSQL 14+

### 2. Database

```bash
# Buat database
psql -U postgres -c "CREATE DATABASE pustaka_filsafat;"

# Import schema
psql -U postgres -d pustaka_filsafat -f ../scripts/schema.sql
```

### 3. Environment

```bash
cp .env.example .env
# Edit .env sesuai konfigurasi database Anda
```

### 4. Install Dependencies

```bash
go mod download
```

### 5. Run Server

```bash
# Development
go run .

# Build & Run
go build -o server .
./server
```

Server berjalan di `http://localhost:3000`

## API Endpoints

### Books
- `GET /api/books` - List semua buku
- `GET /api/books/:id` - Detail buku
- `POST /api/books` - Tambah buku
- `PUT /api/books/:id` - Update buku
- `DELETE /api/books/:id` - Hapus buku (perlu alasan)

### Categories & Positions
- `GET /api/categories` - List kategori
- `GET /api/posisi` - List posisi/rak

### Loans
- `GET /api/loans` - List peminjaman
- `POST /api/loans` - Buat peminjaman
- `PUT /api/loans/:id/return` - Kembalikan buku

### Inventory
- `GET /api/inventory/stats` - Statistik inventaris
- `POST /api/inventory/check` - Absen buku

### Admin
- `GET /api/admins` - List admin
- `POST /api/admins/login` - Login
- `POST /api/admins/logout` - Logout
- `PUT /api/admins/:id/profile` - Update profil
- `PUT /api/admins/:id/password` - Ganti password

### Activity Logs
- `GET /api/logs` - List log aktivitas
- `GET /api/logs/stats` - Statistik log

## Struktur Folder

```
backend/
├── handlers/       # Request handlers
├── middleware/     # Auth middleware
├── models/         # Data models
├── scripts/        # Utility scripts
├── db.go           # Database connection
├── routes.go       # Route definitions
└── main.go         # Entry point
```

## License

MIT
