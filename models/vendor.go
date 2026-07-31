package models

import "time"

// MktMVendor memetakan tabel master vendor Dakota Logistics / Cargo
type MktMVendor struct {
	VendID            string    `gorm:"column:vend_id;primaryKey" json:"vend_id"`
	VendAgenID        string    `gorm:"column:vend_agenid" json:"vend_agenid"`
	VendName          string    `gorm:"column:vend_name;not null" json:"vend_name"`
	VendAlamat1       string    `gorm:"column:vend_alamat1" json:"vend_alamat1"`
	VendAlamat2       string    `gorm:"column:vend_alamat2" json:"vend_alamat2"`
	VendKotaID        string    `gorm:"column:vend_kotaid" json:"vend_kotaid"`
	VendTelp1         string    `gorm:"column:vend_telp1" json:"vend_telp1"`
	VendTelp2         string    `gorm:"column:vend_telp2" json:"vend_telp2"`
	VendFax1          string    `gorm:"column:vend_fax1" json:"vend_fax1"`
	VendFax2          string    `gorm:"column:vend_fax2" json:"vend_fax2"`
	VendContactPerson string    `gorm:"column:vend_contactperson" json:"vend_contactperson"`
	VendMemo          string    `gorm:"column:vend_memo" json:"vend_memo"`
	VendApproveYN     string    `gorm:"column:vend_approveyn;default:'N'" json:"vend_approveyn"`
	VendApproveID     string    `gorm:"column:vend_approveid" json:"vend_approveid"`
	VendPostingYN     string    `gorm:"column:vend_postingyn;default:'N'" json:"vend_postingyn"`
	VendAktifYN       string    `gorm:"column:vend_aktifyn;default:'Y'" json:"vend_aktifyn"`
	VendUpdateID      string    `gorm:"column:vend_updateid" json:"vend_updateid"`
	VendUpdateTime    time.Time `gorm:"column:vend_updatetime;default:now()" json:"vend_updatetime"`
	VendKgMin         float64   `gorm:"column:vend_kgmin;default:0" json:"vend_kgmin"`
	VendVirtualAcc    string    `gorm:"column:vend_virtualacc" json:"vend_virtualacc"`
	VendKreditYN      string    `gorm:"column:vend_kredityn;default:'N'" json:"vend_kredityn"`
	VendKreditLimit   float64   `gorm:"column:vend_kreditlimit;default:0" json:"vend_kreditlimit"`
	VendKreditAktif   float64   `gorm:"column:vend_kreditaktif;default:0" json:"vend_kreditaktif"`
	VendEmail         string    `gorm:"column:vend_email" json:"vend_email"`
	VendPriceListID   string    `gorm:"column:vend_pricelistid" json:"vend_pricelistid"`
	VendNPWP          string    `gorm:"column:vend_npwp" json:"vend_npwp"`
	VendNamaNPWP      string    `gorm:"column:vend_namanpwp" json:"vend_namanpwp"`
	VendPIC           string    `gorm:"column:vend_pic" json:"vend_pic"`
	VendAPIKey        string    `gorm:"column:vend_api_key" json:"vend_api_key"`
	VendTOP           int       `gorm:"column:vend_top;default:0" json:"vend_top"`
}

func (MktMVendor) TableName() string {
	return "public.mkt_m_vendor"
}
