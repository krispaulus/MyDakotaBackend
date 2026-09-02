package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type ProsesPiutangLog struct {
	CustID       string  `json:"cust_id"`
	CustName     string  `json:"cust_name"`
	SaldoAwal    float64 `json:"saldo_awal"`
	TotalInvoice float64 `json:"total_invoice"`
	TotalBayar   float64 `json:"total_bayar"`
	SaldoAkhir   float64 `json:"saldo_akhir"`
}

// POST /api/piutang/proses-piutang/eksekusi
func ExecuteProsesPiutangHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var input struct {
		AgenID string `json:"agen_id"`
		Bulan  string `json:"bulan"` // format: "01", "02", ... "12"
		Tahun  string `json:"tahun"` // format: "2026"
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	bulanInt, err := strconv.Atoi(input.Bulan)
	if err != nil || bulanInt < 1 || bulanInt > 12 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Bulan tidak valid"})
		return
	}
	formattedBulan := fmt.Sprintf("%02d", bulanInt)
	nmField := "sa_bln" + formattedBulan

	// 1. Tentukan nama kolom saldo awal berdasarkan bulan
	kolomSaldoAwal := "sa_awal"
	if bulanInt > 1 {
		kolomSaldoAwal = fmt.Sprintf("sa_bln%02d", bulanInt-1)
	}

	// 2. Ambil list customer aktif pada agen/cabang terkait
	type CustomerItem struct {
		CustID   string `gorm:"column:cust_id"`
		CustName string `gorm:"column:cust_name"`
		AgenID   string `gorm:"column:agen_id"`
	}

	var custList []CustomerItem
	custQuery := `
		SELECT 
			c.cust_id, 
			c.cust_name, 
			COALESCE(c.cust_agenid, a.agen_id::varchar) AS agen_id
		FROM public.mkt_m_customer c
		LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(c.cust_agenid::varchar)
		WHERE c.cust_aktifyn = 'Y' 
		  AND (TRIM(a.agen_id::varchar) = TRIM(?) OR TRIM(a.agen_cabangid::varchar) = TRIM(?))
		ORDER BY c.cust_name ASC
	`
	if err := database.Raw(custQuery, input.AgenID, input.AgenID).Scan(&custList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membaca data customer: " + err.Error()})
		return
	}

	tx := database.Begin()
	var logs []ProsesPiutangLog

	for _, cust := range custList {
		custID := strings.TrimSpace(cust.CustID)
		custAgenID := strings.TrimSpace(cust.AgenID)
		if custAgenID == "" {
			custAgenID = input.AgenID
		}

		// a. Pastikan row ar_t_sapiutang sudah tersedia
		var countRecord int64
		tx.Table("public.ar_t_sapiutang").
			Where("sa_tahun = ? AND sa_custid = ? AND sa_agenid = ?", input.Tahun, custID, custAgenID).
			Count(&countRecord)

		if countRecord == 0 {
			insertInit := `
				INSERT INTO public.ar_t_sapiutang (
					sa_tahun, sa_custid, sa_agenid, sa_awal, 
					sa_bln01, sa_bln02, sa_bln03, sa_bln04, sa_bln05, sa_bln06, 
					sa_bln07, sa_bln08, sa_bln09, sa_bln10, sa_bln11, sa_bln12
				) VALUES (?, ?, ?, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
			`
			if err := tx.Exec(insertInit, input.Tahun, custID, custAgenID).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal inisialisasi saldo: " + err.Error()})
				return
			}
		}

		// b. Ambil Saldo Awal
		var saldoAwal float64
		getSaldoQuery := fmt.Sprintf("SELECT COALESCE(%s, 0) FROM public.ar_t_sapiutang WHERE sa_tahun = ? AND sa_custid = ? AND sa_agenid = ?", kolomSaldoAwal)
		tx.Raw(getSaldoQuery, input.Tahun, custID, custAgenID).Scan(&saldoAwal)

		// c. Hitung Total Invoice Penambahan Tagihan
		var totalInvoiceRaw float64
		invQuery := `
			SELECT COALESCE(SUM(artih_total), 0)
			FROM public.art_t_invoiceh
			WHERE TRIM(artih_custid) = TRIM(?)
			  AND EXTRACT(MONTH FROM artih_tanggal) = ?
			  AND EXTRACT(YEAR FROM artih_tanggal) = ?
			  AND COALESCE(artih_delete, 'N') = 'N'
		`
		tx.Raw(invQuery, custID, bulanInt, input.Tahun).Scan(&totalInvoiceRaw)
		totalInvoice := totalInvoiceRaw * 1.011

		// d. Hitung Total Pembayaran Penerimaan Kas/Bank
		var totalBayarKas float64
		bayarQuery := `
			SELECT COALESCE(SUM(rd.trectd_nilai), 0)
			FROM public.art_t_receipth rh
			INNER JOIN public.art_t_receiptd rd ON TRIM(rd.trectd_trecthno) = TRIM(rh.trecth_no)
			INNER JOIN public.art_t_receiptdd rdd ON TRIM(rdd.trectdd_trectdtrecthno) = TRIM(rh.trecth_no)
			WHERE TRIM(rh.trecth_custid) = TRIM(?)
			  AND EXTRACT(MONTH FROM rh.trecth_tanggal) = ?
			  AND EXTRACT(YEAR FROM rh.trecth_tanggal) = ?
			  AND COALESCE(rh.trecth_deleteyn, 'N') = 'N'
		`
		tx.Raw(bayarQuery, custID, bulanInt, input.Tahun).Scan(&totalBayarKas)

		// e. Hitung Pengurang & Penambah Koreksi (art_t_hubrectd)
		var pengurangPiutang float64
		kurangQuery := `
			SELECT COALESCE(SUM(hrd.thrd_jumlah), 0)
			FROM public.art_t_hubrectd hrd
			INNER JOIN public.art_t_receipth rh ON TRIM(hrd.thrd_trecthno) = TRIM(rh.trecth_no)
			WHERE TRIM(hrd.thrd_custid) = TRIM(?)
			  AND hrd.thrd_kodelk = 'K'
			  AND EXTRACT(MONTH FROM rh.trecth_tanggal) = ?
			  AND EXTRACT(YEAR FROM rh.trecth_tanggal) = ?
			  AND COALESCE(rh.trecth_deleteyn, 'N') = 'N'
		`
		tx.Raw(kurangQuery, custID, bulanInt, input.Tahun).Scan(&pengurangPiutang)

		var penambahPiutang float64
		tambahQuery := `
			SELECT COALESCE(SUM(hrd.thrd_jumlah), 0)
			FROM public.art_t_hubrectd hrd
			INNER JOIN public.art_t_receipth rh ON TRIM(hrd.thrd_trecthno) = TRIM(rh.trecth_no)
			WHERE TRIM(hrd.thrd_custid) = TRIM(?)
			  AND hrd.thrd_kodelk = 'L'
			  AND TRIM(hrd.thrd_caid) = 'B106010300'
			  AND EXTRACT(MONTH FROM rh.trecth_tanggal) = ?
			  AND EXTRACT(YEAR FROM rh.trecth_tanggal) = ?
			  AND COALESCE(rh.trecth_deleteyn, 'N') = 'N'
		`
		tx.Raw(tambahQuery, custID, bulanInt, input.Tahun).Scan(&penambahPiutang)

		totalBayar := totalBayarKas + pengurangPiutang + penambahPiutang

		// f. Saldo Akhir Bulan
		saldoAkhir := (saldoAwal + totalInvoice) - totalBayar

		// g. Update Saldo ke tabel ar_t_sapiutang
		updQuery := fmt.Sprintf(`
			UPDATE public.ar_t_sapiutang 
			SET %s = ? 
			WHERE sa_tahun = ? AND sa_custid = ? AND sa_agenid = ?
		`, nmField)
		if err := tx.Exec(updQuery, saldoAkhir, input.Tahun, custID, custAgenID).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan saldo bulan: " + err.Error()})
			return
		}

		logs = append(logs, ProsesPiutangLog{
			CustID:       custID,
			CustName:     cust.CustName,
			SaldoAwal:    saldoAwal,
			TotalInvoice: totalInvoice,
			TotalBayar:   totalBayar,
			SaldoAkhir:   saldoAkhir,
		})
	}

	// 3. Roll-Over Akhir Tahun (Khusus Bulan 12)
	if bulanInt == 12 {
		thnInt, _ := strconv.Atoi(input.Tahun)
		thnNext := strconv.Itoa(thnInt + 1)

		type CustSumRow struct {
			CustID     string  `gorm:"column:sa_custid"`
			AgenID     string  `gorm:"column:sa_agenid"`
			SaldoAkhir float64 `gorm:"column:saldoakhir"`
		}
		var custSumList []CustSumRow
		sumQuery := `
			SELECT 
				sa.sa_custid, 
				sa.sa_agenid, 
				(COALESCE(sa.sa_awal, 0) + COALESCE(sa.sa_bln01, 0) + COALESCE(sa.sa_bln02, 0) + COALESCE(sa.sa_bln03, 0) + 
				 COALESCE(sa.sa_bln04, 0) + COALESCE(sa.sa_bln05, 0) + COALESCE(sa.sa_bln06, 0) + COALESCE(sa.sa_bln07, 0) + 
				 COALESCE(sa.sa_bln08, 0) + COALESCE(sa.sa_bln09, 0) + COALESCE(sa.sa_bln10, 0) + COALESCE(sa.sa_bln11, 0) + 
				 COALESCE(sa.sa_bln12, 0)) AS saldoakhir
			FROM public.ar_t_sapiutang sa
			WHERE sa.sa_tahun = ? AND sa.sa_agenid = ?
		`
		tx.Raw(sumQuery, input.Tahun, input.AgenID).Scan(&custSumList)

		for _, cs := range custSumList {
			var existNext int64
			tx.Table("public.ar_t_sapiutang").
				Where("sa_tahun = ? AND sa_custid = ? AND sa_agenid = ?", thnNext, cs.CustID, cs.AgenID).
				Count(&existNext)

			if existNext == 0 {
				insNext := `
					INSERT INTO public.ar_t_sapiutang (
						sa_tahun, sa_custid, sa_agenid, sa_awal, 
						sa_bln01, sa_bln02, sa_bln03, sa_bln04, sa_bln05, sa_bln06, 
						sa_bln07, sa_bln08, sa_bln09, sa_bln10, sa_bln11, sa_bln12
					) VALUES (?, ?, ?, ?, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
				`
				tx.Exec(insNext, thnNext, cs.CustID, cs.AgenID, cs.SaldoAkhir)
			} else {
				tx.Exec("UPDATE public.ar_t_sapiutang SET sa_awal = ? WHERE sa_tahun = ? AND sa_custid = ? AND sa_agenid = ?", cs.SaldoAkhir, thnNext, cs.CustID, cs.AgenID)
			}
		}
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":         "success",
		"message":        fmt.Sprintf("Proses rekalkulasi piutang periode %s/%s berhasil diselesaikan", formattedBulan, input.Tahun),
		"total_customer": len(logs),
		"data":           logs,
	})
}

