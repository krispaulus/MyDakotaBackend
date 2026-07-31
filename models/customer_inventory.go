package models

// CustomerInventoryDTO Struct Response List Inventory
type CustomerInventoryDTO struct {
	InvID         int64  `json:"inv_id"`
	InvTanggal    string `json:"inv_tanggal"`
	InvCustID     string `json:"inv_cust_id"`
	InvCustName   string `json:"inv_cust_name"`
	InvItemID     string `json:"inv_item_id"`
	InvItemName   string `json:"inv_item_name"`
	InvQty        int    `json:"inv_qty"`
	InvSupID      string `json:"inv_sup_id"`
	InvSupName    string `json:"inv_sup_name"`
	InvSuratJalan string `json:"inv_surat_jalan"`
	InvUpdateID   string `json:"inv_update_id"`
}

// CustomerInventoryReq Request Body Tambah / Edit Inventory
type CustomerInventoryReq struct {
	InvID         int64  `json:"inv_id"`
	InvTanggal    string `json:"inv_tanggal"`
	InvCustID     string `json:"inv_cust_id" binding:"required"`
	InvCustName   string `json:"inv_cust_name"`
	InvItemID     string `json:"inv_item_id" binding:"required"`
	InvItemName   string `json:"inv_item_name"`
	InvQty        int    `json:"inv_qty" binding:"required"`
	InvSupID      string `json:"inv_sup_id"`
	InvSupName    string `json:"inv_sup_name"`
	InvSuratJalan string `json:"inv_surat_jalan" binding:"required"`
}
