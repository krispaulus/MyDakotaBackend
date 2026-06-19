package models

import (
	"time"

	"gorm.io/gorm"
)

// Agen menampung 100% kolom fisik glb_m_agen untuk operasional loket & closing akuntansi
type Agen struct {
	AgenID            string     `gorm:"column:agen_id;primaryKey;type:varchar(6)" json:"agen_id"`
	AgenKotaID        string     `gorm:"column:agen_kotaid;type:varchar(6)" json:"agen_kotaid"`
	AgenCabangID      string     `gorm:"column:agen_cabangid;type:varchar(6)" json:"agen_cabangid"`
	AgenTlc           string     `gorm:"column:agen_tlc;type:varchar(6)" json:"agen_tlc"`
	AgenKode          string     `gorm:"column:agen_kode;type:varchar(4)" json:"agen_kode"`
	AgenNama          string     `gorm:"column:agen_nama;type:varchar(50)" json:"agen_nama"`
	AgenAlamat        string     `gorm:"column:agen_alamat;type:varchar(200)" json:"agen_alamat"`
	AgenKota          string     `gorm:"column:agen_kota;type:varchar(50)" json:"agen_kota"`
	AgenKecamatan     string     `gorm:"column:agen_kecamatan;type:varchar(50)" json:"agen_kecamatan"`
	AgenPropinsi      string     `gorm:"column:agen_propinsi;type:varchar(50)" json:"agen_propinsi"`
	AgenKomisiKirm    float32    `gorm:"column:agen_komisikirm;type:real" json:"agen_komisikirm"`
	AgenKomisiTerima1 float64    `gorm:"column:agen_komisiterima1;type:numeric" json:"agen_komisiterima1"`
	AgenKomisiTerima2 float64    `gorm:"column:agen_komisiterima2;type:numeric" json:"agen_komisiterima2"`
	AgenKomisiTransit float64    `gorm:"column:agen_komisitransit;type:numeric" json:"agen_komisitransit"`
	AgenAktifYN       string     `gorm:"column:agen_aktifyn;type:varchar(1)" json:"agen_aktifyn"`
	SuratTugas        string     `gorm:"column:agen_postingyn;type:varchar(1)" json:"agen_postingyn"`
	AgenPcaID         string     `gorm:"column:agen_pcaid;type:varchar(20)" json:"agen_pcaid"`           // Akun Piutang Agen
	AgenKomisiCaID    string     `gorm:"column:agen_komisicaid;type:varchar(20)" json:"agen_komisicaid"` // Akun Komisi Agen
	AgenNpwp          string     `gorm:"column:agen_npwp;type:varchar(20)" json:"agen_npwp"`
	AgenNpwpPribadiYN string     `gorm:"column:agen_npwppribadyn;type:varchar(1)" json:"agen_npwppribadyn"` // Y=Pribadi (PPh21), N=Badan (PPh23)
	AgenJemputPusatYN string     `gorm:"column:agen_jemputpusatyn;type:varchar(1)" json:"agen_jemputpusatyn"`
	AgenUpdateTime    *time.Time `gorm:"column:agen_updatetime" json:"agen_updatetime"`
}

// TableName mengunci nama tabel fisik di public schema Postgres
func (Agen) TableName() string {
	return "public.glb_m_agen"
}

// GetActiveAgens menarik list ringkas khusus untuk Dropdown Loket di UI (Tetap Enteng & Cepat!)
func GetActiveAgens(db *gorm.DB) ([]Agen, error) {
	var listAgen []Agen

	// 🚀 TRICK SAKTI: Cukup select kolom-kolom dropdown loket saja biar performa database tetap ngebut!
	err := db.Select("agen_id, agen_kode, agen_nama, agen_kotaid, agen_kota, agen_aktifyn, agen_cabangid").
		Where("agen_aktifyn = ?", "Y").
		Where("agen_nama NOT LIKE ?", "%XXX%").
		Order("agen_nama ASC").
		Find(&listAgen).Error

	return listAgen, err
}
