package models

import (
	"time"
)

type BTT struct {
	ID           string    `gorm:"column:bttt_id;primaryKey" json:"id"`
	Tanggal      time.Time `gorm:"column:bttt_tanggal" json:"tanggal"`
	AsalNama     string    `gorm:"column:bttt_asalname" json:"asal_name"`
	AsalKota     string    `gorm:"column:bttt_asalkota" json:"asal_kota"`
	TujuanNama   string    `gorm:"column:bttt_tujuannama" json:"tujuan_nama"`
	TujuanKota   string    `gorm:"column:bttt_tujuankota" json:"tujuan_kota"`
	TujuanKec    string    `gorm:"column:bttt_tujuankecamatan" json:"tujuan_kecamatan"`
	NamaBarang   string    `gorm:"column:bttt_namabarang" json:"nama_barang"`
	JmlPck       int       `gorm:"column:bttt_jmlpck" json:"jml_pck"`
	Berat        float64   `gorm:"column:bttt_berat" json:"berat"`
	Harga        float64   `gorm:"column:bttt_harga" json:"harga"`
	BiayaPenerus float64   `gorm:"column:bttt_biayapenerus" json:"biaya_penerus"`
}

type Closing struct {
	ClosingPeriode string `gorm:"column:closing_periode;primaryKey"`
	ClosingStatus  string `gorm:"column:closing_status"`
}

func (BTT) TableName() string {
	return "mkt_t_econote"
}

func (Closing) TableName() string {
	return "public.glb_m_closing"
}

// MasterHarga mewakili tabel mkt_m_harga atau mkt_m_eharga_pelanggan
type MasterHarga struct {
	HargaPerKg   float64 `gorm:"column:hrg_perkg"`
	MinBerat     float64 `gorm:"column:hrg_minberat"`
	BiayaPenerus float64 `gorm:"column:hrg_penerus"`
}

type TarifCalculation struct {
	HargaPerKg   float64
	TotalBerat   float64
	BiayaPenerus float64
	HargaTotal   float64
}

type TarifResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		BeratKena       float64 `json:"berat_kena"`
		HargaPerKg      float64 `json:"harga_per_kg"`
		BiayaPenerus    float64 `json:"biaya_penerus"`
		HargaTotal      float64 `json:"harga_total"`
		JenisLayanan    string  `json:"jenis_layanan"`
		AgenID          string  `json:"agen_id"`
		AsalKota        string  `json:"asal_kota"`
		TujuanKecamatan string  `json:"tujuan_kecamatan"`
	} `json:"data"`
}

// BttValidateRequest menangkap data lengkap untuk validasi akhir di server
type BttValidateRequest struct {
	AsalTelp   string  `json:"bttt_asaltelp" binding:"required"`
	TujuanTelp string  `json:"bttt_tujuantelp" binding:"required"`
	CaraBayar  string  `json:"bttt_jenisharga"` // Contoh: '0' Cash, '1' Tagih Tujuan / COD
	GrandTotal float64 `json:"bttt_harga" binding:"required"`
	AsalCustID string  `json:"bttt_asalcustid" binding:"required"` // ID Agen/Customer untuk cek plafon
}

// GlbMAgen mewakili skema tabel public.glb_m_agen asli database kamu, Bro!
type GlbMAgen struct {
	AgenID     string `gorm:"column:agen_id;primaryKey"`
	AgenKotaID string `gorm:"column:agen_kotaid"`
	AgenKode   string `gorm:"column:agen_kode"`
	AgenNama   string `gorm:"column:agen_nama"`
}
