package models

import "time"

// KembaliSJHeader mewakili tabel mkt_t_kembalisj_h kita di pgAdmin bray
type KembaliSJHeader struct {
	ID              string     `gorm:"column:mkt_t_kembalisj_id;primaryKey" json:"mkt_t_kembalisj_id"`
	CustID          string     `gorm:"column:mkt_t_kembalisj_custid" json:"mkt_t_kembalisj_custid"`
	Tanggal         time.Time  `gorm:"column:mkt_t_kembalisj_tanggal" json:"mkt_t_kembalisj_tanggal"`
	Keterangan      string     `gorm:"column:mkt_t_kembalisj_keterangan" json:"mkt_t_kembalisj_keterangan"`
	Diterima        string     `gorm:"column:mkt_t_kembalisj_diterima" json:"mkt_t_kembalisj_diterima"`
	TanggalDiterima *time.Time `gorm:"column:mkt_t_kembalisj_tanggalditerima" json:"mkt_t_kembalisj_tanggalditerima"`
	UpdateID        string     `gorm:"column:mkt_t_kembalisj_updateid" json:"mkt_t_kembalisj_updateid"`
	UpdateTime      time.Time  `gorm:"column:mkt_t_kembalisj_updatetime" json:"mkt_t_kembalisj_updatetime"`
	AktifYN         string     `gorm:"column:mkt_t_kembalisj_aktifyn;default:Y" json:"mkt_t_kembalisj_aktifyn"`
}

// KembaliSJResponse mewakili hasil LEFT JOIN untuk direndering di React DataTableTemplate
type KembaliSJResponse struct {
	ID         string    `json:"mkt_t_kembalisj_id"`
	Tanggal    time.Time `json:"mkt_t_kembalisj_tanggal"`
	CustID     string    `json:"mkt_t_kembalisj_custid"`
	CustName   string    `json:"cust_name"` // 👑 Diambil via JOIN dari mkt_m_customer bray!
	Keterangan string    `json:"mkt_t_kembalisj_keterangan"`
	Diterima   string    `json:"mkt_t_kembalisj_diterima"`
	AktifYN    string    `json:"mkt_t_kembalisj_aktifyn"`
}