// GET /api/piutang/proses-piutang/histori
func GetHistoriSaldoPiutangHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	agenID := strings.TrimSpace(c.Query("agen_id"))
	tahun := strings.TrimSpace(c.Query("tahun"))
	if tahun == "" {
		tahun = "2026"
	}

	type HistoriRow struct {
		Tahun    string  `json:"sa_tahun" gorm:"column:sa_tahun"`
		CustID   string  `json:"sa_custid" gorm:"column:sa_custid"`
		CustName string  `json:"cust_name" gorm:"column:cust_name"`
		AgenID   string  `json:"sa_agenid" gorm:"column:sa_agenid"`
		AgenNama string  `json:"agen_nama" gorm:"column:agen_nama"`
		Awal     float64 `json:"sa_awal" gorm:"column:sa_awal"`
		Bln01    float64 `json:"sa_bln01" gorm:"column:sa_bln01"`
		Bln02    float64 `json:"sa_bln02" gorm:"column:sa_bln02"`
		Bln03    float64 `json:"sa_bln03" gorm:"column:sa_bln03"`
		Bln04    float64 `json:"sa_bln04" gorm:"column:sa_bln04"`
		Bln05    float64 `json:"sa_bln05" gorm:"column:sa_bln05"`
		Bln06    float64 `json:"sa_bln06" gorm:"column:sa_bln06"`
		Bln07    float64 `json:"sa_bln07" gorm:"column:sa_bln07"`
		Bln08    float64 `json:"sa_bln08" gorm:"column:sa_bln08"`
		Bln09    float64 `json:"sa_bln09" gorm:"column:sa_bln09"`
		Bln10    float64 `json:"sa_bln10" gorm:"column:sa_bln10"`
		Bln11    float64 `json:"sa_bln11" gorm:"column:sa_bln11"`
		Bln12    float64 `json:"sa_bln12" gorm:"column:sa_bln12"`
	}

	rawQuery := `
		SELECT 
			sa.sa_tahun,
			sa.sa_custid,
			COALESCE(c.cust_name, sa.sa_custid) AS cust_name,
			sa.sa_agenid,
			COALESCE(a.agen_nama, sa.sa_agenid) AS agen_nama,
			COALESCE(sa.sa_awal, 0) AS sa_awal,
			COALESCE(sa.sa_bln01, 0) AS sa_bln01,
			COALESCE(sa.sa_bln02, 0) AS sa_bln02,
			COALESCE(sa.sa_bln03, 0) AS sa_bln03,
			COALESCE(sa.sa_bln04, 0) AS sa_bln04,
			COALESCE(sa.sa_bln05, 0) AS sa_bln05,
			COALESCE(sa.sa_bln06, 0) AS sa_bln06,
			COALESCE(sa.sa_bln07, 0) AS sa_bln07,
			COALESCE(sa.sa_bln08, 0) AS sa_bln08,
			COALESCE(sa.sa_bln09, 0) AS sa_bln09,
			COALESCE(sa.sa_bln10, 0) AS sa_bln10,
			COALESCE(sa.sa_bln11, 0) AS sa_bln11,
			COALESCE(sa.sa_bln12, 0) AS sa_bln12
		FROM public.ar_t_sapiutang sa
		LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id::varchar) = TRIM(sa.sa_custid::varchar)
		LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(sa.sa_agenid::varchar)
		WHERE sa.sa_tahun = ?
	`
	var args []interface{}
	args = append(args, tahun)

	if agenID != "" && agenID != "ALL" {
		rawQuery += " AND (TRIM(a.agen_id::varchar) = TRIM(?) OR TRIM(a.agen_cabangid::varchar) = TRIM(?))"
		args = append(args, agenID, agenID)
	}

	rawQuery += " ORDER BY c.cust_name ASC LIMIT 500"

	list := make([]HistoriRow, 0)
	if err := database.Raw(rawQuery, args...).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat histori saldo: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// GET /api/piutang/proses-piutang/detail-customer
func GetDetailProsesPiutangCustomerHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	custID := strings.TrimSpace(c.Query("cust_id"))
	bulanStr := strings.TrimSpace(c.Query("bulan"))
	tahunStr := strings.TrimSpace(c.Query("tahun"))

	bulanInt, _ := strconv.Atoi(bulanStr)

	// 1. Ambil Rincian Invoice Tagihan Bulan Ini
	type InvoiceRow struct {
		InvoiceNo  string  `json:"invoice_no" gorm:"column:artih_id"`
		Tanggal    string  `json:"tanggal" gorm:"column:tanggal"`
		Total      float64 `json:"total" gorm:"column:artih_total"`
		Keterangan string  `json:"keterangan" gorm:"column:artih_keterangan"`
	}
	var invoices []InvoiceRow
	invQuery := `
		SELECT 
			artih_id,
			TO_CHAR(artih_tanggal, 'YYYY-MM-DD') AS tanggal,
			COALESCE(artih_total, 0) AS artih_total,
			COALESCE(artih_keterangan, '-') AS artih_keterangan
		FROM public.art_t_invoiceh
		WHERE TRIM(artih_custid) = TRIM(?)
		  AND EXTRACT(MONTH FROM artih_tanggal) = ?
		  AND EXTRACT(YEAR FROM artih_tanggal) = ?
		  AND COALESCE(artih_delete, 'N') = 'N'
		ORDER BY artih_tanggal ASC
	`
	database.Raw(invQuery, custID, bulanInt, tahunStr).Scan(&invoices)

	// 2. Ambil Rincian Pembayaran / Kuitansi Bulan Ini
	type ReceiptRow struct {
		ReceiptNo  string  `json:"receipt_no" gorm:"column:trecth_no"`
		Tanggal    string  `json:"tanggal" gorm:"column:tanggal"`
		NilaiBayar float64 `json:"nilai_bayar" gorm:"column:nilai_bayar"`
		AkunKas    string  `json:"akun_kas" gorm:"column:akun_kas"`
	}
	var receipts []ReceiptRow
	recQuery := `
		SELECT 
			rh.trecth_no,
			TO_CHAR(rh.trecth_tanggal, 'YYYY-MM-DD') AS tanggal,
			COALESCE(rd.trectd_nilai, 0) AS nilai_bayar,
			COALESCE(rd.trectd_caid, '-') AS akun_kas
		FROM public.art_t_receipth rh
		INNER JOIN public.art_t_receiptd rd ON TRIM(rd.trectd_trecthno) = TRIM(rh.trecth_no)
		WHERE TRIM(rh.trecth_custid) = TRIM(?)
		  AND EXTRACT(MONTH FROM rh.trecth_tanggal) = ?
		  AND EXTRACT(YEAR FROM rh.trecth_tanggal) = ?
		  AND COALESCE(rh.trecth_deleteyn, 'N') = 'N'
		ORDER BY rh.trecth_tanggal ASC
	`
	database.Raw(recQuery, custID, bulanInt, tahunStr).Scan(&receipts)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"invoices": invoices,
			"receipts": receipts,
		},
	})
}

