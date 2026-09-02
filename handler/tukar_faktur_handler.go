package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type TukarFakturRow struct {
	InvoiceNo    string  `json:"invoice_no" gorm:"column:artitf_artihid"`
	TglInvoice   string  `json:"tgl_invoice" gorm:"column:tgl_invoice"`
	CustID       string  `json:"cust_id" gorm:"column:artih_custid"`
	CustName     string  `json:"cust_name" gorm:"column:artih_custname"`
	NoKW         string  `json:"no_kw" gorm:"column:artih_nokw"`
	FakturPajak  string  `json:"faktur_pajak" gorm:"column:artih_fktpajak"`
	TotalTagihan float64 `json:"total_tagihan" gorm:"column:artih_total"`
	DPP          float64 `json:"dpp" gorm:"column:artih_dpp"`
	PPN          float64 `json:"ppn" gorm:"column:artih_ppn"`
	TglTukar     string  `json:"tgl_tukar" gorm:"column:tgl_tukar"`
	Penerima     string  `json:"penerima" gorm:"column:artitf_nama"`
	TglJT        string  `json:"tgl_jt" gorm:"column:tgl_jt"`
}

// GET /api/piutang/tukar-faktur/list
func GetListTukarFakturHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	noInvoice := strings.TrimSpace(c.Query("no_invoice"))
	noKW := strings.TrimSpace(c.Query("no_kw"))

	rawQuery := `
		SELECT 
			tf.artitf_artihid,
			TO_CHAR(ih.artih_tanggal, 'YYYY-MM-DD') AS tgl_invoice,
			COALESCE(ih.artih_custid, '') AS artih_custid,
			COALESCE(ih.artih_custname, '-') AS artih_custname,
			COALESCE(ih.artih_nokw, '-') AS artih_nokw,
			COALESCE(ih.artih_fktpajak, '-') AS artih_fktpajak,
			COALESCE(ih.artih_total, 0) AS artih_total,
			COALESCE(ih.artih_dpp, 0) AS artih_dpp,
			COALESCE(ih.artih_ppn, 0) AS artih_ppn,
			TO_CHAR(tf.artitf_tanggal, 'YYYY-MM-DD') AS tgl_tukar,
			COALESCE(tf.artitf_nama, '-') AS artitf_nama,
			TO_CHAR(tf.artitf_tgljt, 'YYYY-MM-DD') AS tgl_jt
		FROM public.art_t_invoicetf tf
		LEFT JOIN public.art_t_invoiceh ih ON TRIM(ih.artih_id) = TRIM(tf.artitf_artihid)
		WHERE 1=1
	`
	var args []interface{}

	if startDate != "" && endDate != "" {
		rawQuery += " AND tf.artitf_tanggal >= ?::date AND tf.artitf_tanggal <= (?::date + INTERVAL '1 day' - INTERVAL '1 second')"
		args = append(args, startDate, endDate)
	}

	if noInvoice != "" {
		rawQuery += " AND LOWER(tf.artitf_artihid) LIKE LOWER(?)"
		args = append(args, "%"+noInvoice+"%")
	}

	if noKW != "" {
		rawQuery += " AND LOWER(ih.artih_nokw) LIKE LOWER(?)"
		args = append(args, "%"+noKW+"%")
	}

	rawQuery += " ORDER BY tf.artitf_tanggal DESC LIMIT 500"

	list := make([]TukarFakturRow, 0)
	if err := database.Raw(rawQuery, args...).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membaca daftar tukar faktur: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// GET /api/piutang/tukar-faktur/available-invoices
func GetAvailableInvoicesForTFHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	search := strings.TrimSpace(c.Query("search"))

	rawQuery := `
		SELECT 
			ih.artih_id AS invoice_no,
			TO_CHAR(ih.artih_tanggal, 'YYYY-MM-DD') AS tgl_invoice,
			COALESCE(ih.artih_custid, '') AS cust_id,
			COALESCE(ih.artih_custname, '-') AS cust_name,
			COALESCE(ih.artih_total, 0) AS total_tagihan
		FROM public.art_t_invoiceh ih
		LEFT JOIN public.art_t_invoicetf tf ON TRIM(tf.artitf_artihid) = TRIM(ih.artih_id)
		WHERE COALESCE(ih.artih_delete, 'N') = 'N' 
		  AND tf.artitf_artihid IS NULL
	`
	var args []interface{}

	if search != "" {
		rawQuery += " AND (LOWER(ih.artih_id) LIKE LOWER(?) OR LOWER(ih.artih_custname) LIKE LOWER(?))"
		args = append(args, "%"+search+"%", "%"+search+"%")
	}

	rawQuery += " ORDER BY ih.artih_tanggal DESC LIMIT 100"

	type InvRow struct {
		InvoiceNo    string  `json:"invoice_no" gorm:"column:invoice_no"`
		TglInvoice   string  `json:"tgl_invoice" gorm:"column:tgl_invoice"`
		CustID       string  `json:"cust_id" gorm:"column:cust_id"`
		CustName     string  `json:"cust_name" gorm:"column:cust_name"`
		TotalTagihan float64 `json:"total_tagihan" gorm:"column:total_tagihan"`
	}

	list := make([]InvRow, 0)
	database.Raw(rawQuery, args...).Scan(&list)

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// POST /api/piutang/tukar-faktur/save
func SaveTukarFakturHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var input struct {
		InvoiceNo string `json:"invoice_no"`
		TglTukar  string `json:"tgl_tukar"`
		Penerima  string `json:"penerima"`
		TglJT     string `json:"tgl_jt"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	input.InvoiceNo = strings.TrimSpace(input.InvoiceNo)
	if input.InvoiceNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Invoice wajib ditentukan"})
		return
	}

	if input.TglTukar == "" {
		input.TglTukar = time.Now().Format("2006-01-02")
	}

	var count int64
	database.Table("public.art_t_invoicetf").Where("TRIM(artitf_artihid) = TRIM(?)", input.InvoiceNo).Count(&count)

	if count == 0 {
		var insQuery string
		var err error
		if input.TglJT != "" {
			insQuery = `INSERT INTO public.art_t_invoicetf (artitf_artihid, artitf_tanggal, artitf_nama, artitf_tgljt) VALUES (?, ?::timestamp, ?, ?::timestamp)`
			err = database.Exec(insQuery, input.InvoiceNo, input.TglTukar, input.Penerima, input.TglJT).Error
		} else {
			insQuery = `INSERT INTO public.art_t_invoicetf (artitf_artihid, artitf_tanggal, artitf_nama, artitf_tgljt) VALUES (?, ?::timestamp, ?, NULL)`
			err = database.Exec(insQuery, input.InvoiceNo, input.TglTukar, input.Penerima).Error
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan tukar faktur: " + err.Error()})
			return
		}
	} else {
		var updQuery string
		var err error
		if input.TglJT != "" {
			updQuery = `UPDATE public.art_t_invoicetf SET artitf_tanggal = ?::timestamp, artitf_nama = ?, artitf_tgljt = ?::timestamp WHERE TRIM(artitf_artihid) = TRIM(?)`
			err = database.Exec(updQuery, input.TglTukar, input.Penerima, input.TglJT, input.InvoiceNo).Error
		} else {
			updQuery = `UPDATE public.art_t_invoicetf SET artitf_tanggal = ?::timestamp, artitf_nama = ?, artitf_tgljt = NULL WHERE TRIM(artitf_artihid) = TRIM(?)`
			err = database.Exec(updQuery, input.TglTukar, input.Penerima, input.InvoiceNo).Error
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update tukar faktur: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Data Tukar Faktur %s berhasil disimpan", input.InvoiceNo)})
}

// DELETE /api/piutang/tukar-faktur/delete
func DeleteTukarFakturHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	invoiceNo := strings.TrimSpace(c.Query("invoice_no"))
	if invoiceNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Invoice wajib diisi"})
		return
	}

	if err := database.Exec("DELETE FROM public.art_t_invoicetf WHERE TRIM(artitf_artihid) = TRIM(?)", invoiceNo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus status tukar faktur: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Status Tukar Faktur %s berhasil dihapus", invoiceNo)})
}
