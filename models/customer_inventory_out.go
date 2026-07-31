package models

// CustomerInventoryOutDTO Struct Response List Pengeluaran Inventory
type CustomerInventoryOutDTO struct {
	OutID       int64  `json:"out_id"`
	OutTanggal  string `json:"out_tanggal"`
	OutCustID   string `json:"out_cust_id"`
	OutCustName string `json:"out_cust_name"`
	OutItemID   string `json:"out_item_id"`
	OutItemName string `json:"out_item_name"`
	OutQty      int    `json:"out_qty"`
	OutNoBTT    string `json:"out_no_btt"`
	OutUpdateID string `json:"out_update_id"`
}

// CustomerInventoryOutReq Request Body Tambah / Edit Pengeluaran Inventory
type CustomerInventoryOutReq struct {
	OutID       int64  `json:"out_id"`
	OutTanggal  string `json:"out_tanggal"`
	OutCustID   string `json:"out_cust_id" binding:"required"`
	OutCustName string `json:"out_cust_name"`
	OutItemID   string `json:"out_item_id" binding:"required"`
	OutItemName string `json:"out_item_name"`
	OutQty      int    `json:"out_qty" binding:"required"`
	OutNoBTT    string `json:"out_no_btt" binding:"required"`
}
