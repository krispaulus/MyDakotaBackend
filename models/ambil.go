package models

// OprTEambil memetakan tabel induk transaksi pengambilan barang sendiri oleh customer
type OprTEambil struct {
	AmbilID      string `gorm:"column:ambil_id;primaryKey" json:"ambil_id"`
	AmbilAgenID  string `gorm:"column:ambil_agenid" json:"ambil_agenid"`
	AmbilTanggal string `gorm:"column:ambil_tanggal" json:"ambil_tanggal"`
	AmbilJam     string `gorm:"column:ambil_jam" json:"ambil_jam"`
	AmbilMenit   string `gorm:"column:ambil_menit" json:"ambil_menit"`
	AmbilBTTID   string `gorm:"column:ambil_bttid" json:"ambil_bttid"`

	// ✅ PERBAIKAN: gorm:"column:ambil_chekernip" (tanpa huruf 'c')
	AmbilCheckerNIP string `gorm:"column:ambil_chekernip" json:"ambil_checkernip"`

	AmbilCustNama   string `gorm:"column:ambil_custnama" json:"ambil_custnama"`
	AmbilCustAlamat string `gorm:"column:ambil_custalamat" json:"ambil_custalamat"`
	AmbilCustTelp   string `gorm:"column:ambil_custtelp" json:"ambil_custtelp"`
	AmbilCustJnsID  string `gorm:"column:ambil_custjnsid" json:"ambil_custjnsid"`
	AmbilCustID     string `gorm:"column:ambil_custid" json:"ambil_custid"`
	AmbilSKYN       string `gorm:"column:ambil_skyn;default:N" json:"ambil_skyn"`
	AmbilFolderSK   string `gorm:"column:ambil_foldersk" json:"ambil_foldersk"`
	AmbilUpdateID   string `gorm:"column:ambil_updateid" json:"ambil_updateid"`
	AmbilUpdateTime string `gorm:"column:ambil_updatetime;default:now()" json:"ambil_updatetime"`
	AmbilAktifYN    string `gorm:"column:ambil_aktifyn;default:Y" json:"ambil_aktifyn"`
	AmbilHistID     string `gorm:"column:ambil_histid" json:"ambil_histid"`

	// Field Relasi Virtual (Nampung nama checker hasil Left Join)
	KryNama string `gorm:"->" json:"kry_nama"`
}

func (OprTEambil) TableName() string {
	return "opr_t_eambil"
}
