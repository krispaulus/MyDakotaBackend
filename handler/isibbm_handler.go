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

// 🔍 1. LIST DATA PENGISIAN BBM
func GetIsiBBMList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	agenNama := strings.TrimSpace(c.Query("agen_nama"))
	noMobil := strings.TrimSpace(c.Query("no_mobil"))
	chkTgl := strings.TrimSpace(c.Query("chktgl"))

	filterClause := ""

	if (chkTgl == "true" || chkTgl == "on") && tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(b.bbm_date AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	if agenNama != "" {
		filterClause += fmt.Sprintf(` AND (UPPER(a.agen_nama) LIKE UPPER('%%%s%%') OR UPPER(b.bbm_agenid) LIKE UPPER('%%%s%%')) `, agenNama, agenNama)
	}

	if noMobil != "" {
		filterClause += fmt.Sprintf(` AND UPPER(b.bbm_kendid) LIKE UPPER('%%%s%%') `, noMobil)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			COALESCE(b.bbm_id, '-') AS bbm_id,
			TO_CHAR(b.bbm_date, 'YYYY-MM-DD HH24:MI:SS') AS bbm_date,
			COALESCE(b.bbm_kendid, '-') AS bbm_kendid,
			COALESCE(b.bbm_km, 0) AS bbm_km,
			COALESCE(b.bbm_isi, 0) AS bbm_isi,
			COALESCE(b.bbm_harga, 0) AS bbm_harga,
			COALESCE(b.bbm_voucherid, '-') AS bbm_voucherid,
			COALESCE(a.agen_nama, b.bbm_agenid, '-') AS agen_nama,
			COALESCE(b.bbm_jenis, '-') AS bbm_jenis,
			COALESCE(b.bbm_lokasipengisian, '-') AS bbm_lokasipengisian,
			COALESCE(b.bbm_aktifyn, 'Y') AS bbm_aktifyn,
			CASE WHEN COALESCE(b.bbm_aktifyn, 'Y') = 'Y' THEN 'YA' ELSE 'TIDAK' END AS aktifjd
		FROM public.opr_t_isibbm b
		LEFT OUTER JOIN public.glb_m_agen a ON CAST(b.bbm_agenid AS VARCHAR) = CAST(a.agen_id AS VARCHAR)
		WHERE 1=1 %s
		ORDER BY b.bbm_date DESC, b.bbm_id DESC
	`, filterClause)

	type RawResult struct {
		BBMID              string  `gorm:"column:bbm_id" json:"bbm_id"`
		BBMDate            string  `gorm:"column:bbm_date" json:"bbm_date"`
		BBMKendID          string  `gorm:"column:bbm_kendid" json:"bbm_kendid"`
		BBMKm              float64 `gorm:"column:bbm_km" json:"bbm_km"`
		BBMIsi             float64 `gorm:"column:bbm_isi" json:"bbm_isi"`
		BBMHarga           float64 `gorm:"column:bbm_harga" json:"bbm_harga"`
		BBMVoucherID       string  `gorm:"column:bbm_voucherid" json:"bbm_voucherid"`
		AgenNama           string  `gorm:"column:agen_nama" json:"agen_nama"`
		BBMJenis           string  `gorm:"column:bbm_jenis" json:"bbm_jenis"`
		BBMLokasiPengisian string  `gorm:"column:bbm_lokasipengisian" json:"bbm_lokasipengisian"`
		BBMAktifYN         string  `gorm:"column:bbm_aktifyn" json:"bbm_aktifyn"`
		AktifJD            string  `gorm:"column:aktifjd" json:"aktifjd"`
	}

	var rawList []RawResult
	if err := database.Raw(queryStr).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query isi BBM: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   rawList,
		"count":  len(rawList),
	})
}

// 💾 2. CREATE PENGISIAN BBM
func CreateIsiBBM(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTIsiBBM
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid: " + err.Error()})
		return
	}

	now := time.Now()
	if req.BBMDate == "" {
		req.BBMDate = now.Format("2006-01-02 15:04:05")
	}
	req.BBMTime = now.Format("15:04")
	req.BBMUpdateTime = now.Format("2006-01-02 15:04:05")
	req.BBMUpdateID = fmt.Sprintf("%v", username)
	req.BBMAktifYN = "Y"

	if req.BBMAgenID == "" {
		req.BBMAgenID = "1"
	}

	if err := database.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal merekam transaksi pengisian BBM: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Transaksi pengisian BBM berhasil direkam!",
		"data":    req,
	})
}

// ✏️ 3. UPDATE PENGISIAN BBM
func UpdateIsiBBM(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTIsiBBM
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	updateSQL := `
		UPDATE public.opr_t_isibbm 
		SET bbm_kendid = ?, 
		    bbm_km = ?, 
		    bbm_isi = ?, 
		    bbm_harga = ?, 
		    bbm_jenis = ?, 
		    bbm_voucherid = ?, 
		    bbm_lokasipengisian = ?, 
		    bbm_nipsopir1 = ?, 
		    bbm_updateid = ?, 
		    bbm_updatetime = ?
		WHERE bbm_id = ?
	`

	if err := database.Exec(
		updateSQL,
		req.BBMKendID,
		req.BBMKm,
		req.BBMIsi,
		req.BBMHarga,
		req.BBMJenis,
		req.BBMVoucherID,
		req.BBMLokasiPengisian,
		req.BBMNipSopir1,
		fmt.Sprintf("%v", username),
		now,
		req.BBMID,
	).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate data BBM: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data pengisian BBM berhasil diperbarui!",
	})
}

// 🗑️ 4. DELETE PENGISIAN BBM
func DeleteIsiBBM(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	bbmID := c.Query("bbm_id")
	if bbmID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter bbm_id wajib diisi"})
		return
	}

	deleteSQL := `DELETE FROM public.opr_t_isibbm WHERE bbm_id = ?`
	if err := database.Exec(deleteSQL, bbmID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus data BBM: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data pengisian BBM berhasil dihapus!",
	})
}
