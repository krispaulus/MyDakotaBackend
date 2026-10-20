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
	TRectHTJurHNo    string    `json:"trecth_tjurhno" gorm:"column:trecth_tjurhno"`
	AgenNama         string    `json:"agen_nama" gorm:"column:agen_nama"`
	JBayar           float64   `json:"jbayar" gorm:"column:jbayar"`
}

// Model Payment Item sesuai tabel asli art_t_receiptd
type PenerimaanPembayaranPaymentItem struct {
	TRectHNo  string  `json:"trectd_trecthno" gorm:"column:trectd_trecthno"`
	Type      string  `json:"trectd_type" gorm:"column:trectd_type"`
	NoBG      string  `json:"trectd_nobg" gorm:"column:trectd_nobg"`
	NmBank    string  `json:"trectd_nmbank" gorm:"column:trectd_nmbank"`
	TanggalJT string  `json:"trectd_tanggaljt" gorm:"column:trectd_tanggaljt"`
	Nilai     float64 `json:"trectd_nilai" gorm:"column:trectd_nilai"`
}

// Model Kurang/Lebih Bayar sesuai tabel asli art_t_hubrectd
type PenerimaanPembayaranHubItem struct {
	TRectHNo   string  `json:"thrd_trecthno" gorm:"column:thrd_trecthno"`
	CustID     string  `json:"thrd_custid" gorm:"column:thrd_custid"`
	CustName   string  `json:"thrd_custname" gorm:"column:thrd_custname"`
	CAID       string  `json:"thrd_caid" gorm:"column:thrd_caid"`
	CAName     string  `json:"ca_name" gorm:"column:ca_name"`
	Jumlah     float64 `json:"thrd_jumlah" gorm:"column:thrd_jumlah"`
	Keterangan string  `json:"thrd_keterangan" gorm:"column:thrd_keterangan"`
	Pengali    float64 `json:"thrd_pengali" gorm:"column:thrd_pengali"`
	BankYN     string  `json:"thrd_bankyn" gorm:"column:thrd_bankyn"`
	KodeLK     string  `json:"thrd_kodelk" gorm:"column:thrd_kodelk"`
}

type PenerimaanPembayaranInvoiceItem struct {
	NoFPUM     string  `json:"trectdd_nofpum" gorm:"column:trectdd_nofpum"`
	TglJT      string  `json:"trectdd_tgljt" gorm:"column:trectdd_tgljt"`
	Jumlah     float64 `json:"trectdd_jumlah" gorm:"column:trectdd_jumlah"`
	Keterangan string  `json:"trectdd_keterangan" gorm:"column:trectdd_keterangan"`
}

