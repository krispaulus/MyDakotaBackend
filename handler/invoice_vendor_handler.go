package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type InvoiceVendorRow struct {
	InvoiceID    string  `json:"invoice_id" gorm:"column:aptivh_id"`
	TglInvoice   string  `json:"tgl_invoice" gorm:"column:tgl_invoice"`
	VendID       string  `json:"vend_id" gorm:"column:aptivh_vendid"`
	VendName     string  `json:"vend_name" gorm:"column:aptivh_vendname"`
	TotalDPP     float64 `json:"total_dpp" gorm:"column:aptivh_total"`
	PPHRate      float64 `json:"pph_rate" gorm:"column:aptivh_pph"`
	PPNRate      float64 `json:"ppn_rate" gorm:"column:aptivh_ppn"`
	TotalTagihan float64 `json:"total_tagihan"`
	TotalBayar   float64 `json:"total_bayar" gorm:"column:total_terbayar"`
	Jenis        string  `json:"jenis" gorm:"column:aptivh_jenis"`
	Keterangan   string  `json:"keterangan" gorm:"column:aptivh_keterangan"`
	NoKW         string  `json:"no_kw" gorm:"column:aptivh_nokw"`
	FktPajak     string  `json:"fkt_pajak" gorm:"column:aptivh_fktpajak"`
	NoJurnal     string  `json:"no_jurnal" gorm:"column:aptivh_tjurhno"`
	PostingYN    string  `json:"posting_yn" gorm:"column:aptivh_postingyn"`
	DeleteYN     string  `json:"delete_yn" gorm:"column:aptivh_delete"`
	KdCabang     string  `json:"kd_cabang" gorm:"column:kdcbg"`
}

