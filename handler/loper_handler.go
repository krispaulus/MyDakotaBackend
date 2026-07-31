package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// GetLoperList Handler untuk Menarik Daftar Manifes Loper
func GetLoperList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	loperID := strings.TrimSpace(c.Query("loper_id"))
	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))

	filterClause := ""
	if loperID != "" {
		filterClause += fmt.Sprintf(` AND UPPER(l.loper_eid) LIKE UPPER('%%%s%%') `, loperID)
	}
	if tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(l.loper_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			l.loper_eid,
			TO_CHAR(l.loper_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS loper_tanggal,
			COALESCE(l.loper_agenid, '-') AS loper_agenid,
			COALESCE(l.loper_nomobil, '-') AS loper_nomobil,
			COALESCE(l.loper_nipsopir, '-') AS loper_nipsopir,
			COALESCE(s.kry_nama, '-') AS nama_sopir,
			COALESCE(l.loper_keraniyn, 'N') AS loper_keraniyn,
			COALESCE(l.loper_nipkerani, '-') AS loper_nipkerani,
			COALESCE(k.kry_nama, '-') AS nama_kerani,
			COALESCE(l.loper_aktifyn, 'Y') AS loper_aktifyn,
			COALESCE(COUNT(d.loperd_bttid), 0) AS total_btt
		FROM public.opr_t_eloper l
		LEFT JOIN public.opr_t_eloperdetail d ON l.loper_eid = d.loperd_eloperid
		LEFT JOIN public.hrd_m_karyawan s ON l.loper_nipsopir = s.kry_nip
		LEFT JOIN public.hrd_m_karyawan k ON l.loper_nipkerani = k.kry_nip
		WHERE 1=1 %s
		GROUP BY l.loper_eid, l.loper_tanggal, l.loper_agenid, l.loper_nomobil, l.loper_nipsopir, s.kry_nama, l.loper_keraniyn, l.loper_nipkerani, k.kry_nama, l.loper_aktifyn
		ORDER BY l.loper_tanggal DESC, l.loper_eid DESC
	`, filterClause)

	type RawLoper struct {
		LoperEID       string `gorm:"column:loper_eid" json:"loper_eid"`
		LoperTanggal   string `gorm:"column:loper_tanggal" json:"loper_tanggal"`
		LoperAgenID    string `gorm:"column:loper_agenid" json:"loper_agenid"`
		LoperNoMobil   string `gorm:"column:loper_nomobil" json:"loper_nomobil"`
		LoperNIPSopir  string `gorm:"column:loper_nipsopir" json:"loper_nipsopir"`
		NamaSopir      string `gorm:"column:nama_sopir" json:"nama_sopir"`
		LoperKeraniYN  string `gorm:"column:loper_keraniyn" json:"loper_keraniyn"`
		LoperNIPKerani string `gorm:"column:loper_nipkerani" json:"loper_nipkerani"`
		NamaKerani     string `gorm:"column:nama_kerani" json:"nama_kerani"`
		LoperAktifYN   string `gorm:"column:loper_aktifyn" json:"loper_aktifyn"`
		TotalBTT       int    `gorm:"column:total_btt" json:"total_btt"`
	}

	var list []RawLoper
	if err := database.Raw(queryStr).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query list loper: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// CreateLoper Handler untuk Membuat Manifes Loper Baru
func CreateLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req struct {
		LoperEID       string `json:"loper_eid" binding:"required"`
		LoperAgenID    string `json:"loper_agenid"`
		LoperNoMobil   string `json:"loper_nomobil" binding:"required"`
		LoperNIPSopir  string `json:"loper_nipsopir" binding:"required"`
		LoperKeraniYN  string `json:"loper_keraniyn"`
		LoperNIPKerani string `json:"loper_nipkerani"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Data input tidak valid: " + err.Error()})
		return
	}

	insertSQL := `
		INSERT INTO public.opr_t_eloper (loper_eid, loper_agenid, loper_tanggal, loper_nomobil, loper_nipsopir, loper_keraniyn, loper_nipkerani, loper_aktifyn)
		VALUES (?, ?, NOW(), ?, ?, ?, ?, 'Y')
	`
	if err := database.Exec(insertSQL, req.LoperEID, req.LoperAgenID, req.LoperNoMobil, req.LoperNIPSopir, req.LoperKeraniYN, req.LoperNIPKerani).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan loper baru: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Manifes Loper berhasil dibuat"})
}
