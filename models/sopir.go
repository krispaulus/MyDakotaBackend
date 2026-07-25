package models

import "time"

// OprMSupir memetakan tabel induk opr_m_supir sesuai skema asli SQL Server
type OprMSupir struct {
	SupirID         string    `gorm:"column:supir_id;primaryKey" json:"supir_id"`
	SupirNama       string    `gorm:"column:supir_nama" json:"supir_nama"`
	SupirBoronganYN string    `gorm:"column:supir_borongan_yn;default:N" json:"supir_borongan_yn"`
	SupirAktifYN    string    `gorm:"column:supir_aktif_yn;default:Y" json:"supir_aktif_yn"`
	SupirImei1      *string   `gorm:"column:supir_imei1" json:"supir_imei1"`
	SupirImei2      *string   `gorm:"column:supir_imei2" json:"supir_imei2"`
	SupirSimcardID1 *string   `gorm:"column:supir_simcard_id1" json:"supir_simcard_id1"`
	SupirSimcardID2 *string   `gorm:"column:supir_simcard_id2" json:"supir_simcard_id2"`
	SupirUpdateID   *string   `gorm:"column:supir_update_id" json:"supir_update_id"`
	SupirUpdateTime time.Time `gorm:"column:supir_update_time;default:now()" json:"supir_update_time"`
}

// TableName menegaskan nama tabel di pgAdmin bray
func (OprMSupir) TableName() string {
	return "opr_m_supir"
}
