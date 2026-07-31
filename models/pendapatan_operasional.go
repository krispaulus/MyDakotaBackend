package models

// Item BTT / Biaya untuk Loper / SP
type DetailBTTItem struct {
	NoBTT      string  `json:"no_btt"`
	Tujuan     string  `json:"tujuan"`
	BiayaKirim float64 `json:"biaya_kirim"`
}

// Item Jurnal / Debet untuk Loper / SP / Jemput
type DetailJurnalItem struct {
	NoJurnal  string  `json:"no_jurnal"`
	KodeBiaya string  `json:"kode_biaya"`
	Debet     float64 `json:"debet"`
}

// Data Pendapatan Loper
type PendapatanLoperDTO struct {
	NoMobil     string             `json:"no_mobil"`
	Tanggal     string             `json:"tanggal"`
	NamaSopir   string             `json:"nama_sopir"`
	NoLoper     string             `json:"no_loper"`
	NoJurnal    string             `json:"no_jurnal"`
	ListBTT     []DetailBTTItem    `json:"list_btt"`
	ListJurnal  []DetailJurnalItem `json:"list_jurnal"`
	TotalBTT    float64            `json:"total_btt"`
	TotalJurnal float64            `json:"total_jurnal"`
}

// Data Pendapatan Surat Pengantar (SP)
type PendapatanSPDTO struct {
	NoMobil     string             `json:"no_mobil"`
	Tanggal     string             `json:"tanggal"`
	NoSP        string             `json:"no_sp"`
	Tujuan      string             `json:"tujuan"`
	ListBTT     []DetailBTTItem    `json:"list_btt"`
	ListJurnal  []DetailJurnalItem `json:"list_jurnal"`
	TotalBTT    float64            `json:"total_btt"`
	TotalJurnal float64            `json:"total_jurnal"`
}

// Data Pendapatan Order Jemput
type PendapatanJemputDTO struct {
	NoMobil     string             `json:"no_mobil"`
	Tanggal     string             `json:"tanggal"`
	Customer    string             `json:"customer"`
	NoJemput    string             `json:"no_jemput"`
	JmlKoli     float64            `json:"jml_koli"`
	Berat       float64            `json:"berat"`
	Volume      float64            `json:"volume"`
	NoJurnal    string             `json:"no_jurnal"`
	ListJurnal  []DetailJurnalItem `json:"list_jurnal"`
	TotalJurnal float64            `json:"total_jurnal"`
}

// Response Utama Laporan Pendapatan Operasional
type LaporanPendapatanOperasionalResponse struct {
	ListLoper  []PendapatanLoperDTO  `json:"list_loper"`
	ListSP     []PendapatanSPDTO     `json:"list_sp"`
	ListJemput []PendapatanJemputDTO `json:"list_jemput"`
}
