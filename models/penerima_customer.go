package models

type PenerimaCustomerDTO struct {
	No               int    `json:"no"`
	CustID           string `json:"cust_id"`
	CustName         string `json:"cust_name"`
	BTTTTujuanNama   string `json:"bttt_tujuan_nama"`
	BTTTTujuanAlamat string `json:"bttt_tujuan_alamat"`
	BTTTUpdateID     string `json:"bttt_update_id"`
	BTTTTanggal      string `json:"bttt_tanggal"`
}
