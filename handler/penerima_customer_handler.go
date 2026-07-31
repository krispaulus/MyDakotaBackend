package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// GetLaporanDataPenerimaCustomer Handler Laporan Data Penerima Customer
func GetLaporanDataPenerimaCustomer(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database gagal"})
		return
	}

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	custID := strings.TrimSpace(c.Query("cust_id"))
	namaCabang := strings.TrimSpace(c.Query("cabang"))

	if tgla == "" {
		tgla = "2017-01-01"
	}
	if tgle == "" {
		tgle = "2026-12-31"
	}

	filterClause := ""

	// Filter Cabang (Hanya aktif jika spesifik memilih cabang tertentu)
	if namaCabang != "" && strings.ToUpper(namaCabang) != "SEMUA" && !strings.Contains(strings.ToUpper(namaCabang), "SEMUA CABANG") {
		cleanCabang := strings.TrimSpace(strings.ReplaceAll(namaCabang, "(HOLDING)", ""))
		filterClause += fmt.Sprintf(` AND (UPPER(a.agen_nama) LIKE UPPER('%%%s%%') OR CAST(e.bttt_asalagenid AS VARCHAR) LIKE '%%%s%%') `, cleanCabang, cleanCabang)
	}

	// Filter Customer ID
	if custID != "" && strings.ToUpper(custID) != "SEMUA" {
		filterClause += fmt.Sprintf(` AND UPPER(e.bttt_asalcustid) = UPPER('%s') `, custID)
	}

	// Query Utama
	queryStr := fmt.Sprintf(`
		SELECT 
			COALESCE(e.bttt_asalcustid, '') AS cust_id,
			COALESCE(c.cust_name, e.bttt_asalname, '-') AS cust_name,
			COALESCE(e.bttt_tujuannama, '-') AS bttt_tujuan_nama,
			COALESCE(e.bttt_tujuanalamat, '-') AS bttt_tujuan_alamat,
			COALESCE(e.bttt_updateid, '-') AS bttt_update_id,
			TO_CHAR(e.bttt_tanggal, 'YYYY-MM-DD') AS bttt_tanggal
		FROM public.mkt_t_econote e
		LEFT JOIN public.mkt_m_customer c ON e.bttt_asalcustid = c.cust_id
		LEFT JOIN public.glb_m_agen a ON CAST(e.bttt_asalagenid AS VARCHAR) = CAST(a.agen_id AS VARCHAR)
		WHERE e.bttt_aktifyn = 'Y'
		  AND CAST(e.bttt_tanggal AS DATE) BETWEEN '%s' AND '%s'
		  %s
		ORDER BY c.cust_name ASC, e.bttt_tanggal DESC
	`, tgla, tgle, filterClause)

	type RawResult struct {
		CustID           string `gorm:"column:cust_id"`
		CustName         string `gorm:"column:cust_name"`
		BTTTTujuanNama   string `gorm:"column:bttt_tujuan_nama"`
		BTTTTujuanAlamat string `gorm:"column:bttt_tujuan_alamat"`
		BTTTUpdateID     string `gorm:"column:bttt_update_id"`
		BTTTTanggal      string `gorm:"column:bttt_tanggal"`
	}

	var rawList []RawResult
	if err := database.Raw(queryStr).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query data penerima customer: " + err.Error()})
		return
	}

	var resultList []models.PenerimaCustomerDTO
	for idx, r := range rawList {
		resultList = append(resultList, models.PenerimaCustomerDTO{
			No:               idx + 1,
			CustID:           r.CustID,
			CustName:         r.CustName,
			BTTTTujuanNama:   r.BTTTTujuanNama,
			BTTTTujuanAlamat: r.BTTTTujuanAlamat,
			BTTTUpdateID:     r.BTTTUpdateID,
			BTTTTanggal:      r.BTTTTanggal,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resultList,
		"count":  len(resultList),
	})
}
