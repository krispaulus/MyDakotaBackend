package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AgingPiutangRow struct {
	CustID        string  `json:"cust_id"`
	CustName      string  `json:"cust_name"`
	CabangNama    string  `json:"cabang_nama"`
	NoInvoice     string  `json:"no_invoice"`
	TglInvoice    string  `json:"tgl_invoice"`
	TglJatuhTempo string  `json:"tgl_jatuh_tempo"`
	TotalTagihan  float64 `json:"total_tagihan"`
	TotalBayar    float64 `json:"total_bayar"`
	SisaPiutang   float64 `json:"sisa_piutang"`
	UmurHari      int     `json:"umur_hari"`
	BucketCurrent float64 `json:"bucket_current"` // 0 - 30 hari
	Bucket31_60   float64 `json:"bucket_31_60"`   // 31 - 60 hari
	Bucket61_90   float64 `json:"bucket_61_90"`   // 61 - 90 hari
	BucketOver90  float64 `json:"bucket_over_90"` // > 90 hari
}

// GET /api/piutang/aging
func GetAgingPiutangHandler(c *gin.Context) {
	dbVal, exists := c.Get("db_corp")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	database := dbVal.(*gorm.DB)

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	cabangID := c.Query("cabang_id")
	custID := c.Query("cust_id")
	bypassTanggal := c.Query("bypass_tanggal") == "true" || c.Query("bypass_tanggal") == "1"

	// Query Dasar Mengambil Saldo Piutang Invoice/BTT dari tabel AR (art_t_piutangusaha)
	query := database.Table("public.art_t_piutangusaha pu").
		Select(`
			pu.pu_custid AS cust_id,
			COALESCE(c.cust_name, pu.pu_custid) AS cust_name,
			COALESCE(a.agen_nama, 'DLI PUSAT') AS cabang_nama,
			pu.pu_notrans AS no_invoice,
			TO_CHAR(pu.pu_tanggal, 'YYYY-MM-DD') AS tgl_invoice,
			TO_CHAR(COALESCE(pu.pu_tgljatuhtempo, pu.pu_tanggal + INTERVAL '30 days'), 'YYYY-MM-DD') AS tgl_jatuh_tempo,
			COALESCE(pu.pu_total, 0) AS total_tagihan,
			COALESCE(pu.pu_terbayar, 0) AS total_bayar,
			(COALESCE(pu.pu_total, 0) - COALESCE(pu.pu_terbayar, 0)) AS sisa_piutang,
			CURRENT_DATE - COALESCE(pu.pu_tgljatuhtempo, pu.pu_tanggal)::date AS umur_hari
		`).
		Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id) = TRIM(pu.pu_custid)").
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(pu.pu_agenid::varchar)").
		Where("(COALESCE(pu.pu_total, 0) - COALESCE(pu.pu_terbayar, 0)) > 0").
		Where("COALESCE(pu.pu_deleteyn, 'N') = 'N'")

	if !bypassTanggal && startDate != "" && endDate != "" {
		query = query.Where("pu.pu_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}

	if cabangID != "" && cabangID != "ALL" {
		query = query.Where("pu.pu_agenid = ?", cabangID)
	}

	if custID != "" && custID != "ALL" {
		query = query.Where("pu.pu_custid = ?", custID)
	}

	var rawData []AgingPiutangRow
	if err := query.Order("pu.pu_custid ASC, pu.pu_tanggal ASC").Scan(&rawData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Klasifikasikan ke bucket aging
	var finalData []AgingPiutangRow
	var totalCurrent, total31_60, total61_90, totalOver90, grandTotal float64

	for _, item := range rawData {
		row := item
		if row.UmurHari <= 30 {
			row.BucketCurrent = row.SisaPiutang
			totalCurrent += row.SisaPiutang
		} else if row.UmurHari <= 60 {
			row.Bucket31_60 = row.SisaPiutang
			total31_60 += row.SisaPiutang
		} else if row.UmurHari <= 90 {
			row.Bucket61_90 = row.SisaPiutang
			total61_90 += row.SisaPiutang
		} else {
			row.BucketOver90 = row.SisaPiutang
			totalOver90 += row.SisaPiutang
		}
		grandTotal += row.SisaPiutang
		finalData = append(finalData, row)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   finalData,
		"summary": gin.H{
			"total_current": totalCurrent,
			"total_31_60":   total31_60,
			"total_61_90":   total61_90,
			"total_over_90": totalOver90,
			"grand_total":   grandTotal,
		},
	})
}
