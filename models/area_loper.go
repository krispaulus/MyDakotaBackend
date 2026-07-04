package models

type AreaLoper struct {
	ID                  int     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AreaAgenID          int     `gorm:"column:area_agenid" json:"area_agenid"`
	TujuanKelurahan     string  `gorm:"column:tujuan_kelurahan;type:varchar(150)" json:"tujuan_kelurahan"`
	TujuanKecamatan     string  `gorm:"column:tujuan_kecamatan;type:varchar(150)" json:"tujuan_kecamatan"`
	TujuanKabupaten     string  `gorm:"column:tujuan_kabupaten;type:varchar(150)" json:"tujuan_kabupaten"`
	TujuanPropinsi      string  `gorm:"column:tujuan_propinsi;type:varchar(150)" json:"tujuan_propinsi"`
	HandDarat           float64 `gorm:"column:hand_darat;type:numeric" json:"hand_darat"`
	HandLaut            float64 `gorm:"column:hand_laut;type:numeric" json:"hand_laut"`
	HandUdara           float64 `gorm:"column:hand_udara;type:numeric" json:"hand_udara"`
	HandDaratKurir      float64 `gorm:"column:hand_daratkurir;type:numeric" json:"hand_daratkurir"`
	HandLautKurir       float64 `gorm:"column:hand_lautkurir;type:numeric" json:"hand_lautkurir"`
	HandUdaraKurir      float64 `gorm:"column:hand_udarakurir;type:numeric" json:"hand_udarakurir"`
	PickupAgenID        int     `gorm:"column:pickup_agenid" json:"pickup_agenid"`
	PenerusYN           string  `gorm:"column:penerusyn;type:varchar(1)" json:"penerusyn"`
	KgMin               float64 `gorm:"column:kgmin;type:numeric" json:"kgmin"`
	HrgPenerus          float64 `gorm:"column:hrgpenerus;type:numeric" json:"hrgpenerus"`
	LeadTime            int     `gorm:"column:leadtime" json:"leadtime"`
	ProsentaseByKirimYN string  `gorm:"column:prosentasebykirimyn;type:varchar(1)" json:"prosentasebykirimyn"`
}

func (AreaLoper) TableName() string {
	return "public.opr_m_earea"
}

// Struct penampung data wilayah yang terlewat/belum terdaftar
type WilayahBelumTerdaftar struct {
	Kecamatan string `json:"kecamatan"`
	Kelurahan string `json:"kelurahan"`
	Kabupaten string `json:"kabupaten"`
	Propinsi  string `json:"propinsi"`
}
