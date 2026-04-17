-- ============================================================
-- MIGRATION: Posisi v2 — 4 Dimensi
-- Format kode baru: R{rak_no}-{baris}{kolom_no}-{letak}
-- Contoh: R1-A1-B, R2-#5-F, R3-C10-B, R4-A-F
-- ============================================================

BEGIN;

-- 1. Backup posisi lama
CREATE TABLE IF NOT EXISTS posisi_backup_v1 AS SELECT * FROM posisi;

-- 2. NULL semua foreign key di buku (agar bisa delete posisi)
UPDATE books SET posisi_id = NULL WHERE posisi_id IS NOT NULL;

-- 3. Hapus semua entri posisi lama
DELETE FROM posisi;

-- 4. Reset sequence ID
ALTER SEQUENCE IF EXISTS posisi_id_seq RESTART WITH 1;

-- 5. Tambah kolom dimensi jika belum ada
ALTER TABLE posisi ADD COLUMN IF NOT EXISTS rak_no   INT;
ALTER TABLE posisi ADD COLUMN IF NOT EXISTS baris    VARCHAR(5);
ALTER TABLE posisi ADD COLUMN IF NOT EXISTS kolom_no INT;
ALTER TABLE posisi ADD COLUMN IF NOT EXISTS letak    CHAR(1);

-- 6. Insert 90 entri baru
-- Format: (kode, rak, rak_no, baris, kolom_no, letak)

INSERT INTO posisi (kode, rak, rak_no, baris, kolom_no, letak) VALUES
-- ============ RAK 1: Baris A,B,C × Kolom 1,2 × Letak B,F = 12 entri ============
('R1-A1-B', 'Rak 1', 1, 'A', 1, 'B'),
('R1-A1-F', 'Rak 1', 1, 'A', 1, 'F'),
('R1-A2-B', 'Rak 1', 1, 'A', 2, 'B'),
('R1-A2-F', 'Rak 1', 1, 'A', 2, 'F'),
('R1-B1-B', 'Rak 1', 1, 'B', 1, 'B'),
('R1-B1-F', 'Rak 1', 1, 'B', 1, 'F'),
('R1-B2-B', 'Rak 1', 1, 'B', 2, 'B'),
('R1-B2-F', 'Rak 1', 1, 'B', 2, 'F'),
('R1-C1-B', 'Rak 1', 1, 'C', 1, 'B'),
('R1-C1-F', 'Rak 1', 1, 'C', 1, 'F'),
('R1-C2-B', 'Rak 1', 1, 'C', 2, 'B'),
('R1-C2-F', 'Rak 1', 1, 'C', 2, 'F'),

-- ============ RAK 2: Baris A,B,C,D × Kolom 3-7 + Baris # × Kolom 5 × Letak B,F = 42 entri ============
-- Baris A
('R2-A3-B', 'Rak 2', 2, 'A', 3, 'B'), ('R2-A3-F', 'Rak 2', 2, 'A', 3, 'F'),
('R2-A4-B', 'Rak 2', 2, 'A', 4, 'B'), ('R2-A4-F', 'Rak 2', 2, 'A', 4, 'F'),
('R2-A5-B', 'Rak 2', 2, 'A', 5, 'B'), ('R2-A5-F', 'Rak 2', 2, 'A', 5, 'F'),
('R2-A6-B', 'Rak 2', 2, 'A', 6, 'B'), ('R2-A6-F', 'Rak 2', 2, 'A', 6, 'F'),
('R2-A7-B', 'Rak 2', 2, 'A', 7, 'B'), ('R2-A7-F', 'Rak 2', 2, 'A', 7, 'F'),
-- Baris B
('R2-B3-B', 'Rak 2', 2, 'B', 3, 'B'), ('R2-B3-F', 'Rak 2', 2, 'B', 3, 'F'),
('R2-B4-B', 'Rak 2', 2, 'B', 4, 'B'), ('R2-B4-F', 'Rak 2', 2, 'B', 4, 'F'),
('R2-B5-B', 'Rak 2', 2, 'B', 5, 'B'), ('R2-B5-F', 'Rak 2', 2, 'B', 5, 'F'),
('R2-B6-B', 'Rak 2', 2, 'B', 6, 'B'), ('R2-B6-F', 'Rak 2', 2, 'B', 6, 'F'),
('R2-B7-B', 'Rak 2', 2, 'B', 7, 'B'), ('R2-B7-F', 'Rak 2', 2, 'B', 7, 'F'),
-- Baris C
('R2-C3-B', 'Rak 2', 2, 'C', 3, 'B'), ('R2-C3-F', 'Rak 2', 2, 'C', 3, 'F'),
('R2-C4-B', 'Rak 2', 2, 'C', 4, 'B'), ('R2-C4-F', 'Rak 2', 2, 'C', 4, 'F'),
('R2-C5-B', 'Rak 2', 2, 'C', 5, 'B'), ('R2-C5-F', 'Rak 2', 2, 'C', 5, 'F'),
('R2-C6-B', 'Rak 2', 2, 'C', 6, 'B'), ('R2-C6-F', 'Rak 2', 2, 'C', 6, 'F'),
('R2-C7-B', 'Rak 2', 2, 'C', 7, 'B'), ('R2-C7-F', 'Rak 2', 2, 'C', 7, 'F'),
-- Baris D
('R2-D3-B', 'Rak 2', 2, 'D', 3, 'B'), ('R2-D3-F', 'Rak 2', 2, 'D', 3, 'F'),
('R2-D4-B', 'Rak 2', 2, 'D', 4, 'B'), ('R2-D4-F', 'Rak 2', 2, 'D', 4, 'F'),
('R2-D5-B', 'Rak 2', 2, 'D', 5, 'B'), ('R2-D5-F', 'Rak 2', 2, 'D', 5, 'F'),
('R2-D6-B', 'Rak 2', 2, 'D', 6, 'B'), ('R2-D6-F', 'Rak 2', 2, 'D', 6, 'F'),
('R2-D7-B', 'Rak 2', 2, 'D', 7, 'B'), ('R2-D7-F', 'Rak 2', 2, 'D', 7, 'F'),
-- Baris # (pagar) — hanya kolom 5, posisi khusus di atas baris A
('R2-#5-B', 'Rak 2', 2, '#', 5, 'B'),
('R2-#5-F', 'Rak 2', 2, '#', 5, 'F'),

