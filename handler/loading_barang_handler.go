package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// GetListLoadingBarang Handler untuk Mengambil Daftar Loading Barang
func GetListLoadingBarang(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	noLoad := strings.TrimSpace(c.Query("noload"))
	noMobil := strings.TrimSpace(c.Query("nomobil"))
	ckTgl := strings.TrimSpace(c.Query("cktgl"))
	ckNoLoad := strings.TrimSpace(c.Query("cknoload"))
	ckNoMobil := strings.TrimSpace(c.Query("cknomobil"))

	filterClause := ""

	if (ckTgl == "true" || ckTgl == "on") && tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(h.loadh_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	if (ckNoLoad == "true" || ckNoLoad == "on") && noLoad != "" {
		filterClause += fmt.Sprintf(` AND UPPER(h.loadh_id) LIKE UPPER('%%%s%%') `, noLoad)
	}

	if (ckNoMobil == "true" || ckNoMobil == "on") && noMobil != "" {
		filterClause += fmt.Sprintf(` AND UPPER(h.loadh_kendid) LIKE UPPER('%%%s%%') `, noMobil)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			h.loadh_id,
			TO_CHAR(h.loadh_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS loadh_tanggal,
			COALESCE(h.loadh_kendid, '-') AS loadh_kendid,
			COALESCE(h.loadh_updateid, '-') AS loadh_updateid,
			COALESCE(h.loadh_approveyn, 'N') AS loadh_approveyn,
			COUNT(d.loadd_bttkoliid) AS jml_barang
		FROM public.opr_t_loadingh h
		LEFT JOIN public.opr_t_loadingd d ON h.loadh_id = d.loadd_loadhid
		WHERE 1=1 %s
		GROUP BY h.loadh_id, h.loadh_tanggal, h.loadh_kendid, h.loadh_updateid, h.loadh_approveyn
		ORDER BY h.loadh_tanggal DESC, h.loadh_id DESC
	`, filterClause)

	type RawResult struct {
		LoadHID        string `gorm:"column:loadh_id"`
		LoadHTanggal   string `gorm:"column:loadh_tanggal"`
		LoadHKendID    string `gorm:"column:loadh_kendid"`
		LoadHUpdateID  string `gorm:"column:loadh_updateid"`
		LoadHApproveYN string `gorm:"column:loadh_approveyn"`
		JmlBarang      int    `gorm:"column:jml_barang"`
	}

	var rawList []RawResult
	if err := database.Raw(queryStr).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query loading barang: " + err.Error()})
		return
	}

	var resultList []models.LoadingBarangDTO
	for idx, r := range rawList {
		resultList = append(resultList, models.LoadingBarangDTO{
			No:             idx + 1,
			LoadHID:        r.LoadHID,
			LoadHTanggal:   r.LoadHTanggal,
			LoadHKendID:    r.LoadHKendID,
			LoadHUpdateID:  r.LoadHUpdateID,
			LoadHApproveYN: r.LoadHApproveYN,
			JmlBarang:      r.JmlBarang,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resultList,
		"count":  len(resultList),
	})
}
