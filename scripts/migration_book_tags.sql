-- ============================================================
-- MIGRATION: Multi-tagging for books
-- Adds optional category grouping and book-category junction table.
-- ============================================================

ALTER TABLE categories
    ADD COLUMN IF NOT EXISTS grouping TEXT CHECK (grouping IN ('bentuk', 'konten', 'lain'));

CREATE TABLE IF NOT EXISTS book_categories (
    id BIGSERIAL PRIMARY KEY,
    book_id INT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    category_id INT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(book_id, category_id)
);

CREATE INDEX IF NOT EXISTS idx_book_categories_book ON book_categories(book_id);
CREATE INDEX IF NOT EXISTS idx_book_categories_category ON book_categories(category_id);

-- Backfill kategori utama existing menjadi tag awal.
INSERT INTO book_categories (book_id, category_id)
SELECT id, kategori_id
FROM books
WHERE kategori_id IS NOT NULL
ON CONFLICT (book_id, category_id) DO NOTHING;
