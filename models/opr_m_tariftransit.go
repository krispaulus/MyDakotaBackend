package models

import (
	"time"
)

type OprMTarifTransit struct {
	ID            uint      `gorm:"primaryKey;column:id;autoIncrement" json:"id"`
	TrKotaAsal    string    `gorm:"column:tr_kotaasal;type:varchar(100);not null" json:"tr_kotaasal"`
	TrKotaTujuan  string    `gorm:"column:tr_kotatujuan;type:varchar(100);not null" json:"tr_kotatujuan"`
	TrKategori    int       `gorm:"column:tr_kategori;default:0" json:"tr_kategori"`       // 0: Surat Perintah, 1: Loper
	TrServiceType int       `gorm:"column:tr_servicetype;default:1" json:"tr_servicetype"` // 1: Darat, 2: Laut, 3: Udara
	TrNominal     float64   `gorm:"column:tr_nominal;default:0" json:"tr_nominal"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`

	// Relasi virtual/alias untuk JOIN Provinsi dari GLB_M_eKodePos
	ProvinsiAsal   string `gorm:"->" json:"provinsi_asal,omitempty"`
	ProvinsiTujuan string `gorm:"->" json:"provinsi_tujuan,omitempty"`
}

func (OprMTarifTransit) TableName() string {
	return "opr_m_tariftransit"
}

// DTO untuk Payload Request (Tambah/Edit)
type TarifTransitRequest struct {
	ProvinsiAsal   string  `json:"provinsi_asal"`
	KotaAsal       string  `json:"kota_asal" binding:"required"`
	ProvinsiTujuan string  `json:"provinsi_tujuan"`
	KotaTujuan     string  `json:"kota_tujuan" binding:"required"`
	Kategori       int     `json:"kategori"`
	Service        int     `json:"service"`
	Nominal        float64 `json:"nominal"`
}
