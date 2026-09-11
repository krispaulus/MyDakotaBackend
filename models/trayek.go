package models

import "time"

type OprMTrayekH struct {
	TrhID         string    `gorm:"column:trh_id;primaryKey" json:"trh_id"`
	TrhName       string    `gorm:"column:trh_name" json:"trh_name"`
	TrhUpdateID   *string   `gorm:"column:trh_updateid" json:"trh_update_id"`
	TrhUpdateTime time.Time `gorm:"column:trh_updatetime;default:now()" json:"trh_update_time"`
	TrhAktifYN    string    `gorm:"column:trh_aktifyn;default:Y" json:"trh_aktif_yn"`
	TrhJnsKend    *string   `gorm:"column:trh_jns_kend" json:"trh_jns_kend"`
	TrhTarifB     float64   `gorm:"column:trh_tarif_b;default:0.00" json:"trh_tarif_b"`
	TrhTarifP     float64   `gorm:"column:trh_tarif_p;default:0.00" json:"trh_tarif_p"`
	TrhBbmRatio   float64   `gorm:"column:trh_bbm_ratio;default:0.00" json:"trh_bbm_ratio"`
	TrhBbmJatah   float64   `gorm:"column:trh_bbm_jatah;default:0.00" json:"trh_bbm_jatah"`
	TrhTotalKm    float64   `gorm:"column:trh_total_km;default:0.00" json:"trh_total_km"`
}

func (OprMTrayekH) TableName() string {
	return "public.opr_m_trayekh"
}

// OprMTrayekD memetakan detail urutan kota singgah
type OprMTrayekD struct {
	ID        int64   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TrdHid    string  `gorm:"column:trd_hid" json:"trd_hid"`
	TrdAgenID string  `gorm:"column:trd_agen_id" json:"trd_agen_id"`
	TrdUrut   int     `gorm:"column:trd_urut" json:"trd_urut"`
	TrdStatus string  `gorm:"column:trd_status" json:"trd_status"` // 'B' / 'P'
	TrdJenis  string  `gorm:"column:trd_jenis" json:"trd_jenis"`   // 'U' / 'C'
	TrdKm     float64 `gorm:"column:trd_km;default:0.00" json:"trd_km"`
	TrdET     int     `gorm:"column:trd_et;default:0" json:"trd_et"`

	AgenNama string `gorm:"-" json:"agen_nama"`
}

func (OprMTrayekD) TableName() string {
	return "public.opr_m_trayekd"
}

// OprMTrayekBplk memetakan komponen biaya jalan driver (BPLK)
type OprMTrayekBplk struct {
	ID           int64   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TriHid       string  `gorm:"column:tri_hid" json:"tri_hid"`
	TriBplkID    string  `gorm:"column:tri_bplk_id" json:"tri_bplk_id"`
	TriNominal   float64 `gorm:"column:tri_nominal;default:0.00" json:"tri_nominal"`
	TriUmYN      string  `gorm:"column:tri_um_yn;default:Y" json:"tri_um_yn"`
	TriBplkSubID *string `gorm:"column:tri_bplk_sub_id" json:"tri_bplk_sub_id"`
}

func (OprMTrayekBplk) TableName() string {
	return "public.opr_m_trayekbplk"
}

// OprMTrayekVoucher memetakan jatah kuota voucher BBM solar SPBU
type OprMTrayekVoucher struct {
	ID        int64   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TrvHid    string  `gorm:"column:trv_hid" json:"trv_hid"`
	TrvSPBU   string  `gorm:"column:trv_spbu" json:"trv_spbu"`
	TrvBplkID string  `gorm:"column:trv_bplk_id" json:"trv_bplk_id"`
	TrvLiter  float64 `gorm:"column:trv_liter;default:0.00" json:"trv_liter"`
	TrvHarga  float64 `gorm:"column:trv_harga;default:0.00" json:"trv_harga"`
}

func (OprMTrayekVoucher) TableName() string {
	return "public.opr_m_trayekvoucher"
}
