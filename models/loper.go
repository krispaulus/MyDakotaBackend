package models

// OprTEloper memetakan tabel induk header manifes loper
type OprTEloper struct {
	ID          int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	LoperEID    string `gorm:"column:loper_eid" json:"loper_eid"`
	LoperAgenID string `gorm:"column:loper_agenid" json:"loper_agenid"`

	// 🟩 DUA TIANG UTAMA DIUBAH MENJADI STRING BIAR SINKRON 100% SAMA DATABASE BRAY!
	LoperTanggal    string `gorm:"column:loper_tanggal" json:"loper_tanggal"`
	LoperUpdateTime string `gorm:"column:loper_updatetime;default:now()" json:"loper_updatetime"`

	LoperNIPSopir    string  `gorm:"column:loper_nipsopir" json:"loper_nipsopir"`
	LoperKeraniYN    string  `gorm:"column:loper_keraniyn;default:N" json:"loper_keraniyn"`
	LoperNIPKerani   *string `gorm:"column:loper_nipkerani" json:"loper_nipkerani"`
	LoperNoMobil     string  `gorm:"column:loper_nomobil" json:"loper_nomobil"`
	LoperAktifYN     string  `gorm:"column:loper_aktifyn;default:Y" json:"loper_aktifyn"`
	LoperUpdateID    *string `gorm:"column:loper_updateid" json:"loper_updateid"`
	LoperOTP         *string `gorm:"column:loper_otp" json:"loper_otp"`
	LoperLoadHID     *string `gorm:"column:loper_loadhid" json:"loper_loadhid"`
	LoperLoadChecker *string `gorm:"column:loper_loadchecker" json:"loper_loadchecker"`
}

func (OprTEloper) TableName() string {
	return "opr_t_eloper"
}

// OprTEloperDetail memetakan detail resi/conote muatan loper
type OprTEloperDetail struct {
	ID               int64   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	LoperdELoperID   string  `gorm:"column:loperd_eloperid" json:"loperd_eloperid"`
	LoperdBTTID      string  `gorm:"column:loperd_bttid" json:"loperd_bttid"`
	LoperdHistID     *string `gorm:"column:loperd_histid" json:"loperd_histid"`
	LoperdUpdateTime string  `gorm:"column:loperd_updatetime;default:now()" json:"loperd_updatetime"`
	LoperdGPS        *string `gorm:"column:loperd_gps" json:"loperd_gps"`
	LoperdJarak      float64 `gorm:"column:loperd_jarak;default:0.00" json:"loperd_jarak"`
	LoperdWaktu      int     `gorm:"column:loperd_waktu;default:0" json:"loperd_waktu"`
	LoperdUrut       int     `gorm:"column:loperd_urut;default:1" json:"loperd_urut"`
}

func (OprTEloperDetail) TableName() string {
	return "opr_t_eloperdetail"
}

// OprTEloperInsentif memetakan induk komisi loper
type OprTEloperInsentif struct {
	ID               int64   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	LopInsID         string  `gorm:"column:lopins_id" json:"lopins_id"`
	LoperAgenID      string  `gorm:"column:loper_agenid" json:"loper_agenid"`
	LopInsTanggal    string  `gorm:"column:lopins_tanggal" json:"lopins_tanggal"`
	LoperUpdateTime  string  `gorm:"column:loper_updatetime;default:now()" json:"loper_updatetime"`
	LoperNIPSopir    string  `gorm:"column:loper_nipsopir" json:"loper_nipsopir"`
	LoperKeraniYN    string  `gorm:"column:loper_keraniyn;default:N" json:"loper_keraniyn"`
	LoperNIPKerani   *string `gorm:"column:loper_nipkerani" json:"loper_nipkerani"`
	LoperNoMobil     string  `gorm:"column:loper_nomobil" json:"loper_nomobil"`
	LoperAktifYN     string  `gorm:"column:loper_aktifyn;default:Y" json:"loper_aktifyn"`
	LoperUpdateID    *string `gorm:"column:loper_updateid" json:"loper_updateid"`
	LoperOTP         *string `gorm:"column:loper_otp" json:"loper_otp"`
	LoperLoadHID     *string `gorm:"column:loper_loadhid" json:"loper_loadhid"`
	LoperLoadChecker *string `gorm:"column:loper_loadchecker" json:"loper_loadchecker"`
}

func (OprTEloperInsentif) TableName() string {
	return "opr_t_eloper_insentif"
}

// OprTEloperInsentifDetail memetakan detail rincian komisi per conote
type OprTEloperInsentifDetail struct {
	ID              int64   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	LopInsdLopInsID string  `gorm:"column:lopinsd_lopinsid" json:"lopinsd_lopinsid"`
	LopInsdLoperID  string  `gorm:"column:lopinsd_loperid" json:"lopinsd_loperid"`
	LopInsdBTTID    string  `gorm:"column:lopinsd_bttid" json:"lopinsd_bttid"`
	LopInsdKomisi   float64 `gorm:"column:lopinsd_komisi;default:0.00" json:"lopinsd_komisi"`
}

func (OprTEloperInsentifDetail) TableName() string {
	return "opr_t_eloper_insentifdetail"
}
