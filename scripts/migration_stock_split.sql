-- ============================================================
-- MIGRATION: Split stok per posisi + alokasi pinjaman per posisi
-- ============================================================

CREATE TABLE IF NOT EXISTS book_stock_locations (
    id BIGSERIAL PRIMARY KEY,
    book_id INT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    posisi_id INT REFERENCES posisi(id) ON DELETE SET NULL,
    qty INT NOT NULL DEFAULT 0 CHECK (qty >= 0),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(book_id, posisi_id)
);

CREATE INDEX IF NOT EXISTS idx_book_stock_locations_book_id ON book_stock_locations(book_id);
CREATE INDEX IF NOT EXISTS idx_book_stock_locations_posisi_id ON book_stock_locations(posisi_id);

CREATE TABLE IF NOT EXISTS loan_stock_allocations (
    id BIGSERIAL PRIMARY KEY,
    loan_id INT NOT NULL REFERENCES loans(id) ON DELETE CASCADE,
    book_id INT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    posisi_id INT REFERENCES posisi(id) ON DELETE SET NULL,
    qty INT NOT NULL DEFAULT 1 CHECK (qty > 0),
    allocated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    returned_at TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS idx_loan_stock_allocations_loan_id ON loan_stock_allocations(loan_id);
CREATE INDEX IF NOT EXISTS idx_loan_stock_allocations_book_posisi_active ON loan_stock_allocations(book_id, posisi_id) WHERE returned_at IS NULL;

-- Backfill stok awal berdasarkan books.qty + books.posisi_id
INSERT INTO book_stock_locations (book_id, posisi_id, qty)
SELECT b.id, b.posisi_id, GREATEST(COALESCE(b.qty, 1), 1)
FROM books b
WHERE b.posisi_id IS NOT NULL
ON CONFLICT (book_id, posisi_id) DO UPDATE
SET qty = EXCLUDED.qty, updated_at = NOW();

INSERT INTO book_stock_locations (book_id, posisi_id, qty)
SELECT b.id, NULL, GREATEST(COALESCE(b.qty, 1), 1)
FROM books b
WHERE b.posisi_id IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM book_stock_locations s WHERE s.book_id = b.id
  );
