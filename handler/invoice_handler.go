package handler

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type InvoiceListRow struct {
	ARTIHID         string     `json:"artih_id" gorm:"column:artih_id"`
	ARTIHTanggal    time.Time  `json:"artih_tanggal" gorm:"column:artih_tanggal"`
	ARTIHCustID     string     `json:"artih_custid" gorm:"column:artih_custid"`
	CustName        string     `json:"cust_name" gorm:"column:cust_name"`
	ARTIHCustName   string     `json:"artih_custname" gorm:"column:artih_custname"`
	ARTIHKeterangan string     `json:"artih_keterangan" gorm:"column:artih_keterangan"`
	ARTIHDelete     string     `json:"artih_delete" gorm:"column:artih_delete"`
	ARTIHNoKW       string     `json:"artih_nokw" gorm:"column:artih_nokw"`
	ARTIHFktPajak   string     `json:"artih_fktpajak" gorm:"column:artih_fktpajak"`
	ARTIHTotal      float64    `json:"artih_total" gorm:"column:artih_total"`
	Terbayar        float64    `json:"terbayar" gorm:"column:terbayar"`
	ARTIHJenis      string     `json:"artih_jenis" gorm:"column:artih_jenis"`
	ARTIHPostingYN  string     `json:"artih_postingyn" gorm:"column:artih_postingyn"`
	ARTIHJournalID  *string    `json:"artih_journalid" gorm:"column:artih_journalid"` // <-- Tambahkan ini
	ARTIHTglACC     *time.Time `json:"artih_tglacc" gorm:"column:artih_tglacc"`
	AgenNama        string     `json:"agen_nama" gorm:"column:agen_nama"`
}

type InvoiceBTTDetailRow struct {
	BTTTID           string    `json:"bttt_id" gorm:"column:bttt_id"`
	BTTTServID       string    `json:"bttt_servid" gorm:"column:bttt_servid"`
	BTTTTanggal      time.Time `json:"bttt_tanggal" gorm:"column:bttt_tanggal"`
	BTTTAsalName     string    `json:"bttt_asalname" gorm:"column:bttt_asalname"`
	BTTTTujuanNama   string    `json:"bttt_tujuannama" gorm:"column:bttt_tujuannama"`
	BTTTTujuanKota   string    `json:"bttt_tujuankota" gorm:"column:bttt_tujuankota"`
	BTTTNoSuratJalan string    `json:"bttt_nosuratjalan" gorm:"column:bttt_nosuratjalan"`
	BTTTNamaBarang   string    `json:"bttt_namabarang" gorm:"column:bttt_namabarang"`
	BTTTJmlUnit      float64   `json:"bttt_jmlunit" gorm:"column:bttt_jmlunit"`
	BTTTBerat        float64   `json:"bttt_berat" gorm:"column:bttt_berat"`
	BTTTUkuran       float64   `json:"bttt_ukuran" gorm:"column:bttt_ukuran"`
	BTTTHarga        float64   `json:"bttt_harga" gorm:"column:bttt_harga"`
	BTTTBiayaPenerus float64   `json:"bttt_biayapenerus" gorm:"column:bttt_biayapenerus"`
	BiayaPacking     float64   `json:"biaya_packing" gorm:"column:biaya_packing"`
	BiayaAsuransi    float64   `json:"biaya_asuransi" gorm:"column:biaya_asuransi"`
	NoSKB            string    `json:"no_skb" gorm:"column:no_skb"`
	Subtotal         float64   `json:"subtotal" gorm:"column:subtotal"`
}

type CreateInvoiceReq struct {
	ARTIHTanggal    string   `json:"artih_tanggal" binding:"required"`
	ARTIHCustID     string   `json:"artih_custid" binding:"required"`
	ARTIHCustName   string   `json:"artih_custname"`
	ARTIHAgenID     string   `json:"artih_agenid"`
	ARTIHJenis      string   `json:"artih_jenis" binding:"required"` // K, B, T
	ARTIHFktPajak   string   `json:"artih_fktpajak"`
	ARTIHKeterangan string   `json:"artih_keterangan"`
	BTTList         []string `json:"btt_list" binding:"required"`
}

// Helper konversi nama/kode cabang ke 3-digit kode agen resmi Dakota
func resolveKodeAgenNumeric(agenID string, agenNama string) string {
	cleanID := strings.TrimSpace(strings.ToUpper(agenID))
	cleanNama := strings.TrimSpace(strings.ToUpper(agenNama))

	// Mapping paten cabang utama
	if cleanID == "001" || cleanID == "1" || cleanID == "OTA" || strings.Contains(cleanNama, "PUSAT") {
		return "001" // DLI Pusat
	}
	if cleanID == "002" || cleanID == "2" || strings.Contains(cleanNama, "SURABAYA") {
		return "002" // DLI Surabaya
	}
	if cleanID == "006" || cleanID == "6" || strings.Contains(cleanNama, "SEMARANG") {
		return "006" // DLI Semarang
	}

	// Jika berupa angka lain, pastikan pad 3 digit (misal: "3" -> "003")
	numericOnly := regexp.MustCompile(`\D`).ReplaceAllString(cleanID, "")
	if numericOnly != "" {
		if num, err := strconv.Atoi(numericOnly); err == nil {
			return fmt.Sprintf("%03d", num)
		}
	}

	return "001" // Default ke DLI Pusat
}

