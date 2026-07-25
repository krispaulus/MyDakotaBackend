package models

import "time"

// Model untuk BTT Pengiriman (MKT_T_eConote)
type Econote struct {
	BtttID           string    `gorm:"column:bttt_id;primaryKey" json:"bttt_id"`
	BtttTanggal      time.Time `gorm:"column:bttt_tanggal" json:"bttt_tanggal"`
	BtttServID       int       `gorm:"column:bttt_servid" json:"bttt_servid"`
	BtttAsalCustID   string    `gorm:"column:bttt_asalcustid" json:"bttt_asalcustid"`
	BtttAsalName     string    `gorm:"column:bttt_asalname" json:"bttt_asalname"`
	BtttTujuanNama   string    `gorm:"column:bttt_tujuannama" json:"bttt_tujuannama"`
	BtttPembayaran   int       `gorm:"column:bttt_pembayaran" json:"bttt_pembayaran"`
	BtttNamaBarang   string    `gorm:"column:bttt_namabarang" json:"bttt_namabarang"`
	BtttNoSuratJalan string    `gorm:"column:bttt_nosuratjalan" json:"bttt_nosuratjalan"`
	BtttJmlUnit      float64   `gorm:"column:bttt_jmlunit" json:"bttt_jmlunit"`
	BtttBerat        float64   `gorm:"column:bttt_berat" json:"bttt_berat"`
	BtttUkuran       float64   `gorm:"column:bttt_ukuran" json:"bttt_ukuran"`
	BtttTagihTujuan  float64   `gorm:"column:bttt_tagihtujuan" json:"bttt_tagihtujuan"`
	BtttAktifYN      string    `gorm:"column:bttt_aktifyn" json:"bttt_aktifyn"`

	// Field tambahan dari JOIN MKT_M_Customer & Formatting Alias
	CustName string `gorm:"column:cust_name" json:"cust_name"`
	JnBayar  string `gorm:"-" json:"jnbayar"`
	AktifJD  string `gorm:"-" json:"aktifjd"`
	ServisJD string `gorm:"-" json:"servisjd"`
}

func (Econote) TableName() string {
	return "mkt_t_econote"
}

// Request Filter DTO
type EconoteFilterRequest struct {
	Page         int    `form:"page"`
	Limit        int    `form:"limit"`
	TanggalStart string `form:"tanggalStart"`
	TanggalEnd   string `form:"tanggalEnd"`
	Service      string `form:"service"`
	Customer     string `form:"customer"`
	Tujuan       string `form:"tujuan"`
	TujPulau     string `form:"tujPulau"`
	Posting      string `form:"posting"` // 'Ya' / 'Tidak'
	Kirim        string `form:"kirim"`   // 'Ya' / 'Tidak'
	NoBTT        string `form:"nobtt"`
	NoSMU        string `form:"nosmu"`
	Agen         string `form:"agen"` // 'Ya' / 'Tidak'
	Bayar        string `form:"bayar"`
	Pembayaran   string `form:"pembayaran"` // '1'=Tunai, '2'=Kredit, '3'=Tagih
	NoSJ         string `form:"nosj"`
}

// Struct Hak Akses WebRights User
type UserMenuRights struct {
	CanAdd    bool `json:"can_add"`    // E2a
	CanEdit   bool `json:"can_edit"`   // E2b
	CanDelete bool `json:"can_delete"` // E2c
}
