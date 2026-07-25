package models

import "time"

// MktMeHargaCharter memetakan tabel core tarif carter Dakota Cargo
type MktMeHargaCharter struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AgenidAsal      string    `gorm:"column:agenid_asal" json:"agenid_asal"`
	KendJenis       string    `gorm:"column:kend_jenis" json:"kend_jenis"`
	TujuanKabupaten string    `gorm:"column:tujuan_kabupaten" json:"tujuan_kabupaten"`
	TujuanPropinsi  string    `gorm:"column:tujuan_propinsi" json:"tujuan_propinsi"`
	HargaPokok      float64   `gorm:"column:hargapokok;default:0.00" json:"hargapokok"`
	Keterangan      string    `gorm:"column:keterangan" json:"keterangan"`
	UpdateID        *string   `gorm:"column:update_id" json:"update_id"`
	UpdateTime      time.Time `gorm:"column:update_time;default:now()" json:"update_time"`
}

func (MktMeHargaCharter) TableName() string {
	return "mkt_m_eharga_charter"
}

// DTO Penampung untuk List Halaman Utama (Index bray)
type CarterIndexDTO struct {
	AgenID     string `json:"agen_id"`
	AgenNama   string `json:"agen_nama"`
	AgenAlamat string `json:"agen_alamat"`
	AgenKota   string `json:"agen_kota"`
	AgenPhone1 string `json:"agen_phone1"`
	Jml        int64  `json:"jml"`
}
