package models

// BarangTurunDetailDTO Struct untuk Laporan Barang Turun Detail (Tipe 1)
type BarangTurunDetailDTO struct {
	SPeID          string  `json:"sp_eid" gorm:"column:SP_eID"`
	SPTTanggal     string  `json:"spt_tanggal" gorm:"column:SPT_Tanggal"`
	AgenNama       string  `json:"agen_nama" gorm:"column:Agen_Nama"`
	TujuanPropinsi string  `json:"tujuan_propinsi" gorm:"column:TujuanPropinsi"`
	BTTTTujuanKota string  `json:"bttt_tujuan_kota" gorm:"column:BTTT_TujuanKota"`
	ViaText        string  `json:"via_text"`
	SPBTTID        string  `json:"sp_bttid" gorm:"column:SP_BTTID"`
	BTTTTanggal    string  `json:"bttt_tanggal" gorm:"column:BTTT_Tanggal"`
	CustName       string  `json:"cust_name" gorm:"column:Cust_Name"`
	BTTTTujuanNama string  `json:"bttt_tujuan_nama" gorm:"column:BTTT_TujuanNama"`
	BTTTJmlUnit    float64 `json:"bttt_jml_unit" gorm:"column:BTTT_JmlUnit"`
	BTTTBerat      float64 `json:"bttt_berat" gorm:"column:BTTT_Berat"`
	BTTTBeratVol   float64 `json:"bttt_berat_vol" gorm:"column:BTTT_Beratvol"`
	KreditTunai    float64 `json:"kredit_tunai"`
	Tagih          float64 `json:"tagih"`
	BiayaPenerus   float64 `json:"biaya_penerus" gorm:"column:BTTT_BiayaPenerus"`
	TransitYN      string  `json:"transit_yn"`
	BTTTService    string  `json:"bttt_service" gorm:"column:BTTT_Service"`
	TarifDesc      string  `json:"tarif_desc"`
	JasaHandling   float64 `json:"jasa_handling"`
}

// BarangTurunRekapDTO Struct untuk Laporan Barang Turun Rekap Per Agen/Cabang (Tipe 2)
type BarangTurunRekapDTO struct {
	AgenNama     string  `json:"agen_nama"`
	JmlBTT       int64   `json:"jml_btt"`
	Colly        float64 `json:"colly"`
	Berat        float64 `json:"berat"`
	Volume       float64 `json:"volume"`
	KreditTunai  float64 `json:"kredit_tunai"`
	Tagih        float64 `json:"tagih"`
	BiayaPenerus float64 `json:"biaya_penerus"`
	JasaHandling float64 `json:"jasa_handling"`
}
