package models

import "time"

// TransportLSPBWithCarInOut Struct untuk data logistik mobil
type TransportLSPBWithCarInOut struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	NoSKB          string    `gorm:"column:noskb" json:"noskb"`
	NoDO           string    `gorm:"column:nodo;not null" json:"nodo"`
	NoMobil        string    `gorm:"column:nomobil" json:"nomobil"`
	JamMobilMasuk  string    `gorm:"column:jammobilmasuk" json:"jammobilmasuk"`
	JamMobilKeluar string    `gorm:"column:jammobilkeluar" json:"jammobilkeluar"`
	JamMobilTiba   string    `gorm:"column:jammobiltiba" json:"jammobiltiba"`
	Kategori       string    `gorm:"column:kategori" json:"kategori"`
	CreatedAt      time.Time `gorm:"column:created_at;default:now()" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;default:now()" json:"updated_at"`
}

func (TransportLSPBWithCarInOut) TableName() string {
	return "public.opr_t_transportlspbwithcarinoutanddatetime"
}

// LSPBReportDTO Struct hasil gabungan query Laporan LSBP
type LSPBReportDTO struct {
	DNODSJ            string  `json:"dn_od_sj" gorm:"column:DN_OD_SJ"`
	CustName          string  `json:"cust_name" gorm:"column:Cust_Name"`
	TanggalHandOver   string  `json:"tanggal_handover" gorm:"column:Tanggal_HandOver"`
	NomorBTT          string  `json:"nomorbtt" gorm:"column:nomorbtt"`
	PickupFrom        string  `json:"pickup_from" gorm:"column:PickupFrom"`
	BTTTTujuanNama    string  `json:"bttt_tujuan_nama" gorm:"column:BTTT_TujuanNama"`
	BTTTTujuanKota    string  `json:"bttt_tujuan_kota" gorm:"column:BTTT_TujuanKota"`
	ServName          string  `json:"serv_name" gorm:"column:Serv_Name"`
	TanggalBerangkat  string  `json:"tanggal_berangkat" gorm:"column:Tanggal_berangkat"`
	Darat             int     `json:"darat" gorm:"column:Darat"`
	Laut              int     `json:"laut" gorm:"column:Laut"`
	Udara             int     `json:"udara" gorm:"column:Udara"`
	TanggalDiterima   string  `json:"tanggal_diterima" gorm:"column:Tanggal_Diterima"`
	NamaPenerima      string  `json:"nama_penerima" gorm:"column:Nama_Penerima"`
	Keterangan        string  `json:"keterangan" gorm:"column:Keterangan"`
	SPBID             string  `json:"spb_id" gorm:"column:spb_id"`
	ArtihNoKw         string  `json:"artih_nokw" gorm:"column:artih_nokw"`
	NoMobil           string  `json:"no_mobil" gorm:"column:no_mobil"`
	JamMobilMasuk     string  `json:"jam_mobil_masuk" gorm:"column:jam_mobil_masuk"`
	JamMobilKeluar    string  `json:"jam_mobil_keluar" gorm:"column:jam_mobil_keluar"`
	TanggalKendTiba   string  `json:"tanggal_kend_tiba" gorm:"column:Tanggal_kendaraan_tiba"`
	ArtihTglAcc       string  `json:"artih_tgl_acc" gorm:"column:artih_tglacc"`
	INV               string  `json:"inv" gorm:"column:INV"`
	GRN               string  `json:"grn" gorm:"column:GRN"`
	FP                string  `json:"fp" gorm:"column:FP"`
	Dll               string  `json:"dll" gorm:"column:Dll"`
	BTTTJmlUnit       float64 `json:"bttt_jml_unit" gorm:"column:BTTT_JmlUnit"`
	BTTTBeratVol      float64 `json:"bttt_berat_vol" gorm:"column:BTTT_BeratVol"`
	BTTTUkuran        string  `json:"bttt_ukuran" gorm:"column:BTTT_Ukuran"`
	BTTTBerat         float64 `json:"bttt_berat" gorm:"column:BTTT_Berat"`
	TglDokumenKembali string  `json:"tgl_dokumen_kembali" gorm:"column:TB_TglKembali"`
	OnTime            int     `json:"ontime"`
	MissLeadTime      int     `json:"miss_lead_time"`
	DalamPengiriman   int     `json:"dalam_pengiriman"`
}
