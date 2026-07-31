package models

import "time"

type OprMEarea struct {
	ID                  uint      `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	AreaAgenID          string    `gorm:"column:area_agenid;not null" json:"area_agenid"`
	TujuanKecamatan     string    `gorm:"column:tujuan_kecamatan" json:"tujuan_kecamatan"`
	TujuanKabupaten     string    `gorm:"column:tujuan_kabupaten" json:"tujuan_kabupaten"`
	TujuanPropinsi      string    `gorm:"column:tujuan_propinsi" json:"tujuan_propinsi"`
	HandDarat           float64   `gorm:"column:hand_darat;default:0" json:"hand_darat"`
	HandLaut            float64   `gorm:"column:hand_laut;default:0" json:"hand_laut"`
	HandUdara           float64   `gorm:"column:hand_udara;default:0" json:"hand_udara"`
	HandDaratKurir      float64   `gorm:"column:hand_daratkurir;default:0" json:"hand_daratkurir"`
	HandLautKurir       float64   `gorm:"column:hand_lautkurir;default:0" json:"hand_lautkurir"`
	HandUdaraKurir      float64   `gorm:"column:hand_udarakurir;default:0" json:"hand_udarakurir"`
	PickupAgenID        string    `gorm:"column:pickup_agenid" json:"pickup_agenid"`
	PenerusYN           string    `gorm:"column:penerusyn;default:'N'" json:"penerusyn"`
	KgMin               float64   `gorm:"column:kgmin;default:0" json:"kgmin"`
	HrgPenerus          float64   `gorm:"column:hrgpenerus;default:0" json:"hrgpenerus"`
	LeadTime            string    `gorm:"column:leadtime" json:"leadtime"`
	ProsentaseByKirimYN string    `gorm:"column:prosentasebykirimyn;default:'N'" json:"prosentasebykirimyn"`
	CreatedAt           time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (OprMEarea) TableName() string {
	return "public.opr_m_earea"
}
