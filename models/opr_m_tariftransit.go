package models

import (
	"time"
)

type OprMTarifTransit struct {
	ID            uint       `json:"id" gorm:"column:id;primaryKey"`
	TrKotaAsal    string     `json:"tr_kotaasal" gorm:"column:tr_kotaasal"`
	TrKotaTujuan  string     `json:"tr_kotatujuan" gorm:"column:tr_kotatujuan"`
	TrKategori    float64    `json:"tr_kategori" gorm:"column:tr_kategori"`       // 👈 Wajib float64 karena di Postgres numeric(10,2)
	TrServiceType float64    `json:"tr_servicetype" gorm:"column:tr_servicetype"` // 👈 Wajib float64 karena di Postgres numeric(10,2)
	TrNominal     float64    `json:"tr_nominal" gorm:"column:tr_nominal"`         // 👈 numeric(15,2)
	CreatedAt     *time.Time `json:"created_at,omitempty" gorm:"column:created_at"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty" gorm:"column:updated_at"`

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
