package models

// OprTSuratMuatanUdara Struct memetakan tabel public.opr_t_suratmuatanudara
type OprTSuratMuatanUdara struct {
	ID                  int64   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SMUNo               string  `gorm:"column:smu_no;primaryKey" json:"smu_no"`
	SMUDari             string  `gorm:"column:smu_dari" json:"smu_dari"`
	SMUTujuan           string  `gorm:"column:smu_tujuan" json:"smu_tujuan"`
	SMUKepada           string  `gorm:"column:smu_kepada" json:"smu_kepada"`
	SMUMaskapai         string  `gorm:"column:smu_maskapai" json:"smu_maskapai"`
	SMUNomorPenerbangan string  `gorm:"column:smu_nomorpenerbangan" json:"smu_nomorpenerbangan"`
	SMUColly            int     `gorm:"column:smu_colly" json:"smu_colly"`
	SMUBerat            float64 `gorm:"column:smu_berat" json:"smu_berat"`
	SMUHarga            float64 `gorm:"column:smu_harga" json:"smu_harga"`
	SMUDokumen          string  `gorm:"column:smu_dokumen" json:"smu_dokumen"`
	SMUPPN              float64 `gorm:"column:smu_ppn" json:"smu_ppn"`
	SMUTanggal          string  `gorm:"column:smu_tanggal" json:"smu_tanggal"`
	SMUUpdateID         string  `gorm:"column:smu_updateid" json:"smu_updateid"`
	SMUServerID         int     `gorm:"column:smu_serverid;default:1" json:"smu_serverid"`
	SMUAktifYN          string  `gorm:"column:smu_aktifyn;default:Y" json:"smu_aktifyn"`
	SMUTerimaYN         string  `gorm:"column:smu_terimayn;default:N" json:"smu_terimayn"`
	SMUServerIDTerima   int     `gorm:"column:smu_serveridterima" json:"smu_serveridterima"`
	SMUHistID           string  `gorm:"column:smu_histid" json:"smu_histid"`

	// Relasi Virtual / Join
	BandaraDari   string `gorm:"->" json:"bandara_dari"`
	BandaraTujuan string `gorm:"->" json:"bandara_tujuan"`
}

func (OprTSuratMuatanUdara) TableName() string {
	return "opr_t_suratmuatanudara"
}
