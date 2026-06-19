package models

import "time"

// Tarif Reguler
type TarifReguler struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	GeneratedID        string    `json:"generated_id"`
	AsalKota           string    `json:"asal_kota"`
	ServID             int       `json:"serv_id"`
	TujuanPropinsi     string    `json:"tujuan_propinsi"`
	TujuanKabupaten    string    `json:"tujuan_kabupaten"`
	TujuanKecamatan    string    `json:"tujuan_kecamatan"`
	MinimalKG          float64   `json:"minimal_kg"`
	HargaPokok         float64   `json:"harga_pokok"`
	HargaKGSelanjutnya float64   `json:"harga_kg_selanjutnya"`
	EstimasiHari       int       `json:"estimasi_hari"`
	FlagDS             string    `json:"flag_ds"`
	CreatedAt          time.Time `json:"created_at"`
}

func (TarifReguler) TableName() string {
	return "master_tarif_reguler"
}

// Tarif Ekonomis
type TarifEkonomis struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	GeneratedID        string    `json:"generated_id"`
	AsalKota           string    `json:"asal_kota"`
	ServID             int       `json:"serv_id"`
	TujuanPropinsi     string    `json:"tujuan_propinsi"`
	TujuanKabupaten    string    `json:"tujuan_kabupaten"`
	TujuanKecamatan    string    `json:"tujuan_kecamatan"`
	MinimalKG          float64   `json:"minimal_kg"`
	HargaPokok         float64   `json:"harga_pokok"`
	HargaKGSelanjutnya float64   `json:"harga_kg_selanjutnya"`
	EstimasiHari       int       `json:"estimasi_hari"`
	FlagDS             string    `json:"flag_ds"`
	CreatedAt          time.Time `json:"created_at"`
}

func (TarifEkonomis) TableName() string {
	return "master_tarif_ekonomis" // GANTI dengan nama tabel asli di Postgres kamu!
}

// Tarif Unit
type TarifUnit struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	NamaKategori string    `json:"nama_kategori"`
	Jenis        string    `json:"jenis"`
	Satuan       string    `json:"satuan"`
	CCMin        int       `json:"cc_min"`
	CCMax        int       `json:"cc_max"`
	BeratStd     float64   `json:"berat_std"`
	AktifYN      string    `json:"aktif_yn"`
	FaktorX      float64   `json:"faktor_x"`
	UpdateID     string    `json:"update_id"`
	UpdateTime   time.Time `json:"update_time"`
}

func (TarifUnit) TableName() string {
	return "master_tarif_unit"
}

// TarifRequest menangkap data inputan dimensi & rute dari form React
type TarifRequest struct {
	AgenID       string  `json:"agen_id"`
	AsalKota     string  `json:"asal_kota" binding:"required"`
	TujuanKec    string  `json:"tujuan_kecamatan" binding:"required"`
	BeratAsli    float64 `json:"berat_asli" binding:"required"`
	Panjang      float64 `json:"panjang"`
	Lebar        float64 `json:"lebar"`
	Tinggi       float64 `json:"tinggi"`
	JenisLayanan string  `json:"jenis_layanan"` // "REGULER" atau "EKONOMIS"
}
