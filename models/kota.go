package models

type Kota struct {
	Kota_ID   int    `gorm:"column:kota_id;primaryKey" json:"kota_id"`
	Kota_Nama string `gorm:"column:kota_nama" json:"kota_nama"`
	// Tambahkan field lain sesuai kolom di GLB_M_Kota
}

func (Kota) TableName() string {
	return "glb_m_kota"
}
