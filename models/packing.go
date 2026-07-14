package models

// PackingResponse mewakili hasil LEFT/INNER JOIN untuk direndering di React DataTable
type PackingResponse struct {
	BTTTID         string `json:"bttt_id"`          // Dari MKT_T_eConote
	PCKID          string `json:"pck_id"`           // Dari PCK_T_Packing
	AgenNama       string `json:"agen_nama"`        // Dari GLB_M_Agen
	BTTTAsalName   string `json:"bttt_asal_name"`   // Dari MKT_T_eConote
	BTTTTujuanNama string `json:"bttt_tujuan_nama"` // Dari MKT_T_eConote
	BTTTTujuanKota string `json:"bttt_tujuan_kota"` // Dari MKT_T_eConote
	PCKIsiKiriman  string `json:"pck_isikiriman"`   // Dari PCK_T_Packing
	PCKJumlah      int    `json:"pck_jumlah"`       // Dari PCK_T_Packing
	PCKMenjadi     int    `json:"pck_menjadi"`      // Dari PCK_T_Packing
	Appjd          string `json:"appjd"`            // Status 'Ya' jika PCK_ApproveYN = 'Y'
}
