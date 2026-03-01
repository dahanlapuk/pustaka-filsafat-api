package models

import "time"

type Loan struct {
	ID             int        `json:"id"`
	BookID         int        `json:"book_id"`
	NamaPeminjam   string     `json:"nama_peminjam"`
	TanggalPinjam  time.Time  `json:"tanggal_pinjam"`
	TanggalKembali *time.Time `json:"tanggal_kembali"`
	Catatan        *string    `json:"catatan"`
	DicatatOleh    *int       `json:"dicatat_oleh"`

	// Joined fields
	JudulBuku *string `json:"judul_buku,omitempty"`
	KodeBuku  *string `json:"kode_buku,omitempty"`
}

type LoanInput struct {
	BookID       int     `json:"book_id"`
	NamaPeminjam string  `json:"nama_peminjam"`
	Catatan      *string `json:"catatan"`
}