// 1. GET /api/piutang/invoice (List Grid Invoice)
func GetInvoiceListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	agenID := strings.TrimSpace(c.Query("agen_id"))
	customer := strings.TrimSpace(c.Query("customer"))
	noInvoice := strings.TrimSpace(c.Query("no_invoice"))
	noKwitansi := strings.TrimSpace(c.Query("no_kwitansi"))
	noBTT := strings.TrimSpace(c.Query("no_btt"))
	jenis := strings.TrimSpace(c.Query("jenis"))
	terbayar := strings.TrimSpace(c.Query("terbayar"))
	bypassTanggal := c.Query("bypass_tanggal") == "true" || c.Query("bypass_tanggal") == "1"

	query := database.Table("public.art_t_invoiceh h").
		Select(`
			h.artih_id,
			h.artih_tanggal,
			h.artih_custid,
			COALESCE(c.cust_name, h.artih_custname) AS cust_name,
			COALESCE(h.artih_custname, c.cust_name) AS artih_custname,
			COALESCE(h.artih_keterangan, '') AS artih_keterangan,
			COALESCE(h.artih_delete, 'N') AS artih_delete,
			COALESCE(h.artih_nokw, '') AS artih_nokw,
			COALESCE(h.artih_fktpajak, '') AS artih_fktpajak,
			COALESCE(h.artih_total, 0) AS artih_total,
			COALESCE(rp.total_terbayar, 0) AS terbayar,
			COALESCE(h.artih_jenis, 'K') AS artih_jenis,
			COALESCE(h.artih_postingyn, 'N') AS artih_postingyn,
			COALESCE(h.artih_journalid, '') AS artih_journalid,
			h.artih_tglacc,
			COALESCE(a.agen_nama, 'DLI PUSAT') AS agen_nama
		`).
		Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id) = TRIM(h.artih_custid)").
		Joins("LEFT JOIN public.glb_m_agen a ON a.agen_id::varchar = LPAD(LEFT(h.artih_id, 3), 3, '0')").
		Joins(`LEFT JOIN (
			SELECT d.artid_artihid, SUM(r.trectddd_bayar) AS total_terbayar
			FROM public.art_t_invoiced d
			LEFT JOIN public.art_t_receiptdddd r ON TRIM(r.trectddd_nobtt) = TRIM(d.artid_bttid)
			GROUP BY d.artid_artihid
		) rp ON TRIM(rp.artid_artihid) = TRIM(h.artih_id)`).
		Where("COALESCE(h.artih_delete, 'N') = 'N'")

	if !bypassTanggal && startDate != "" && endDate != "" {
		query = query.Where("h.artih_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if agenID != "" && agenID != "ALL" {
		query = query.Where("LPAD(LEFT(h.artih_id, 3), 3, '0') = LPAD(?, 3, '0')", agenID)
	}
	if customer != "" {
		query = query.Where("(c.cust_name ILIKE ? OR h.artih_custname ILIKE ? OR h.artih_custid ILIKE ?)", "%"+customer+"%", "%"+customer+"%", "%"+customer+"%")
	}
	if noInvoice != "" {
		query = query.Where("h.artih_id ILIKE ?", "%"+noInvoice+"%")
	}
	if noKwitansi != "" {
		query = query.Where("h.artih_nokw ILIKE ?", "%"+noKwitansi+"%")
	}
	if jenis != "" {
		query = query.Where("h.artih_jenis = ?", jenis)
	}
	if terbayar != "" {
		query = query.Where("COALESCE(h.artih_terbayar, 'N') = ?", terbayar)
	}
	if noBTT != "" {
		query = query.Where("EXISTS (SELECT 1 FROM public.art_t_invoiced invd WHERE invd.artid_artihid = h.artih_id AND invd.artid_bttid ILIKE ?)", "%"+noBTT+"%")
	}

	var list []InvoiceListRow
	if err := query.Order("h.artih_tanggal DESC, h.artih_id DESC").Limit(500).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// 2. GET /api/piutang/invoice/detail
func GetInvoiceDetailHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	invoiceID := strings.TrimSpace(c.Query("id"))
	if invoiceID == "" {
		invoiceID = strings.TrimSpace(c.Param("id"))
	}
	invoiceID = strings.TrimPrefix(invoiceID, "/")

	// 1. Query Header Invoice
	var header InvoiceListRow
	err := database.Table("public.art_t_invoiceh h").
		Select(`
			h.artih_id,
			h.artih_tanggal,
			h.artih_custid,
			COALESCE(c.cust_name, h.artih_custname) AS cust_name,
			COALESCE(h.artih_custname, c.cust_name) AS artih_custname,
			COALESCE(h.artih_keterangan, '') AS artih_keterangan,
			COALESCE(h.artih_delete, 'N') AS artih_delete,
			COALESCE(h.artih_nokw, '') AS artih_nokw,
			COALESCE(h.artih_fktpajak, '') AS artih_fktpajak,
			COALESCE(h.artih_total::numeric, 0) AS artih_total,
			COALESCE(h.artih_jenis, 'K') AS artih_jenis,
			COALESCE(h.artih_postingyn, 'N') AS artih_postingyn,
			COALESCE(h.artih_journalid, '') AS artih_journalid,
			h.artih_tglacc,
			COALESCE(a.agen_nama, 'DLI PUSAT') AS agen_nama
		`).
		Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id) = TRIM(h.artih_custid)").
		Joins("LEFT JOIN public.glb_m_agen a ON a.agen_id::varchar = LPAD(LEFT(h.artih_id, 3), 3, '0')").
		Where("TRIM(LOWER(h.artih_id)) = TRIM(LOWER(?))", invoiceID).
		Take(&header).Error

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Invoice tidak ditemukan: " + err.Error()})
		return
	}

	// 2. Query Detail BTT (Deklarasi bttList diletakkan di sini)
	var bttList []InvoiceBTTDetailRow
	err = database.Table("public.art_t_invoiced d").
		Select(`
			COALESCE(e.bttt_id::varchar, d.artid_bttid::varchar) AS bttt_id,
			COALESCE(e.bttt_servid::varchar, '1') AS bttt_servid,
			COALESCE(e.bttt_tanggal, NOW()) AS bttt_tanggal,
			COALESCE(e.bttt_asalname::varchar, '') AS bttt_asalname,
			COALESCE(e.bttt_tujuannama::varchar, '') AS bttt_tujuannama,
			COALESCE(e.bttt_tujuankota::varchar, '') AS bttt_tujuankota,
			COALESCE(e.bttt_nosuratjalan::varchar, '') AS bttt_nosuratjalan,
			COALESCE(e.bttt_namabarang::varchar, '') AS bttt_namabarang,
			COALESCE(e.bttt_jmlunit::numeric, 0) AS bttt_jmlunit,
			COALESCE(e.bttt_berat::numeric, 0) AS bttt_berat,
			COALESCE(e.bttt_ukuran::numeric, 0) AS bttt_ukuran,
			COALESCE(e.bttt_harga::numeric, 0) AS bttt_harga,
			COALESCE(e.bttt_biayapenerus::numeric, 0) AS bttt_biayapenerus,
			COALESCE(p.pck_biaya::numeric, 0) AS biaya_packing,
			COALESCE(asr.totalbiaya::numeric, 0) AS biaya_asuransi,
			COALESCE(e.bttt_nosuratjalan::varchar, '') AS no_skb,
			(COALESCE(e.bttt_harga::numeric, 0) + COALESCE(e.bttt_biayapenerus::numeric, 0) + COALESCE(p.pck_biaya::numeric, 0) + COALESCE(asr.totalbiaya::numeric, 0)) AS subtotal
		`).
		Joins("LEFT JOIN public.mkt_t_econote e ON TRIM(LOWER(e.bttt_id::varchar)) = TRIM(LOWER(d.artid_bttid::varchar))").
		Joins("LEFT JOIN public.pck_t_packing p ON TRIM(p.pck_id::varchar) = TRIM(e.bttt_packingid::varchar)").
		Joins("LEFT JOIN public.mkt_t_asuransi asr ON TRIM(asr.bttt_id::varchar) = TRIM(d.artid_bttid::varchar)").
		Where("TRIM(LOWER(d.artid_artihid::varchar)) = TRIM(LOWER(?))", invoiceID).
		Order("COALESCE(e.bttt_tanggal, NOW()) ASC, d.artid_bttid ASC").
		Scan(&bttList).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengambil daftar BTT invoice: " + err.Error()})
		return
	}

	// 3. Query Kepala Akunting dari Parameter glb_m_param (Murni dari Database)
	var accHead string
	database.Table("public.glb_m_param").
		Select("COALESCE(set_varvalue, '')").
		Where("TRIM(LOWER(set_varname)) = 'acc_head'").
		Order("set_updatetime DESC NULLS LAST, set_cabid ASC").
		Limit(1).
		Scan(&accHead)

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"header":   header,
		"acc_head": strings.TrimSpace(accHead), // Murni dari database
		"btt_list": bttList,
	})
}

func GetUnbilledBTTHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	custID := strings.TrimSpace(c.Query("cust_id"))
	custName := strings.TrimSpace(c.Query("cust_name"))
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	bypassTanggal := c.Query("bypass_tanggal") == "true" || c.Query("bypass_tanggal") == "1"
	displayType := strings.ToUpper(strings.TrimSpace(c.Query("display_type")))

	query := database.Table("public.mkt_t_econote e").
		Select(`
			e.bttt_id::varchar AS bttt_id,
			COALESCE(e.bttt_servid::varchar, '1') AS bttt_servid,
			COALESCE(e.bttt_tanggal, NOW()) AS bttt_tanggal,
			COALESCE(e.bttt_asalname::varchar, '') AS bttt_asalname,
			COALESCE(e.bttt_tujuannama::varchar, '') AS bttt_tujuannama,
			COALESCE(e.bttt_tujuankota::varchar, '') AS bttt_tujuankota,
			COALESCE(e.bttt_nosuratjalan::varchar, '') AS bttt_nosuratjalan,
			COALESCE(e.bttt_namabarang::varchar, '') AS bttt_namabarang,
			COALESCE(e.bttt_jmlunit::numeric, 0) AS bttt_jmlunit,
			COALESCE(e.bttt_berat::numeric, 0) AS bttt_berat,
			COALESCE(e.bttt_ukuran::numeric, 0) AS bttt_ukuran,
			COALESCE(e.bttt_harga::numeric, 0) AS bttt_harga,
			COALESCE(e.bttt_biayapenerus::numeric, 0) AS bttt_biayapenerus,
			COALESCE(p.pck_biaya::numeric, 0) AS biaya_packing,
			0::numeric AS biaya_asuransi,
			''::varchar AS no_skb,
			(
				(COALESCE(e.bttt_harga::numeric, 0) - (COALESCE(e.bttt_harga::numeric, 0) * (COALESCE(e.bttt_disc::numeric, 0) / 100.0))) 
				+ COALESCE(e.bttt_biayapenerus::numeric, 0) 
				+ COALESCE(p.pck_biaya::numeric, 0)
			) AS subtotal
		`).
		Joins("LEFT JOIN public.pck_t_packing p ON TRIM(p.pck_id::varchar) = TRIM(e.bttt_packingid::varchar)").
		Where("COALESCE(e.bttt_aktifyn, 'Y') = 'Y'")

	// 1. Ekstraksi kata kunci customer secara dinamis tanpa hardcode
	var searchKeywords []string
	reg := regexp.MustCompile(`(?i)\b(PT|CV|TBK|UD|TOKO|CORP|INC|LTD)\b|[^a-zA-Z0-9\s]`)
	cleanName := reg.ReplaceAllString(custName, " ")

	for _, w := range strings.Fields(cleanName) {
		if len(w) >= 3 {
			searchKeywords = append(searchKeywords, w)
		}
	}

	cleanCustID := strings.TrimLeft(custID, "0")

	if len(searchKeywords) > 0 {
		var nameConditions []string
		var args []interface{}

		for _, kw := range searchKeywords {
			nameConditions = append(nameConditions, "e.bttt_asalname ILIKE ?")
			args = append(args, "%"+kw+"%")
		}

		nameClause := strings.Join(nameConditions, " OR ")
		finalClause := fmt.Sprintf(`(
			(%s)
			OR TRIM(LOWER(COALESCE(e.bttt_asalcustid::varchar, ''))) = TRIM(LOWER(?))
			OR TRIM(LOWER(COALESCE(e.bttt_asalcustid::varchar, ''))) = TRIM(LOWER(?))
		)`, nameClause)

		args = append(args, custID, cleanCustID)
		query = query.Where(finalClause, args...)
	} else if custID != "" {
		query = query.Where("(TRIM(LOWER(COALESCE(e.bttt_asalcustid::varchar, ''))) = TRIM(LOWER(?)) OR TRIM(LOWER(COALESCE(e.bttt_asalcustid::varchar, ''))) = TRIM(LOWER(?)))", custID, cleanCustID)
	}

	// 2. Filter Tanggal (jika tidak bypass)
	if !bypassTanggal && startDate != "" && endDate != "" {
		query = query.Where("e.bttt_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}

	// 3. Filter Unbilled vs PI
	if displayType == "PI" {
		query = query.Joins("INNER JOIN public.art_t_proformad pid ON TRIM(pid.pid_bttid::varchar) = TRIM(e.bttt_id::varchar)")
	} else {
		query = query.Where(`
			NOT EXISTS (
				SELECT 1 FROM public.art_t_invoiced invd 
				INNER JOIN public.art_t_invoiceh invh ON TRIM(LOWER(invh.artih_id::varchar)) = TRIM(LOWER(invd.artid_artihid::varchar)) 
				WHERE TRIM(LOWER(invd.artid_bttid::varchar)) = TRIM(LOWER(e.bttt_id::varchar)) 
				  AND COALESCE(invh.artih_delete, 'N') <> 'Y'
			)
		`)
	}

	var list []InvoiceBTTDetailRow
	if err := query.Order("e.bttt_tanggal DESC, e.bttt_id DESC").Limit(450).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengambil daftar unbilled BTT: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// 4. POST /api/piutang/invoice/save (Simpan Invoice Baru / Update)
func SaveInvoiceHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	userID, _ := c.Get("username")

	var req CreateInvoiceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if len(req.BTTList) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Minimal pilih 1 resi BTT untuk difakturkan"})
		return
	}

	tgl, err := time.Parse("2006-01-02", strings.TrimSpace(req.ARTIHTanggal))
	if err != nil {
		tgl = time.Now()
	}

	// 🎯 Pastikan kode agen selalu berupa 3 digit angka (contoh: "001", "002", "006")
	agenIDPad := resolveKodeAgenNumeric(req.ARTIHAgenID, "")

	tx := database.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Generate Nomor Invoice
	mmyy := tgl.Format("0106")
	var maxInv string
	tx.Raw(`
        SELECT artih_id FROM public.art_t_invoiceh
        WHERE LEFT(artih_id, 3) = ? AND SUBSTRING(artih_id, 4, 4) = ?
        ORDER BY artih_id DESC LIMIT 1 FOR UPDATE
    `, agenIDPad, mmyy).Scan(&maxInv)

	urut := 1
	if len(maxInv) >= 11 {
		var lastUrut int
		fmt.Sscanf(maxInv[7:], "%d", &lastUrut)
		urut = lastUrut + 1
	}
	newInvoiceID := fmt.Sprintf("%s%s%04d", agenIDPad, mmyy, urut)

	// Format Nomor Kwitansi Resmi: 0010/DLI/001/09/26
	noKW := fmt.Sprintf("%04d/DLI/%s/%s/%s", urut, agenIDPad, tgl.Format("01"), tgl.Format("06"))

	// 2. Hitung Total Tagihan dari BTT Terpilih
	var jbttTotal float64 = 0
	var bpckTotal float64 = 0
	var basrTotal float64 = 0

	for _, bttID := range req.BTTList {
		var row struct {
			Harga        float64
			Disc         float64
			BiayaPenerus float64
			Packing      float64
			Asuransi     float64
		}
		tx.Table("public.mkt_t_econote e").
			Select(`
                COALESCE(e.bttt_harga, 0) AS harga,
                COALESCE(e.bttt_disc, 0) AS disc,
                COALESCE(e.bttt_biayapenerus, 0) AS biaya_penerus,
                COALESCE(p.pck_biaya, 0) AS packing,
                COALESCE(asr.totalbiaya, 0) AS asuransi
            `).
			Joins("LEFT JOIN public.pck_t_packing p ON TRIM(p.pck_id) = TRIM(e.bttt_packingid::varchar)").
			Joins("LEFT JOIN public.mkt_t_asuransi asr ON TRIM(asr.bttt_id) = TRIM(e.bttt_id)").
			Where("TRIM(e.bttt_id) = TRIM(?)", bttID).
			Scan(&row)

		// Rumus harga bersih setelah diskon persis aplikasi lawas ASP:
		hargaNetto := row.Harga - (row.Harga * (row.Disc / 100.0))
		jbttTotal += (hargaNetto + row.BiayaPenerus)
		bpckTotal += row.Packing
		basrTotal += row.Asuransi

		// Insert Detail
		detailMap := map[string]interface{}{
			"artid_artihid": newInvoiceID,
			"artid_bttid":   strings.TrimSpace(bttID),
		}
		if err := tx.Table("public.art_t_invoiced").Create(&detailMap).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan rincian BTT: " + err.Error()})
			return
		}
	}

	// Aturan PPN Dakota Multi-Tahun
	subtotalKenaPajak := jbttTotal + bpckTotal
	var dppNilai float64 = subtotalKenaPajak
	var ppnNilai float64 = 0

	if tgl.Year() >= 2025 {
		ppnNilai = dppNilai * 0.012 // Tarif PPN 1.2%
	} else {
		ppnNilai = subtotalKenaPajak * 0.011 // Default 1.1% jika masa pajak lama
	}

	grandTotalInvoice := subtotalKenaPajak + ppnNilai + basrTotal

	// 3. Insert Header Invoice
	headerMap := map[string]interface{}{
		"artih_id":         newInvoiceID,
		"artih_nokw":       noKW,
		"artih_tanggal":    tgl,
		"artih_custid":     strings.TrimSpace(req.ARTIHCustID),
		"artih_custname":   strings.TrimSpace(req.ARTIHCustName),
		"artih_keterangan": strings.TrimSpace(req.ARTIHKeterangan),
		"artih_dpp":        dppNilai,
		"artih_ppn":        ppnNilai,
		"artih_total":      grandTotalInvoice,
		"artih_fktpajak":   strings.TrimSpace(req.ARTIHFktPajak),
		"artih_jenis":      strings.TrimSpace(req.ARTIHJenis),
		"artih_terbayar":   "N",
		"artih_postingyn":  "N",
		"artih_delete":     "N",
		"artih_updateid":   fmt.Sprintf("%v", userID),
		"artih_updatetime": time.Now(),
	}

	if err := tx.Table("public.art_t_invoiceh").Create(&headerMap).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan header invoice: " + err.Error()})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    fmt.Sprintf("Invoice %s (Kwitansi %s) berhasil diterbitkan.", newInvoiceID, noKW),
		"invoice_id": newInvoiceID,
		"no_kw":      noKW,
	})
}

