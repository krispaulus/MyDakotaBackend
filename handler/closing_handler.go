package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// =========================================================================
// 📄 1. API UNTUK MENAMPILKAN RIWAYAT CLOSING HARIAN AGEN (GAMBAR 1)
// =========================================================================
func GetClosingAgenList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database tenant gagal resolved"})
		return
	}

	tglDari := c.Query("tgl_dari")
	tglSampai := c.Query("tgl_sampai")
	agenID := c.Query("agen_id")
	noLaporan := c.Query("no_laporan")
	noBtt := c.Query("no_btt")
	noJurnal := c.Query("no_jurnal")
	postingStatus := c.Query("posting_status")

	query := database.Table("public.art_t_penjualanbtth AS h").
		Select(`h.btth_id AS no_laporan, h.btth_tanggal AS tanggal, h.btth_agenid AS cabang, 
		        h.btth_pembayaran, h.btth_cbid AS no_kas, h.btth_postingyn AS posting, 
		        h.btth_tjurhno AS no_jurnal, h.btth_activeyn AS aktif`)

	if tglDari != "" && tglSampai != "" {
		query = query.Where("h.btth_tanggal BETWEEN ? AND ?", tglDari+" 00:00:00", tglSampai+" 23:59:59")
	}
	if agenID != "" && agenID != "ALL" {
		// 🚀 SAFE CASTING: Paksa perbandingan kode agen sebagai string varchar
		query = query.Where("CAST(h.btth_agenid AS VARCHAR) = CAST(? AS VARCHAR)", agenID)
	}
	if noLaporan != "" {
		query = query.Where("h.btth_id ILIKE ?", "%"+noLaporan+"%")
	}
	if noJurnal != "" {
		query = query.Where("h.btth_tjurhno ILIKE ?", "%"+noJurnal+"%")
	}
	if postingStatus != "" {
		query = query.Where("h.btth_postingyn = ?", postingStatus)
	}
	if noBtt != "" {
		query = query.Joins("JOIN public.art_t_penjualanbttd AS d ON d.bttd_btthid = h.btth_id").
			Where("d.bttd_bttid ILIKE ?", "%"+noBtt+"%")
	}

	var results []map[string]interface{}
	if err := query.Where("h.btth_id LIKE ?", "%/SB").Order("h.btth_tanggal DESC").Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal fetch data closing harian: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

// =========================================================================
// 📄 ENGINE CORE: PROSES TAMBAH CLOSING HARIAN + AUTOMATIC RELOAD BTT (CAST SAFELY)
// =========================================================================
func ProcessTambahClosingHarian(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	if fmt.Sprintf("%v", ptID) == "<nil>" || fmt.Sprintf("%v", ptID) == "" {
		if altID, ok := c.Get("selected_pt"); ok {
			ptID = altID
		} else {
			ptID = "A"
		}
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Resolving database tenant gagal"})
		return
	}

	var payload struct {
		TanggalClosing string `json:"tanggal_closing"`
		CabangAgen     string `json:"cabang_agen"`
		UpdateID       string `json:"update_id"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Struktur payload data tidak sah!"})
		return
	}

	tglParsed, err := time.Parse("2006-01-02", payload.TanggalClosing)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format tanggal closing salah!"})
		return
	}

	cleanAgenID := strings.TrimSpace(payload.CabangAgen)
	if len(cleanAgenID) > 3 {
		cleanAgenID = cleanAgenID[len(cleanAgenID)-3:]
	}
	//finalClosingID := fmt.Sprintf("%s%s%s/SB", cleanAgenID, tglParsed.Format("01"), tglParsed.Format("2006"))
	finalClosingID := fmt.Sprintf("%s%s/SB", cleanAgenID, tglParsed.Format("20060102"))

	var countHeader int64
	database.Table("public.art_t_penjualanbtth").Where("btth_id = ?", finalClosingID).Count(&countHeader)
	if countHeader > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal Proses! Agen dengan nomor ID Closing " + finalClosingID + " sudah terdaftar!"})
		return
	}

	// 🚀 FIX CASTING LINE 135: Paksa bttt_asalagenid dibandingkan secara aman menggunakan string casting
	var listBttNaik []string
	errFetchNaik := database.Table("public.mkt_t_econote").
		Where("DATE(bttt_tanggal) = DATE(?) AND CAST(bttt_asalagenid AS VARCHAR) = CAST(? AS VARCHAR) AND bttt_aktifyn = 'Y'",
			payload.TanggalClosing, payload.CabangAgen).
		Where("bttt_id NOT IN (SELECT bttd_bttid FROM public.art_t_penjualanbttd)").
		Pluck("bttt_id", &listBttNaik).Error

	if errFetchNaik != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memindai manifest BTT Naik: " + errFetchNaik.Error()})
		return
	}

	// 🚀 FIX CASTING LINE 144 (BIANG KEROK): Paksa bbl_agenid dan bttt_pembayaran menggunakan komparasi tipe data yang setara
	var listBttTurun []string
	errFetchTurun := database.Table("public.opr_t_ebbl AS bbl").
		Select("bbl.bbl_bttid").
		Joins("JOIN public.mkt_t_econote AS btt ON btt.bttt_id = bbl.bbl_bttid").
		Where("DATE(btt.bttt_tanggal) = DATE(?) AND CAST(bbl.bbl_agenid AS VARCHAR) = CAST(? AS VARCHAR)",
			payload.TanggalClosing, payload.CabangAgen).
		Where("bbl.bbl_bttid NOT IN (SELECT bttd_bttid FROM public.art_t_penjualanbttd)").
		Pluck("bbl.bbl_bttid", &listBttTurun).Error

	if errFetchTurun != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memindai manifest BTT Turun (BBL): " + errFetchTurun.Error()})
		return
	}

	totalResiSiapClosing := append(listBttNaik, listBttTurun...)

	if len(totalResiSiapClosing) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal Closing! Tidak ditemukan transaksi manifest resi BTT baru (Naik/Turun) yang aktif pada tanggal tersebut!"})
		return
	}

	tx := database.Begin()

	headerRow := models.PenjualanBttH{
		BtthID:         finalClosingID,
		BtthTanggal:    tglParsed,
		BtthAgenID:     payload.CabangAgen,
		BtthPembayaran: 1,
		BtthActiveYN:   "Y",
		BtthCbid:       "KM-" + tglParsed.Format("20060102") + "-" + cleanAgenID,
		BtthPostingYN:  "N",
		BtthTjurhNo:    "-",
		BtthNoKW:       "-",
		BtthUpdateID:   payload.UpdateID,
		BtthUpdateTime: time.Now(),
	}

	if err := tx.Create(&headerRow).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengunci header kas closing harian: " + err.Error()})
		return
	}

	for _, bttID := range totalResiSiapClosing {
		detailRow := models.PenjualanBttD{
			BttdBtthID:    finalClosingID,
			BttdBttID:     bttID,
			BttdBttSeries: "",
		}
		if err := tx.Create(&detailRow).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal merakit detail manifest closing: " + err.Error()})
			return
		}
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"no_laporan": finalClosingID,
		"total_resi": len(totalResiSiapClosing),
		"btt_naik":   len(listBttNaik),
		"btt_turun":  len(listBttTurun),
	})
}
