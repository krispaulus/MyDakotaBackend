package models

type GlbMKendaraan struct {
	KendID            string  `gorm:"column:kend_id;primaryKey" json:"kend_id"`
	KendPemilik       *string `gorm:"column:kend_pemilik" json:"kend_pemilik"`
	KendAlamat        *string `gorm:"column:kend_alamat" json:"kend_alamat"`
	KendMerk          *string `gorm:"column:kend_merk" json:"kend_merk"`
	KendType          *string `gorm:"column:kend_type" json:"kend_type"`
	KendJenis         *string `gorm:"column:kend_jenis" json:"kend_jenis"`
	KendWarna         *string `gorm:"column:kend_warna" json:"kend_warna"`
	KendNIK           *string `gorm:"column:kend_nik" json:"kend_nik"`
	KendNoMesin       *string `gorm:"column:kend_nomesin" json:"kend_nomesin"`
	KendIdentID       *string `gorm:"column:kend_identid" json:"kend_identid"`
	KendWarnaTNKB     *string `gorm:"column:kend_warnatnkb" json:"kend_warnatnkb"`
	KendBahanBakar    *string `gorm:"column:kend_bahanbakar" json:"kend_bahanbakar"`
	KendLokasiID      *string `gorm:"column:kend_lokasiid" json:"kend_lokasiid"`
	KendUpdateID      *string `gorm:"column:kend_updateid" json:"kend_updateid"`
	KendGpsImei       *string `gorm:"column:kend_gps_imei" json:"kend_gps_imei"`
	KendNomorSpasi    *string `gorm:"column:kend_nomor_spasi" json:"kend_nomor_spasi"`
	KendPT            *string `gorm:"column:kend_pt" json:"kend_pt"`
	KendWarnaPlat     *string `gorm:"column:kend_warnaplat" json:"kend_warnaplat"`
	KendLeasing       *string `gorm:"column:kend_leasing" json:"kend_leasing"`
	KendLat           *string `gorm:"column:kend_lat" json:"kend_lat"`
	KendLon           *string `gorm:"column:kend_lon" json:"kend_lon"`
	KendSerahTerimaID *string `gorm:"column:kend_serahterimaid" json:"kend_serahterimaid"`
	KendSopir1        *string `gorm:"column:kend_sopir1" json:"kend_sopir1"`
	KendSopir2        *string `gorm:"column:kend_sopir2" json:"kend_sopir2"`

	// 👑 PENJINAK UTAMA RANJAU "" STRIP: Ubah Kolom Angka Menjadi *string bray!
	KendThnBuat     *string `gorm:"column:kend_thnbuat" json:"kend_thnbuat"`
	KendThnRakit    *string `gorm:"column:kend_thnrakit" json:"kend_thnrakit"`
	KendIsiSilinder *string `gorm:"column:kend_isisilinder" json:"kend_isisilinder"`
	KendJarak       *string `gorm:"column:kend_jarak" json:"kend_jarak"`

	// 🛡️ PENJINAK KOLOM TANGGAL DATE / TIMESTAMP
	KendBerlakuSTNK      *string `gorm:"column:kend_berlakustnk" json:"kend_berlakustnk"`
	KendBerlakuPajak     *string `gorm:"column:kend_berlakupajak" json:"kend_berlakupajak"`
	KendUpdateTime       *string `gorm:"column:kend_updatetime" json:"kend_updatetime"`
	KendBerlakuKIR       *string `gorm:"column:kend_berlakukir" json:"kend_berlakukir"`
	KendLastGpsUpdate    *string `gorm:"column:kend_lastgpsupdate" json:"kend_lastgpsupdate"`
	KendSerahTerimaTime  *string `gorm:"column:kend_serahterimatime" json:"kend_serahterimatime"`
	KendServiceStartDate *string `gorm:"column:kend_servicestartdate" json:"kend_servicestartdate"`
	KendServiceEndDate   *string `gorm:"column:kend_serviceenddate" json:"kend_serviceenddate"`

	// Status String Normal
	KendAktifYN   string `gorm:"column:kend_aktifyn;default:Y" json:"kend_aktifyn"`
	KendPostingYN string `gorm:"column:kend_postingyn;default:N" json:"kend_postingyn"`
	KendServiceYN string `gorm:"column:kend_serviceyn;default:N" json:"kend_serviceyn"`
}

func (GlbMKendaraan) TableName() string {
	return "glb_m_kendaraan"
}

type KendaraanRequest struct {
	KendID          string `json:"kend_id" binding:"required"`
	KendPemilik     string `json:"kend_pemilik"`
	KendAlamat      string `json:"kend_alamat"`
	KendMerk        string `json:"kend_merk"`
	KendJenis       string `json:"kend_jenis"`
	KendBerlakuSTNK string `json:"kend_berlakustnk"` // Format: YYYY-MM-DD
	KendAktifYN     string `json:"kend_aktifyn"`
	KendGpsImei     string `json:"kend_gps_imei"`
}
