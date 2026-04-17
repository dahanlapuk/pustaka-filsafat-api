-- ============================================
-- PUSTAKA FILSAFAT — DATABASE SCHEMA
-- PostgreSQL
-- ============================================

-- Drop existing tables if any (for clean setup)
DROP TABLE IF EXISTS logs CASCADE;
DROP TABLE IF EXISTS loans CASCADE;
DROP TABLE IF EXISTS books CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS posisi CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- ============================================
-- USERS TABLE
-- ============================================
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'viewer',  -- 'admin' or 'viewer'
    created_at TIMESTAMP DEFAULT NOW()
);

-- ============================================
-- POSISI TABLE (Posisi Rak)
-- ============================================
CREATE TABLE posisi (
    id SERIAL PRIMARY KEY,
    kode TEXT UNIQUE NOT NULL,  -- 'A1', 'A2', 'B3', etc.
    rak TEXT NOT NULL,          -- 'Rak 1', 'Rak 2'
    deskripsi TEXT
);

-- ============================================
-- CATEGORIES TABLE
-- ============================================
CREATE TABLE categories (
    id SERIAL PRIMARY KEY,
    nama TEXT UNIQUE NOT NULL
);

-- ============================================
-- BOOKS TABLE
-- ============================================
CREATE TABLE books (
    id SERIAL PRIMARY KEY,
    kode TEXT,                              -- bisa kosong/NULL (49% buku tanpa kode)
    judul TEXT NOT NULL,
    kategori_id INT REFERENCES categories(id) ON DELETE SET NULL,
    posisi_id INT REFERENCES posisi(id) ON DELETE SET NULL,
    qty INT DEFAULT 1,
    keterangan TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Index for search
CREATE INDEX idx_books_judul ON books USING GIN(to_tsvector('indonesian', judul));
CREATE INDEX idx_books_kode ON books(kode);
CREATE INDEX idx_books_kategori ON books(kategori_id);
CREATE INDEX idx_books_posisi ON books(posisi_id);

-- ============================================
-- LOANS TABLE (Peminjaman)
-- ============================================
CREATE TABLE loans (
    id SERIAL PRIMARY KEY,
    book_id INT REFERENCES books(id) ON DELETE CASCADE,
    nama_peminjam TEXT NOT NULL,
    tanggal_pinjam DATE DEFAULT CURRENT_DATE,
    tanggal_kembali DATE,                   -- NULL = belum dikembalikan
    catatan TEXT,
    dicatat_oleh INT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_loans_book ON loans(book_id);
CREATE INDEX idx_loans_active ON loans(book_id) WHERE tanggal_kembali IS NULL;

-- ============================================
-- LOGS TABLE (Activity Log)
-- ============================================
CREATE TABLE logs (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id) ON DELETE SET NULL,
    book_id INT REFERENCES books(id) ON DELETE CASCADE,
    action TEXT NOT NULL,                   -- 'create', 'update', 'delete', 'borrow', 'return'
    detail TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_logs_book ON logs(book_id);
CREATE INDEX idx_logs_created ON logs(created_at);

-- ============================================
-- SEED DATA — POSISI RAK
-- Based on real data: Rak 1 (A1-C2), Rak 2 (A3-B5)
-- ============================================
INSERT INTO posisi (kode, rak) VALUES
    ('A1', 'Rak 1'),
    ('A2', 'Rak 1'),
    ('B1', 'Rak 1'),
    ('B2', 'Rak 1'),
    ('C1', 'Rak 1'),
    ('C2', 'Rak 1'),
    ('A3', 'Rak 2'),
    ('A4', 'Rak 2'),
    ('A5', 'Rak 2'),
    ('A6', 'Rak 2'),
    ('A7', 'Rak 2'),
    ('B3', 'Rak 2'),
    ('B4', 'Rak 2'),
    ('B5', 'Rak 2');

-- ============================================
-- SEED DATA — DEFAULT ADMIN USER
-- Password: admin123 (change in production!)
-- ============================================
INSERT INTO users (name, email, password, role) VALUES
    ('Admin', 'admin@pustaka.filsafat', '$2a$10$rIC/bXnO8PBH7CsH0N.E4.eBgINhG2MzYzF9K9MZl9nW.Q9mJ2kNO', 'admin');

-- Done!
SELECT 'Schema created successfully!' as status;
