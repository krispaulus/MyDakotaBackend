package models

import "time"

// SetoranTunaiResponse mewakili data list setoran tunai hasil query JOIN
type SetoranTunaiResponse struct {
	STID         string    `gorm:"column:st_id" json:"st_id"`
	STTanggal    time.Time `gorm:"column:st_tanggal" json:"st_tanggal"`
	STAgenID     string    `gorm:"column:st_agenid" json:"st_agenid"`
	AgenNama     string    `gorm:"column:agen_nama" json:"agen_nama"`
	STBTTHID     string    `gorm:"column:st_btthid" json:"st_btthid"`
	STJumlah     float64   `gorm:"column:st_jumlah" json:"st_jumlah"`
	STApproveYN  string    `gorm:"column:st_approveyn" json:"st_approveyn"`
	STPostingYN  string    `gorm:"column:st_postingyn" json:"st_postingyn"`
	STTjurHNo    string    `gorm:"column:st_tjurhno" json:"st_tjurhno"`
	STUpdateID   string    `gorm:"column:st_updateid" json:"st_updateid"`
	STUpdateTime time.Time `gorm:"column:st_updatetime" json:"st_updatetime"`
	STAktifYN    string    `gorm:"column:st_aktifyn" json:"st_aktifyn"`
}

// Request payload pembuatan setoran tunai baru
type CreateSetoranTunaiReq struct {
	Tanggal    string  `json:"tanggal" binding:"required"`
	AgenID     string  `json:"agen_id" binding:"required"`
	BTTHID     string  `json:"btth_id" binding:"required"` // Kode LPH
	Jumlah     float64 `json:"jumlah" binding:"required"`
	Keterangan string  `json:"keterangan"`
}
