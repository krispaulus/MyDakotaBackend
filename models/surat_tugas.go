package models

// OprTESuratJalan Struct memetakan tabel header public.opr_t_esuratjalan
type OprTESuratJalan struct {
	ID                int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SJHID             string `gorm:"column:sjh_id;primaryKey" json:"sjh_id"`
	SJHTanggal        string `gorm:"column:sjh_tanggal" json:"sjh_tanggal"`
	SJHSopir1Nip      string `gorm:"column:sjh_sopir1_nip" json:"sjh_sopir1_nip"`
	SJHSopir2Nip      string `gorm:"column:sjh_sopir2_nip" json:"sjh_sopir2_nip"`
	SJHKendID         string `gorm:"column:sjh_kendid" json:"sjh_kendid"`
	SJHAssID          string `gorm:"column:sjh_assid" json:"sjh_assid"`
	SJHTrayekID       string `gorm:"column:sjh_trayekid" json:"sjh_trayekid"`
	SJHStartAgenID    string `gorm:"column:sjh_startagenid" json:"sjh_startagenid"`
	SJHEndAgenID      string `gorm:"column:sjh_endagenid" json:"sjh_endagenid"`
	SJHKeterangan     string `gorm:"column:sjh_keterangan" json:"sjh_keterangan"`
	SJHAktifYN        string `gorm:"column:sjh_aktifyn;default:Y" json:"sjh_aktifyn"`
	SJHUpdateID       string `gorm:"column:sjh_updateid" json:"sjh_updateid"`
	SJHUpdateTime     string `gorm:"column:sjh_updatetime" json:"sjh_updatetime"`
	SJHCompleteYN     string `gorm:"column:sjh_completeyn;default:N" json:"sjh_completeyn"`
	SJHForceEndKet    string `gorm:"column:sjh_forceendket" json:"sjh_forceendket"`
	SJHForceEndID     string `gorm:"column:sjh_forceendid" json:"sjh_forceendid"`
	SJHForceEndTgl    string `gorm:"column:sjh_forceendtgl" json:"sjh_forceendtgl"`
	SJHTanggalKembali string `gorm:"column:sjh_tanggalkembali" json:"sjh_tanggalkembali"`
	SJHApprove        string `gorm:"column:sjh_approve" json:"sjh_approve"`
	SJHReason         string `gorm:"column:sjh_reason" json:"sjh_reason"`
	SJHPosting        string `gorm:"column:sjh_posting;default:N" json:"sjh_posting"`

	// Field Virtual / Relasi Join
	NmSopir1   string  `gorm:"->" json:"nmsopir1"`
	NmSopir2   string  `gorm:"->" json:"nmsopir2"`
	CbgMulai   string  `gorm:"->" json:"cbgmulai"`
	CbgSelesai string  `gorm:"->" json:"cbgselesai"`
	AssNama    string  `gorm:"->" json:"ass_nama"`
	NominalUM  float64 `gorm:"->" json:"nominal_um"`
}

func (OprTESuratJalan) TableName() string {
	return "opr_t_esuratjalan"
}

// OprTESuratJalanRealisasi Struct memetakan tabel public.opr_t_esuratjalanrealisasi
type OprTESuratJalanRealisasi struct {
	SJRID        int64   `gorm:"column:sjr_id;primaryKey;autoIncrement" json:"sjr_id"`
	SJRSJHID     string  `gorm:"column:sjr_sjhid" json:"sjr_sjhid"`
	SJRBPLKID    string  `gorm:"column:sjr_bplkid" json:"sjr_bplkid"`
	SJRKet       string  `gorm:"column:sjr_ket" json:"sjr_ket"`
	SJRNominal   float64 `gorm:"column:sjr_nominal" json:"sjr_nominal"`
	SJRBPLKIDUM  string  `gorm:"column:sjr_bplkidum" json:"sjr_bplkidum"`
	SJRNominalUM float64 `gorm:"column:sjr_nominalum" json:"sjr_nominalum"`
}

func (OprTESuratJalanRealisasi) TableName() string {
	return "opr_t_esuratjalanrealisasi"
}

// Request Payload untuk Tambah / Update Surat Tugas
type SuratTugasReq struct {
	SJHID             string  `json:"sjh_id" binding:"required"`
	SJHTanggal        string  `json:"sjh_tanggal"`
	SJHTanggalKembali string  `json:"sjh_tanggalkembali"`
	SJHKendID         string  `json:"sjh_kendid" binding:"required"`
	SJHSopir1Nip      string  `json:"sjh_sopir1_nip"`
	SJHSopir2Nip      string  `json:"sjh_sopir2_nip"`
	SJHAssID          string  `json:"sjh_assid" binding:"required"`
	SJHTrayekID       string  `json:"sjh_trayekid"`
	SJHStartAgenID    string  `json:"sjh_startagenid" binding:"required"`
	SJHEndAgenID      string  `json:"sjh_endagenid" binding:"required"`
	SJHKeterangan     string  `json:"sjh_keterangan"`
	NominalUM         float64 `json:"nominal_um"`
}
