package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// GetListUnloadBarang Handler untuk Menarik Daftar Pembongkaran Barang
func GetListUnloadBarang(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	noUnload := strings.TrimSpace(c.Query("nounload"))
	chkTgl := strings.TrimSpace(c.Query("chktgl"))
	chkNoUnload := strings.TrimSpace(c.Query("chknounload"))

	filterClause := ""

	if (chkTgl == "true" || chkTgl == "on") && tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(h.unloadh_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	if (chkNoUnload == "true" || chkNoUnload == "on") && noUnload != "" {
		filterClause += fmt.Sprintf(` AND UPPER(h.unloadh_id) LIKE UPPER('%%%s%%') `, noUnload)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			h.unloadh_id,
			COALESCE(h.unloadh_agenid, '-') AS unloadh_agenid,
			TO_CHAR(h.unloadh_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS unloadh_tanggal,
			COALESCE(h.unloadh_updateid, '-') AS unloadh_updateid,
			COALESCE(h.unloadh_approveyn, 'N') AS unloadh_approveyn,
			COUNT(d.unloadd_sptid) AS jml_sp
		FROM public.opr_t_unloadh h
		LEFT JOIN public.opr_t_unloadd d ON h.unloadh_id = d.unloadd_unloadhid
		WHERE 1=1 %s
		GROUP BY h.unloadh_id, h.unloadh_agenid, h.unloadh_tanggal, h.unloadh_updateid, h.unloadh_approveyn
		ORDER BY h.unloadh_tanggal DESC, h.unloadh_id DESC
	`, filterClause)

	type RawResult struct {
		UnloadHID        string `gorm:"column:unloadh_id"`
		UnloadHAgenID    string `gorm:"column:unloadh_agenid"`
		UnloadHTanggal   string `gorm:"column:unloadh_tanggal"`
		UnloadHUpdateID  string `gorm:"column:unloadh_updateid"`
		UnloadHApproveYN string `gorm:"column:unloadh_approveyn"`
		JmlSP            int    `gorm:"column:jml_sp"`
	}

	var rawList []RawResult
	if err := database.Raw(queryStr).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query pembongkaran barang: " + err.Error()})
		return
	}

	var resultList []models.UnloadBarangDTO
	for idx, r := range rawList {
		resultList = append(resultList, models.UnloadBarangDTO{
			No:               idx + 1,
			UnloadHID:        r.UnloadHID,
			UnloadHAgenID:    r.UnloadHAgenID,
			UnloadHTanggal:   r.UnloadHTanggal,
			UnloadHUpdateID:  r.UnloadHUpdateID,
			UnloadHApproveYN: r.UnloadHApproveYN,
			JmlSP:            r.JmlSP,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resultList,
		"count":  len(resultList),
	})
}

// CreateUnloadBarang Handler Tambah Manifest Pembongkaran Barang
func CreateUnloadBarang(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.UnloadBarangReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	if req.UnloadHAgenID == "" {
		req.UnloadHAgenID = "1"
	}
	if req.UnloadHApproveYN == "" {
		req.UnloadHApproveYN = "N"
	}

	insertSQL := `
		INSERT INTO public.opr_t_unloadh (unloadh_id, unloadh_agenid, unloadh_tanggal, unloadh_approveyn, unloadh_updateid)
		VALUES (?, ?, NOW(), ?, ?)
	`
	if err := database.Exec(insertSQL, req.UnloadHID, req.UnloadHAgenID, req.UnloadHApproveYN, fmt.Sprintf("%v", username)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menambah manifest pembongkaran: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Manifest Pembongkaran Barang berhasil ditambahkan"})
}

// UpdateUnloadBarang Handler Update Manifest Pembongkaran Barang
func UpdateUnloadBarang(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.UnloadBarangReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	updateSQL := `
		UPDATE public.opr_t_unloadh 
		SET unloadh_approveyn = ?, unloadh_updateid = ?, unloadh_updatetime = NOW() 
		WHERE unloadh_id = ?
	`
	if err := database.Exec(updateSQL, req.UnloadHApproveYN, fmt.Sprintf("%v", username), req.UnloadHID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui manifest pembongkaran: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Manifest Pembongkaran Barang berhasil diperbarui"})
}

// DeleteUnloadBarang Handler Hapus Manifest Pembongkaran Barang
func DeleteUnloadBarang(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	unloadID := c.Query("unload_id")

	deleteSQL := `DELETE FROM public.opr_t_unloadh WHERE unloadh_id = ?`
	if err := database.Exec(deleteSQL, unloadID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus manifest pembongkaran: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Manifest Pembongkaran Barang berhasil dihapus"})
}
