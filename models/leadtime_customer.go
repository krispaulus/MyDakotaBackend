package models

type TransportPlanningLeadTime struct {
	CustomerID string `gorm:"column:customer_id" json:"customer_id"`
	CustName   string `gorm:"-" json:"cust_name"` // <-- Tambahkan gorm:"-"
	KotaAsal   string `gorm:"column:kota_asal" json:"kota_asal"`
	KotaTujuan string `gorm:"column:kota_tujuan" json:"kota_tujuan"`
	Kabupaten  string `gorm:"column:kabupaten" json:"kabupaten"`
	Darat      int    `gorm:"column:darat" json:"darat"`
	Laut       int    `gorm:"column:laut" json:"laut"`
	Udara      int    `gorm:"column:udara" json:"udara"`
}

func (TransportPlanningLeadTime) TableName() string {
	return `"public"."opr_t_transportplanningleadtime"`
}

type LeadTimeFilter struct {
	CustomerID string `form:"customer_id"`
	KotaAsal   string `form:"kota_asal"`
	KotaTujuan string `form:"kota_tujuan"`
	Page       int    `form:"page,default=1"`
	Limit      int    `form:"limit,default=15"`
}