// 5. POST /api/piutang/invoice/acc (Update Tanggal ACC Faktur Penagihan)
func UpdateTglACCInvoiceHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	userVal, exists := c.Get("username")
	userStr := "SYSTEM"
	if exists && userVal != nil && fmt.Sprintf("%v", userVal) != "" {
		userStr = fmt.Sprintf("%v", userVal)
	}

	var req struct {
		ARTIHID string `json:"artih_id" binding:"required"`
		TglACC  string `json:"tgl_acc" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	tgl, err := time.Parse("2006-01-02", strings.TrimSpace(req.TglACC))
	if err != nil {
		tgl = time.Now()
	}

	res := database.Table("public.art_t_invoiceh").
		Where("TRIM(LOWER(artih_id)) = TRIM(LOWER(?))", req.ARTIHID).
		Updates(map[string]interface{}{
			"artih_tglacc":     tgl,
			"artih_updateid":   userStr,
			"artih_updatetime": time.Now(),
		})

	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update tanggal ACC: " + res.Error.Error()})
		return
	}

	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Nomor Invoice tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Tanggal ACC Faktur berhasil diperbarui.",
	})
}

// 6. DELETE /api/piutang/invoice
func DeleteInvoiceHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	invoiceID := strings.TrimSpace(c.Query("id"))
	if invoiceID == "" {
		invoiceID = strings.TrimSpace(c.Param("id"))
	}
	invoiceID = strings.TrimPrefix(invoiceID, "/")
	userID, _ := c.Get("username")

	var postingYN string
	database.Table("public.art_t_invoiceh").
		Select("COALESCE(artih_postingyn, 'N')").
		Where("TRIM(artih_id) = TRIM(?)", invoiceID).
		Scan(&postingYN)

	if postingYN == "Y" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invoice sudah diposting dan tidak dapat dihapus."})
		return
	}

	// Soft delete header invoice (Otomatis membebaskan BTT kembali jadi unbilled)
	if err := database.Table("public.art_t_invoiceh").Where("TRIM(artih_id) = TRIM(?)", invoiceID).Updates(map[string]interface{}{
		"artih_delete":     "Y",
		"artih_updateid":   fmt.Sprintf("%v", userID),
		"artih_updatetime": time.Now(),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus invoice: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Invoice %s berhasil dihapus.", invoiceID),
	})
}

// POST /api/piutang/invoice/update
func UpdateInvoiceFullHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	userID, _ := c.Get("username")

	var req struct {
		ARTIHID         string   `json:"artih_id" binding:"required"`
		ARTIHTanggal    string   `json:"artih_tanggal"`
		ARTIHFktPajak   string   `json:"artih_fktpajak"`
		ARTIHKeterangan string   `json:"artih_keterangan"`
		RemoveBTTList   []string `json:"remove_btt_list"`
		AddBTTList      []string `json:"add_btt_list"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	invoiceID := strings.TrimSpace(req.ARTIHID)

	var postingYN string
	database.Table("public.art_t_invoiceh").
		Select("COALESCE(artih_postingyn, 'N')").
		Where("TRIM(artih_id) = TRIM(?)", invoiceID).
		Scan(&postingYN)

	if postingYN == "Y" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invoice sudah diposting dan tidak dapat diubah."})
		return
	}

	tx := database.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Hapus BTT yang dicopot/dikeluarkan dari invoice
	if len(req.RemoveBTTList) > 0 {
		if err := tx.Table("public.art_t_invoiced").
			Where("TRIM(artid_artihid) = TRIM(?) AND TRIM(artid_bttid) IN ?", invoiceID, req.RemoveBTTList).
			Delete(map[string]interface{}{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus BTT terpilih: " + err.Error()})
			return
		}
	}

	// 2. Tambah BTT baru ke dalam invoice (cegah duplikasi)
	for _, bttID := range req.AddBTTList {
		bttID = strings.TrimSpace(bttID)
		if bttID == "" {
			continue
		}

		var count int64
		tx.Table("public.art_t_invoiced").
			Where("TRIM(artid_artihid) = TRIM(?) AND TRIM(artid_bttid) = TRIM(?)", invoiceID, bttID).
			Count(&count)

		if count == 0 {
			detailMap := map[string]interface{}{
				"artid_artihid":    invoiceID,
				"artid_bttid":      bttID,
				"artid_updateid":   fmt.Sprintf("%v", userID),
				"artid_updatetime": time.Now(),
			}
			if err := tx.Table("public.art_t_invoiced").Create(&detailMap).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menambah rincian BTT: " + err.Error()})
				return
			}
		}
	}

	// 3. Recalculate Total, DPP, dan PPN Invoice secara akurat
	var summary struct {
		BiayaKirim float64
		Penerus    float64
		Packing    float64
	}
	tx.Table("public.art_t_invoiced d").
		Select(`
			COALESCE(SUM(COALESCE(e.bttt_harga, 0)), 0) AS biaya_kirim,
			COALESCE(SUM(COALESCE(e.bttt_biayapenerus, 0)), 0) AS penerus,
			COALESCE(SUM(COALESCE(p.pck_biaya, 0)), 0) AS packing
		`).
		Joins("LEFT JOIN public.mkt_t_econote e ON TRIM(e.bttt_id) = TRIM(d.artid_bttid)").
		Joins("LEFT JOIN public.pck_t_packing p ON TRIM(p.pck_id::varchar) = TRIM(e.bttt_packingid::varchar)").
		Where("TRIM(d.artid_artihid) = TRIM(?)", invoiceID).
		Scan(&summary)

	nilaiJual := summary.BiayaKirim + summary.Penerus + summary.Packing
	dpp := nilaiJual
	ppn := dpp * 0.011 // Tarif PPN logistik 1.1%
	totalTagihan := nilaiJual + ppn

	// 4. Update Header Invoice
	updateHeader := map[string]interface{}{
		"artih_dpp":        dpp,
		"artih_ppn":        ppn,
		"artih_total":      totalTagihan,
		"artih_fktpajak":   strings.TrimSpace(req.ARTIHFktPajak),
		"artih_keterangan": strings.TrimSpace(req.ARTIHKeterangan),
		"artih_updateid":   fmt.Sprintf("%v", userID),
		"artih_updatetime": time.Now(),
	}

	if req.ARTIHTanggal != "" {
		if tgl, err := time.Parse("2006-01-02", req.ARTIHTanggal); err == nil {
			updateHeader["artih_tanggal"] = tgl
		}
	}

	if err := tx.Table("public.art_t_invoiceh").Where("TRIM(artih_id) = TRIM(?)", invoiceID).Updates(updateHeader).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update header: " + err.Error()})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Invoice %s berhasil diperbarui. Total Tagihan Baru: Rp %.2f", invoiceID, totalTagihan),
		"total":   totalTagihan,
		"dpp":     dpp,
		"ppn":     ppn,
	})
}

func UnpostingInvoiceHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	userID, _ := c.Get("username")
	if userID == nil || fmt.Sprintf("%v", userID) == "" {
		userID = "system"
	}

	var req struct {
		InvoiceID string `json:"invoice_id" binding:"required"`
		AgenID    string `json:"agen_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	invoiceID := strings.TrimSpace(req.InvoiceID)

	// 1. Ambil data Header Invoice (tanpa artih_agenid)
	var inv struct {
		ARTIHID        string    `gorm:"column:artih_id"`
		ARTIHTanggal   time.Time `gorm:"column:artih_tanggal"`
		ARTIHJournalID *string   `gorm:"column:artih_journalid"`
		ARTIHPostingYN string    `gorm:"column:artih_postingyn"`
		ARTIHTerbayar  *string   `gorm:"column:artih_terbayar"`
	}

	err := database.Table("public.art_t_invoiceh").
		Select("artih_id, artih_tanggal, artih_journalid, artih_postingyn, artih_terbayar").
		Where("TRIM(LOWER(artih_id)) = TRIM(LOWER(?))", invoiceID).
		Take(&inv).Error

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data Invoice tidak ditemukan: " + err.Error()})
		return
	}

	if strings.ToUpper(strings.TrimSpace(inv.ARTIHPostingYN)) != "Y" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invoice ini belum diposting (masih berstatus Draft)"})
		return
	}

	// Validasi pembayaran jika kolom artih_terbayar ada isi nominal > 0
	if inv.ARTIHTerbayar != nil {
		valBayar := strings.TrimSpace(*inv.ARTIHTerbayar)
		if valBayar != "" && valBayar != "0" && valBayar != "0.00" && strings.ToUpper(valBayar) != "N" {
			if nominal, errParse := strconv.ParseFloat(valBayar, 64); errParse == nil && nominal > 0 {
				c.JSON(http.StatusBadRequest, gin.H{
					"status":  "error",
					"message": fmt.Sprintf("Invoice %s sudah memiliki pembayaran sebesar Rp %.2f! Batalkan pelunasan terlebih dahulu sebelum unposting.", invoiceID, nominal),
				})
				return
			}
		}
	}

	// 2. Validasi Periode Closing Bulanan
	agenID := strings.TrimSpace(req.AgenID)
	if agenID == "" || strings.ToUpper(agenID) == "OTA" {
		agenID = "001"
	}

	bulan := int(inv.ARTIHTanggal.Month())
	tahun := inv.ARTIHTanggal.Year()

	var closingCount int64
	database.Table("public.glb_m_closing").
		Where("bulan = ? AND tahun = ? AND (agenid = ? OR agenid = '001')", bulan, tahun, agenID).
		Count(&closingCount)

	if closingCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Transaksi periode %02d/%d sudah diclosing! Unposting ditolak.", bulan, tahun),
		})
		return
	}

	tx := database.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 3. Batalkan Jurnal Akuntansi (Dukung varian OTA vs 001)
	if inv.ARTIHJournalID != nil && strings.TrimSpace(*inv.ARTIHJournalID) != "" {
		journalNo := strings.TrimSpace(*inv.ARTIHJournalID)

		journalNoAlt := strings.ReplaceAll(journalNo, "001", "OTA")
		if strings.Contains(journalNo, "OTA") {
			journalNoAlt = strings.ReplaceAll(journalNo, "OTA", "001")
		}

		// Hapus Detail Jurnal (gl_t_jurnald)
		if err := tx.Exec("DELETE FROM public.gl_t_jurnald WHERE TRIM(tjurd_tjurhno) = TRIM(?) OR TRIM(tjurd_tjurhno) = TRIM(?)", journalNo, journalNoAlt).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus jurnal detail: " + err.Error()})
			return
		}

		// Hapus Header Jurnal (gl_t_jurnalh)
		if err := tx.Exec("DELETE FROM public.gl_t_jurnalh WHERE TRIM(tjurh_no) = TRIM(?) OR TRIM(tjurh_no) = TRIM(?)", journalNo, journalNoAlt).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus jurnal header: " + err.Error()})
			return
		}
	}

	// 4. Update Status Invoice kembali ke Draft
	updateSQL := `
		UPDATE public.art_t_invoiceh SET 
			artih_postingyn = 'N', 
			artih_journalid = NULL,
			artih_updateid = ?, 
			artih_updatetime = NOW() 
		WHERE TRIM(LOWER(artih_id)) = TRIM(LOWER(?))
	`
	if err := tx.Exec(updateSQL, fmt.Sprintf("%v", userID), invoiceID).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update status invoice: " + err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal commit database: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Invoice %s berhasil di-Unposting. Status kembali menjadi Draft.", invoiceID),
	})
}

func PostingInvoiceHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		InvoiceID string `json:"invoice_id" binding:"required"`
		AgenID    string `json:"agen_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Data tidak valid: " + err.Error()})
		return
	}

	invoiceID := strings.TrimSpace(req.InvoiceID)
	username := c.GetString("username")
	if username == "" {
		username = "system"
	}

	// 1. Ambil header invoice untuk mendapatkan tanggal, total, dan agen
	var invHeader struct {
		ArtihID      string    `gorm:"column:artih_id"`
		ArtihTanggal time.Time `gorm:"column:artih_tanggal"`
		ArtihTotal   float64   `gorm:"column:artih_total"`
		ArtihAgenID  string    `gorm:"column:artih_agenid"`
	}

	if err := database.Table("public.art_t_invoiceh").
		Where("TRIM(LOWER(artih_id)) = TRIM(LOWER(?))", invoiceID).
		Take(&invHeader).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Invoice tidak ditemukan"})
		return
	}

	// Normalisasi agenID (hindari OTA, pastikan '001')
	agenID := strings.TrimSpace(req.AgenID)
	if agenID == "" {
		agenID = strings.TrimSpace(invHeader.ArtihAgenID)
	}
	if strings.ToUpper(agenID) == "OTA" || agenID == "" || agenID == "0" || agenID == "000" {
		agenID = "001"
	}

	tglInvoice := invHeader.ArtihTanggal
	if tglInvoice.IsZero() {
		tglInvoice = time.Now()
	}

	// 2. 🎯 GENERATE NOMOR JURNAL RESMI GL (Contoh: 2609001J00001)
	noJurnalResmi := generateNoJurnalGL(database, tglInvoice, agenID, "J")

	// 3. 🎯 SIMPAN HEADER JURNAL DI gl_t_jurnalh
	insertJH := map[string]interface{}{
		"tjurh_no":         noJurnalResmi,
		"tjurh_tanggal":    tglInvoice,
		"tjurh_keterangan": fmt.Sprintf("PENJUALAN KREDIT %s", invoiceID),
		"tjurh_type":       "J",
		"tjurh_deleteyn":   "N",
		"tjurh_postyn":     "Y",
		"tjurh_updateid":   username,
		"tjurh_updatetime": time.Now(),
	}
	if err := database.Table("public.gl_t_jurnalh").Create(&insertJH).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan header jurnal: " + err.Error()})
		return
	}

	// 4. Hitung rincian biaya packing dari detail BTT terkait
	type BttSummary struct {
		TotalPacking float64 `gorm:"column:tot_packing"`
	}
	var bttSum BttSummary
	database.Table("public.art_t_invoiced d").
		Select("COALESCE(SUM(b.biaya_packing), 0) AS tot_packing").
		Joins("LEFT JOIN public.btt_t_btt b ON b.bttt_id = d.artid_btttid").
		Where("TRIM(d.artid_artihid) = TRIM(?)", invoiceID).
		Scan(&bttSum)

	totalTagihan := invHeader.ArtihTotal
	packingNominal := bttSum.TotalPacking
	ppnNominal := 0.0 // Set nominal PPN jika transaksi bertarif PPN
	pendapatanKirim := totalTagihan - packingNominal - ppnNominal

	// 5. 🎯 SUSUN DETAIL BARIS JURNAL (gl_t_jurnald)
	var details []map[string]interface{}

	// Akun A102010100: Piutang Usaha (Debet)
	details = append(details, map[string]interface{}{
		"tjurd_tjurhno":    noJurnalResmi,
		"tjurd_acccode":    "A102010100",
		"tjurd_agenid":     agenID,
		"tjurd_keterangan": fmt.Sprintf("PENJUALAN KREDIT %s", invoiceID),
		"tjurd_debet":      totalTagihan,
		"tjurd_kredit":     0,
	})

	// Akun B102010600: Utang PPN (Kredit)
	details = append(details, map[string]interface{}{
		"tjurd_tjurhno":    noJurnalResmi,
		"tjurd_acccode":    "B102010600",
		"tjurd_agenid":     agenID,
		"tjurd_keterangan": fmt.Sprintf("UTANG PPN : UTANG PPN PENJUALAN KREDIT %s", invoiceID),
		"tjurd_debet":      0,
		"tjurd_kredit":     ppnNominal,
	})

	// Akun D101010200: Pendapatan Kredit (Kredit)
	details = append(details, map[string]interface{}{
		"tjurd_tjurhno":    noJurnalResmi,
		"tjurd_acccode":    "D101010200",
		"tjurd_agenid":     agenID,
		"tjurd_keterangan": fmt.Sprintf("PENJUALAN KREDIT %s", invoiceID),
		"tjurd_debet":      0,
		"tjurd_kredit":     pendapatanKirim,
	})

	// Akun D102010300: Pendapatan Jasa Packing (Kredit - jika ada)
	if packingNominal > 0 {
		details = append(details, map[string]interface{}{
			"tjurd_tjurhno":    noJurnalResmi,
			"tjurd_acccode":    "D102010300",
			"tjurd_agenid":     agenID,
			"tjurd_keterangan": fmt.Sprintf("BIAYA PACKING PENJUALAN KREDIT %s", invoiceID),
			"tjurd_debet":      0,
			"tjurd_kredit":     packingNominal,
		})
	}

	// Simpan seluruh rincian ke database gl_t_jurnald
	for _, d := range details {
		database.Table("public.gl_t_jurnald").Create(&d)
	}

	// 6. 🎯 UPDATE STATUS INVOICE DI art_t_invoiceh
	database.Table("public.art_t_invoiceh").
		Where("TRIM(LOWER(artih_id)) = TRIM(LOWER(?))", invoiceID).
		Updates(map[string]interface{}{
			"artih_postingyn": "Y",
			"artih_journalid": noJurnalResmi,
		})

	// 7. 🎯 KIRIM RESPONS SUKSES KE FRONTEND REACT
	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "Invoice berhasil diposting",
		"journal_id": noJurnalResmi,
	})
}

