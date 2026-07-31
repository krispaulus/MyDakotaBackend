package models

import "time"

// Model untuk tabel mkt_m_customer
type MktMCustomer struct {
	CustID            string    `gorm:"primaryKey;column:cust_id" json:"cust_id"`
	CustAgenID        *string   `gorm:"column:cust_agenid" json:"cust_agenid"`
	CustName          string    `gorm:"column:cust_name" json:"cust_name"`
	CustAlamat1       *string   `gorm:"column:cust_alamat1" json:"cust_alamat1"`
	CustAlamat2       *string   `gorm:"column:cust_alamat2" json:"cust_alamat2"`
	CustKotaID        *string   `gorm:"column:cust_kotaid" json:"cust_kotaid"`
	CustTelp1         *string   `gorm:"column:cust_telp1" json:"cust_telp1"`
	CustTelp2         *string   `gorm:"column:cust_telp2" json:"cust_telp2"`
	CustFax1          *string   `gorm:"column:cust_fax1" json:"cust_fax1"`
	CustFax2          *string   `gorm:"column:cust_fax2" json:"cust_fax2"`
	CustContactPerson *string   `gorm:"column:cust_contactperson" json:"cust_contactperson"`
	CustMemo          *string   `gorm:"column:cust_memo" json:"cust_memo"`
	CustApproveYN     string    `gorm:"column:cust_approveyn;default:'N'" json:"cust_approveyn"`
	CustApproveID     *string   `gorm:"column:cust_approveid" json:"cust_approveid"`
	CustPostingYN     string    `gorm:"column:cust_postingyn;default:'N'" json:"cust_postingyn"`
	CustAktifYN       string    `gorm:"column:cust_aktifyn;default:'Y'" json:"cust_aktifyn"`
	CustUpdateID      *string   `gorm:"column:cust_updateid" json:"cust_updateid"`
	CustUpdateTime    time.Time `gorm:"column:cust_updatetime;autoUpdateTime" json:"cust_updatetime"`
	CustKgMin         float64   `gorm:"column:cust_kgmin;default:0" json:"cust_kgmin"`
	CustVirtualAcc    *string   `gorm:"column:cust_virtualacc" json:"cust_virtualacc"`
	CustKreditYN      string    `gorm:"column:cust_kredityn;default:'N'" json:"cust_kredityn"`
	CustKreditLimit   float64   `gorm:"column:cust_kreditlimit;default:0" json:"cust_kreditlimit"`
	CustKreditAktif   float64   `gorm:"column:cust_kreditaktif;default:0" json:"cust_kreditaktif"`
	CustEmail         *string   `gorm:"column:cust_email" json:"cust_email"`
	CustPriceListID   *string   `gorm:"column:cust_pricelistid" json:"cust_pricelistid"`
	CustNPWP          *string   `gorm:"column:cust_npwp" json:"cust_npwp"`
	CustNamaNPWP      *string   `gorm:"column:cust_namanpwp" json:"cust_namanpwp"`
	CustPIC           *string   `gorm:"column:cust_pic" json:"cust_pic"`
	CustAPIKey        *string   `gorm:"column:cust_api_key" json:"cust_api_key"`
	CustTOP           int       `gorm:"column:cust_top;default:0" json:"cust_top"`

	// Relasi ke Hari Kerja
	WorkDays *MktMCustomerWorkDays `gorm:"foreignKey:CustID;references:CustID" json:"work_days"`
}

func (MktMCustomer) TableName() string {
	return "mkt_m_customer"
}

// Model untuk tabel mkt_m_customer_workdays
type MktMCustomerWorkDays struct {
	CustID   string `gorm:"primaryKey;column:cust_id" json:"cust_id"`
	SabtuYN  string `gorm:"column:sabtuyn;default:'N'" json:"sabtuyn"`
	MingguYN string `gorm:"column:mingguyn;default:'N'" json:"mingguyn"`
	LiburYN  string `gorm:"column:liburyn;default:'N'" json:"liburyn"`
}

func (MktMCustomerWorkDays) TableName() string {
	return "mkt_m_customer_workdays"
}
