package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type PenerimaanPembayaranHeaderRow struct {
	TRectHNo         string    `json:"trecth_no" gorm:"column:trecth_no"`
	TRectHTanggal    time.Time `json:"trecth_tanggal" gorm:"column:trecth_tanggal"`
	TRectHTanggalStr string    `json:"trecth_tanggal_str" gorm:"-"`
	TRectHCustID     string    `json:"trecth_custid" gorm:"column:trecth_custid"`
	TRectHCustName   string    `json:"trecth_custname" gorm:"column:trecth_custname"`
	TRectHTotal      float64   `json:"trecth_total" gorm:"column:trecth_total"`
	TRectHKeterangan string    `json:"trecth_keterangan" gorm:"column:trecth_keterangan"`
	TRectHDeleteYN   string    `json:"trecth_deleteyn" gorm:"column:trecth_deleteyn"`
	TRectHPostingYN  string    `json:"trecth_postingyn" gorm:"column:trecth_postingyn"`
	AgenNama         string    `json:"agen_nama" gorm:"column:agen_nama"`
	JBayar           float64   `json:"jbayar" gorm:"column:jbayar"`
}

type PenerimaanPembayaranPaymentItem struct {
	CAID       string  `json:"trectd_caid" gorm:"column:trectd_caid"`
	CANama     string  `json:"ca_nama" gorm:"column:ca_nama"`
	Keterangan string  `json:"trectd_keterangan" gorm:"column:trectd_keterangan"`
	Nilai      float64 `json:"trectd_nilai" gorm:"column:trectd_nilai"`
	Tipe       string  `json:"trectd_tipe" gorm:"column:trectd_tipe"`
	NoGiro     string  `json:"trectd_nogiro" gorm:"column:trectd_nogiro"`
}

type PenerimaanPembayaranInvoiceItem struct {
	NoFPUM     string  `json:"trectdd_nofpum" gorm:"column:trectdd_nofpum"`
	Jumlah     float64 `json:"trectdd_jumlah" gorm:"column:trectdd_jumlah"`
	Keterangan string  `json:"trectdd_keterangan" gorm:"column:trectdd_keterangan"`
}