func generateNoJurnalGL(database *gorm.DB, tglTrans time.Time, agenID string, tipeJurnal string) string {
	cleanAgen := strings.TrimSpace(agenID)

	// 🎯 KUNCI BEBAS OTA: Jika agenID mengandung 'OTA', kosong, atau bukan angka murni, jadikan '001' (PUSAT)
	if strings.ToUpper(cleanAgen) == "OTA" || cleanAgen == "" || cleanAgen == "0" || cleanAgen == "000" {
		cleanAgen = "001"
	}

	// Pastikan hanya angka dan panjangnya tepat 3 digit
	if _, err := strconv.Atoi(cleanAgen); err != nil {
		cleanAgen = "001"
	}
	cb3Digit := fmt.Sprintf("%03s", cleanAgen)
	if len(cb3Digit) > 3 {
		cb3Digit = cb3Digit[len(cb3Digit)-3:]
	}

	if tipeJurnal == "" {
		tipeJurnal = "J"
	}

	// Prefix standar GL: YYMM + 001 + J (contoh: 2609001J)
	prefix := fmt.Sprintf("%s%s%s", tglTrans.Format("0601"), cb3Digit, tipeJurnal)

	var lastNo string
	database.Table("public.gl_t_jurnalh").
		Select("tjurh_no").
		Where("tjurh_no LIKE ?", prefix+"%").
		Order("tjurh_no DESC").
		Limit(1).
		Scan(&lastNo)

	nextUrut := 1
	if lastNo != "" && len(lastNo) >= len(prefix)+5 {
		if num, err := strconv.Atoi(lastNo[len(prefix):]); err == nil {
			nextUrut = num + 1
		}
	}

	return fmt.Sprintf("%s%05d", prefix, nextUrut)
}
