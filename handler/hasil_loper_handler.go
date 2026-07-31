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

// 🔍 1. LIST DATA HASIL LOPER (BBL)
// 🔍 1. LIST DATA HASIL LOPER (BBL)
func GetHasilLoperList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	noBtt := strings.TrimSpace(c.Query("no_btt"))
	noLoper := strings.TrimSpace(c.Query("no_loper"))
	kota := strings.TrimSpace(c.Query("kota"))
	status := strings.TrimSpace(c.Query("status"))
	pengirim := strings.TrimSpace(c.Query("pengirim"))
	chkTgl := strings.TrimSpace(c.Query("chktgl"))

	filterClause := ""

	if (chkTgl == "true" || chkTgl == "on") && tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(b.bbl_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	if noBtt != "" {
		filterClause += fmt.Sprintf(` AND UPPER(b.bbl_bttid) LIKE UPPER('%%%s%%') `, noBtt)
	}

	if noLoper != "" {
		filterClause += fmt.Sprintf(` AND UPPER(b.bbl_noloper) LIKE UPPER('%%%s%%') `, noLoper)
	}

	if kota != "" {
		filterClause += fmt.Sprintf(` AND UPPER(c.bttt_asalkota) LIKE UPPER('%%%s%%') `, kota)
	}

	if status != "" {
		if status == "Y" {
			filterClause += ` AND b.bbl_terimayn = 'Y' `
		} else {
			filterClause += ` AND b.bbl_terimayn <> 'Y' `
		}
	}

	if pengirim != "" {
		filterClause += fmt.Sprintf(` AND UPPER(c.bttt_asalname) LIKE UPPER('%%%s%%') `, pengirim)
	}

	// ✅ PERBAIKAN: lakukan CAST pada JOIN b.bbl_reasonid dengan r.reason_id
	queryStr := fmt.Sprintf(`
		SELECT 
			COALESCE(b.bbl_eid, '-') AS bbl_eid,
			COALESCE(b.bbl_noloper, '-') AS bbl_noloper,
			TO_CHAR(b.bbl_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS bbl_tanggal,
			COALESCE(b.bbl_bttid, '-') AS bbl_bttid,
			COALESCE(c.bttt_nosuratjalan, '-') AS bttt_nosuratjalan,
			COALESCE(c.bttt_asalkota, '-') AS bttt_asalkota,
			COALESCE(c.bttt_asalname, '-') AS bttt_asalname,
			COALESCE(c.bttt_tujuanalamat, '-') AS bttt_tujuanalamat,
			COALESCE(c.bttt_tujuankota, '-') AS bttt_tujuankota,
			COALESCE(c.bttt_tagihtujuan, 0) AS bttt_tagihtujuan,
			COALESCE(b.bbl_terimayn, 'Y') AS bbl_terimayn,
			CASE WHEN COALESCE(b.bbl_terimayn, 'Y') = 'Y' THEN 'Diterima' ELSE 'Gagal' END AS statusterima,
			COALESCE(r.reason_lokal, b.bbl_keterangan, '-') AS reason_lokal,
			COALESCE(b.bbl_penerima, '-') AS bbl_penerima,
			COALESCE(b.bbl_updateid, '-') AS bbl_updateid,
			TO_CHAR(b.bbl_updatetime, 'YYYY-MM-DD HH24:MI:SS') AS bbl_updatetime,
			COALESCE(b.bbl_aktifyn, 'Y') AS bbl_aktifyn,
			CASE WHEN COALESCE(b.bbl_aktifyn, 'Y') = 'Y' THEN 'Ya' ELSE 'Tidak' END AS aktifjd
		FROM public.opr_t_ebbl b
		LEFT OUTER JOIN public.mkt_t_econote c ON b.bbl_bttid = c.bttt_id
		LEFT OUTER JOIN public.opr_m_reason r ON CAST(NULLIF(REGEXP_REPLACE(b.bbl_reasonid, '[^0-9]', '', 'g'), '') AS INTEGER) = r.reason_id
		WHERE 1=1 %s
		ORDER BY b.bbl_tanggal DESC, b.bbl_eid DESC
	`, filterClause)

	type RawResult struct {
		BBLeID        string  `gorm:"column:bbl_eid" json:"bbl_eid"`
		BBLNoLoper    string  `gorm:"column:bbl_noloper" json:"bbl_noloper"`
		BBLTanggal    string  `gorm:"column:bbl_tanggal" json:"bbl_tanggal"`
		BBLBTTID      string  `gorm:"column:bbl_bttid" json:"bbl_bttid"`
		NoSuratJalan  string  `gorm:"column:bttt_nosuratjalan" json:"bttt_nosuratjalan"`
		AsalKota      string  `gorm:"column:bttt_asalkota" json:"bttt_asalkota"`
		AsalName      string  `gorm:"column:bttt_asalname" json:"bttt_asalname"`
		TujuanAlamat  string  `gorm:"column:bttt_tujuanalamat" json:"bttt_tujuanalamat"`
		TujuanKota    string  `gorm:"column:bttt_tujuankota" json:"bttt_tujuankota"`
		TagihTujuan   float64 `gorm:"column:bttt_tagihtujuan" json:"bttt_tagihtujuan"`
		BBLTerimaYN   string  `gorm:"column:bbl_terimayn" json:"bbl_terimayn"`
		StatusTerima  string  `gorm:"column:statusterima" json:"statusterima"`
		ReasonLokal   string  `gorm:"column:reason_lokal" json:"reason_lokal"`
		BBLPenerima   string  `gorm:"column:bbl_penerima" json:"bbl_penerima"`
		BBLUpdateID   string  `gorm:"column:bbl_updateid" json:"bbl_updateid"`
		BBLUpdateTime string  `gorm:"column:bbl_updatetime" json:"bbl_updatetime"`
		BBLAktifYN    string  `gorm:"column:bbl_aktifyn" json:"bbl_aktifyn"`
		AktifJD       string  `gorm:"column:aktifjd" json:"aktifjd"`
	}

	var rawList []RawResult
	if err := database.Raw(queryStr).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query hasil loper: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   rawList,
		"count":  len(rawList),
	})
}