// GET /api/piutang/penerimaan-pembayaran-kredit
func GetPenerimaanPembayaranKreditListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	cabang := strings.TrimSpace(c.Query("cabang"))
	customer := strings.TrimSpace(c.Query("customer"))
	noReceipt := strings.TrimSpace(c.Query("no_receipt"))
	noKwitansi := strings.TrimSpace(c.Query("no_kwitansi"))
	postingYN := strings.TrimSpace(c.Query("posting_yn"))

	query := database.Table("public.art_t_receipth h").
		Select(`
			h.trecth_no,
			h.trecth_tanggal,
			COALESCE(h.trecth_custid, '') AS trecth_custid,
			COALESCE(h.trecth_custname, '') AS trecth_custname,
			COALESCE(h.trecth_total, 0) AS trecth_total,
			COALESCE(h.trecth_keterangan, '') AS trecth_keterangan,
			COALESCE(h.trecth_deleteyn, 'N') AS trecth_deleteyn,
			COALESCE(h.trecth_postingyn, 'N') AS trecth_postingyn,
			COALESCE(a.agen_nama, 'PUSAT') AS agen_nama,
			COALESCE((SELECT SUM(d.trectd_nilai) FROM public.art_t_receiptd d WHERE d.trectd_trecthno = h.trecth_no), 0) AS jbayar
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(SUBSTRING(h.trecth_no, 1, 3))")

	if startDate != "" && endDate != "" {
		query = query.Where("h.trecth_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if cabang != "" && cabang != "ALL" {
		query = query.Where("(a.agen_nama ILIKE ? OR a.agen_id::varchar = ?)", "%"+cabang+"%", cabang)
	}
	if customer != "" {
		query = query.Where("(h.trecth_custname ILIKE ? OR h.trecth_custid ILIKE ?)", "%"+customer+"%", "%"+customer+"%")
	}
	if noReceipt != "" {
		query = query.Where("h.trecth_no ILIKE ?", "%"+noReceipt+"%")
	}
	if noKwitansi != "" {
		query = query.Where("EXISTS (SELECT 1 FROM public.art_t_receiptdd dd WHERE dd.trectdd_trectdtrecthno = h.trecth_no AND dd.trectdd_nofpum ILIKE ?)", "%"+noKwitansi+"%")
	}
	if postingYN != "" && postingYN != "ALL" {
		query = query.Where("COALESCE(h.trecth_postingyn, 'N') = ?", postingYN)
	}

	list := make([]PenerimaanPembayaranHeaderRow, 0)
	err := query.Order("h.trecth_tanggal DESC, h.trecth_no DESC").
		Limit(300).
		Scan(&list).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data pembayaran kredit: " + err.Error()})
		return
	}

	for i := range list {
		list[i].TRectHTanggalStr = list[i].TRectHTanggal.Format("2006-01-02")
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// GET /api/piutang/penerimaan-pembayaran-kredit/detail
func GetPenerimaanPembayaranKreditDetailHandler(c *gin.Context) {
	noReceipt := strings.TrimSpace(c.Query("no_receipt"))
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var head PenerimaanPembayaranHeaderRow
	err := database.Table("public.art_t_receipth h").
		Select(`
			h.trecth_no,
			h.trecth_tanggal,
			COALESCE(h.trecth_custid, '') AS trecth_custid,
			COALESCE(h.trecth_custname, '') AS trecth_custname,
			COALESCE(h.trecth_total, 0) AS trecth_total,
			COALESCE(h.trecth_keterangan, '') AS trecth_keterangan,
			COALESCE(h.trecth_deleteyn, 'N') AS trecth_deleteyn,
			COALESCE(h.trecth_postingyn, 'N') AS trecth_postingyn
		`).
		Where("TRIM(h.trecth_no) = TRIM(?)", noReceipt).
		Scan(&head).Error

	if err != nil || head.TRectHNo == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data bukti pembayaran tidak ditemukan"})
		return
	}

	// 1. Cek struktur kolom tabel public.art_t_receiptd secara aman
	var rCols []string
	database.Raw(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE LOWER(table_name) = 'art_t_receiptd'
	`).Scan(&rCols)

	caCol := "''"
	ketCol := "''"
	nilaiCol := "0"
	tipeCol := "'KAS'"
	giroCol := "''"

	for _, col := range rCols {
		cl := strings.ToLower(col)
		if cl == "trectd_caid" || cl == "trectd_ca_id" {
			caCol = "d." + col
		} else if cl == "trectd_keterangan" || cl == "trectd_ket" {
			ketCol = "d." + col
		} else if cl == "trectd_nilai" || cl == "trectd_jumlah" {
			nilaiCol = "d." + col
		} else if cl == "trectd_tipe" {
			tipeCol = "d." + col
		} else if cl == "trectd_nogiro" {
			giroCol = "d." + col
		}
	}

	// 2. Query Detail Pembayaran Kas/Bank
	payments := make([]PenerimaanPembayaranPaymentItem, 0)
	queryPayment := fmt.Sprintf(`
		SELECT 
			COALESCE(%s, '') AS trectd_caid,
			COALESCE(ca.ca_name, %s, '') AS ca_nama,
			COALESCE(%s, '') AS trectd_keterangan,
			COALESCE(%s, 0) AS trectd_nilai,
			COALESCE(%s, 'KAS') AS trectd_tipe,
			COALESCE(%s, '') AS trectd_nogiro
		FROM public.art_t_receiptd d
		LEFT JOIN public.gl_m_chartaccount ca ON TRIM(ca.ca_id::varchar) = TRIM((%s)::varchar)
		WHERE TRIM(d.trectd_trecthno) = TRIM(?)
	`, caCol, caCol, ketCol, nilaiCol, tipeCol, giroCol, caCol)

	database.Raw(queryPayment, noReceipt).Scan(&payments)

	// 3. Query Rincian Invoice yang Dilunasi
	invoices := make([]PenerimaanPembayaranInvoiceItem, 0)
	database.Table("public.art_t_receiptdd dd").
		Select(`
			dd.trectdd_nofpum,
			COALESCE(dd.trectdd_jumlah, 0) AS trectdd_jumlah,
			'' AS trectdd_keterangan
		`).
		Where("TRIM(dd.trectdd_trectdtrecthno) = TRIM(?)", noReceipt).
		Scan(&invoices)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"header":   head,
			"payments": payments,
			"invoices": invoices,
		},
	})
}

