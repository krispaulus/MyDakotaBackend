package models

// OprTIsiBBM Struct memetakan tabel opr_t_isibbm
type OprTIsiBBM struct {
	ID                   int64   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	BBMID                string  `gorm:"column:bbm_id;primaryKey" json:"bbm_id"`
	BBMDate              string  `gorm:"column:bbm_date" json:"bbm_date"`
	BBMKendID            string  `gorm:"column:bbm_kendid" json:"bbm_kendid"`
	BBMKm                float64 `gorm:"column:bbm_km" json:"bbm_km"`
	BBMIsi               float64 `gorm:"column:bbm_isi" json:"bbm_isi"`
	BBMHarga             float64 `gorm:"column:bbm_harga" json:"bbm_harga"`
	BBMAktifYN           string  `gorm:"column:bbm_aktifyn;default:Y" json:"bbm_aktifyn"`
	BBMUpdateID          string  `gorm:"column:bbm_updateid" json:"bbm_updateid"`
	BBMUpdateTime        string  `gorm:"column:bbm_updatetime" json:"bbm_updatetime"`
	BBMAgenID            string  `gorm:"column:bbm_agenid" json:"bbm_agenid"`
	BBMTime              string  `gorm:"column:bbm_time" json:"bbm_time"`
	BBMJenis             string  `gorm:"column:bbm_jenis" json:"bbm_jenis"`
	BBMVoucherID         string  `gorm:"column:bbm_voucherid" json:"bbm_voucherid"`
	BBMVoucherCompleteYN string  `gorm:"column:bbm_vouchercompleteyn;default:N" json:"bbm_vouchercompleteyn"`
	BBMLokasiPengisian   string  `gorm:"column:bbm_lokasipengisian" json:"bbm_lokasipengisian"`
	BBMNipSopir1         string  `gorm:"column:bbm_nipsopir1" json:"bbm_nipsopir1"`
	BBMNipSopir2         string  `gorm:"column:bbm_nipsopir2" json:"bbm_nipsopir2"`
	BBMSJID              string  `gorm:"column:bbm_sjid" json:"bbm_sjid"`

	// Relasi Virtual / Display
	AgenNama string `gorm:"->" json:"agen_nama"`
}

func (OprTIsiBBM) TableName() string {
	return "opr_t_isibbm"
}
