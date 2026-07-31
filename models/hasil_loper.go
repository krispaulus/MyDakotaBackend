package models

// OprTEBBL Struct memetakan tabel public.opr_t_ebbl (Hasil Loper / BBL)
type OprTEBBL struct {
	ID            int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	BBLeID        string `gorm:"column:bbl_eid;primaryKey" json:"bbl_eid"`
	BBLNoLoper    string `gorm:"column:bbl_noloper" json:"bbl_noloper"`
	BBLTTD        string `gorm:"column:bbl_ttd" json:"bbl_ttd"`
	BBLLatitude   string `gorm:"column:bbl_latitude" json:"bbl_latitude"`
	BBLLongitude  string `gorm:"column:bbl_longitude" json:"bbl_longitude"`
	BBLTanggal    string `gorm:"column:bbl_tanggal" json:"bbl_tanggal"`
	BBLBTTID      string `gorm:"column:bbl_bttid" json:"bbl_bttid"`
	BBLPenerima   string `gorm:"column:bbl_penerima" json:"bbl_penerima"`
	BBLTerimaYN   string `gorm:"column:bbl_terimayn;default:Y" json:"bbl_terimayn"`
	BBLReasonID   int    `gorm:"column:bbl_reasonid" json:"bbl_reasonid"`
	BBLKeterangan string `gorm:"column:bbl_keterangan" json:"bbl_keterangan"`
	BBLAgenID     string `gorm:"column:bbl_agenid" json:"bbl_agenid"`
	BBLAktifYN    string `gorm:"column:bbl_aktifyn;default:Y" json:"bbl_aktifyn"`
	BBLUpdateID   string `gorm:"column:bbl_updateid" json:"bbl_updateid"`
	BBLUpdateTime string `gorm:"column:bbl_updatetime" json:"bbl_updatetime"`

	// Relasi Virtual / Join
	AsalKota     string  `gorm:"->" json:"bttt_asalkota"`
	AsalName     string  `gorm:"->" json:"bttt_asalname"`
	TujuanAlamat string  `gorm:"->" json:"bttt_tujuanalamat"`
	TujuanKota   string  `gorm:"->" json:"bttt_tujuankota"`
	TagihTujuan  float64 `gorm:"->" json:"bttt_tagihtujuan"`
	NoSuratJalan string  `gorm:"->" json:"bttt_nosuratjalan"`
	ReasonLokal  string  `gorm:"->" json:"reason_lokal"`
}

func (OprTEBBL) TableName() string {
	return "opr_t_ebbl"
}

// OprMReason Struct memetakan tabel public.opr_m_reason
type OprMReason struct {
	ReasonID      int    `gorm:"column:reason_id;primaryKey" json:"reason_id"`
	ReasonLokal   string `gorm:"column:reason_lokal" json:"reason_lokal"`
	ReasonInter   string `gorm:"column:reason_inter" json:"reason_inter"`
	ReasonAktifYN string `gorm:"column:reason_aktifyn;default:Y" json:"reason_aktifyn"`
}

func (OprMReason) TableName() string {
	return "opr_m_reason"
}
