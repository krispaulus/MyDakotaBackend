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

// 🔍 1. LIST DATA SURAT MUATAN UDARA (SMU)
func GetSMUList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	noSmu := strings.TrimSpace(c.Query("no_smu"))
	dari := strings.TrimSpace(c.Query("dari"))
	tujuan := strings.TrimSpace(c.Query("tujuan"))
	chkTgl := strings.TrimSpace(c.Query("chktgl"))

	filterClause := ""

	if (chkTgl == "true" || chkTgl == "on") && tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(s.smu_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	if noSmu != "" {
		filterClause += fmt.Sprintf(` AND UPPER(s.smu_no) LIKE UPPER('%%%s%%') `, noSmu)
	}

	if dari != "" {
		filterClause += fmt.Sprintf(` AND (UPPER(b1.airport_location) LIKE UPPER('%%%s%%') OR UPPER(s.smu_dari) LIKE UPPER('%%%s%%')) `, dari, dari)
	}

	if tujuan != "" {
		filterClause += fmt.Sprintf(` AND (UPPER(b2.airport_location) LIKE UPPER('%%%s%%') OR UPPER(s.smu_tujuan) LIKE UPPER('%%%s%%')) `, tujuan, tujuan)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			COALESCE(s.smu_no, '-') AS smu_no,
			TO_CHAR(s.smu_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS smu_tanggal,
			COALESCE(s.smu_dari, '-') AS smu_dari,
			COALESCE(b1.airport_location, s.smu_dari, '-') AS bandara_dari,
			COALESCE(s.smu_tujuan, '-') AS smu_tujuan,
			COALESCE(b2.airport_location, s.smu_tujuan, '-') AS bandara_tujuan,
			COALESCE(s.smu_kepada, '-') AS smu_kepada,
			COALESCE(s.smu_maskapai, '-') AS smu_maskapai,
			COALESCE(s.smu_nomorpenerbangan, '-') AS smu_nomorpenerbangan,
			COALESCE(s.smu_colly, 0) AS smu_colly,
			COALESCE(s.smu_berat, 0) AS smu_berat,
			COALESCE(s.smu_harga, 0) AS smu_harga,
			COALESCE(s.smu_updateid, '-') AS smu_updateid,
			COALESCE(s.smu_aktifyn, 'Y') AS smu_aktifyn,
			CASE WHEN COALESCE(s.smu_aktifyn, 'Y') = 'Y' THEN 'Ya' ELSE 'Tidak' END AS aktifjd
		FROM public.opr_t_suratmuatanudara s
		LEFT OUTER JOIN public.glb_m_bandara b1 ON CAST(s.smu_dari AS VARCHAR) = CAST(b1.iata_code AS VARCHAR)
		LEFT OUTER JOIN public.glb_m_bandara b2 ON CAST(s.smu_tujuan AS VARCHAR) = CAST(b2.iata_code AS VARCHAR)
		WHERE 1=1 %s
		ORDER BY s.smu_tanggal DESC, s.smu_no DESC
	`, filterClause)

	type RawResult struct {
		SMUNo               string  `gorm:"column:smu_no" json:"smu_no"`
		SMUTanggal          string  `gorm:"column:smu_tanggal" json:"smu_tanggal"`
		SMUDari             string  `gorm:"column:smu_dari" json:"smu_dari"`
		BandaraDari         string  `gorm:"column:bandara_dari" json:"bandara_dari"`
		SMUTujuan           string  `gorm:"column:smu_tujuan" json:"smu_tujuan"`
		BandaraTujuan       string  `gorm:"column:bandara_tujuan" json:"bandara_tujuan"`
		SMUKepada           string  `gorm:"column:smu_kepada" json:"smu_kepada"`
		SMUMaskapai         string  `gorm:"column:smu_maskapai" json:"smu_maskapai"`
		SMUNomorPenerbangan string  `gorm:"column:smu_nomorpenerbangan" json:"smu_nomorpenerbangan"`
		SMUColly            int     `gorm:"column:smu_colly" json:"smu_colly"`
		SMUBerat            float64 `gorm:"column:smu_berat" json:"smu_berat"`
		SMUHarga            float64 `gorm:"column:smu_harga" json:"smu_harga"`
		SMUUpdateID         string  `gorm:"column:smu_updateid" json:"smu_updateid"`
		SMUAktifYN          string  `gorm:"column:smu_aktifyn" json:"smu_aktifyn"`
		AktifJD             string  `gorm:"column:aktifjd" json:"aktifjd"`
	}

	var rawList []RawResult
	if err := database.Raw(queryStr).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query SMU: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   rawList,
		"count":  len(rawList),
	})
}

// 💾 2. CREATE SURAT MUATAN UDARA
func CreateSMU(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTSuratMuatanUdara
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid: " + err.Error()})
		return
	}

	now := time.Now()
	if req.SMUTanggal == "" {
		req.SMUTanggal = now.Format("2006-01-02 15:04:05")
	}
	req.SMUUpdateID = fmt.Sprintf("%v", username)
	req.SMUAktifYN = "Y"
	req.SMUTerimaYN = "N"

	if err := database.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal merekam Surat Muatan Udara: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Surat Muatan Udara berhasil direkam!",
		"data":    req,
	})
}

// ✏️ 3. UPDATE SURAT MUATAN UDARA
func UpdateSMU(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTSuratMuatanUdara
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	updateSQL := `
		UPDATE public.opr_t_suratmuatanudara 
		SET smu_dari = ?, 
		    smu_tujuan = ?, 
		    smu_kepada = ?, 
		    smu_maskapai = ?, 
		    smu_nomorpenerbangan = ?, 
		    smu_colly = ?, 
		    smu_berat = ?, 
		    smu_harga = ?, 
		    smu_updateid = ?
		WHERE smu_no = ?
	`

	if err := database.Exec(
		updateSQL,
		req.SMUDari, req.SMUTujuan, req.SMUKepada, req.SMUMaskapai,
		req.SMUNomorPenerbangan, req.SMUColly, req.SMUBerat, req.SMUHarga,
		fmt.Sprintf("%v", username), req.SMUNo,
	).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate SMU: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Surat Muatan Udara berhasil diperbarui!",
	})
}

// 🗑️ 4. DELETE SURAT MUATAN UDARA
func DeleteSMU(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	smuNo := c.Query("smu_no")
	if smuNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter smu_no wajib diisi"})
		return
	}

	deleteSQL := `DELETE FROM public.opr_t_suratmuatanudara WHERE smu_no = ?`
	if err := database.Exec(deleteSQL, smuNo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus SMU: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Surat Muatan Udara berhasil dihapus!",
	})
}
