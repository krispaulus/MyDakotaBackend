package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// GetLoperDeadlineList Handler untuk Menarik Daftar BTT Outstanding Melewati Tengat Waktu
func GetLoperDeadlineList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	nipNama := strings.TrimSpace(c.Query("nipnama"))
	noMobil := strings.TrimSpace(c.Query("nomobil"))
	chkTgl := strings.TrimSpace(c.Query("chktgl"))

	filterClause := ""

	// Filter tanggal jika checkbox dicentang
	if (chkTgl == "true" || chkTgl == "on") && tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(l.loper_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	// Filter NIP atau Nama Sopir/Kerani
	if nipNama != "" {
		filterClause += fmt.Sprintf(` AND (
			UPPER(l.loper_nipsopir) LIKE UPPER('%%%s%%') OR 
			UPPER(s.kry_nama) LIKE UPPER('%%%s%%') OR 
			UPPER(l.loper_nipkerani) LIKE UPPER('%%%s%%') OR 
			UPPER(k.kry_nama) LIKE UPPER('%%%s%%')
		) `, nipNama, nipNama, nipNama, nipNama)
	}

	// Filter Nomor Mobil Armada
	if noMobil != "" {
		filterClause += fmt.Sprintf(` AND UPPER(l.loper_nomobil) LIKE UPPER('%%%s%%') `, noMobil)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			l.loper_eid AS loper_id,
			ld.loperd_bttid AS btt_id,
			TO_CHAR(l.loper_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS loper_tanggal,
			COALESCE(l.loper_nipsopir, '-') AS nip_sopir,
			COALESCE(s.kry_nama, '-') AS nama_sopir,
			COALESCE(l.loper_nipkerani, '-') AS nip_kerani,
			COALESCE(k.kry_nama, '-') AS nama_kerani,
			COALESCE(l.loper_nomobil, '-') AS no_mobil
		FROM public.opr_t_eloper l
		INNER JOIN public.opr_t_eloperdetail ld ON l.loper_eid = ld.loperd_eloperid
		LEFT JOIN public.hrd_m_karyawan s ON l.loper_nipsopir = s.kry_nip
		LEFT JOIN public.hrd_m_karyawan k ON l.loper_nipkerani = k.kry_nip
		LEFT JOIN public.opr_t_ebbl b ON ld.loperd_bttid = b.bbl_bttid AND l.loper_eid = b.bbl_noloper
		WHERE l.loper_aktifyn = 'Y' 
		  AND b.bbl_eid IS NULL 
		  AND l.loper_tanggal <= NOW()
		  %s
		ORDER BY l.loper_tanggal DESC, l.loper_eid DESC
	`, filterClause)

	var list []models.LoperDeadlineDTO
	if err := database.Raw(queryStr).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query data loper deadline: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
		"count":  len(list),
	})
}

// ProcessLoperDeadlineAction Handler untuk Eksekusi Approval BTT Deadline (Terima / Tolak)
func ProcessLoperDeadlineAction(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.LoperDeadlineActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload input tidak valid: " + err.Error()})
		return
	}

	req.Status = strings.ToUpper(req.Status)
	if req.Status != "Y" && req.Status != "N" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Status harus 'Y' (Terima) atau 'N' (Tolak)"})
		return
	}

	// Generate BBL ID unik
	bblID := fmt.Sprintf("BBL-%s-%s", req.LoperID, req.BttID)

	insertBBLSQL := `
		INSERT INTO public.opr_t_ebbl (
			bbl_eid, bbl_noloper, bbl_bttid, bbl_terimayn, bbl_tanggal, bbl_updateid
		) VALUES (?, ?, ?, ?, NOW(), ?)
		ON CONFLICT (bbl_eid) DO UPDATE 
		SET bbl_terimayn = EXCLUDED.bbl_terimayn, 
		    bbl_tanggal = NOW(), 
		    bbl_updateid = EXCLUDED.bbl_updateid;
	`

	if err := database.Exec(insertBBLSQL, bblID, req.LoperID, req.BttID, req.Status, fmt.Sprintf("%v", username)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memproses status BTT deadline: " + err.Error()})
		return
	}

	statusText := "Diterima"
	if req.Status == "N" {
		statusText = "Ditolak"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("BTT %s berhasil %s", req.BttID, statusText),
	})
}

// CreateLoperDeadline Handler Tambah Record BTT Deadline Manual
func CreateLoperDeadline(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.LoperDeadlineDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	insertSQL := `
		INSERT INTO public.opr_t_eloperdetail (loperd_eloperid, loperd_bttid)
		VALUES (?, ?)
	`
	if err := database.Exec(insertSQL, req.LoperID, req.BttID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menambah BTT deadline: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Record BTT Deadline berhasil ditambahkan"})
}

// UpdateLoperDeadline Handler Update Record BTT Deadline
func UpdateLoperDeadline(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.LoperDeadlineDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	updateSQL := `
		UPDATE public.opr_t_eloperdetail 
		SET loperd_bttid = ? 
		WHERE loperd_eloperid = ?
	`
	if err := database.Exec(updateSQL, req.BttID, req.LoperID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update BTT deadline: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Record BTT Deadline berhasil diperbarui"})
}

// DeleteLoperDeadline Handler Hapus Record BTT Deadline
func DeleteLoperDeadline(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	loperID := c.Query("loper_id")
	bttID := c.Query("btt_id")

	deleteSQL := `
		DELETE FROM public.opr_t_eloperdetail 
		WHERE loperd_eloperid = ? AND loperd_bttid = ?
	`
	if err := database.Exec(deleteSQL, loperID, bttID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus BTT deadline: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Record BTT Deadline berhasil dihapus"})
}
