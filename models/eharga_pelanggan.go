package models

import "time"

type MktMEHargaPelanggan struct {
	ID            uint      `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	CustomerID    string    `gorm:"column:customerid;not null" json:"customerid"`
	KotaAsal      string    `gorm:"column:kota_asal;not null" json:"kota_asal"`
	KotaTujuan    string    `gorm:"column:kota_tujuan;not null" json:"kota_tujuan"`
	Kabupaten     string    `gorm:"column:kabupaten" json:"kabupaten"`
	HargaDarat    float64   `gorm:"column:harga_darat;default:0" json:"harga_darat"`
	HargaLaut     float64   `gorm:"column:harga_laut;default:0" json:"harga_laut"`
	HargaUdara    float64   `gorm:"column:harga_udara;default:0" json:"harga_udara"`
	HargaVolume   float64   `gorm:"column:harga_volume;default:0" json:"harga_volume"`
	HargaKubikasi float64   `gorm:"column:harga_kubikasi;default:0" json:"harga_kubikasi"`
	MinKg         float64   `gorm:"column:min_kg;default:0" json:"min_kg"`
	JenisHarga    int       `gorm:"column:jenis_harga;default:0" json:"jenis_harga"` // 0: Berat, 1: Volume, 2: Kubikasi
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (MktMEHargaPelanggan) TableName() string {
	return "public.mkt_m_eharga_pelanggan"
}