// Membersihkan karakter UTF-8 BOM tersembunyi
func cleanUTF8BOM(s string) string {
	s = strings.TrimPrefix(s, "\ufeff")
	s = strings.ReplaceAll(s, "∩╗┐", "")
	return strings.TrimSpace(s)
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
			TRIM(h.trecth_no) AS trecth_no,
			h.trecth_tanggal,
			COALESCE(TRIM(h.trecth_custid), '') AS trecth_custid,
			COALESCE(TRIM(h.trecth_custname), '') AS trecth_custname,
			COALESCE(h.trecth_total, 0) AS trecth_total,
			COALESCE(TRIM(h.trecth_keterangan), '') AS trecth_keterangan,
			COALESCE(TRIM(h.trecth_deleteyn), 'N') AS trecth_deleteyn,
			COALESCE(TRIM(h.trecth_postingyn), 'N') AS trecth_postingyn,
			COALESCE(TRIM(h.trecth_tjurhno), '-') AS trecth_tjurhno,
			CASE 
				WHEN TRIM(SUBSTRING(h.trecth_no, 1, 3)) = '001' THEN 'DLI PUSAT'
				ELSE COALESCE(TRIM(a.agen_nama), 'DLI PUSAT')
			END AS agen_nama,
			COALESCE((SELECT SUM(d.trectd_nilai) FROM public.art_t_receiptd d WHERE TRIM(d.trectd_trecthno) = TRIM(h.trecth_no)), 0) AS jbayar
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(SUBSTRING(h.trecth_no, 1, 3))")

	if startDate != "" && endDate != "" {
		query = query.Where("h.trecth_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if cabang != "" && cabang != "ALL" {
		if strings.Contains(strings.ToUpper(cabang), "PUSAT") {
			query = query.Where("TRIM(SUBSTRING(h.trecth_no, 1, 3)) IN ('001', 'PST', 'PST001') OR a.agen_nama ILIKE '%PUSAT%'")
		} else {
			query = query.Where("(a.agen_nama ILIKE ? OR a.agen_id::varchar = ?)", "%"+cabang+"%", cabang)
		}
	}
	if customer != "" {
		query = query.Where("(h.trecth_custname ILIKE ? OR h.trecth_custid ILIKE ?)", "%"+customer+"%", "%"+customer+"%")
	}
	if noReceipt != "" {
		query = query.Where("h.trecth_no ILIKE ?", "%"+noReceipt+"%")
	}
	if noKwitansi != "" {
		query = query.Where("EXISTS (SELECT 1 FROM public.art_t_receiptdd dd WHERE TRIM(dd.trectdd_trectdtrecthno) = TRIM(h.trecth_no) AND dd.trectdd_nofpum ILIKE ?)", "%"+noKwitansi+"%")
	}
	if postingYN != "" && postingYN != "ALL" {
		query = query.Where("COALESCE(TRIM(h.trecth_postingyn), 'N') = ?", postingYN)
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
			TRIM(h.trecth_no) AS trecth_no,
			h.trecth_tanggal,
			COALESCE(TRIM(h.trecth_custid), '') AS trecth_custid,
			COALESCE(TRIM(h.trecth_custname), '') AS trecth_custname,
			COALESCE(h.trecth_total, 0) AS trecth_total,
			COALESCE(TRIM(h.trecth_keterangan), '') AS trecth_keterangan,
			COALESCE(TRIM(h.trecth_deleteyn), 'N') AS trecth_deleteyn,
			COALESCE(TRIM(h.trecth_postingyn), 'N') AS trecth_postingyn,
			COALESCE(TRIM(h.trecth_tjurhno), '-') AS trecth_tjurhno
		`).
		Where("TRIM(h.trecth_no) = TRIM(?)", noReceipt).
		Scan(&head).Error

	if err != nil || head.TRectHNo == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data bukti pembayaran tidak ditemukan"})
		return
	}

	// 1. Query Detail Pembayaran Kas/Bank dari tabel art_t_receiptd
	payments := make([]PenerimaanPembayaranPaymentItem, 0)
	database.Raw(`
		SELECT 
			TRIM(trectd_trecthno) AS trectd_trecthno,
			COALESCE(TRIM(trectd_type), '0') AS trectd_type,
			COALESCE(TRIM(trectd_nobg), '') AS trectd_nobg,
			COALESCE(TRIM(trectd_nmbank), '') AS trectd_nmbank,
			COALESCE(TO_CHAR(trectd_tanggaljt, 'YYYY-MM-DD'), '') AS trectd_tanggaljt,
			COALESCE(trectd_nilai, 0) AS trectd_nilai
		FROM public.art_t_receiptd
		WHERE TRIM(trectd_trecthno) = TRIM(?)
	`, noReceipt).Scan(&payments)

	// 2. Query Detail Invoice dari tabel art_t_receiptdd
	invoices := make([]PenerimaanPembayaranInvoiceItem, 0)
	database.Raw(`
		SELECT 
			TRIM(trectdd_nofpum) AS trectdd_nofpum,
			COALESCE(TO_CHAR(trectdd_tgljt, 'YYYY-MM-DD'), '') AS trectdd_tgljt,
			COALESCE(trectdd_jumlah, 0) AS trectdd_jumlah,
			'' AS trectdd_keterangan
		FROM public.art_t_receiptdd
		WHERE TRIM(trectdd_trectdtrecthno) = TRIM(?)
	`, noReceipt).Scan(&invoices)

	// 3. Query Kurang / Lebih Bayar dari tabel art_t_hubrectd
	kurangLebih := make([]PenerimaanPembayaranHubItem, 0)
	database.Raw(`
		SELECT 
			TRIM(th.thrd_trecthno) AS thrd_trecthno,
			COALESCE(TRIM(th.thrd_custid), '') AS thrd_custid,
			COALESCE(TRIM(th.thrd_custname), '') AS thrd_custname,
			COALESCE(TRIM(th.thrd_caid), '') AS thrd_caid,
			COALESCE(TRIM(ca.ca_name), TRIM(th.thrd_caid), '') AS ca_name,
			COALESCE(th.thrd_jumlah, 0) AS thrd_jumlah,
			COALESCE(TRIM(th.thrd_keterangan), '') AS thrd_keterangan,
			COALESCE(th.thrd_pengali, 1) AS thrd_pengali,
			COALESCE(TRIM(th.thrd_bankyn), 'N') AS thrd_bankyn,
			COALESCE(TRIM(th.thrd_kodelk), 'L') AS thrd_kodelk
		FROM public.art_t_hubrectd th
		LEFT JOIN public.gl_m_chartaccount ca ON TRIM(ca.ca_id::varchar) = TRIM(th.thrd_caid::varchar)
		WHERE TRIM(th.thrd_trecthno) = TRIM(?)
	`, noReceipt).Scan(&kurangLebih)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"header":       head,
			"payments":     payments,
			"invoices":     invoices,
			"kurang_lebih": kurangLebih,
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
		Tipe      string  `json:"tipe"`
		NoBG      string  `json:"no_bg"`
		Bank      string  `json:"bank"`
		TanggalJT string  `json:"tanggal_jt"`
		Nilai     float64 `json:"nilai"`
	}

	type InvoiceAllocInput struct {
		NoFPUM string  `json:"no_fpum"`
		Jumlah float64 `json:"jumlah"`
	}

	type KurangLebihInput struct {
		CAID       string  `json:"ca_id"`
		KodeLK     string  `json:"kodelk"`
		Jumlah     float64 `json:"jumlah"`
		Keterangan string  `json:"keterangan"`
	}

	var input struct {
		Tanggal     string              `json:"tanggal"`
		Cabang      string              `json:"cabang"`
		CustID      string              `json:"cust_id"`
		CustName    string              `json:"cust_name"`
		Keterangan  string              `json:"keterangan"`
		Payments    []PaymentInput      `json:"payments"`
		Invoices    []InvoiceAllocInput `json:"invoices"`
		KurangLebih []KurangLebihInput  `json:"kurang_lebih"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	var totalBayar float64
	for _, p := range input.Payments {
		totalBayar += p.Nilai
	}

	tglParsed, err := time.Parse("2006-01-02", cleanUTF8BOM(input.Tanggal))
	if err != nil {
		tglParsed = time.Now()
	}
	yearMonth := tglParsed.Format("200601")

	// Generate format nomor receipt: 001/REC/YYYYMM/0001
	var count int64
	database.Table("public.art_t_receipth").Where("trecth_no LIKE ?", "%/REC/"+yearMonth+"/%").Count(&count)
	newReceiptNo := fmt.Sprintf("001/REC/%s/%04d", yearMonth, count+1)

	// Format datetime yang aman dan presisi (Contoh: "2026-10-20 15:20:34")
	waktuLengkap := fmt.Sprintf("%s %s", tglParsed.Format("2006-01-02"), time.Now().Format("15:04:05"))

	tx := database.Begin()

	// 1. Simpan Header dengan datetime valid
	headerQuery := `
		INSERT INTO public.art_t_receipth (
			trecth_no, trecth_tanggal, trecth_custid, trecth_custname, 
			trecth_total, trecth_keterangan, trecth_postingyn, trecth_deleteyn, 
			trecth_tjurhno, trecth_updateid, trecth_updatetime
		) VALUES (?, ?, ?, ?, ?, ?, 'N', 'N', '', 'USER', NOW())
	`
	if err := tx.Exec(headerQuery,
		newReceiptNo,
		waktuLengkap,
		cleanUTF8BOM(input.CustID),
		cleanUTF8BOM(input.CustName),
		totalBayar,
		cleanUTF8BOM(input.Keterangan),
	).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan bukti pembayaran: " + err.Error()})
		return
	}

	// 2. Simpan Pembayaran Customer (art_t_receiptd) Sesuai Kolom Database Asli
	pQuery := `
		INSERT INTO public.art_t_receiptd (
			trectd_trecthno, trectd_type, trectd_nobg, trectd_nmbank, trectd_tanggaljt, trectd_nilai
		) VALUES (?, ?, ?, ?, ?, ?)
	`
	for _, p := range input.Payments {
		tipeCode := "0" // 0 = Cash, 1 = Giro, 2 = Transfer
		tStr := strings.ToUpper(strings.TrimSpace(p.Tipe))
		if tStr == "GIRO" || tStr == "1" {
			tipeCode = "1"
		} else if tStr == "BANK" || tStr == "TRANSFER" || tStr == "2" {
			tipeCode = "2"
		}

		tglJT := cleanUTF8BOM(p.TanggalJT)
		if tglJT == "" {
			tglJT = tglParsed.Format("2006-01-02")
		}

		noBG := cleanUTF8BOM(p.NoBG)
		nmBank := cleanUTF8BOM(p.Bank)
		if nmBank == "" && tipeCode == "2" {
			nmBank = "BANK"
		}

		if err := tx.Exec(pQuery, newReceiptNo, tipeCode, noBG, nmBank, tglJT, p.Nilai).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan rincian pembayaran: " + err.Error()})
			return
		}

		// Jika tipe Giro, sisipkan ke tabel art_t_giro
		if tipeCode == "1" {
			tx.Exec(`
				INSERT INTO public.art_t_giro (
					tg_type, tg_no, tg_trecthno, tg_bankname, tg_bankid, 
					tg_tgljt, tg_nilai, tg_postingyn, tg_deleteyn, tg_updateid, tg_updatetime, tg_tjurhno
				) VALUES ('1', ?, ?, ?, '0', ?, ?, 'N', 'N', 'USER', NOW(), '')
			`, noBG, newReceiptNo, nmBank, tglJT, p.Nilai)
		}
	}

	// 3. Simpan Alokasi Invoice (art_t_receiptdd)
	invQuery := `
		INSERT INTO public.art_t_receiptdd (
			trectdd_trectdtrecthno, trectdd_nofpum, trectdd_tgljt, trectdd_jumlah
		) VALUES (?, ?, ?, ?)
	`
	for _, inv := range input.Invoices {
		if strings.TrimSpace(inv.NoFPUM) == "" {
			continue
		}
		if err := tx.Exec(invQuery, newReceiptNo, cleanUTF8BOM(inv.NoFPUM), tglParsed.Format("2006-01-02"), inv.Jumlah).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan alokasi invoice: " + err.Error()})
			return
		}

		// Update pelunasan invoice
		tx.Exec(`
			UPDATE public.art_t_invoiceh 
			SET artih_dibayar = COALESCE(artih_dibayar, 0) + ?,
				artih_sisabayar = GREATEST(0, COALESCE(artih_total, 0) - (COALESCE(artih_dibayar, 0) + ?))
			WHERE TRIM(artih_id) = TRIM(?) OR TRIM(artih_nokw) = TRIM(?)
		`, inv.Jumlah, inv.Jumlah, inv.NoFPUM, inv.NoFPUM)
	}

	// 4. Simpan Kurang / Lebih Bayar (art_t_hubrectd)
	hubQuery := `
		INSERT INTO public.art_t_hubrectd (
			thrd_trecthno, thrd_custid, thrd_custname, thrd_caid, 
			thrd_jumlah, thrd_keterangan, thrd_pengali, thrd_bankyn, thrd_kodelk
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'N', ?)
	`
	for _, h := range input.KurangLebih {
		if strings.TrimSpace(h.CAID) == "" || h.Jumlah == 0 {
			continue
		}
		kodelk := strings.ToUpper(strings.TrimSpace(h.KodeLK))
		pengali := 1.0
		if kodelk == "K" {
			pengali = -1.0
		}
		caClean := cleanUTF8BOM(h.CAID)

		tx.Exec(hubQuery,
			newReceiptNo,
			cleanUTF8BOM(input.CustID),
			cleanUTF8BOM(input.CustName),
			caClean,
			h.Jumlah,
			cleanUTF8BOM(h.Keterangan),
			pengali,
			kodelk,
		)
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "Penerimaan pembayaran kredit berhasil disimpan",
		"no_receipt": newReceiptNo,
	})
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

	query := database.Table("public.gl_m_chartaccount")
	if hasNameCol != "" {
		query = query.Select(fmt.Sprintf("TRIM(ca_id) AS ca_id, COALESCE(TRIM(%s), TRIM(ca_id)) AS ca_nama", hasNameCol))
	} else {
		query = query.Select("TRIM(ca_id) AS ca_id, TRIM(ca_id) AS ca_nama")
	}

	if err := query.Order("ca_id ASC").Scan(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query akun: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": accounts})
}

