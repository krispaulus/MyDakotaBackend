package models

type OprTKendaraanSewa struct {
	SewaID       string  `gorm:"column:sewa_id;primaryKey" json:"sewa_id"`
	CabangID     string  `gorm:"column:cabang_id" json:"cabang_id"`
	NoKendaraan  string  `gorm:"column:no_kendaraan" json:"no_kendaraan"`
	JnsKendID    *string `gorm:"column:jns_kend_id" json:"jns_kend_id"`
	VendorSewaID *string `gorm:"column:vendor_sewa_id" json:"vendor_sewa_id"`
	BiayaSewa    float64 `gorm:"column:biaya_sewa;default:0.00" json:"biaya_sewa"`
	AktifYN      string  `gorm:"column:aktif_yn;default:Y" json:"aktif_yn"`
	UpdateIDSewa *string `gorm:"column:update_id_sewa" json:"update_id_sewa"`
	KendSopir1   *string `gorm:"column:kend_sopir1" json:"kend_sopir1"`
	KendSopir2   *string `gorm:"column:kend_sopir2" json:"kend_sopir2"`

	// 🛡️ AMAN DARIPADA []uint8 WINDOWS LOCALHOST BRAY!
	TglStartSewa   *string `gorm:"column:tgl_start_sewa" json:"tgl_start_sewa"`
	TglEndSewa     *string `gorm:"column:tgl_end_sewa" json:"tgl_end_sewa"`
	UpdateTimeSewa *string `gorm:"column:update_time_sewa" json:"update_time_sewa"`

	// Field Join Tambahan untuk menampung nama jenis kendaraan dari tabel GLB_M_JnsKend
	JnsKendNama string `gorm:"-" json:"jnskend_nama"`
}

func (OprTKendaraanSewa) TableName() string {
	return "opr_t_kendaraan_sewa"
}
