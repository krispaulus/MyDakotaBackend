package models

// UnloadBarangDTO Struct Response List Pembongkaran Barang
type UnloadBarangDTO struct {
	No               int    `json:"no"`
	UnloadHID        string `json:"unloadh_id"`
	UnloadHAgenID    string `json:"unloadh_agen_id"`
	UnloadHTanggal   string `json:"unloadh_tanggal"`
	UnloadHUpdateID  string `json:"unloadh_update_id"`
	UnloadHApproveYN string `json:"unloadh_approve_yn"`
	JmlSP            int    `json:"jml_sp"`
}

// UnloadBarangReq Request Body Tambah / Edit Unloading
type UnloadBarangReq struct {
	UnloadHID        string `json:"unloadh_id" binding:"required"`
	UnloadHAgenID    string `json:"unloadh_agen_id"`
	UnloadHTanggal   string `json:"unloadh_tanggal"`
	UnloadHUpdateID  string `json:"unloadh_update_id"`
	UnloadHApproveYN string `json:"unloadh_approve_yn"`
}
