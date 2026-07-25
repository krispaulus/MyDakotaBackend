package models

// OprTEambilRetur memetakan tabel induk transaksi pengambilan barang retur oleh pengirim
type OprTEambilRetur struct {
	ID                   int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AmbilReturID         string `gorm:"column:ambilretur_id" json:"ambilretur_id"`
	AmbilReturAgenID     string `gorm:"column:ambilretur_agenid" json:"ambilretur_agenid"`
	AmbilReturTanggal    string `gorm:"column:ambilretur_tanggal" json:"ambilretur_tanggal"`
	AmbilReturJam        string `gorm:"column:ambilretur_jam" json:"ambilretur_jam"`
	AmbilReturMenit      string `gorm:"column:ambilretur_menit" json:"ambilretur_menit"`
	AmbilReturBTTID      string `gorm:"column:ambilretur_bttid" json:"ambilretur_bttid"`
	AmbilReturCheckerNIP string `gorm:"column:ambilretur_checkernip" json:"ambilretur_checkernip"`
	AmbilReturCustNama   string `gorm:"column:ambilretur_custnama" json:"ambilretur_custnama"`
	AmbilReturCustAlamat string `gorm:"column:ambilretur_custalamat" json:"ambilretur_custalamat"`
	AmbilReturCustTelp   string `gorm:"column:ambilretur_custtelp" json:"ambilretur_custtelp"`
	AmbilReturCustJnsID  string `gorm:"column:ambilretur_custjnsid" json:"ambilretur_custjnsid"`
	AmbilReturCustID     string `gorm:"column:ambilretur_custid" json:"ambilretur_custid"`
	AmbilReturSKYN       string `gorm:"column:ambilretur_skyn;default:N" json:"ambilretur_skyn"`
	AmbilReturFolderSK   string `gorm:"column:ambilretur_foldersk" json:"ambilretur_foldersk"`
	AmbilReturUpdateID   string `gorm:"column:ambilretur_updateid" json:"ambilretur_updateid"`
	AmbilReturUpdateTime string `gorm:"column:ambilretur_updatetime;default:now()" json:"ambilretur_updatetime"`
	AmbilReturAktifYN    string `gorm:"column:ambilretur_aktifyn;default:Y" json:"ambilretur_aktifyn"`
	AmbilReturHistID     string `gorm:"column:ambilretur_histid" json:"ambilretur_histid"`

	// Field Relasi Virtual (Nampung nama checker hasil Left Join)
	KryNama string `gorm:"->" json:"kry_nama"`
}

func (OprTEambilRetur) TableName() string {
	return "opr_t_eambilretur"
}
