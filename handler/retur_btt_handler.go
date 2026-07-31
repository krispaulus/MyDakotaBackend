package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 🔍 GET LIST RETUR BTT / BARANG BERMASALAH
func GetListReturBTT(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	noRetur := strings.TrimSpace(c.Query("no_retur"))
	agenNama := strings.TrimSpace(c.Query("agen_nama"))
	noBtt := strings.TrimSpace(c.Query("no_btt"))
	chkTgl := strings.TrimSpace(c.Query("chktgl"))

	filterClause := ""

	if (chkTgl == "true" || chkTgl == "on") && tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(r.rb_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	if noRetur != "" {
		filterClause += fmt.Sprintf(` AND UPPER(r.rb_eid) LIKE UPPER('%%%s%%') `, noRetur)
	}

	if agenNama != "" {
		filterClause += fmt.Sprintf(` AND (UPPER(a.agen_nama) LIKE UPPER('%%%s%%') OR UPPER(r.rb_tujuanagenid) LIKE UPPER('%%%s%%')) `, agenNama, agenNama)
	}

	if noBtt != "" {
		filterClause += fmt.Sprintf(` AND UPPER(d.rbd_bttid) LIKE UPPER('%%%s%%') `, noBtt)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			COALESCE(r.rb_eid, '-') AS rb_eid,
			TO_CHAR(r.rb_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS rb_tanggal,
			COALESCE(r.rb_agenid, '-') AS rb_agen_id,
			COALESCE(r.rb_tujuanagenid, '-') AS rb_tujuan_agen_id,
			COALESCE(a.agen_nama, r.rb_tujuanagenid, '-') AS agen_nama,
			COALESCE(r.rb_updateid, '-') AS rb_update_id,
			COALESCE(r.rb_aktifyn, 'Y') AS rb_aktif_yn,
			CASE WHEN COALESCE(r.rb_aktifyn, 'Y') = 'Y' THEN 'Ya' ELSE 'Tidak' END AS aktifjd
		FROM public.opr_t_ereturbtt r
		LEFT OUTER JOIN public.glb_m_agen a ON CAST(r.rb_tujuanagenid AS VARCHAR) = CAST(a.agen_id AS VARCHAR)
		LEFT OUTER JOIN public.opr_t_ereturbttdetil d ON r.rb_eid = d.rbd_rbeid
		WHERE 1=1 %s
		GROUP BY r.rb_eid, r.rb_tanggal, r.rb_agenid, r.rb_tujuanagenid, a.agen_nama, r.rb_updateid, r.rb_aktifyn
		ORDER BY r.rb_tanggal DESC, r.rb_eid DESC
	`, filterClause)

	type RawResult struct {
		RBEid          string `gorm:"column:rb_eid" json:"rb_eid"`
		RBTanggal      string `gorm:"column:rb_tanggal" json:"rb_tanggal"`
		RBAgenID       string `gorm:"column:rb_agen_id" json:"rb_agen_id"`
		RBTujuanAgenID string `gorm:"column:rb_tujuan_agen_id" json:"rb_tujuan_agen_id"`
		AgenNama       string `gorm:"column:agen_nama" json:"agen_nama"`
		RBUpdateID     string `gorm:"column:rb_update_id" json:"rb_update_id"`
		RBAktifYN      string `gorm:"column:rb_aktif_yn" json:"rb_aktif_yn"`
		AktifJD        string `gorm:"column:aktifjd" json:"aktifjd"`
	}

	var rawList []RawResult
	if err := database.Raw(queryStr).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query retur BTT: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   rawList,
		"count":  len(rawList),
	})
}

// 💾 CREATE RETUR BTT
func CreateReturBTT(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.ReturBTTReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	insertHeader := `
		INSERT INTO public.opr_t_ereturbtt (rb_eid, rb_tanggal, rb_agenid, rb_tujuanagenid, rb_aktifyn, rb_updateid)
		VALUES (?, NOW(), '1', ?, 'Y', ?)
	`
	if err := database.Exec(insertHeader, req.RBEid, req.RBTujuanAgenID, fmt.Sprintf("%v", username)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan header retur BTT: " + err.Error()})
		return
	}

	insertDetail := `INSERT INTO public.opr_t_ereturbttdetil (rbd_rbeid, rbd_bttid, rbd_keterangan, rbd_returyn, rbd_updateid) VALUES (?, ?, ?, 'Y', ?)`
	for _, btt := range req.ListBttID {
		if strings.TrimSpace(btt) != "" {
			database.Exec(insertDetail, req.RBEid, strings.TrimSpace(btt), req.Keterangan, fmt.Sprintf("%v", username))
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Retur BTT berhasil direkam!"})
}

// ✏️ UPDATE RETUR BTT
func UpdateReturBTT(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.ReturBTTReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	updateHeader := `
		UPDATE public.opr_t_ereturbtt 
		SET rb_tujuanagenid = ?, rb_updateid = ?, rb_updatetime = NOW() 
		WHERE rb_eid = ?
	`
	if err := database.Exec(updateHeader, req.RBTujuanAgenID, fmt.Sprintf("%v", username), req.RBEid).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui retur BTT: " + err.Error()})
		return
	}

	database.Exec(`DELETE FROM public.opr_t_ereturbttdetil WHERE rbd_rbeid = ?`, req.RBEid)
	insertDetail := `INSERT INTO public.opr_t_ereturbttdetil (rbd_rbeid, rbd_bttid, rbd_keterangan, rbd_returyn, rbd_updateid) VALUES (?, ?, ?, 'Y', ?)`
	for _, btt := range req.ListBttID {
		if strings.TrimSpace(btt) != "" {
			database.Exec(insertDetail, req.RBEid, strings.TrimSpace(btt), req.Keterangan, fmt.Sprintf("%v", username))
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data Retur BTT berhasil diperbarui!"})
}

// 🗑️ DELETE RETUR BTT
func DeleteReturBTT(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	rbEid := c.Query("rb_eid")
	if rbEid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter rb_eid wajib diisi"})
		return
	}

	database.Exec(`DELETE FROM public.opr_t_ereturbttdetil WHERE rbd_rbeid = ?`, rbEid)
	if err := database.Exec(`DELETE FROM public.opr_t_ereturbtt WHERE rb_eid = ?`, rbEid).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus retur BTT: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Retur BTT berhasil dihapus!"})
}
