package handlers

import (
	"database/sql"
	"fmt"
	"sync"
)

var ensureBookTagSchemaOnce sync.Once
var ensureBookTagSchemaErr error

// EnsureBookTagSchema memastikan tabel relasi buku-kategori untuk multi-tagging tersedia.
func EnsureBookTagSchema(db *sql.DB) error {
	ensureBookTagSchemaOnce.Do(func() {
		statements := []string{
			`ALTER TABLE categories
				ADD COLUMN IF NOT EXISTS grouping TEXT CHECK (grouping IN ('bentuk', 'konten', 'lain'))`,
			`CREATE TABLE IF NOT EXISTS book_categories (
				id BIGSERIAL PRIMARY KEY,
				book_id INT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
				category_id INT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
				created_at TIMESTAMP NOT NULL DEFAULT NOW(),
				UNIQUE(book_id, category_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_book_categories_book_id ON book_categories(book_id)`,
			`CREATE INDEX IF NOT EXISTS idx_book_categories_category_id ON book_categories(category_id)`,
			`INSERT INTO book_categories (book_id, category_id)
				SELECT id, kategori_id
				FROM books
				WHERE kategori_id IS NOT NULL
				ON CONFLICT (book_id, category_id) DO NOTHING`,
		}

		for _, stmt := range statements {
			if _, err := db.Exec(stmt); err != nil {
				ensureBookTagSchemaErr = fmt.Errorf("gagal memastikan skema book tags: %w", err)
				return
			}
		}
	})

	return ensureBookTagSchemaErr
}
