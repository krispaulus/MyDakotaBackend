package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type MutasiRekapRow struct {
	CabangNama float64 `json:"-"`
	CabangStr  string  `json:"cabang_nama"`
	CustID     string  `json:"cust_id"`
	CustName   string  `json:"cust_name"`
	SaldoAwal  float64 `json:"saldo_awal"`
	Transaksi  float64 `json:"transaksi"`
	Pembayaran float64 `json:"pembayaran"`
	SaldoAkhir float64 `json:"saldo_akhir"`
}

type MutasiDetailItem struct {
	Tanggal          string  `json:"tanggal"`
	NoBukti          string  `json:"no_bukti"`
	Tipe             string  `json:"tipe"` // INVOICE / RECEIPT / CREDIT_NOTE
	InvoiceNominal   float64 `json:"invoice_nominal"`
	TotalTerbayar    float64 `json:"total_terbayar"`
	PengurangPiutang float64 `json:"pengurang_piutang"`
	BayarKwitansi    float64 `json:"bayar_kwitansi"`
	SelisihPenambah  float64 `json:"selisih_penambah"`
	SaldoBerjalan    float64 `json:"saldo_berjalan"`
}

type MutasiCustomerDetailGroup struct {
	CustID           string             `json:"cust_id"`
	CustName         string             `json:"cust_name"`
	CabangNama       string             `json:"cabang_nama"`
	SaldoAwal        float64            `json:"saldo_awal"`
	SubTotalInvoice  float64            `json:"subtotal_invoice"`
	SubTotalTerbayar float64            `json:"subtotal_terbayar"`
	SubTotalPotongan float64            `json:"subtotal_potongan"`
	SubTotalKwitansi float64            `json:"subtotal_kwitansi"`
	SubTotalPenambah float64            `json:"subtotal_penambah"`
	SaldoAkhir       float64            `json:"saldo_akhir"`
	Details          []MutasiDetailItem `json:"details"`
}

type MutasiKartuRow struct {
	Tanggal     string  `json:"tanggal"`
	NoBukti     string  `json:"no_bukti"`
	Keterangan  string  `json:"keterangan"`
	Penjualan   float64 `json:"penjualan"`
	OrderJemput float64 `json:"order_jemput"`
	TagihTurun  float64 `json:"tagih_turun"`
	Pembayaran  float64 `json:"pembayaran"`
	Saldo       float64 `json:"saldo"`
}

type MutasiKartuGroup struct {
	CustID     string           `json:"cust_id"`
	CustName   string           `json:"cust_name"`
	CabangNama string           `json:"cabang_nama"`
	SaldoAwal  float64          `json:"saldo_awal"`
	SaldoAkhir float64          `json:"saldo_akhir"`
	Rows       []MutasiKartuRow `json:"rows"`
}

