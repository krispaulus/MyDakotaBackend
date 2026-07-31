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

// 🔍 1. GET LIST SURAT TUGAS / SURAT JALAN
func GetSuratTugasList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	noSt := strings.TrimSpace(c.Query("no_st"))
	noMobil := strings.TrimSpace(c.Query("no_mobil"))
	sopirNama := strings.TrimSpace(c.Query("sopir_nama"))
	chkTgl := strings.TrimSpace(c.Query("chktgl"))

	filterClause := ""

	if (chkTgl == "true" || chkTgl == "on") && tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(s.sjh_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	if noSt != "" {
		filterClause += fmt.Sprintf(` AND UPPER(s.sjh_id) LIKE UPPER('%%%s%%') `, noSt)
	}

	if noMobil != "" {
		filterClause += fmt.Sprintf(` AND UPPER(s.sjh_kendid) LIKE UPPER('%%%s%%') `, noMobil)
	}

	if sopirNama != "" {
		filterClause += fmt.Sprintf(` AND (UPPER(sp1.kry_nama) LIKE UPPER('%%%s%%') OR UPPER(sp2.kry_nama) LIKE UPPER('%%%s%%') OR UPPER(s.sjh_sopir1_nip) LIKE UPPER('%%%s%%')) `, sopirNama, sopirNama, sopirNama)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			COALESCE(s.sjh_id, '-') AS sjh_id,
			TO_CHAR(s.sjh_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS sjh_tanggal,
			TO_CHAR(s.sjh_tanggalkembali, 'YYYY-MM-DD HH24:MI:SS') AS sjh_tanggalkembali,
			COALESCE(s.sjh_kendid, '-') AS sjh_kendid,
			COALESCE(s.sjh_sopir1_nip, '-') AS sjh_sopir1_nip,
			COALESCE(s.sjh_sopir2_nip, '-') AS sjh_sopir2_nip,
			COALESCE(sp1.kry_nama, s.sjh_sopir1_nip, '-') AS nmsopir1,
			COALESCE(sp2.kry_nama, s.sjh_sopir2_nip, '-') AS nmsopir2,
			COALESCE(s.sjh_assid, '-') AS sjh_assid,
			COALESCE(a.ass_nama, s.sjh_assid, '-') AS ass_nama,
			COALESCE(cStart.agen_nama, s.sjh_startagenid, '-') AS cbgmulai,
			COALESCE(cEnd.agen_nama, s.sjh_endagenid, '-') AS cbgselesai,
			COALESCE(s.sjh_keterangan, '-') AS sjh_keterangan,
			COALESCE(s.sjh_completeyn, 'N') AS sjh_completeyn,
			COALESCE(s.sjh_approve, '') AS sjh_approve,
			COALESCE(s.sjh_aktifyn, 'Y') AS sjh_aktifyn,
			COALESCE(r.sjr_nominalum, r.sjr_nominal, 0) AS nominal_um
		FROM public.opr_t_esuratjalan s
		LEFT OUTER JOIN public.hrd_m_karyawan sp1 ON CAST(s.sjh_sopir1_nip AS VARCHAR) = CAST(sp1.kry_nip AS VARCHAR)
		LEFT OUTER JOIN public.hrd_m_karyawan sp2 ON CAST(s.sjh_sopir2_nip AS VARCHAR) = CAST(sp2.kry_nip AS VARCHAR)
		LEFT OUTER JOIN public.glb_m_agen cStart ON CAST(s.sjh_startagenid AS VARCHAR) = CAST(cStart.agen_id AS VARCHAR)
		LEFT OUTER JOIN public.glb_m_agen cEnd ON CAST(s.sjh_endagenid AS VARCHAR) = CAST(cEnd.agen_id AS VARCHAR)
		LEFT OUTER JOIN public.opr_m_assignment a ON CAST(s.sjh_assid AS VARCHAR) = CAST(a.ass_id AS VARCHAR)
		LEFT OUTER JOIN public.opr_t_esuratjalanrealisasi r ON s.sjh_id = r.sjr_sjhid
		WHERE 1=1 %s
		ORDER BY s.sjh_tanggal DESC, s.sjh_id DESC
	`, filterClause)

	type RawResult struct {
		SJHID             string  `gorm:"column:sjh_id" json:"sjh_id"`
		SJHTanggal        string  `gorm:"column:sjh_tanggal" json:"sjh_tanggal"`
		SJHTanggalKembali string  `gorm:"column:sjh_tanggalkembali" json:"sjh_tanggalkembali"`
		SJHKendID         string  `gorm:"column:sjh_kendid" json:"sjh_kendid"`
		SJHSopir1Nip      string  `gorm:"column:sjh_sopir1_nip" json:"sjh_sopir1_nip"`
		SJHSopir2Nip      string  `gorm:"column:sjh_sopir2_nip" json:"sjh_sopir2_nip"`
		NmSopir1          string  `gorm:"column:nmsopir1" json:"nmsopir1"`
		NmSopir2          string  `gorm:"column:nmsopir2" json:"nmsopir2"`
		SJHAssID          string  `gorm:"column:sjh_assid" json:"sjh_assid"`
		AssNama           string  `gorm:"column:ass_nama" json:"ass_nama"`
		CbgMulai          string  `gorm:"column:cbgmulai" json:"cbgmulai"`
		CbgSelesai        string  `gorm:"column:cbgselesai" json:"cbgselesai"`
		SJHKeterangan     string  `gorm:"column:sjh_keterangan" json:"sjh_keterangan"`
		SJHCompleteYN     string  `gorm:"column:sjh_completeyn" json:"sjh_completeyn"`
		SJHApprove        string  `gorm:"column:sjh_approve" json:"sjh_approve"`
		SJHAktifYN        string  `gorm:"column:sjh_aktifyn" json:"sjh_aktifyn"`
		NominalUM         float64 `gorm:"column:nominal_um" json:"nominal_um"`
	}

	var rawList []RawResult
	if err := database.Raw(queryStr).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query Surat Tugas: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   rawList,
		"count":  len(rawList),
	})
}

// 💾 2. CREATE SURAT TUGAS / SURAT JALAN
func CreateSuratTugas(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.SuratTugasReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input data tidak valid: " + err.Error()})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	if req.SJHTanggal == "" {
		req.SJHTanggal = now
	}

	insertHeader := `
		INSERT INTO public.opr_t_esuratjalan (
			sjh_id, sjh_tanggal, sjh_tanggalkembali, sjh_kendid, sjh_sopir1_nip, sjh_sopir2_nip,
			sjh_assid, sjh_trayekid, sjh_startagenid, sjh_endagenid, sjh_keterangan,
			sjh_completeyn, sjh_aktifyn, sjh_updateid, sjh_updatetime
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'N', 'Y', ?, ?)
	`

	if err := database.Exec(
		insertHeader,
		req.SJHID, req.SJHTanggal, req.SJHTanggalKembali, req.SJHKendID, req.SJHSopir1Nip, req.SJHSopir2Nip,
		req.SJHAssID, req.SJHTrayekID, req.SJHStartAgenID, req.SJHEndAgenID, req.SJHKeterangan,
		fmt.Sprintf("%v", username), now,
	).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan header Surat Tugas: " + err.Error()})
		return
	}

	if req.NominalUM > 0 {
		insertUM := `
			INSERT INTO public.opr_t_esuratjalanrealisasi (sjr_sjhid, sjr_ket, sjr_nominal, sjr_nominalum)
			VALUES (?, 'UANG MUKA OPERASIONAL', ?, ?)
		`
		database.Exec(insertUM, req.SJHID, req.NominalUM, req.NominalUM)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Surat Tugas/Jalan berhasil direkam!",
	})
}

// ✏️ 3. UPDATE SURAT TUGAS
func UpdateSuratTugas(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.SuratTugasReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	updateHeader := `
		UPDATE public.opr_t_esuratjalan 
		SET sjh_kendid = ?, 
		    sjh_sopir1_nip = ?, 
		    sjh_sopir2_nip = ?, 
		    sjh_assid = ?, 
		    sjh_startagenid = ?, 
		    sjh_endagenid = ?, 
		    sjh_keterangan = ?, 
		    sjh_updateid = ?, 
		    sjh_updatetime = ?
		WHERE sjh_id = ?
	`

	if err := database.Exec(
		updateHeader,
		req.SJHKendID, req.SJHSopir1Nip, req.SJHSopir2Nip, req.SJHAssID,
		req.SJHStartAgenID, req.SJHEndAgenID, req.SJHKeterangan,
		fmt.Sprintf("%v", username), now, req.SJHID,
	).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui Surat Tugas: " + err.Error()})
		return
	}

	database.Exec(`DELETE FROM public.opr_t_esuratjalanrealisasi WHERE sjr_sjhid = ?`, req.SJHID)
	if req.NominalUM > 0 {
		insertUM := `
			INSERT INTO public.opr_t_esuratjalanrealisasi (sjr_sjhid, sjr_ket, sjr_nominal, sjr_nominalum)
			VALUES (?, 'UANG MUKA OPERASIONAL', ?, ?)
		`
		database.Exec(insertUM, req.SJHID, req.NominalUM, req.NominalUM)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Surat Tugas berhasil diperbarui!",
	})
}

// 🗑️ 4. DELETE SURAT TUGAS
func DeleteSuratTugas(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	stID := c.Query("sjh_id")
	if stID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter sjh_id wajib diisi"})
		return
	}

	database.Exec(`DELETE FROM public.opr_t_esuratjalandetil WHERE sjd_sjhid = ?`, stID)
	database.Exec(`DELETE FROM public.opr_t_esuratjalanrealisasi WHERE sjr_sjhid = ?`, stID)
	if err := database.Exec(`DELETE FROM public.opr_t_esuratjalan WHERE sjh_id = ?`, stID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus Surat Tugas: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Surat Tugas berhasil dihapus!",
	})
}
