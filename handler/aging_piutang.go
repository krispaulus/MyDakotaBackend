package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Database corporate tidak terhubung",
		})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	cabangID := strings.TrimSpace(c.Query("cabang_id"))
	custID := strings.TrimSpace(c.Query("cust_id"))
	bypassTanggal := c.Query("bypass_tanggal") == "true" || c.Query("bypass_tanggal") == "1"

	// Query mengambil Piutang Outstanding langsung dari tabel art_t_invoiceh
	query := database.Table("public.art_t_invoiceh h").
		Select(`
			h.artih_custid AS cust_id,
			COALESCE(c.cust_name, h.artih_custname) AS cust_name,
			COALESCE(a.agen_nama, 'DLI PUSAT') AS cabang_nama,
			h.artih_id AS no_invoice,
			TO_CHAR(h.artih_tanggal, 'YYYY-MM-DD') AS tgl_invoice,
			TO_CHAR(h.artih_tanggal + INTERVAL '30 days', 'YYYY-MM-DD') AS tgl_jatuh_tempo,
			COALESCE(h.artih_total, 0) AS total_tagihan,
			COALESCE(rp.total_terbayar, 0) AS total_bayar,
			(COALESCE(h.artih_total, 0) - COALESCE(rp.total_terbayar, 0)) AS sisa_piutang,
			CURRENT_DATE - (h.artih_tanggal)::date AS umur_hari
		`).
		Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id) = TRIM(h.artih_custid)").
		Joins("LEFT JOIN public.glb_m_agen a ON a.agen_id::varchar = LPAD(LEFT(h.artih_id, 3), 3, '0')").
		Joins(`LEFT JOIN (
			SELECT d.artid_artihid, SUM(r.trectddd_bayar) AS total_terbayar
			FROM public.art_t_invoiced d
			LEFT JOIN public.art_t_receiptdddd r ON TRIM(r.trectddd_nobtt) = TRIM(d.artid_bttid)
			GROUP BY d.artid_artihid
		) rp ON TRIM(rp.artid_artihid) = TRIM(h.artih_id)`).
		Where("COALESCE(h.artih_delete, 'N') = 'N'").
		Where("(COALESCE(h.artih_total, 0) - COALESCE(rp.total_terbayar, 0)) > 0")

	if !bypassTanggal && startDate != "" && endDate != "" {
		query = query.Where("h.artih_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}

	if cabangID != "" && cabangID != "ALL" {
		query = query.Where("LPAD(LEFT(h.artih_id, 3), 3, '0') = LPAD(?, 3, '0')", cabangID)
	}

	if custID != "" && custID != "ALL" {
		query = query.Where("h.artih_custid = ?", custID)
	}

	var rawData []AgingPiutangRow
	if err := query.Order("h.artih_custid ASC, h.artih_tanggal ASC").Scan(&rawData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Hitung bucket aging dan ringkasan
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
