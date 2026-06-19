package handler

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GetSuratPengantarFetch menarik daftar data SP untuk data table React (Gambar 1)
func GetSuratPengantarFetch(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database tenant gagal resolved"})
		return
	}

	// 🔍 Tangkap Seluruh parameter saringan filter dari Gambar 1
	tglDari := c.Query("tgl_dari") // Format: YYYY-MM-DD
	tglSampai := c.Query("tgl_sampai")
	agenID := c.Query("agen_id")
	tujuan := c.Query("tujuan")
	noSP := c.Query("no_sp")
	noBTT := c.Query("no_btt")
	noLoading := c.Query("no_loading")
	transit := c.Query("transit") // L = Langsung, T = Transit
	noMobil := c.Query("no_mobil")
	noSuratTugas := c.Query("no_surat_tugas")

	// Pipa Query Dasar
	query := database.Table("public.opr_t_sp AS sp").
		Select(`sp.sph_id AS no_sp, sp.sph_tanggal AS tanggal, sp.sph_asalid, sp.sph_tujuanid, 
		        sp.sph_surat_tugas AS no_st, sp.sph_mobilid AS no_mobil, sp.sph_sopir AS sopir, 
		        sp.sph_status_transit, sp.sph_aktif AS aktif`)

	// 🛡️ SUNTIKAN FILTER DYNAMIC LAYER BERLAPIS (GAMBAR 1)
	if tglDari != "" && tglSampai != "" {
		query = query.Where("sp.sph_tanggal BETWEEN ? AND ?", tglDari+" 00:00:00", tglSampai+" 23:59:59")
	}
	if agenID != "" && agenID != "PUSAT DAKOTA" {
		query = query.Where("sp.sph_asalid = ?", agenID)
	}
	if tujuan != "" {
		query = query.Where("UPPER(sp.sph_tujuanid) LIKE ?", "%"+strings.ToUpper(tujuan)+"%")
	}
	if noSP != "" {
		query = query.Where("sp.sph_id ILIKE ?", "%"+noSP+"%")
	}
	if transit != "" {
		query = query.Where("sp.sph_status_transit = ?", transit)
	}
	if noMobil != "" {
		query = query.Where("sp.sph_mobilid = ?", noMobil)
	}
	if noSuratTugas != "" {
		query = query.Where("sp.sph_surat_tugas ILIKE ?", "%"+noSuratTugas+"%")
	}

	// Filter Join khusus jika mencari berdasarkan No BTT atau No Loading
	if noBTT != "" {
		query = query.Joins("JOIN public.opr_t_sp_detail AS det ON det.spd_sphid = sp.sph_id").
			Where("det.spd_bttnum ILIKE ?", "%"+noBTT+"%")
	}
	if noLoading != "" {
		query = query.Where("sp.sph_loadingnum ILIKE ?", "%"+noLoading+"%")
	}

	var results []map[string]interface{}
	if err := query.Order("sp.sph_tanggal DESC").Limit(100).Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal fetch data SP: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

// CreateSuratPengantar menyimpan data SP baru (Aksi dari Gambar 2 & Gambar 3)
func CreateSuratPengantar(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload data tidak valid: " + err.Error()})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	// Formula generator nomor urut SP otomatis: SP + KODE_ASAL + YYYYMMDD + 4 DIGIT SERIAL RUNNING
	asalID := fmt.Sprintf("%v", payload["sph_asalid"])
	prefixSP := fmt.Sprintf("SP%s%s", asalID, now.Format("20060102"))

	var count int64
	database.Table("public.opr_t_sp").Where("sph_id LIKE ?", prefixSP+"%").Count(&count)
	finalSpID := fmt.Sprintf("%s%04d", prefixSP, count+1)

	// Mapping row database steril
	dbRow := map[string]interface{}{
		"sph_id":             finalSpID,
		"sph_tanggal":        now,
		"sph_asalid":         payload["sph_asalid"],
		"sph_tujuanid":       payload["sph_tujuanid"],
		"sph_mobilid":        payload["sph_mobilid"],
		"sph_sopir":          payload["sph_sopir"],
		"sph_surat_tugas":    payload["sph_surat_tugas"],
		"sph_status_transit": payload["sph_status_transit"],
		"sph_status_gajian":  payload["sph_status_gajian"],
		"sph_aktif":          "Y",
	}

	if err := database.Table("public.opr_t_sp").Create(&dbRow).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengunci Surat Pengantar: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "no_sp": finalSpID})
}
