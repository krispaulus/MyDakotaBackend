package models

type TransportPlanningLeadTime struct {
	CustomerID string `gorm:"column:customer_id;primaryKey" json:"customer_id"`
	CustName   string `gorm:"column:cust_name;->;" json:"cust_name,omitempty"` // Read only dari JOIN
	KotaAsal   string `gorm:"column:kota_asal;primaryKey" json:"kota_asal"`
	KotaTujuan string `gorm:"column:kota_tujuan;primaryKey" json:"kota_tujuan"`
	Kabupaten  string `gorm:"column:kabupaten;primaryKey" json:"kabupaten"`
	Darat      int    `gorm:"column:darat" json:"darat"`
	Laut       int    `gorm:"column:laut" json:"laut"`
	Udara      int    `gorm:"column:udara" json:"udara"`
}

// TableName mengarahkan GORM ke nama tabel & schema PostgreSQL yang tepat
func (TransportPlanningLeadTime) TableName() string {
	return `"public"."opr_t_transportplanningleadtime"` // 🟢 WAJIB SERTAKAN "public".
}

// Struct untuk Pagination & Filter Request
type LeadTimeFilter struct {
	CustomerID string `form:"customer_id"`
	KotaAsal   string `form:"kota_asal"`
	KotaTujuan string `form:"kota_tujuan"`
	Page       int    `form:"page,default=1"`
	Limit      int    `form:"limit,default=15"`
}
