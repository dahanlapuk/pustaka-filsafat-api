-- Migration: tambah tabel loan_requests dan delete_requests
-- Jalankan: psql -h localhost -U postgres -d pustaka_filsafat -f migration_requests.sql

-- Tabel pengajuan peminjaman dari publik
CREATE TABLE IF NOT EXISTS loan_requests (
    id SERIAL PRIMARY KEY,
    book_id INT REFERENCES books(id) ON DELETE CASCADE,
    nama_pemohon TEXT NOT NULL,
    whatsapp TEXT,
    email TEXT,
    keperluan TEXT,
    status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    processed_by INT REFERENCES admins(id) ON DELETE SET NULL,
    processed_at TIMESTAMP,
    catatan_admin TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Tabel pengajuan penghapusan buku (dari admin biasa, disetujui superadmin)
CREATE TABLE IF NOT EXISTS delete_requests (
    id SERIAL PRIMARY KEY,
    book_id INT REFERENCES books(id) ON DELETE CASCADE,
    judul_snapshot TEXT NOT NULL,    -- simpan judul saat pengajuan agar tidak hilang jika buku sudah dihapus
    alasan TEXT NOT NULL,
    requested_by INT REFERENCES admins(id) ON DELETE SET NULL,
    requested_by_nama TEXT,
    status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by INT REFERENCES admins(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMP,
    catatan_review TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Index untuk query cepat
CREATE INDEX IF NOT EXISTS idx_loan_requests_status ON loan_requests(status);
CREATE INDEX IF NOT EXISTS idx_loan_requests_book ON loan_requests(book_id);
CREATE INDEX IF NOT EXISTS idx_delete_requests_status ON delete_requests(status);
CREATE INDEX IF NOT EXISTS idx_delete_requests_book ON delete_requests(book_id);

-- Tabel sesi admin untuk auth token-based (single-session)
CREATE TABLE IF NOT EXISTS admin_sessions (
    id BIGSERIAL PRIMARY KEY,
    admin_id INT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    issued_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    invalidated_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_sessions_admin_active
    ON admin_sessions(admin_id, invalidated_at);

CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires_at
    ON admin_sessions(expires_at);
