package models

import "time"

type Book struct {
	ID          int        `json:"id"`
	Kode        *string    `json:"kode"`
	Judul       string     `json:"judul"`
	KategoriID  *int       `json:"kategori_id"`
	PosisiID    *int       `json:"posisi_id"`
	Qty         int        `json:"qty"`
	Keterangan  *string    `json:"keterangan"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastChecked *time.Time `json:"last_checked,omitempty"`
	CheckedBy   *string    `json:"checked_by,omitempty"`

	// Joined fields
	KategoriNama *string `json:"kategori_nama,omitempty"`
	PosisiKode   *string `json:"posisi_kode,omitempty"`
	PosisiRak    *string `json:"posisi_rak,omitempty"`
	IsDipinjam   bool    `json:"is_dipinjam"`
	Peminjam     *string `json:"peminjam,omitempty"`
}

type BookInput struct {
	Kode       *string `json:"kode"`
	Judul      string  `json:"judul"`
	KategoriID *int    `json:"kategori_id"`
	PosisiID   *int    `json:"posisi_id"`
	Qty        int     `json:"qty"`
	Keterangan *string `json:"keterangan"`
	AdminID    *int    `json:"admin_id"`   // ID admin yang melakukan aksi
	AdminNama  string  `json:"admin_nama"` // Nama admin
}

// InventoryCheckInput - untuk fitur absen buku
type InventoryCheckInput struct {
	BookIDs     []int  `json:"book_ids"`      // ID buku yang dicek
	CheckedBy   string `json:"checked_by"`    // Nama petugas
	NewPosisiID *int   `json:"new_posisi_id"` // Posisi baru (opsional, jika pindah)
}
