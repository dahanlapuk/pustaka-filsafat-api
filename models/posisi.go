package models

type Posisi struct {
	ID        int     `json:"id"`
	Kode      string  `json:"kode"`
	Rak       string  `json:"rak"`
	Deskripsi *string `json:"deskripsi"`
}
