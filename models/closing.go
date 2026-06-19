package models

import "time"

// PenjualanBttH merepresentasikan Header Closing Harian Agen (public.art_t_penjualanbtth)
type PenjualanBttH struct {
	BtthID         string    `gorm:"column:btth_id;primaryKey;type:varchar(50)" json:"btth_id"`
	BtthTanggal    time.Time `gorm:"column:btth_tanggal;type:timestamp;not null" json:"btth_tanggal"`
	BtthAgenID     string    `gorm:"column:btth_agenid;type:varchar(6);not null" json:"btth_agenid"`
	BtthPembayaran int       `gorm:"column:btth_pembayaran;type:int;default:1" json:"btth_pembayaran"`
	BtthActiveYN   string    `gorm:"column:btth_activeyn;type:char(1);default:'Y'" json:"btth_activeyn"`
	BtthCbid       string    `gorm:"column:btth_cbid;type:varchar(50);default:'-'" json:"btth_cbid"`
	BtthPostingYN  string    `gorm:"column:btth_postingyn;type:char(1);default:'N'" json:"btth_postingyn"`
	BtthTjurhNo    string    `gorm:"column:btth_tjurhno;type:varchar(50);default:'-'" json:"btth_tjurhno"`
	BtthNoKW       string    `gorm:"column:btth_nokw;type:varchar(50);default:'-'" json:"btth_nokw"`
	BtthUpdateID   string    `gorm:"column:btth_updateid;type:varchar(100);default:'SYSTEM'" json:"btth_updateid"`
	BtthUpdateTime time.Time `gorm:"column:btth_updatetime;type:timestamp;default:CURRENT_TIMESTAMP" json:"btth_updatetime"`
}

func (PenjualanBttH) TableName() string {
	return "public.art_t_penjualanbtth"
}

// PenjualanBttD merepresentasikan Detail Manifest Resi Per-Closing (public.art_t_penjualanbttd)
type PenjualanBttD struct {
	BttdBtthID    string `gorm:"column:bttd_btthid;primaryKey;type:varchar(50)" json:"bttd_btthid"`
	BttdBttID     string `gorm:"column:bttd_bttid;primaryKey;type:varchar(50)" json:"bttd_bttid"`
	BttdBttSeries string `gorm:"column:bttd_bttseries;type:varchar(50);default:''" json:"bttd_bttseries"`
}

func (PenjualanBttD) TableName() string {
	return "public.art_t_penjualanbttd"
}
