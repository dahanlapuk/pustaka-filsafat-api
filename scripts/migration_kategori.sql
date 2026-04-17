-- ============================================================
-- MIGRATION: Category Requests Table
-- Untuk workflow: admin ajukan kategori baru → superadmin approve
-- ============================================================

CREATE TABLE IF NOT EXISTS category_requests (
    id                 SERIAL PRIMARY KEY,
    nama_requested     TEXT        NOT NULL,
    alasan             TEXT,
    requested_by       INT         REFERENCES admins(id) ON DELETE SET NULL,
    requested_by_nama  TEXT        NOT NULL,
    status             TEXT        NOT NULL DEFAULT 'pending'
                                   CHECK (status IN ('pending','approved','rejected')),
    reviewed_by        INT         REFERENCES admins(id) ON DELETE SET NULL,
    reviewed_by_nama   TEXT,
    reviewed_at        TIMESTAMP,
    catatan_review     TEXT,
    created_at         TIMESTAMP   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cat_requests_status ON category_requests(status);
CREATE INDEX IF NOT EXISTS idx_cat_requests_created ON category_requests(created_at DESC);