// GET /api/piutang/proses-piutang/preview
func PreviewProsesPiutangHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	agenID := strings.TrimSpace(c.Query("agen_id"))
	bulanStr := strings.TrimSpace(c.Query("bulan"))
	tahunStr := strings.TrimSpace(c.Query("tahun"))

	if tahunStr == "" {
		tahunStr = "2026"
	}
	bulanInt, err := strconv.Atoi(bulanStr)
	if err != nil || bulanInt < 1 || bulanInt > 12 {
		bulanInt = 1
	}

	kolomSaldoAwal := "sa_awal"
	if bulanInt > 1 {
		kolomSaldoAwal = fmt.Sprintf("sa_bln%02d", bulanInt-1)
	}

	type CustomerItem struct {
		CustID   string `gorm:"column:cust_id"`
		CustName string `gorm:"column:cust_name"`
		AgenID   string `gorm:"column:agen_id"`
	}

	var custList []CustomerItem
	custQuery := `
		SELECT 
			c.cust_id, 
			c.cust_name, 
			COALESCE(c.cust_agenid, a.agen_id::varchar) AS agen_id
		FROM public.mkt_m_customer c
		LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(c.cust_agenid::varchar)
		WHERE c.cust_aktifyn = 'Y' 
		  AND (TRIM(a.agen_id::varchar) = TRIM(?) OR TRIM(a.agen_cabangid::varchar) = TRIM(?))
		ORDER BY c.cust_name ASC
	`
	database.Raw(custQuery, agenID, agenID).Scan(&custList)

	type PreviewRow struct {
		CustID        string  `json:"cust_id"`
		CustName      string  `json:"cust_name"`
		SaldoAwal     float64 `json:"saldo_awal"`
		TotalInvoice  float64 `json:"total_invoice"`
		CountInvoice  int64   `json:"count_invoice"`
		TotalBayar    float64 `json:"total_bayar"`
		CountBayar    int64   `json:"count_bayar"`
		EstimasiAkhir float64 `json:"estimasi_akhir"`
	}

	previewList := make([]PreviewRow, 0)

	for _, cust := range custList {
		custID := strings.TrimSpace(cust.CustID)
		custAgenID := strings.TrimSpace(cust.AgenID)
		if custAgenID == "" {
			custAgenID = agenID
		}

		// 1. Saldo Awal
		var saldoAwal float64
		getSaldoQuery := fmt.Sprintf("SELECT COALESCE(%s, 0) FROM public.ar_t_sapiutang WHERE sa_tahun = ? AND sa_custid = ? AND sa_agenid = ?", kolomSaldoAwal)
		database.Raw(getSaldoQuery, tahunStr, custID, custAgenID).Scan(&saldoAwal)

		// 2. Total Invoice
		var totalInvRaw float64
		var countInv int64
		invQuery := `
			SELECT COALESCE(SUM(artih_total), 0), COUNT(artih_id)
			FROM public.art_t_invoiceh
			WHERE TRIM(artih_custid) = TRIM(?)
			  AND EXTRACT(MONTH FROM artih_tanggal) = ?
			  AND EXTRACT(YEAR FROM artih_tanggal) = ?
			  AND COALESCE(artih_delete, 'N') = 'N'
		`
		database.Raw(invQuery, custID, bulanInt, tahunStr).Row().Scan(&totalInvRaw, &countInv)
		totalInvoice := totalInvRaw * 1.011

		// 3. Total Bayar
		var totalBayarKas float64
		var countBayar int64
		bayarQuery := `
			SELECT COALESCE(SUM(rd.trectd_nilai), 0), COUNT(DISTINCT rh.trecth_no)
			FROM public.art_t_receipth rh
			INNER JOIN public.art_t_receiptd rd ON TRIM(rd.trectd_trecthno) = TRIM(rh.trecth_no)
			WHERE TRIM(rh.trecth_custid) = TRIM(?)
			  AND EXTRACT(MONTH FROM rh.trecth_tanggal) = ?
			  AND EXTRACT(YEAR FROM rh.trecth_tanggal) = ?
			  AND COALESCE(rh.trecth_deleteyn, 'N') = 'N'
		`
		database.Raw(bayarQuery, custID, bulanInt, tahunStr).Row().Scan(&totalBayarKas, &countBayar)

		var pengurangPiutang float64
		kurangQuery := `
			SELECT COALESCE(SUM(hrd.thrd_jumlah), 0)
			FROM public.art_t_hubrectd hrd
			INNER JOIN public.art_t_receipth rh ON TRIM(hrd.thrd_trecthno) = TRIM(rh.trecth_no)
			WHERE TRIM(hrd.thrd_custid) = TRIM(?)
			  AND hrd.thrd_kodelk = 'K'
			  AND EXTRACT(MONTH FROM rh.trecth_tanggal) = ?
			  AND EXTRACT(YEAR FROM rh.trecth_tanggal) = ?
			  AND COALESCE(rh.trecth_deleteyn, 'N') = 'N'
		`
		database.Raw(kurangQuery, custID, bulanInt, tahunStr).Scan(&pengurangPiutang)

		var penambahPiutang float64
		tambahQuery := `
			SELECT COALESCE(SUM(hrd.thrd_jumlah), 0)
			FROM public.art_t_hubrectd hrd
			INNER JOIN public.art_t_receipth rh ON TRIM(hrd.thrd_trecthno) = TRIM(rh.trecth_no)
			WHERE TRIM(hrd.thrd_custid) = TRIM(?)
			  AND hrd.thrd_kodelk = 'L'
			  AND TRIM(hrd.thrd_caid) = 'B106010300'
			  AND EXTRACT(MONTH FROM rh.trecth_tanggal) = ?
			  AND EXTRACT(YEAR FROM rh.trecth_tanggal) = ?
			  AND COALESCE(rh.trecth_deleteyn, 'N') = 'N'
		`
		database.Raw(tambahQuery, custID, bulanInt, tahunStr).Scan(&penambahPiutang)

		totalBayar := totalBayarKas + pengurangPiutang + penambahPiutang
		estimasiAkhir := (saldoAwal + totalInvoice) - totalBayar

		previewList = append(previewList, PreviewRow{
			CustID:        custID,
			CustName:      cust.CustName,
			SaldoAwal:     saldoAwal,
			TotalInvoice:  totalInvoice,
			CountInvoice:  countInv,
			TotalBayar:    totalBayar,
			CountBayar:    countBayar,
			EstimasiAkhir: estimasiAkhir,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   previewList,
	})
}
