package models

import (
	"time"
)

// Model untuk Tabel Transaksi Log GPS
type HrdTKaryawanAbsenUpdateLokasi struct {
	KaulID         uint      `gorm:"primaryKey;column:kaul_id;autoIncrement" json:"kaul_id"`
	KaulNip        string    `gorm:"column:kaul_nip;type:varchar(50);not null" json:"kaul_nip"`
	KaulUpdateTime time.Time `gorm:"column:kaul_updatetime;default:CURRENT_TIMESTAMP" json:"kaul_updatetime"`
	KaulLat        float64   `gorm:"column:kaul_lat;type:numeric(12,8)" json:"kaul_lat"`
	KaulLong       float64   `gorm:"column:kaul_long;type:numeric(12,8)" json:"kaul_long"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (HrdTKaryawanAbsenUpdateLokasi) TableName() string {
	return "public.hrd_t_karyawanabsenupdatelokasi"
}

// Model DTO untuk Response List Monitoring Lokasi Terakhir
type KaryawanLokasiResponse struct {
	KryNip         string    `json:"kry_nip"`
	KryNama        string    `json:"kry_nama"`
	KaulUpdateTime time.Time `json:"kaul_updatetime"`
	KaulLat        float64   `json:"kaul_lat"`
	KaulLong       float64   `json:"kaul_long"`
}

// Model DTO untuk Request Payload kirim lokasi dari HP/Mobile
type UpdateLokasiRequest struct {
	Nip string  `json:"nip" binding:"required"`
	Lat float64 `json:"lat" binding:"required"`
	Lng float64 `json:"lng" binding:"required"`
}
