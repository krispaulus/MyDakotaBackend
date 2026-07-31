package models

type LoadingBarangDTO struct {
	No             int    `json:"no"`
	LoadHID        string `json:"loadh_id"`
	LoadHTanggal   string `json:"loadh_tanggal"`
	LoadHKendID    string `json:"loadh_kend_id"`
	LoadHUpdateID  string `json:"loadh_update_id"`
	LoadHApproveYN string `json:"loadh_approve_yn"`
	JmlBarang      int    `json:"jml_barang"`
}
