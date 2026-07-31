package models

// OprTESPTerima memetakan tabel public.opr_t_esp_terima
type OprTESPTerima struct {
	ID              int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SPTeID          string `gorm:"column:spt_eid;primaryKey" json:"spt_eid"`
	SPTTanggal      string `gorm:"column:spt_tanggal" json:"spt_tanggal"`
	SPTAsalAgenID   string `gorm:"column:spt_asalagenid" json:"spt_asalagenid"`
	SPTTujuanAgenID string `gorm:"column:spt_tujuanagenid" json:"spt_tujuanagenid"`
	SPTTransitYN    string `gorm:"column:spt_transityn;default:N" json:"spt_transityn"`
	SPTNoMobil      string `gorm:"column:spt_nomobil" json:"spt_nomobil"`
	SPTNamaSopir    string `gorm:"column:spt_namasopir" json:"spt_namasopir"`
	SPTAktifYN      string `gorm:"column:spt_aktifyn;default:Y" json:"spt_aktifyn"`
	SPTUpdateID     string `gorm:"column:spt_updateid" json:"spt_updateid"`
	SPTUpdateTime   string `gorm:"column:spt_updatetime" json:"spt_updatetime"`

	// Virtual Display / Join Fields
	AsalNama   string  `gorm:"->" json:"asal"`
	TujuanNama string  `gorm:"->" json:"tujuan"`
	JmlBTT     int     `gorm:"->" json:"jmlbtt"`
	JmlCOD     float64 `gorm:"->" json:"jmlcod"`
}

func (OprTESPTerima) TableName() string {
	return "opr_t_esp_terima"
}

// OprTESPTerimaDetil memetakan tabel public.opr_t_esp_terimadetil
type OprTESPTerimaDetil struct {
	SPTDID     int64  `gorm:"column:sptd_id;primaryKey;autoIncrement" json:"sptd_id"`
	SPTDeSPTID string `gorm:"column:sptd_esptid" json:"sptd_esptid"`
	SPTDBTTID  string `gorm:"column:sptd_bttid" json:"sptd_bttid"`
}

func (OprTESPTerimaDetil) TableName() string {
	return "opr_t_esp_terimadetil"
}

// SPNaikReq DTO Request Payload dari React JSX
type SPNaikReq struct {
	SptAsalAgenNama   string   `json:"spt_asal_agen_nama"`
	SptTujuanAgenNama string   `json:"spt_tujuan_agen_nama" binding:"required"`
	SptTransitYN      string   `json:"spt_transityn"`
	SptNamaSopir      string   `json:"spt_namasopir" binding:"required"`
	SptNoMobil        string   `json:"spt_nomobil" binding:"required"`
	SptSuratTugas     string   `json:"spt_surattugas"`
	DaftarBTT         []string `json:"daftar_btt" binding:"required"`
}
