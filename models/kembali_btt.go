package models

// =========================================================================
// 1. PENGEMBALIAN BTT (OPR_T_eKembaliBTT)
// =========================================================================

// OprTEkembaliBTT Struct memetakan tabel header pengembalian BTT
type OprTEkembaliBTT struct {
	KBEid          string `gorm:"column:kb_eid;primaryKey" json:"kb_eid"`
	KBTanggal      string `gorm:"column:kb_tanggal" json:"kb_tanggal"`
	KBAgenID       string `gorm:"column:kb_agenid" json:"kb_agen_id"`
	KBTujuanAgenID string `gorm:"column:kb_tujuanagenid" json:"kb_tujuan_agen_id"`
	KBBdbID        string `gorm:"column:kb_bdbid" json:"kb_bdb_id"`
	KBUpdateID     string `gorm:"column:kb_updateid" json:"kb_update_id"`
	KBAktifYN      string `gorm:"column:kb_aktifyn;default:Y" json:"kb_aktif_yn"`

	// Field Virtual
	AgenNama string `gorm:"->" json:"agen_nama"`
}

func (OprTEkembaliBTT) TableName() string {
	return "opr_t_ekembalibtt"
}

// OprTEkembaliBTTDetil Struct memetakan detail BTT
type OprTEkembaliBTTDetil struct {
	KBDID       int64  `gorm:"column:kbd_id;primaryKey;autoIncrement" json:"kbd_id"`
	KBDKbeID    string `gorm:"column:kbd_kbeid" json:"kbd_kbeid"`
	KBDBttID    string `gorm:"column:kbd_bttid" json:"kbd_bttid"`
	KBDUpdateID string `gorm:"column:kbd_updateid" json:"kbd_update_id"`
}

func (OprTEkembaliBTTDetil) TableName() string {
	return "opr_t_ekembalibttdetil"
}

// Request Payload untuk Tambah / Update Pengembalian BTT
type KembaliBTTReq struct {
	KBEid          string   `json:"kb_eid" binding:"required"`
	KBTujuanAgenID string   `json:"kb_tujuan_agen_id" binding:"required"`
	KBBdbID        string   `json:"kb_bdb_id"`
	ListBttID      []string `json:"list_btt_id" binding:"required"`
}

// =========================================================================
// 2. RETUR BTT / BARANG BERMASALAH (OPR_T_eReturBTT)
// =========================================================================

// OprTEreturBTT Struct memetakan tabel header retur BTT / Barang Bermasalah
type OprTEreturBTT struct {
	RBEid          string `gorm:"column:rb_eid;primaryKey" json:"rb_eid"`
	RBAgenID       string `gorm:"column:rb_agenid" json:"rb_agen_id"`
	RBTanggal      string `gorm:"column:rb_tanggal" json:"rb_tanggal"`
	RBTujuanAgenID string `gorm:"column:rb_tujuanagenid" json:"rb_tujuan_agen_id"`
	RBAktifYN      string `gorm:"column:rb_aktifyn;default:Y" json:"rb_aktif_yn"`
	RBUpdateID     string `gorm:"column:rb_updateid" json:"rb_update_id"`
	RBUpdateTime   string `gorm:"column:rb_updatetime" json:"rb_update_time"`

	// Field Virtual
	AgenNama string `gorm:"->" json:"agen_nama"`
}

func (OprTEreturBTT) TableName() string {
	return "opr_t_ereturbtt"
}

// OprTEreturBTTDetil Struct memetakan detail resi retur
type OprTEreturBTTDetil struct {
	RBDID         int64  `gorm:"column:rbd_id;primaryKey;autoIncrement" json:"rbd_id"`
	RBDRbeID      string `gorm:"column:rbd_rbeid" json:"rbd_rbeid"`
	RBDBttID      string `gorm:"column:rbd_bttid" json:"rbd_bttid"`
	RBDKeterangan string `gorm:"column:rbd_keterangan" json:"rbd_keterangan"`
	RBDReturYN    string `gorm:"column:rbd_returyn;default:Y" json:"rbd_retur_yn"`
	RBDHistID     string `gorm:"column:rbd_histid" json:"rbd_histid"`
	RBDUpdateID   string `gorm:"column:rbd_updateid" json:"rbd_update_id"`
}

func (OprTEreturBTTDetil) TableName() string {
	return "opr_t_ereturbttdetil"
}

// Request Payload untuk Tambah / Update Retur BTT
type ReturBTTReq struct {
	RBEid          string   `json:"rb_eid" binding:"required"`
	RBTujuanAgenID string   `json:"rb_tujuan_agen_id" binding:"required"`
	ListBttID      []string `json:"list_btt_id" binding:"required"`
	Keterangan     string   `json:"keterangan"`
}
