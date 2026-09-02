package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type AgingHutangItem struct {
	AgenNama      string  `json:"agen_nama" gorm:"column:agen_nama"`
	VendID        string  `json:"vend_id" gorm:"column:vend_id"`
	VendName      string  `json:"vend_name" gorm:"column:vend_name"`
	TglInvoice    string  `json:"tgl_invoice" gorm:"column:tgl_invoice"`
	InvoiceID     string  `json:"invoice_id" gorm:"column:aptivh_id"`
	NoKW          string  `json:"no_kw" gorm:"column:aptivh_nokw"`
	NoJurnal      string  `json:"no_jurnal" gorm:"column:aptivh_tjurhno"`
	TglJT         string  `json:"tgl_jt" gorm:"column:tgl_jt"`
	DPP           float64 `json:"dpp" gorm:"column:dpp"`
	NilaiTotal    float64 `json:"nilai_total" gorm:"column:aptivh_total"`
	Aging0        float64 `json:"aging_0"`
	Aging30       float64 `json:"aging_30"`
	Aging60       float64 `json:"aging_60"`
	Aging90       float64 `json:"aging_90"`
	Aging180      float64 `json:"aging_180"`
	Aging360      float64 `json:"aging_360"`
	AgingOver360  float64 `json:"aging_over_360"`
	UmurAktual    int     `json:"umur_aktual"`
	TotalTerbayar float64 `json:"total_terbayar" gorm:"column:total_payment_final"`
	SisaHutang    float64 `json:"sisa_hutang"`
}

// GET /api/hutang/aging-vendor/list
func GetAgingHutangVendorHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	cabangID := strings.TrimSpace(c.Query("cabang_id"))
	vendID := strings.TrimSpace(c.Query("vend_id"))
	bypassTanggal := c.Query("bypass_tanggal") == "1" || c.Query("bypass_tanggal") == "true"

	rawQuery := `
		SELECT 
			COALESCE(a.agen_nama, '-') AS agen_nama,
			v.vend_id,
			v.vend_name,
			TO_CHAR(ivh.aptivh_tanggal, 'YYYY-MM-DD') AS tgl_invoice,
			ivh.aptivh_id,
			COALESCE(ivh.aptivh_nokw, '-') AS aptivh_nokw,
			COALESCE(ivh.aptivh_tjurhno, '-') AS aptivh_tjurhno,
			COALESCE(TO_CHAR(ivh.aptivh_tanggaljt, 'YYYY-MM-DD'), TO_CHAR(ivh.aptivh_tanggal + INTERVAL '14 days', 'YYYY-MM-DD')) AS tgl_jt,
			COALESCE(dpp_data.total_dpp, 0) AS dpp,
			COALESCE(ivh.aptivh_total, 0) AS aptivh_total,
			(COALESCE(pay_data.total_payment, 0) + COALESCE(tax_data.tpayh_ppn, 0) + COALESCE(tax_data.tpayh_taxlain, 0) + COALESCE(tax_data.tpayh_pph, 0)) AS total_payment_final
		FROM public.mkt_m_vendor v
		LEFT JOIN public.glb_m_agen a ON a.agen_id::text = v.vend_agenid::text
		LEFT JOIN public.apt_t_invoicevendorh ivh ON v.vend_id = ivh.aptivh_vendid
		LEFT JOIN (
			SELECT trectd_aptivhid, SUM(trectd_nilai) AS total_payment
			FROM public.gl_t_pembayaranvendord
			GROUP BY trectd_aptivhid
		) pay_data ON ivh.aptivh_id = pay_data.trectd_aptivhid
		LEFT JOIN (
			SELECT d.trectd_aptivhid, SUM(h.tpayh_ppn) AS tpayh_ppn, SUM(h.tpayh_pph) AS tpayh_pph, SUM(h.tpayh_taxlain) AS tpayh_taxlain
			FROM public.gl_t_pembayaranvendord d
			INNER JOIN public.gl_t_pembayaranvendorh h ON d.trectd_tpayhno = h.tpayh_no
			GROUP BY d.trectd_aptivhid
		) tax_data ON ivh.aptivh_id = tax_data.trectd_aptivhid
		LEFT JOIN (
			SELECT aptivd_aptivhid, SUM(aptivd_total) AS total_dpp
			FROM public.apt_t_invoicevendord
			GROUP BY aptivd_aptivhid
		) dpp_data ON ivh.aptivh_id = dpp_data.aptivd_aptivhid
		WHERE v.vend_id IS NOT NULL 
		  AND COALESCE(ivh.aptivh_delete, 'N') = 'N'
		  AND (COALESCE(ivh.aptivh_total, 0) - (COALESCE(pay_data.total_payment, 0) + COALESCE(tax_data.tpayh_ppn, 0) + COALESCE(tax_data.tpayh_taxlain, 0) + COALESCE(tax_data.tpayh_pph, 0))) > 0
	`

	var args []interface{}

	if !bypassTanggal && startDate != "" && endDate != "" {
		rawQuery += " AND ivh.aptivh_tanggal >= ?::date AND ivh.aptivh_tanggal <= (?::date + INTERVAL '1 day' - INTERVAL '1 second')"
		args = append(args, startDate, endDate)
	}

	if cabangID != "" && strings.ToUpper(cabangID) != "ALL" {
		rawQuery += " AND (v.vend_agenid = ? OR a.agen_cabangid::text = ?)"
		args = append(args, cabangID, cabangID)
	}

	if vendID != "" && strings.ToUpper(vendID) != "ALL" {
		rawQuery += " AND v.vend_id = ?"
		args = append(args, vendID)
	}

	rawQuery += " ORDER BY v.vend_name ASC, ivh.aptivh_tanggal ASC"

	var rawItems []AgingHutangItem
	if err := database.Raw(rawQuery, args...).Scan(&rawItems).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghitung aging hutang: " + err.Error()})
		return
	}

	now := time.Now()
	var result []AgingHutangItem

	for _, item := range rawItems {
		item.SisaHutang = item.NilaiTotal - item.TotalTerbayar
		if item.SisaHutang <= 0 {
			continue
		}

		var acuanTgl time.Time
		if item.TglJT != "" {
			acuanTgl, _ = time.Parse("2006-01-02", item.TglJT)
		} else {
			tglInv, _ := time.Parse("2006-01-02", item.TglInvoice)
			acuanTgl = tglInv.AddDate(0, 0, 14)
		}

		diffDays := int(now.Sub(acuanTgl).Hours() / 24)
		item.UmurAktual = diffDays

		// Klasifikasi Umur Hutang (Aging Buckets)[cite: 3]
		if diffDays <= 0 {
			item.Aging0 = item.SisaHutang
		} else if diffDays <= 30 {
			item.Aging30 = item.SisaHutang
		} else if diffDays <= 60 {
			item.Aging60 = item.SisaHutang
		} else if diffDays <= 90 {
			item.Aging90 = item.SisaHutang
		} else if diffDays <= 180 {
			item.Aging180 = item.SisaHutang
		} else if diffDays <= 360 {
			item.Aging360 = item.SisaHutang
		} else {
			item.AgingOver360 = item.SisaHutang
		}

		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   result,
		"meta": gin.H{
			"total_rows": len(result),
			"tgl_proses": now.Format("2006-01-02"),
		},
	})
}
