package models

import "time"

type OprMTarifHandlingByPropinsi struct {
	Propinsi      string    `gorm:"primaryKey;column:propinsi" json:"propinsi"`
	ProsentaseYN  string    `gorm:"column:prosentaseyn;default:'N'" json:"prosentaseyn"`
	ProsentaseVal float64   `gorm:"column:prosentaseval;default:0" json:"prosentaseval"`
	TarifYN       string    `gorm:"column:tarifyn;default:'N'" json:"tarifyn"`
	TarifVal      float64   `gorm:"column:tarifval;default:0" json:"tarifval"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (OprMTarifHandlingByPropinsi) TableName() string {
	return "public.opr_m_tarifhandlingbypropinsi"
}
