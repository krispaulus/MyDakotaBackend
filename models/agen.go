package models

import (
	"time"

	"gorm.io/gorm"
)

// Agen menampung 100% kolom fisik glb_m_agen untuk operasional loket & closing akuntansi
type Agen struct {
	AgenID                         string     `gorm:"column:agen_id;primaryKey;type:varchar(6)" json:"agen_id"`
	AgenKotaID                     string     `gorm:"column:agen_kotaid;type:varchar(6)" json:"agen_kotaid"`
	AgenCabangID                   string     `gorm:"column:agen_cabangid;type:varchar(6)" json:"agen_cabangid"`
	AgenTlc                        string     `gorm:"column:agen_tlc;type:varchar(6)" json:"agen_tlc"`
	AgenKode                       string     `gorm:"column:agen_kode;type:varchar(4)" json:"agen_kode"`
	AgenNama                       string     `gorm:"column:agen_nama;type:varchar(50)" json:"agen_nama"`
	AgenAlamat                     string     `gorm:"column:agen_alamat;type:varchar(200)" json:"agen_alamat"`
	AgenKota                       string     `gorm:"column:agen_kota;type:varchar(50)" json:"agen_kota"`
	AgenKecamatan                  string     `gorm:"column:agen_kecamatan;type:varchar(50)" json:"agen_kecamatan"`
	AgenPropinsi                   string     `gorm:"column:agen_propinsi;type:varchar(50)" json:"agen_propinsi"`
	AgenContactPerson              string     `gorm:"column:agen_contactperson;type:varchar(30)" json:"agen_contactperson"`
	AgenStt                        string     `gorm:"column:agen_stt;type:varchar(20)" json:"agen_stt"`
	AgenPhone1                     string     `gorm:"column:agen_phone1;type:varchar(20)" json:"agen_phone1"`
	AgenPhone2                     string     `gorm:"column:agen_phone2;type:varchar(20)" json:"agen_phone2"`
	AgenPhone3                     string     `gorm:"column:agen_phone3;type:varchar(20)" json:"agen_phone3"`
	AgenDialString                 string     `gorm:"column:agen_dialstring;type:varchar(50)" json:"agen_dialstring"`
	AgenKomisiKirm                 float32    `gorm:"column:agen_komisikirm;type:real" json:"agen_komisikirm"`
	AgenKomisiTerima1              float64    `gorm:"column:agen_komisiterima1;type:numeric" json:"agen_komisiterima1"`
	AgenKomisiTerima2              float64    `gorm:"column:agen_komisiterima2;type:numeric" json:"agen_komisiterima2"`
	AgenKomisiTransit              float64    `gorm:"column:agen_komisitransit;type:numeric" json:"agen_komisitransit"`
	AgenTransitYN                  string     `gorm:"column:agen_transityn;type:varchar(1)" json:"agen_transit_yn"`
	AgenServerName                 string     `gorm:"column:agen_servername;type:varchar(50)" json:"agen_servername"`
	AgenAktifYN                    string     `gorm:"column:agen_aktifyn;type:varchar(1)" json:"agen_aktifyn"`
	AgenPostingYN                  string     `gorm:"column:agen_postingyn;type:varchar(1)" json:"agen_postingyn"`
	AgenUpdateID                   string     `gorm:"column:agen_updateid;type:varchar(20)" json:"agen_updateid"`
	AgenUpdateTime                 *time.Time `gorm:"column:agen_updatetime" json:"agen_updatetime"`
	AgenPcaID                      string     `gorm:"column:agen_pcaid;type:varchar(20)" json:"agen_pcaid"`
	AgenKomisiCarter               float64    `gorm:"column:agen_komisicarter;type:numeric" json:"agen_komisicarter"`
	AgenStatus                     string     `gorm:"column:agen_status;type:varchar(10)" json:"agen_status"`
	AgenNpwp                       string     `gorm:"column:agen_npwp;type:varchar(20)" json:"agen_npwp"`
	AgenKodePajak                  string     `gorm:"column:agen_kodepajak;type:varchar(5)" json:"agen_kodepajak"`
	AgenAlamatNpwp                 string     `gorm:"column:agen_alamatnpwp;type:varchar(200)" json:"agen_alamatnpwp"`
	AgenLimitJual                  float64    `gorm:"column:agen_limitjual;type:numeric" json:"agen_limitjual"`
	AgenLimitBtt                   float64    `gorm:"column:agen_limitbtt;type:numeric" json:"agen_limitbtt"`
	AgenMinHand                    float64    `gorm:"column:agen_minhand;type:numeric" json:"agen_minhand"`
	AgenInsentifGudang             float64    `gorm:"column:agen_insentifgudang;type:numeric" json:"agen_insentifgudang"`
	AgenLong                       float64    `gorm:"column:agen_long;type:numeric" json:"agen_long"`
	AgenLat                        float64    `gorm:"column:agen_lat;type:numeric" json:"agen_lat"`
	AgenVirtualAcc                 string     `gorm:"column:agen_virtualacc;type:varchar(30)" json:"agen_virtualacc"`
	AgenClosingTime                string     `gorm:"column:agen_closingtime;type:varchar(5)" json:"agen_closingtime"`
	AgenUmr                        float64    `gorm:"column:agen_umr;type:numeric" json:"agen_umr"`
	AgenTarifKota                  float64    `gorm:"column:agen_tarifkota;type:numeric" json:"agen_tarifkota"`
	AgenTarifKecamatan             float64    `gorm:"column:agen_tarifkecamatan;type:numeric" json:"agen_tarifkecamatan"`
	AgenBarcodeScannerPrinterReady string     `gorm:"column:agen_barcodescannerprinterready;type:varchar(1)" json:"agen_barcodescannerprinterready"`
	AgenMd5                        string     `gorm:"column:agen_md5;type:varchar(32)" json:"agen_md5"`
	AgenAktifTanggal               *time.Time `gorm:"column:agen_aktiftanggal" json:"agen_aktiftanggal"`
	AgenNonAktifTanggal            *time.Time `gorm:"column:agen_nonaktiftanggal" json:"agen_nonaktiftanggal"`
	AgenVirtualAcc2                string     `gorm:"column:agen_virtualacc2;type:varchar(30)" json:"agen_virtualacc2"`
	AgenKaAkunting                 string     `gorm:"column:agen_kaakunting;type:varchar(30)" json:"agen_kaakunting"`
	AgenKaContact                  string     `gorm:"column:agen_kacontact;type:varchar(30)" json:"agen_kacontact"`
	AgenKomisiCaID                 string     `gorm:"column:agen_komisicaid;type:varchar(20)" json:"agen_komisicaid"`
	AgenNpwpPribadiYN              string     `gorm:"column:agen_npwppribadyn;type:varchar(1)" json:"agen_npwppribadyn"`
	AgenJemputPusatYN              string     `gorm:"column:agen_jemputpusatyn;type:varchar(1)" json:"agen_jemputpusatyn"`
	AgenKomisiKirim                float64    `gorm:"column:agen_komisikirim;type:numeric" json:"agen_komisikirim"`
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