// GET /api/piutang/mutasi-piutang
func GetMutasiPiutangHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	cabangID := strings.TrimSpace(c.Query("cabang_id"))
	custID := strings.TrimSpace(c.Query("cust_id"))
	modeLaporan := strings.TrimSpace(c.Query("mode")) // 1 = Detail, 2 = Rekap, 3 = Kartu

	if startDate == "" || endDate == "" {
		startDate = time.Now().Format("2006-01") + "-01"
		endDate = time.Now().Format("2006-01-02")
	}

	// 1. Ambil daftar Customer aktif yang relevan
	type CustHead struct {
		CustID     string `gorm:"column:cust_id"`
		CustName   string `gorm:"column:cust_name"`
		CabangNama string `gorm:"column:agen_nama"`
	}

	custQuery := database.Table("public.mkt_m_customer c").
		Select(`
			c.cust_id,
			c.cust_name,
			COALESCE(a.agen_nama, 'PUSAT') AS agen_nama
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(c.cust_agenid::varchar)").
		Where("COALESCE(c.cust_aktifyn, 'Y') = 'Y'")

	if custID != "" && custID != "ALL" {
		custQuery = custQuery.Where("TRIM(c.cust_id::varchar) = TRIM(?)", custID)
	}
	if cabangID != "" && cabangID != "ALL" {
		custQuery = custQuery.Where("(TRIM(c.cust_agenid::varchar) = TRIM(?) OR TRIM(a.agen_cabangid::varchar) = TRIM(?))", cabangID, cabangID)
	}

	var custList []CustHead
	if err := custQuery.Order("c.cust_name ASC").Scan(&custList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query data customer: " + err.Error()})
		return
	}

	// Helper function untuk mengambil Saldo Awal secara akurat & aman
	getSaldoAwal := func(customerID string) float64 {
		var invLalu, bayarLalu, potongLalu, cnLalu float64

		// 1. Akumulasi Invoice sebelum start_date
		database.Table("public.art_t_invoiceh").
			Select("COALESCE(SUM(artih_total), 0) * 1.011").
			Where("TRIM(artih_custid::varchar) = TRIM(?) AND artih_tanggal < ? AND COALESCE(artih_delete, 'N') = 'N'", customerID, startDate+" 00:00:00").
			Scan(&invLalu)

		// 2. Akumulasi Penerimaan Kas/Bank sebelum start_date
		database.Table("public.art_t_receipth h").
			Joins("INNER JOIN public.art_t_receiptd d ON d.trectd_trecthno = h.trecth_no").
			Where("TRIM(h.trecth_custid::varchar) = TRIM(?) AND h.trecth_tanggal < ? AND COALESCE(h.trecth_deleteyn, 'N') = 'N'", customerID, startDate+" 00:00:00").
			Select("COALESCE(SUM(d.trectd_nilai), 0)").
			Scan(&bayarLalu)

		// 3. Akumulasi Potongan Pengurang sebelum start_date
		database.Table("public.art_t_hubrectd h").
			Joins("INNER JOIN public.art_t_receipth r ON r.trecth_no = h.thrd_trecthno").
			Where("TRIM(h.thrd_custid::varchar) = TRIM(?) AND h.thrd_kodelk = 'K' AND r.trecth_tanggal < ? AND COALESCE(r.trecth_deleteyn, 'N') = 'N'", customerID, startDate+" 00:00:00").
			Select("COALESCE(SUM(h.thrd_jumlah), 0)").
			Scan(&potongLalu)

		// 4. Akumulasi Credit Note sebelum start_date
		database.Table("public.art_t_creditnoteh").
			Where("TRIM(artcnh_custid::varchar) = TRIM(?) AND artcnh_tanggal < ? AND artcnh_postingyn = 'Y' AND COALESCE(artcnh_deleteyn, 'N') = 'N'", customerID, startDate+" 00:00:00").
			Select("COALESCE(SUM(artcnh_total), 0)").
			Scan(&cnLalu)

		// Saldo Awal = Piutang Lalu - Total Pembayaran/Potongan Lalu
		saldoAwal := invLalu - (bayarLalu + potongLalu + cnLalu)
		return saldoAwal
	}

	// ==========================================
	// MODE 2: REKAP
	// ==========================================
	if modeLaporan == "2" {
		var rekapList []MutasiRekapRow

		for _, ch := range custList {
			saldoAwal := getSaldoAwal(ch.CustID)

			// Hitung Total Invoice
			var totalInv float64
			database.Table("public.art_t_invoiceh").
				Select("COALESCE(SUM(artih_total), 0) * 1.011").
				Where("TRIM(artih_custid::varchar) = TRIM(?) AND artih_tanggal BETWEEN ? AND ? AND COALESCE(artih_delete, 'N') = 'N'", ch.CustID, startDate+" 00:00:00", endDate+" 23:59:59").
				Scan(&totalInv)

			// Hitung Total Pembayaran Receipt
			var totalBayar float64
			database.Table("public.art_t_receipth h").
				Joins("INNER JOIN public.art_t_receiptd d ON d.trectd_trecthno = h.trecth_no").
				Where("TRIM(h.trecth_custid::varchar) = TRIM(?) AND h.trecth_tanggal BETWEEN ? AND ? AND COALESCE(h.trecth_deleteyn, 'N') = 'N'", ch.CustID, startDate+" 00:00:00", endDate+" 23:59:59").
				Select("COALESCE(SUM(d.trectd_nilai), 0)").
				Scan(&totalBayar)

			// Hitung Potongan Pengurang
			var totalPengurang float64
			database.Table("public.art_t_hubrectd").
				Where("TRIM(thrd_custid::varchar) = TRIM(?) AND thrd_kodelk = 'K'", ch.CustID).
				Select("COALESCE(SUM(thrd_jumlah), 0)").
				Scan(&totalPengurang)

			// Hitung Credit Note
			var totalCN float64
			database.Table("public.art_t_creditnoteh").
				Where("TRIM(artcnh_custid::varchar) = TRIM(?) AND artcnh_tanggal BETWEEN ? AND ? AND artcnh_postingyn = 'Y' AND COALESCE(artcnh_deleteyn, 'N') = 'N'", ch.CustID, startDate+" 00:00:00", endDate+" 23:59:59").
				Select("COALESCE(SUM(artcnh_total), 0)").
				Scan(&totalCN)

			totalPengurangBayar := totalBayar + totalPengurang + totalCN
			saldoAkhir := (saldoAwal + totalInv) - totalPengurangBayar

			if saldoAwal != 0 || totalInv != 0 || totalPengurangBayar != 0 {
				rekapList = append(rekapList, MutasiRekapRow{
					CabangStr:  ch.CabangNama,
					CustID:     ch.CustID,
					CustName:   ch.CustName,
					SaldoAwal:  saldoAwal,
					Transaksi:  totalInv,
					Pembayaran: totalPengurangBayar,
					SaldoAkhir: saldoAkhir,
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"mode":   "2",
			"data":   rekapList,
		})
		return
	}

	// ==========================================
	// MODE 3: KARTU PIUTANG
	// ==========================================
	if modeLaporan == "3" {
		var kartuGroups []MutasiKartuGroup

		for _, ch := range custList {
			saldoAwal := getSaldoAwal(ch.CustID)
			saldoBerjalan := saldoAwal
			var rows []MutasiKartuRow

			// Invoice Rows
			type InvRow struct {
				ID      string    `gorm:"column:artih_id"`
				Tanggal time.Time `gorm:"column:artih_tanggal"`
				Total   float64   `gorm:"column:artih_total"`
			}
			var invRows []InvRow
			database.Table("public.art_t_invoiceh").
				Select("artih_id, artih_tanggal, artih_total").
				Where("TRIM(artih_custid::varchar) = TRIM(?) AND artih_tanggal BETWEEN ? AND ? AND COALESCE(artih_delete, 'N') = 'N'", ch.CustID, startDate+" 00:00:00", endDate+" 23:59:59").
				Order("artih_tanggal ASC, artih_id ASC").
				Scan(&invRows)

			for _, inv := range invRows {
				nominal := inv.Total * 1.011
				saldoBerjalan += nominal
				rows = append(rows, MutasiKartuRow{
					Tanggal:    inv.Tanggal.Format("2006-01-02"),
					NoBukti:    inv.ID,
					Keterangan: "Penjualan Kredit (Invoice)",
					Penjualan:  nominal,
					Saldo:      saldoBerjalan,
				})
			}

			// Receipt / Payment Rows
			type RectRow struct {
				No      string    `gorm:"column:trecth_no"`
				Tanggal time.Time `gorm:"column:trecth_tanggal"`
				Nilai   float64   `gorm:"column:nilai"`
			}
			var rectRows []RectRow
			database.Table("public.art_t_receipth h").
				Select("h.trecth_no, h.trecth_tanggal, COALESCE(SUM(d.trectd_nilai), 0) AS nilai").
				Joins("INNER JOIN public.art_t_receiptd d ON d.trectd_trecthno = h.trecth_no").
				Where("TRIM(h.trecth_custid::varchar) = TRIM(?) AND h.trecth_tanggal BETWEEN ? AND ? AND COALESCE(h.trecth_deleteyn, 'N') = 'N'", ch.CustID, startDate+" 00:00:00", endDate+" 23:59:59").
				Group("h.trecth_no, h.trecth_tanggal").
				Order("h.trecth_tanggal ASC").
				Scan(&rectRows)

			for _, r := range rectRows {
				saldoBerjalan -= r.Nilai
				rows = append(rows, MutasiKartuRow{
					Tanggal:    r.Tanggal.Format("2006-01-02"),
					NoBukti:    r.No,
					Keterangan: "Penerimaan Pembayaran Kasir",
					Pembayaran: r.Nilai,
					Saldo:      saldoBerjalan,
				})
			}

			if saldoAwal != 0 || len(rows) > 0 {
				kartuGroups = append(kartuGroups, MutasiKartuGroup{
					CustID:     ch.CustID,
					CustName:   ch.CustName,
					CabangNama: ch.CabangNama,
					SaldoAwal:  saldoAwal,
					SaldoAkhir: saldoBerjalan,
					Rows:       rows,
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"mode":   "3",
			"data":   kartuGroups,
		})
		return
	}

	// ==========================================
	// MODE 1: DETAIL (DEFAULT)
	// ==========================================
	var detailGroups []MutasiCustomerDetailGroup

	for _, ch := range custList {
		saldoAwal := getSaldoAwal(ch.CustID)
		var details []MutasiDetailItem
		var subInv, subTerima, subPotong, subKwitansi, subPenambah float64

		// A. Rincian Invoice
		type InvDetail struct {
			ID      string    `gorm:"column:artih_id"`
			Tanggal time.Time `gorm:"column:artih_tanggal"`
			Total   float64   `gorm:"column:artih_total"`
		}
		var invoices []InvDetail
		database.Table("public.art_t_invoiceh").
			Select("artih_id, artih_tanggal, artih_total").
			Where("TRIM(artih_custid::varchar) = TRIM(?) AND artih_tanggal BETWEEN ? AND ? AND COALESCE(artih_delete, 'N') = 'N'", ch.CustID, startDate+" 00:00:00", endDate+" 23:59:59").
			Order("artih_tanggal ASC, artih_id ASC").
			Scan(&invoices)

		for _, inv := range invoices {
			nom := inv.Total * 1.011
			subInv += nom
			details = append(details, MutasiDetailItem{
				Tanggal:        inv.Tanggal.Format("2006-01-02"),
				NoBukti:        inv.ID,
				Tipe:           "INVOICE",
				InvoiceNominal: nom,
			})
		}

		// B. Rincian Kwitansi Pembayaran
		type ReceiptDetail struct {
			No      string    `gorm:"column:trecth_no"`
			Tanggal time.Time `gorm:"column:trecth_tanggal"`
			Nilai   float64   `gorm:"column:nilai"`
			Terima  float64   `gorm:"column:terima"`
		}
		var receipts []ReceiptDetail
		database.Table("public.art_t_receipth h").
			Select(`
				h.trecth_no,
				h.trecth_tanggal,
				COALESCE(d.trectd_nilai, 0) AS nilai,
				COALESCE(SUM(dd.trectdd_jumlah), 0) AS terima
			`).
			Joins("LEFT JOIN public.art_t_receiptd d ON d.trectd_trecthno = h.trecth_no").
			Joins("LEFT JOIN public.art_t_receiptdd dd ON dd.trectdd_trectdtrecthno = h.trecth_no").
			Where("TRIM(h.trecth_custid::varchar) = TRIM(?) AND h.trecth_tanggal BETWEEN ? AND ? AND COALESCE(h.trecth_deleteyn, 'N') = 'N'", ch.CustID, startDate+" 00:00:00", endDate+" 23:59:59").
			Group("h.trecth_no, h.trecth_tanggal, d.trectd_nilai").
			Scan(&receipts)

		for _, rec := range receipts {
			var jmlPotong float64
			database.Table("public.art_t_hubrectd").
				Where("thrd_trecthno = ? AND thrd_kodelk = 'K'", rec.No).
				Select("COALESCE(SUM(thrd_jumlah), 0)").
				Scan(&jmlPotong)

			var jmlTambah float64
			database.Table("public.art_t_hubrectd").
				Where("thrd_trecthno = ? AND thrd_kodelk = 'L' AND thrd_caid = 'B106010300'", rec.No).
				Select("COALESCE(SUM(thrd_jumlah), 0)").
				Scan(&jmlTambah)

			terimaPpn := rec.Terima * 1.011
			subTerima += terimaPpn
			subPotong += jmlPotong
			subKwitansi += rec.Nilai
			subPenambah += jmlTambah

			details = append(details, MutasiDetailItem{
				Tanggal:          rec.Tanggal.Format("2006-01-02"),
				NoBukti:          rec.No,
				Tipe:             "RECEIPT",
				TotalTerbayar:    terimaPpn,
				PengurangPiutang: jmlPotong,
				BayarKwitansi:    rec.Nilai,
				SelisihPenambah:  jmlTambah,
			})
		}

		// C. Rincian Credit Note
		type CNDetail struct {
			No      string    `gorm:"column:artcnh_no"`
			Tanggal time.Time `gorm:"column:artcnh_tanggal"`
			Total   float64   `gorm:"column:artcnh_total"`
		}
		var cns []CNDetail
		database.Table("public.art_t_creditnoteh").
			Select("artcnh_no, artcnh_tanggal, artcnh_total").
			Where("TRIM(artcnh_custid::varchar) = TRIM(?) AND artcnh_tanggal BETWEEN ? AND ? AND artcnh_postingyn = 'Y' AND COALESCE(artcnh_deleteyn, 'N') = 'N'", ch.CustID, startDate+" 00:00:00", endDate+" 23:59:59").
			Scan(&cns)

		for _, cn := range cns {
			subPotong += cn.Total
			details = append(details, MutasiDetailItem{
				Tanggal:          cn.Tanggal.Format("2006-01-02"),
				NoBukti:          "CN - " + cn.No,
				Tipe:             "CREDIT_NOTE",
				PengurangPiutang: cn.Total,
			})
		}

		saldoAkhir := (saldoAwal + subInv) - (subKwitansi + subPotong + subPenambah)

		if saldoAwal != 0 || len(details) > 0 {
			detailGroups = append(detailGroups, MutasiCustomerDetailGroup{
				CustID:           ch.CustID,
				CustName:         ch.CustName,
				CabangNama:       ch.CabangNama,
				SaldoAwal:        saldoAwal,
				SubTotalInvoice:  subInv,
				SubTotalTerbayar: subTerima,
				SubTotalPotongan: subPotong,
				SubTotalKwitansi: subKwitansi,
				SubTotalPenambah: subPenambah,
				SaldoAkhir:       saldoAkhir,
				Details:          details,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"mode":   "1",
		"data":   detailGroups,
	})
}

// GET /api/customers & GET /api/marketing/customers
func GetCustomerDropdownHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	type CustOption struct {
		CustID   string `json:"cust_id" gorm:"column:cust_id"`
		CustName string `json:"cust_name" gorm:"column:cust_name"`
		AgenID   string `json:"cust_agenid" gorm:"column:cust_agenid"`
	}

	var customers []CustOption
	database.Table("public.mkt_m_customer").
		Select("cust_id, cust_name, COALESCE(cust_agenid, '') AS cust_agenid").
		Where("COALESCE(cust_aktifyn, 'Y') = 'Y'").
		Order("cust_name ASC").
		Scan(&customers)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   customers,
	})
}