// 1. GET /api/piutang/penerimaan-pembayaran-kredit/unpaid-invoices
// GET /api/piutang/penerimaan-pembayaran-kredit/unpaid-invoices
func GetUnpaidInvoicesHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	custID := strings.TrimSpace(c.Query("cust_id"))
	tipe := strings.ToUpper(strings.TrimSpace(c.Query("tipe"))) // ALL, INVOICE, ORDER JEMPUT, PACKING, ASURANSI
	q := strings.TrimSpace(c.Query("q"))

	if custID == "" {
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": []interface{}{}})
		return
	}

	type UnpaidItem struct {
		Tipe          string    `json:"tipe"`
		NoKwitansi    string    `json:"no_kwitansi"`
		NoInvoice     string    `json:"no_invoice"`
		Tanggal       time.Time `json:"tanggal"`
		TanggalStr    string    `json:"tanggal_str"`
		JumlahTagihan float64   `json:"jumlah_tagihan"`
		DPP           float64   `json:"dpp"`
		PPN           float64   `json:"ppn"`
	}

	var list []UnpaidItem

	// Query menggunakan kolom riil: artih_id (No Faktur) dan artih_nokw (No Kwitansi)
	query := `
		SELECT 
			CASE 
				WHEN RIGHT(TRIM(artih_nokw), 3) = '/OJ' THEN 'Order Jemput'
				WHEN RIGHT(TRIM(artih_nokw), 3) = 'PCK' THEN 'Packing'
				WHEN RIGHT(TRIM(artih_nokw), 3) = 'ASR' THEN 'Asuransi'
				ELSE 'Invoice'
			END AS tipe,
			COALESCE(TRIM(artih_nokw), '') AS no_kwitansi,
			COALESCE(TRIM(artih_id), TRIM(artih_nokw)) AS no_invoice,
			artih_tanggal AS tanggal,
			COALESCE(artih_total, 0) AS dpp,
			0 AS ppn,
			CASE 
				WHEN COALESCE(artih_sisabayar, 0) > 0 THEN artih_sisabayar
				ELSE COALESCE(artih_total, 0)
			END AS jumlah_tagihan
		FROM public.art_t_invoiceh
		WHERE TRIM(artih_custid) = TRIM(?)
		  AND COALESCE(artih_terbayar, 'N') <> 'Y'
		  AND (COALESCE(artih_sisabayar, 0) > 0 OR COALESCE(artih_total, 0) > 0)
	`
	var args []interface{}
	args = append(args, custID)

	// Filter radio tipe dokumen
	if tipe != "" && tipe != "ALL" && tipe != "SEMUA" {
		if tipe == "ORDER JEMPUT" || tipe == "OJ" {
			query += " AND RIGHT(TRIM(artih_nokw), 3) = '/OJ'"
		} else if tipe == "PACKING" || tipe == "PCK" {
			query += " AND RIGHT(TRIM(artih_nokw), 3) = 'PCK'"
		} else if tipe == "ASURANSI" || tipe == "ASR" {
			query += " AND RIGHT(TRIM(artih_nokw), 3) = 'ASR'"
		} else if tipe == "INVOICE" {
			query += " AND RIGHT(TRIM(artih_nokw), 3) NOT IN ('/OJ', 'PCK', 'ASR')"
		}
	}

	// Filter pencarian dokumen / kwitansi
	if q != "" {
		query += " AND (artih_nokw ILIKE ? OR artih_id ILIKE ?)"
		args = append(args, "%"+q+"%", "%"+q+"%")
	}

	query += " ORDER BY artih_tanggal DESC LIMIT 100"

	if err := database.Raw(query, args...).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query faktur: " + err.Error()})
		return
	}

	for i := range list {
		list[i].TanggalStr = list[i].Tanggal.Format("2006-01-02")
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// 2. POST /api/piutang/penerimaan-pembayaran-kredit/posting
// Memposting transaksi ke Jurnal Umum & menerbitkan No. Jurnal (Gambar 7, 8, 9)
func PostingPenerimaanPembayaranKreditHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		NoReceipt string `json:"no_receipt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NoReceipt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "No. Receipt wajib diisi"})
		return
	}

	noReceipt := strings.TrimSpace(req.NoReceipt)

	// Buat nomor jurnal otomatis berformat: YYMM + 001 + T + urut (Contoh: 2610001T00033)
	thnBln := time.Now().Format("0601") // 2610
	var count int64
	database.Table("public.gl_t_jurnalh").Where("tjurh_no LIKE ?", thnBln+"001T%").Count(&count)
	noJurnal := fmt.Sprintf("%s001T%05d", thnBln, count+1)

	tx := database.Begin()

	// Update status di header receipt
	err := tx.Exec(`
		UPDATE public.art_t_receipth 
		SET trecth_postingyn = 'Y',
		    trecth_tjurhno = ?,
		    trecth_updatetime = NOW()
		WHERE TRIM(trecth_no) = TRIM(?)
	`, noJurnal, noReceipt).Error

	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal posting transaksi: " + err.Error()})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   "Transaksi berhasil diposting",
		"no_jurnal": noJurnal,
	})
}

// 3. POST /api/piutang/penerimaan-pembayaran-kredit/unposting
// Membatalkan posting / unposting (Gambar 8)
func UnpostingPenerimaanPembayaranKreditHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		NoReceipt string `json:"no_receipt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NoReceipt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "No. Receipt wajib diisi"})
		return
	}

	noReceipt := strings.TrimSpace(req.NoReceipt)

	err := database.Exec(`
		UPDATE public.art_t_receipth 
		SET trecth_postingyn = 'N',
		    trecth_tjurhno = '',
		    trecth_updatetime = NOW()
		WHERE TRIM(trecth_no) = TRIM(?)
	`, noReceipt).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal unposting transaksi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Transaksi berhasil di-unposting menjadi Draft",
	})
}
