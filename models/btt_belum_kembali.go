package models

// BTTBelumKembaliDetailDTO untuk tampilan Rincian/Detail
type BTTBelumKembaliDetailDTO struct {
	BTTTTanggal      string  `json:"bttt_tanggal"`
	BTTTID           string  `json:"bttt_id"`
	BTTDBS           string  `json:"btt_dbs"`
	BTTTNoSuratJalan string  `json:"bttt_nosuratjalan"`
	CustName         string  `json:"cust_name"`
	JHarga           float64 `json:"jharga"`
	BTTTJmlUnit      float64 `json:"bttt_jml_unit"`
	JBerat           float64 `json:"jberat"`
	BTTTTujuanNama   string  `json:"bttt_tujuan_nama"`
	BTTTTujuanKota   string  `json:"bttt_tujuan_kota"`
	AgenNama         string  `json:"agen_nama"`
	CustPIC          string  `json:"cust_pic"`
	ARTIDARTIHID     string  `json:"invoice_no"`
	Umur             int     `json:"umur"`
}

// BTTBelumKembaliRekapDTO untuk tampilan Rekap By Cabang / PIC
type BTTBelumKembaliRekapDTO struct {
	GroupKey string  `json:"group_key"` // Nama Agen / Cust PIC
	JmlBTT   int64   `json:"jml_btt"`
	JHarga   float64 `json:"jharga"`
}