-- ============ RAK 3: Baris A,B,C × Kolom 8-12 × Letak B,F = 30 entri ============
-- Baris A
('R3-A8-B',  'Rak 3', 3, 'A', 8,  'B'), ('R3-A8-F',  'Rak 3', 3, 'A', 8,  'F'),
('R3-A9-B',  'Rak 3', 3, 'A', 9,  'B'), ('R3-A9-F',  'Rak 3', 3, 'A', 9,  'F'),
('R3-A10-B', 'Rak 3', 3, 'A', 10, 'B'), ('R3-A10-F', 'Rak 3', 3, 'A', 10, 'F'),
('R3-A11-B', 'Rak 3', 3, 'A', 11, 'B'), ('R3-A11-F', 'Rak 3', 3, 'A', 11, 'F'),
('R3-A12-B', 'Rak 3', 3, 'A', 12, 'B'), ('R3-A12-F', 'Rak 3', 3, 'A', 12, 'F'),
-- Baris B
('R3-B8-B',  'Rak 3', 3, 'B', 8,  'B'), ('R3-B8-F',  'Rak 3', 3, 'B', 8,  'F'),
('R3-B9-B',  'Rak 3', 3, 'B', 9,  'B'), ('R3-B9-F',  'Rak 3', 3, 'B', 9,  'F'),
('R3-B10-B', 'Rak 3', 3, 'B', 10, 'B'), ('R3-B10-F', 'Rak 3', 3, 'B', 10, 'F'),
('R3-B11-B', 'Rak 3', 3, 'B', 11, 'B'), ('R3-B11-F', 'Rak 3', 3, 'B', 11, 'F'),
('R3-B12-B', 'Rak 3', 3, 'B', 12, 'B'), ('R3-B12-F', 'Rak 3', 3, 'B', 12, 'F'),
-- Baris C
('R3-C8-B',  'Rak 3', 3, 'C', 8,  'B'), ('R3-C8-F',  'Rak 3', 3, 'C', 8,  'F'),
('R3-C9-B',  'Rak 3', 3, 'C', 9,  'B'), ('R3-C9-F',  'Rak 3', 3, 'C', 9,  'F'),
('R3-C10-B', 'Rak 3', 3, 'C', 10, 'B'), ('R3-C10-F', 'Rak 3', 3, 'C', 10, 'F'),
('R3-C11-B', 'Rak 3', 3, 'C', 11, 'B'), ('R3-C11-F', 'Rak 3', 3, 'C', 11, 'F'),
('R3-C12-B', 'Rak 3', 3, 'C', 12, 'B'), ('R3-C12-F', 'Rak 3', 3, 'C', 12, 'F'),

-- ============ RAK 4: Baris A,B,C × (tanpa kolom) × Letak B,F = 6 entri ============
('R4-A-B', 'Rak 4', 4, 'A', NULL, 'B'),
('R4-A-F', 'Rak 4', 4, 'A', NULL, 'F'),
('R4-B-B', 'Rak 4', 4, 'B', NULL, 'B'),
('R4-B-F', 'Rak 4', 4, 'B', NULL, 'F'),
('R4-C-B', 'Rak 4', 4, 'C', NULL, 'B'),
('R4-C-F', 'Rak 4', 4, 'C', NULL, 'F');

-- 7. Verifikasi
SELECT COUNT(*) AS total_posisi FROM posisi;
SELECT rak, COUNT(*) AS jumlah FROM posisi GROUP BY rak ORDER BY rak;
SELECT COUNT(*) AS buku_tanpa_posisi FROM books WHERE posisi_id IS NULL;

COMMIT;