// 📋 2. GET DROPDOWN MASTER REASON
func GetReasonList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var reasons []models.OprMReason
	database.Where("reason_aktifyn = 'Y'").Order("reason_id ASC").Find(&reasons)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   reasons,
	})
}

// 💾 3. CREATE HASIL LOPER
func CreateHasilLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTEBBL
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	now := time.Now()
	if req.BBLeID == "" {
		req.BBLeID = fmt.Sprintf("BBL/%s/%s", now.Format("20060102"), now.Format("150405"))
	}
	req.BBLTanggal = now.Format("2006-01-02 15:04:05")
	req.BBLUpdateID = fmt.Sprintf("%v", username)
	req.BBLUpdateTime = now.Format("2006-01-02 15:04:05")
	req.BBLAktifYN = "Y"

	if err := database.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal merekam Hasil Loper: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Hasil Loper berhasil direkam!",
		"data":    req,
	})
}

// ✏️ 4. UPDATE HASIL LOPER
func UpdateHasilLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTEBBL
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	updateSQL := `
		UPDATE public.opr_t_ebbl 
		SET bbl_noloper = ?, 
		    bbl_bttid = ?, 
		    bbl_penerima = ?, 
		    bbl_terimayn = ?, 
		    bbl_reasonid = ?, 
		    bbl_keterangan = ?, 
		    bbl_updateid = ?, 
		    bbl_updatetime = NOW() 
		WHERE bbl_eid = ?
	`

	if err := database.Exec(
		updateSQL,
		req.BBLNoLoper, req.BBLBTTID, req.BBLPenerima, req.BBLTerimaYN,
		req.BBLReasonID, req.BBLKeterangan, fmt.Sprintf("%v", username), req.BBLeID,
	).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate Hasil Loper: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Hasil Loper berhasil diperbarui!",
	})
}

// 🗑️ 5. DELETE HASIL LOPER
func DeleteHasilLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	bblEid := c.Query("bbl_eid")
	if bblEid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter bbl_eid wajib diisi"})
		return
	}

	deleteSQL := `DELETE FROM public.opr_t_ebbl WHERE bbl_eid = ?`
	if err := database.Exec(deleteSQL, bblEid).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus Hasil Loper: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Hasil Loper berhasil dihapus!",
	})
}
