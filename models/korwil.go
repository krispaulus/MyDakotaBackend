package models

// GlbMKorwil memetakan tabel induk glb_m_korwil
type GlbMKorwil struct {
	KdKorwil      string  `gorm:"column:kd_korwil;primaryKey" json:"kd_korwil"`
	NamaWilayah   string  `gorm:"column:nama_wilayah" json:"nama_wilayah"`
	NipKaryawan   *string `gorm:"column:nip_karyawan" json:"nip_karyawan"`
	KorwilAktifYN string  `gorm:"column:korwil_aktif_yn;default:Y" json:"korwil_aktif_yn"`
	KetKorwil     *string `gorm:"column:ket_korwil" json:"ket_korwil"`
	UpdateID      *string `gorm:"column:update_id" json:"update_id"`
	UpdateTime    *string `gorm:"column:update_time" json:"update_time"`

	// Field tambahan penampung join data HRD Karyawan
	KryNama  string `gorm:"-" json:"kry_nama"`
	KryTelp1 string `gorm:"-" json:"kry_telp1"`
	KryTelp2 string `gorm:"-" json:"kry_telp2"`
}

func (GlbMKorwil) TableName() string {
	return "glb_m_korwil"
}

// GlbMKorwilD memetakan tabel detail glb_m_korwil_d
type GlbMKorwilD struct {
	KdWilayahD      string `gorm:"column:kd_wilayah_d;primaryKey" json:"kd_wilayah_d"`
	KdWilayahAgenID string `gorm:"column:kd_wilayah_agen_id;primaryKey" json:"kd_wilayah_agen_id"`

	// Field tambahan penampung nama cabang/agen hasil join
	AgenNama string `gorm:"-" json:"agen_nama"`
}

func (GlbMKorwilD) TableName() string {
	return "glb_m_korwil_d"
}
