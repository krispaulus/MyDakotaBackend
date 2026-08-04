package handler

import (
	"fmt"
	"log"
	"net/http"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

type SPPrintHeaderDTO struct {
	NoSP       string `json:"no_sp" gorm:"column:no_sp"`
	TglSP      string `json:"tgl_sp" gorm:"column:tgl_sp"`
	NoMobil    string `json:"no_mobil" gorm:"column:no_mobil"`
	NamaSopir  string `json:"nama_sopir" gorm:"column:nama_sopir"`
	AgenAsal   string `json:"agen_asal" gorm:"column:agen_asal"`
	AgenTujuan string `json:"agen_tujuan" gorm:"column:agen_tujuan"`
}

type SPPrintDetailDTO struct {
	NoBTT        string `json:"no_btt" gorm:"column:no_btt"`
	NamaPenerima string `json:"nama_penerima" gorm:"column:nama_penerima"`
	KotaTujuan   string `json:"kota_tujuan" gorm:"column:kota_tujuan"`
	Keterangan   string `json:"keterangan" gorm:"column:keterangan"`
}

type SPPrintResponseDTO struct {
	Header  SPPrintHeaderDTO   `json:"header"`
	Details []SPPrintDetailDTO `json:"details"`
}

// GetPrintSuratPengiriman Handler untuk mengambil data cetak SP berdasarkan No SP
func GetPrintSuratPengiriman(c *gin.Context) {
	noSP := c.Query("nobtt")
	if noSP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor SP wajib diisi!"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		// Fallback ke Default Database jika resolver pt_id mismatch
		database = db.GetDB()
	}

	log.Printf("🔍 [SP PRINT] Memproses pencarian Nomor SP: '%s' (pt_id: %v)", noSP, ptID)

	// 1. Query Header SP Utama (Gunakan ILIKE dan TRIM agar case-insensitive)
	headerQuery := `
		SELECT 
			TRIM(sp.spt_eid) AS no_sp,
			TO_CHAR(sp.spt_tanggal, 'DD/MM/YYYY') AS tgl_sp,
			COALESCE(sp.spt_nomobil, '-') AS no_mobil,
			COALESCE(sp.spt_namasopir, '-') AS nama_sopir,
			COALESCE(asal.agen_nama, '-') AS agen_asal,
			COALESCE(tujuan.agen_nama, '-') AS agen_tujuan
		FROM public.opr_t_esp_terima sp
		LEFT JOIN public.glb_m_agen asal ON CAST(sp.spt_asalagenid AS VARCHAR) = CAST(asal.agen_id AS VARCHAR)
		LEFT JOIN public.glb_m_agen tujuan ON CAST(sp.spt_tujuanagenid AS VARCHAR) = CAST(tujuan.agen_id AS VARCHAR)
		WHERE TRIM(sp.spt_eid) ILIKE TRIM(?)
		LIMIT 1
	`

	var header SPPrintHeaderDTO
	err := database.Raw(headerQuery, noSP).Scan(&header).Error

	// Jika masih kosong, coba fallback ke db default pusat
	if err != nil || header.NoSP == "" {
		log.Printf("⚠️ Header tidak ditemukan di resolver DB, mencoba db fallback...")
		db.GetDB().Raw(headerQuery, noSP).Scan(&header)
	}

	if header.NoSP == "" {
		log.Printf("❌ ERROR: Nomor SP '%s' benar-benar tidak ditemukan di database!", noSP)
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": fmt.Sprintf("Data Nomor SP '%s' tidak ditemukan di database!", noSP)})
		return
	}

	// 2. Query Detail BTT
	detailQuery := `
		SELECT 
			COALESCE(TRIM(spd.sptd_bttid), '-') AS no_btt,
			COALESCE(e.bttt_tujuannama, '-') AS nama_penerima,
			COALESCE(e.bttt_tujuankota, '-') AS kota_tujuan,
			COALESCE(e.bttt_ket, '-') AS keterangan
		FROM public.opr_t_esp_terimadetil spd
		LEFT JOIN public.mkt_t_econote e ON TRIM(UPPER(spd.sptd_bttid)) = TRIM(UPPER(e.bttt_id))
		WHERE TRIM(spd.sptd_esptid) ILIKE TRIM(?)
		ORDER BY spd.sptd_bttid ASC
	`

	var details []SPPrintDetailDTO
	database.Raw(detailQuery, noSP).Scan(&details)
	if len(details) == 0 {
		db.GetDB().Raw(detailQuery, noSP).Scan(&details)
	}

	if details == nil {
		details = []SPPrintDetailDTO{}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": SPPrintResponseDTO{
			Header:  header,
			Details: details,
		},
	})
}
