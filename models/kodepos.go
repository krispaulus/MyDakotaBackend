package models

type KodePos struct {
	KodePosID        string `gorm:"primaryKey;column:kodepos_id" json:"kodepos_id"`
	KodePos          string `gorm:"column:kodepos" json:"kodepos"`
	DesaKelurahan    string `gorm:"column:desakelurahan" json:"desakelurahan"`
	KecamatanDistrik string `gorm:"column:kecamatandistrik" json:"kecamatandistrik"`
	KotaKabupaten    string `gorm:"column:kotakabupaten" json:"kotakabupaten"`
	Propinsi         string `gorm:"column:propinsi" json:"propinsi"`
	Area             string `gorm:"column:area" json:"area"`
}

func (KodePos) TableName() string {
	return "glb_m_kodepos"
}