// POST /api/piutang/penerimaan-pembayaran-kredit
func CreatePenerimaanPembayaranKreditHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	type PaymentInput struct {
		CAID       string  `json:"ca_id"`
		Keterangan string  `json:"keterangan"`
		Nilai      float64 `json:"nilai"`
		Tipe       string  `json:"tipe"`
		NoGiro     string  `json:"no_giro"`
	}

	type InvoiceAllocInput struct {
		NoFPUM string  `json:"no_fpum"`
		Jumlah float64 `json:"jumlah"`
	}

	var input struct {
		Tanggal    string              `json:"tanggal"`
		CustID     string              `json:"cust_id"`
		CustName   string              `json:"cust_name"`
		Keterangan string              `json:"keterangan"`
		Payments   []PaymentInput      `json:"payments"`
		Invoices   []InvoiceAllocInput `json:"invoices"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	var totalBayar float64
	for _, p := range input.Payments {
		totalBayar += p.Nilai
	}

	tglParsed, _ := time.Parse("2006-01-02", input.Tanggal)
	yearMonth := tglParsed.Format("200601")
	var count int64
	database.Table("public.art_t_receipth").Where("trecth_no LIKE ?", "%/REC/"+yearMonth+"/%").Count(&count)
	newReceiptNo := fmt.Sprintf("001/REC/%s/%04d", yearMonth, count+1)

	tx := database.Begin()

	headerQuery := `
		INSERT INTO public.art_t_receipth (trecth_no, trecth_tanggal, trecth_custid, trecth_custname, trecth_total, trecth_keterangan, trecth_postingyn, trecth_deleteyn, trecth_updateid, trecth_updatetime)
		VALUES (?, ?, ?, ?, ?, ?, 'N', 'N', 'USER', NOW())
	`
	if err := tx.Exec(headerQuery, newReceiptNo, input.Tanggal+" 10:00:00", input.CustID, input.CustName, totalBayar, input.Keterangan).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan bukti pembayaran: " + err.Error()})
		return
	}

	for _, p := range input.Payments {
		pQuery := `
			INSERT INTO public.art_t_receiptd (trectd_trecthno, trectd_caid, trectd_keterangan, trectd_nilai, trectd_tipe, trectd_nogiro, trectd_updateid, trectd_updatetime)
			VALUES (?, ?, ?, ?, ?, ?, 'USER', NOW())
		`
		if err := tx.Exec(pQuery, newReceiptNo, p.CAID, p.Keterangan, p.Nilai, p.Tipe, p.NoGiro).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan rincian pembayaran: " + err.Error()})
			return
		}
	}

	for _, inv := range input.Invoices {
		invQuery := `
			INSERT INTO public.art_t_receiptdd (trectdd_trectdtrecthno, trectdd_nofpum, trectdd_jumlah, trectdd_keterangan, trectdd_updateid, trectdd_updatetime)
			VALUES (?, ?, ?, '', 'USER', NOW())
		`
		if err := tx.Exec(invQuery, newReceiptNo, inv.NoFPUM, inv.Jumlah).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan alokasi invoice: " + err.Error()})
			return
		}

		tx.Exec(`
			UPDATE public.art_t_invoiceh 
			SET artih_dibayar = COALESCE(artih_dibayar, 0) + ?,
			    artih_sisabayar = GREATEST(0, COALESCE(artih_total, 0) - (COALESCE(artih_dibayar, 0) + ?))
			WHERE artih_id = ? OR artih_nokw = ?
		`, inv.Jumlah, inv.Jumlah, inv.NoFPUM, inv.NoFPUM)
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Penerimaan pembayaran kredit berhasil disimpan", "no_receipt": newReceiptNo})
}

// PUT /api/piutang/penerimaan-pembayaran-kredit/batal
func CancelPenerimaanPembayaranKreditHandler(c *gin.Context) {
	noReceipt := strings.TrimSpace(c.Query("no_receipt"))
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	err := database.Table("public.art_t_receipth").
		Where("TRIM(trecth_no) = TRIM(?)", noReceipt).
		Update("trecth_deleteyn", "Y").Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membatalkan pembayaran: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Dokumen penerimaan pembayaran berhasil dibatalkan"})
}

// GET /api/akun/kas-bank-dropdown
func GetKasBankAccountsHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	type AccountOpt struct {
		CAID   string `json:"ca_id" gorm:"column:ca_id"`
		CANama string `json:"ca_nama" gorm:"column:ca_nama"`
	}

	accounts := make([]AccountOpt, 0)

	// 1. Cek kolom apa saja yang benar-benar ada di tabel gl_m_chartaccount
	var columns []string
	database.Raw(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_schema = 'public' AND table_name = 'gl_m_chartaccount'
	`).Scan(&columns)

	hasNameCol := ""
	for _, col := range columns {
		colLower := strings.ToLower(col)
		if colLower == "ca_name" || colLower == "ca_nama" || colLower == "ca_deskripsi" || colLower == "ca_keterangan" {
			hasNameCol = col
			break
		}
	}

	// 2. Susun query berdasarkan kolom yang valid
	query := database.Table("public.gl_m_chartaccount")
	if hasNameCol != "" {
		query = query.Select(fmt.Sprintf("ca_id, COALESCE(%s, ca_id) AS ca_nama", hasNameCol))
	} else {
		query = query.Select("ca_id, ca_id AS ca_nama")
	}

	if err := query.Order("ca_id ASC").Scan(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query akun: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": accounts})
}