// GET /api/hutang/invoice-vendor/list
func GetListInvoiceVendorHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	cabang := strings.TrimSpace(c.Query("cabang"))
	vendName := strings.TrimSpace(c.Query("vendor_name"))
	noInvoice := strings.TrimSpace(c.Query("no_invoice"))
	noKW := strings.TrimSpace(c.Query("no_kw"))
	postingYN := strings.TrimSpace(c.Query("posting_yn"))

	rawQuery := `
		SELECT 
			h.aptivh_id,
			TO_CHAR(h.aptivh_tanggal, 'YYYY-MM-DD') AS tgl_invoice,
			COALESCE(h.aptivh_vendid, '') AS aptivh_vendid,
			COALESCE(h.aptivh_vendname, '-') AS aptivh_vendname,
			COALESCE(h.aptivh_total, 0) AS aptivh_total,
			COALESCE(h.aptivh_pph, 0) AS aptivh_pph,
			COALESCE(h.aptivh_ppn, 0) AS aptivh_ppn,
			COALESCE(h.aptivh_jenis, 'R') AS aptivh_jenis,
			COALESCE(h.aptivh_keterangan, '-') AS aptivh_keterangan,
			COALESCE(h.aptivh_nokw, '-') AS aptivh_nokw,
			COALESCE(h.aptivh_fktpajak, '-') AS aptivh_fktpajak,
			COALESCE(h.aptivh_tjurhno, '-') AS aptivh_tjurhno,
			COALESCE(h.aptivh_postingyn, 'N') AS aptivh_postingyn,
			COALESCE(h.aptivh_delete, 'N') AS aptivh_delete,
			LEFT(h.aptivh_id, 3) AS kdcbg,
			COALESCE(SUM(CASE WHEN ph.tpayh_postingyn = 'Y' AND COALESCE(ph.tpayh_deleteyn, 'N') <> 'Y' THEN pd.trectd_nilai ELSE 0 END), 0) AS total_terbayar
		FROM public.apt_t_invoicevendorh h
		LEFT JOIN public.gl_t_pembayaranvendord pd ON h.aptivh_id = pd.trectd_aptivhid
		LEFT JOIN public.gl_t_pembayaranvendorh ph ON pd.trectd_tpayhno = ph.tpayh_no
		WHERE 1=1
	`
	var args []interface{}

	if startDate != "" && endDate != "" {
		rawQuery += " AND h.aptivh_tanggal >= ?::date AND h.aptivh_tanggal <= (?::date + INTERVAL '1 day' - INTERVAL '1 second')"
		args = append(args, startDate, endDate)
	}

	if cabang != "" && strings.ToUpper(cabang) != "ALL" {
		rawQuery += " AND LEFT(h.aptivh_id, 3) = ?"
		args = append(args, cabang)
	}

	if vendName != "" {
		rawQuery += " AND LOWER(h.aptivh_vendname) LIKE LOWER(?)"
		args = append(args, "%"+vendName+"%")
	}

	if noInvoice != "" {
		rawQuery += " AND LOWER(h.aptivh_id) LIKE LOWER(?)"
		args = append(args, "%"+noInvoice+"%")
	}

	if noKW != "" {
		rawQuery += " AND LOWER(h.aptivh_nokw) LIKE LOWER(?)"
		args = append(args, "%"+noKW+"%")
	}

	if postingYN != "" && strings.ToUpper(postingYN) != "ALL" {
		rawQuery += " AND h.aptivh_postingyn = ?"
		args = append(args, postingYN)
	}

	rawQuery += `
		GROUP BY h.aptivh_id, h.aptivh_tanggal, h.aptivh_vendid, h.aptivh_vendname, h.aptivh_total, 
		         h.aptivh_pph, h.aptivh_ppn, h.aptivh_jenis, h.aptivh_keterangan, h.aptivh_nokw, 
		         h.aptivh_fktpajak, h.aptivh_tjurhno, h.aptivh_postingyn, h.aptivh_delete
		ORDER BY h.aptivh_tanggal DESC, h.aptivh_id DESC LIMIT 500
	`

	var list []InvoiceVendorRow
	if err := database.Raw(rawQuery, args...).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membaca daftar invoice vendor: " + err.Error()})
		return
	}

	for i := range list {
		dpp := list[i].TotalDPP
		pphAmt := (dpp * list[i].PPHRate) / 100
		ppnAmt := (dpp * list[i].PPNRate) / 100
		list[i].TotalTagihan = (dpp + ppnAmt) - pphAmt
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// POST /api/hutang/invoice-vendor/save
func SaveInvoiceVendorHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var input struct {
		InvoiceID  string  `json:"invoice_id"`
		Tanggal    string  `json:"tanggal"`
		VendID     string  `json:"vend_id"`
		VendName   string  `json:"vend_name"`
		TotalDPP   float64 `json:"total_dpp"`
		PPHRate    float64 `json:"pph_rate"`
		PPNRate    float64 `json:"ppn_rate"`
		Jenis      string  `json:"jenis"`
		NoKW       string  `json:"no_kw"`
		FktPajak   string  `json:"fkt_pajak"`
		Keterangan string  `json:"keterangan"`
		PostingYN  string  `json:"posting_yn"`
		IsEdit     bool    `json:"is_edit"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	input.InvoiceID = strings.TrimSpace(input.InvoiceID)
	if input.InvoiceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Invoice wajib diisi"})
		return
	}

	if input.Tanggal == "" {
		input.Tanggal = time.Now().Format("2006-01-02")
	}
	if input.PostingYN == "" {
		input.PostingYN = "N"
	}
	if input.Jenis == "" {
		input.Jenis = "R"
	}

	if !input.IsEdit {
		var count int64
		database.Table("public.apt_t_invoicevendorh").Where("TRIM(aptivh_id) = TRIM(?)", input.InvoiceID).Count(&count)
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Invoice sudah terdaftar"})
			return
		}

		insertQuery := `
			INSERT INTO public.apt_t_invoicevendorh 
			(aptivh_id, aptivh_tanggal, aptivh_vendid, aptivh_vendname, aptivh_total, aptivh_pph, aptivh_ppn, aptivh_jenis, aptivh_nokw, aptivh_fktpajak, aptivh_keterangan, aptivh_postingyn, aptivh_delete)
			VALUES (?, ?::timestamp, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'N')
		`
		if err := database.Exec(insertQuery, input.InvoiceID, input.Tanggal, input.VendID, input.VendName, input.TotalDPP, input.PPHRate, input.PPNRate, input.Jenis, input.NoKW, input.FktPajak, input.Keterangan, input.PostingYN).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan invoice vendor: " + err.Error()})
			return
		}
	} else {
		updateQuery := `
			UPDATE public.apt_t_invoicevendorh 
			SET aptivh_tanggal = ?::timestamp, aptivh_vendid = ?, aptivh_vendname = ?, aptivh_total = ?, 
			    aptivh_pph = ?, aptivh_ppn = ?, aptivh_jenis = ?, aptivh_nokw = ?, aptivh_fktpajak = ?, 
			    aptivh_keterangan = ?, aptivh_postingyn = ?
			WHERE TRIM(aptivh_id) = TRIM(?)
		`
		if err := database.Exec(updateQuery, input.Tanggal, input.VendID, input.VendName, input.TotalDPP, input.PPHRate, input.PPNRate, input.Jenis, input.NoKW, input.FktPajak, input.Keterangan, input.PostingYN, input.InvoiceID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update invoice vendor: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Invoice Vendor %s berhasil disimpan", input.InvoiceID)})
}

// DELETE /api/hutang/invoice-vendor/delete
func DeleteInvoiceVendorHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	invoiceID := strings.TrimSpace(c.Query("invoice_id"))
	if invoiceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Invoice wajib diisi"})
		return
	}

	// Soft delete: set aptivh_delete = 'Y'
	if err := database.Exec("UPDATE public.apt_t_invoicevendorh SET aptivh_delete = 'Y' WHERE TRIM(aptivh_id) = TRIM(?)", invoiceID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membatalkan invoice vendor: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Invoice Vendor %s berhasil dibatalkan", invoiceID)})
}
