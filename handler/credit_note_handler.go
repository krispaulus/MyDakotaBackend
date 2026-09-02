package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type CreditNoteHeaderRow struct {
	ARTCNHNo         string    `json:"artcnh_no" gorm:"column:artcnh_no"`
	ARTCNHTanggal    time.Time `json:"artcnh_tanggal" gorm:"column:artcnh_tanggal"`
	ARTCNHCustID     string    `json:"artcnh_custid" gorm:"column:artcnh_custid"`
	CustName         string    `json:"cust_name" gorm:"column:cust_name"`
	ARTCNHAlasan     string    `json:"artcnh_alasan" gorm:"column:artcnh_alasan"`
	ARTCNHKeterangan string    `json:"artcnh_keterangan" gorm:"column:artcnh_keterangan"`
	ARTCNHTotal      float64   `json:"artcnh_total" gorm:"column:artcnh_total"`
	ARTCNHAgenID     string    `json:"artcnh_agenid" gorm:"column:artcnh_agenid"`
	AgenNama         string    `json:"agen_nama" gorm:"column:agen_nama"`
	ARTCNHPostingYN  string    `json:"artcnh_postingyn" gorm:"column:artcnh_postingyn"`
	ARTCNHJournalID  string    `json:"artcnh_journalid" gorm:"column:artcnh_journalid"`
	ARTCNHDeleteYN   string    `json:"artcnh_deleteyn" gorm:"column:artcnh_deleteyn"`
}

type CreditNoteDetailRow struct {
	ARTCNDID           int       `json:"artcnd_id" gorm:"column:artcnd_id"`
	ARTCNDARTCNHNo     string    `json:"artcnd_artcnhno" gorm:"column:artcnd_artcnhno"`
	ARTCNDARTIHID      string    `json:"artcnd_artihid" gorm:"column:artcnd_artihid"`
	ARTCNDARTIHNoKW    string    `json:"artcnd_artihnokw" gorm:"column:artcnd_artihnokw"`
	ARTCNDNilai        float64   `json:"artcnd_nilai" gorm:"column:artcnd_nilai"`
	ARTCNDKeterangan   string    `json:"artcnd_keterangan" gorm:"column:artcnd_keterangan"`
	ARTIH_Tanggal      time.Time `json:"artih_tanggal" gorm:"column:artih_tanggal"`
	ARTIH_Total        float64   `json:"artih_total" gorm:"column:artih_total"`
	OutstandingSaatIni float64   `json:"outstanding_saat_ini" gorm:"column:outstanding_saat_ini"`
}

type SaveCreditNoteReq struct {
	ARTCNHNo         string `json:"artcnh_no"`
	ARTCNHTanggal    string `json:"artcnh_tanggal" binding:"required"`
	ARTCNHCustID     string `json:"artcnh_custid" binding:"required"`
	ARTCNHAlasan     string `json:"artcnh_alasan" binding:"required"`
	ARTCNHKeterangan string `json:"artcnh_keterangan"`
	ARTCNHAgenID     string `json:"artcnh_agenid"`
	Details          []struct {
		ARTIHID    string  `json:"artih_id"`
		ARTIHNoKW  string  `json:"artih_nokw" binding:"required"`
		Nilai      float64 `json:"nilai" binding:"required"`
		Keterangan string  `json:"keterangan"`
	} `json:"details"`
}

// 1. GET /api/piutang/credit-note (List Grid)
func GetCreditNoteListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	agenID := strings.TrimSpace(c.Query("agen_id"))
	customer := strings.TrimSpace(c.Query("customer"))
	noCN := strings.TrimSpace(c.Query("no_cn"))
	alasan := strings.TrimSpace(c.Query("alasan"))
	postingYN := strings.TrimSpace(c.Query("posting_yn"))
	bypassTanggal := c.Query("bypass_tanggal") == "true" || c.Query("bypass_tanggal") == "1"

	query := database.Table("public.art_t_creditnoteh h").
		Select(`
			h.artcnh_no,
			h.artcnh_tanggal,
			h.artcnh_custid,
			COALESCE(c.cust_name, h.artcnh_custid) AS cust_name,
			h.artcnh_alasan,
			COALESCE(h.artcnh_keterangan, '') AS artcnh_keterangan,
			COALESCE(h.artcnh_total, 0) AS artcnh_total,
			h.artcnh_agenid,
			COALESCE(a.agen_nama, 'DLI PUSAT') AS agen_nama,
			COALESCE(h.artcnh_postingyn, 'N') AS artcnh_postingyn,
			COALESCE(h.artcnh_journalid, '') AS artcnh_journalid,
			COALESCE(h.artcnh_deleteyn, 'N') AS artcnh_deleteyn
		`).
		Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id) = TRIM(h.artcnh_custid)").
		Joins("LEFT JOIN public.glb_m_agen a ON (TRIM(a.agen_id::varchar) = TRIM(h.artcnh_agenid::varchar) OR a.agen_id::varchar = LPAD(LEFT(h.artcnh_no, 3), 3, '0'))").
		Where("COALESCE(h.artcnh_deleteyn, 'N') = 'N'")

	if !bypassTanggal && startDate != "" && endDate != "" {
		query = query.Where("h.artcnh_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if agenID != "" && agenID != "ALL" {
		query = query.Where("(h.artcnh_agenid = ? OR LPAD(h.artcnh_agenid, 3, '0') = LPAD(?, 3, '0'))", agenID, agenID)
	}
	if customer != "" {
		query = query.Where("(c.cust_name ILIKE ? OR h.artcnh_custid ILIKE ?)", "%"+customer+"%", "%"+customer+"%")
	}
	if noCN != "" {
		query = query.Where("h.artcnh_no ILIKE ?", "%"+noCN+"%")
	}
	if alasan != "" {
		query = query.Where("h.artcnh_alasan = ?", alasan)
	}
	if postingYN != "" {
		query = query.Where("COALESCE(h.artcnh_postingyn, 'N') = ?", postingYN)
	}

	var list []CreditNoteHeaderRow
	if err := query.Order("h.artcnh_tanggal DESC, h.artcnh_no DESC").Limit(500).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// 2. GET /api/piutang/credit-note/detail/:no
func GetCreditNoteDetailHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	noCN := strings.TrimSpace(c.Param("no"))

	var header CreditNoteHeaderRow
	err := database.Table("public.art_t_creditnoteh h").
		Select(`
			h.artcnh_no,
			h.artcnh_tanggal,
			h.artcnh_custid,
			COALESCE(c.cust_name, h.artcnh_custid) AS cust_name,
			h.artcnh_alasan,
			COALESCE(h.artcnh_keterangan, '') AS artcnh_keterangan,
			COALESCE(h.artcnh_total, 0) AS artcnh_total,
			h.artcnh_agenid,
			COALESCE(a.agen_nama, 'DLI PUSAT') AS agen_nama,
			COALESCE(h.artcnh_postingyn, 'N') AS artcnh_postingyn,
			COALESCE(h.artcnh_journalid, '') AS artcnh_journalid,
			COALESCE(h.artcnh_deleteyn, 'N') AS artcnh_deleteyn
		`).
		Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id) = TRIM(h.artcnh_custid)").
		Joins("LEFT JOIN public.glb_m_agen a ON (TRIM(a.agen_id::varchar) = TRIM(h.artcnh_agenid::varchar) OR a.agen_id::varchar = LPAD(LEFT(h.artcnh_no, 3), 3, '0'))").
		Where("TRIM(h.artcnh_no) = TRIM(?)", noCN).
		Take(&header).Error

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Credit Note tidak ditemukan: " + err.Error()})
		return
	}

	var details []CreditNoteDetailRow
	database.Table("public.art_t_creditnoted d").
		Select(`
			d.artcnd_id,
			d.artcnd_artcnhno,
			d.artcnd_artihid,
			d.artcnd_artihnokw,
			COALESCE(d.artcnd_nilai, 0) AS artcnd_nilai,
			COALESCE(d.artcnd_keterangan, '') AS artcnd_keterangan,
			COALESCE(v.artih_tanggal, '2000-01-01') AS artih_tanggal,
			COALESCE(v.artih_total, 0) AS artih_total,
			COALESCE(v.outstanding, 0) AS outstanding_saat_ini
		`).
		Joins("LEFT JOIN public.v_art_invoiceoutstanding v ON TRIM(v.artih_nokw) = TRIM(d.artcnd_artihnokw)").
		Where("TRIM(d.artcnd_artcnhno) = TRIM(?)", noCN).
		Order("d.artcnd_id ASC").
		Scan(&details)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"header":  header,
		"details": details,
	})
}

// 3. GET /api/piutang/credit-note/invoices-outstanding (Picker Invoice Milik Customer)
func GetInvoicesOutstandingHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	custID := strings.TrimSpace(c.Query("cust_id"))
	if custID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Customer ID wajib diisi"})
		return
	}

	type InvoiceOutstandingRow struct {
		ARTIH_ID       string    `json:"artih_id" gorm:"column:artih_id"`
		ARTIH_NoKW     string    `json:"artih_nokw" gorm:"column:artih_nokw"`
		ARTIH_CustID   string    `json:"artih_custid" gorm:"column:artih_custid"`
		ARTIH_CustName string    `json:"artih_custname" gorm:"column:artih_custname"`
		ARTIH_Tanggal  time.Time `json:"artih_tanggal" gorm:"column:artih_tanggal"`
		ARTIH_Total    float64   `json:"artih_total" gorm:"column:artih_total"`
		Outstanding    float64   `json:"outstanding" gorm:"column:outstanding"`
		UmurHari       int       `json:"umur_hari" gorm:"column:umur_hari"`
	}

	var list []InvoiceOutstandingRow
	err := database.Table("public.v_art_invoiceoutstanding v").
		Select(`
			v.artih_id,
			v.artih_nokw,
			v.artih_custid,
			v.artih_custname,
			v.artih_tanggal,
			v.artih_total,
			v.outstanding,
			EXTRACT(DAY FROM (NOW() - v.artih_tanggal))::int AS umur_hari
		`).
		Where("TRIM(v.artih_custid) = TRIM(?) AND v.outstanding > 0", custID).
		Order("v.artih_tanggal ASC").
		Scan(&list).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// 4. POST /api/piutang/credit-note/save (Simpan Header & Detail)
func SaveCreditNoteHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	userID, _ := c.Get("username")

	var req SaveCreditNoteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	tx := database.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	tgl, err := time.Parse("2006-01-02", strings.TrimSpace(req.ARTCNHTanggal))
	if err != nil {
		tgl = time.Now()
	}

	noCN := strings.TrimSpace(req.ARTCNHNo)
	agenIDPad := fmt.Sprintf("%03s", strings.TrimSpace(req.ARTCNHAgenID))
	if len(agenIDPad) > 3 {
		agenIDPad = agenIDPad[len(agenIDPad)-3:]
	}
	if agenIDPad == "000" || agenIDPad == "" {
		agenIDPad = "001"
	}

	// 1. Generate No CN jika baru[cite: 6]
	if noCN == "" {
		mmyy := tgl.Format("0106")
		var maxNo string
		tx.Raw(`
			SELECT artcnh_no FROM public.art_t_creditnoteh
			WHERE LEFT(artcnh_no, 3) = ? AND SUBSTRING(artcnh_no, 4, 4) = ?
			ORDER BY artcnh_no DESC LIMIT 1 FOR UPDATE
		`, agenIDPad, mmyy).Scan(&maxNo)

		urut := 1
		if len(maxNo) >= 11 {
			var lastUrut int
			fmt.Sscanf(maxNo[7:], "%d", &lastUrut)
			urut = lastUrut + 1
		}
		noCN = fmt.Sprintf("%s%s%04d", agenIDPad, mmyy, urut)

		// Insert Header[cite: 6]
		headerInsert := map[string]interface{}{
			"artcnh_no":         noCN,
			"artcnh_tanggal":    tgl,
			"artcnh_custid":     strings.TrimSpace(req.ARTCNHCustID),
			"artcnh_alasan":     strings.TrimSpace(req.ARTCNHAlasan),
			"artcnh_keterangan": strings.TrimSpace(req.ARTCNHKeterangan),
			"artcnh_total":      0,
			"artcnh_agenid":     agenIDPad,
			"artcnh_postingyn":  "N",
			"artcnh_deleteyn":   "N",
			"artcnh_updateid":   fmt.Sprintf("%v", userID),
			"artcnh_updatetime": time.Now(),
		}
		if err := tx.Table("public.art_t_creditnoteh").Create(&headerInsert).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membuat Credit Note: " + err.Error()})
			return
		}
	} else {
		// Cek apakah sudah posting[cite: 6, 7]
		var postedYN string
		tx.Table("public.art_t_creditnoteh").Select("artcnh_postingyn").Where("artcnh_no = ?", noCN).Scan(&postedYN)
		if postedYN == "Y" {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Credit Note sudah diposting dan tidak dapat diubah. Unposting terlebih dahulu."})
			return
		}

		// Update Header[cite: 6]
		headerUpdate := map[string]interface{}{
			"artcnh_alasan":     strings.TrimSpace(req.ARTCNHAlasan),
			"artcnh_keterangan": strings.TrimSpace(req.ARTCNHKeterangan),
			"artcnh_updateid":   fmt.Sprintf("%v", userID),
			"artcnh_updatetime": time.Now(),
		}
		if err := tx.Table("public.art_t_creditnoteh").Where("artcnh_no = ?", noCN).Updates(headerUpdate).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update header: " + err.Error()})
			return
		}
	}

	// 2. Refresh Details: Hapus detail lama dan masukkan detail baru[cite: 6]
	if err := tx.Table("public.art_t_creditnoted").Where("artcnd_artcnhno = ?", noCN).Delete(&CreditNoteDetailRow{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal sinkron detail: " + err.Error()})
		return
	}

	var grandTotal float64 = 0
	for _, d := range req.Details {
		if d.Nilai <= 0 {
			continue
		}
		grandTotal += d.Nilai
		detailInsert := map[string]interface{}{
			"artcnd_artcnhno":   noCN,
			"artcnd_artihid":    strings.TrimSpace(d.ARTIHID),
			"artcnd_artihnokw":  strings.TrimSpace(d.ARTIHNoKW),
			"artcnd_nilai":      d.Nilai,
			"artcnd_keterangan": strings.TrimSpace(d.Keterangan),
		}
		if err := tx.Table("public.art_t_creditnoted").Create(&detailInsert).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal insert detail: " + err.Error()})
			return
		}
	}

	// 3. Recalculate Header Total[cite: 6]
	if err := tx.Table("public.art_t_creditnoteh").Where("artcnh_no = ?", noCN).Update("artcnh_total", grandTotal).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal recalculate total: " + err.Error()})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   fmt.Sprintf("Credit Note %s berhasil disimpan.", noCN),
		"artcnh_no": noCN,
	})
}

// 5. POST /api/piutang/credit-note/posting (Posting GL Jurnal)
func PostingCreditNoteHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	userID, _ := c.Get("username")

	var req struct {
		ARTCNHNo string `json:"artcnh_no" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	noCN := strings.TrimSpace(req.ARTCNHNo)

	var header CreditNoteHeaderRow
	if err := database.Table("public.art_t_creditnoteh").Where("artcnh_no = ?", noCN).Take(&header).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Credit Note tidak ditemukan"})
		return
	}

	if header.ARTCNHPostingYN == "Y" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Credit Note ini sudah diposting sebelumnya."})
		return
	}
	if header.ARTCNHTotal <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Total Credit Note 0. Tambahkan rincian invoice sebelum posting."})
		return
	}

	tx := database.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Generate Nomor Jurnal Umum (Format J)[cite: 6]
	jurnalNo := fmt.Sprintf("J%s%s%04d", header.ARTCNHAgenID, header.ARTCNHTanggal.Format("0106"), time.Now().Unix()%10000)

	// Jurnal Debit: E102020040 (Biaya Kerugian Piutang Tak Tertagih)[cite: 6]
	// Jurnal Kredit: A102010100 (Piutang Usaha)[cite: 6]
	jurH := map[string]interface{}{
		"tjurh_no":         jurnalNo,
		"tjurh_tgl":        header.ARTCNHTanggal,
		"tjurh_type":       "J",
		"tjurh_ket":        fmt.Sprintf("Credit Note %s (%s) - %s", header.ARTCNHNo, header.ARTCNHAlasan, header.ARTCNHKeterangan),
		"tjurh_total":      header.ARTCNHTotal,
		"tjurh_deleteyn":   "N",
		"tjurh_updateid":   fmt.Sprintf("%v", userID),
		"tjurh_updatetime": time.Now(),
	}
	_ = tx.Table("public.gl_t_jurnalh").Create(&jurH)

	jurD1 := map[string]interface{}{
		"tjurd_tjurhno": jurnalNo,
		"tjurd_rekid":   "E102020040",
		"tjurd_pos":     "D",
		"tjurd_nominal": header.ARTCNHTotal,
		"tjurd_ket":     fmt.Sprintf("CN %s Biaya Kerugian Piutang", header.ARTCNHNo),
	}
	_ = tx.Table("public.gl_t_jurnald").Create(&jurD1)

	jurD2 := map[string]interface{}{
		"tjurd_tjurhno": jurnalNo,
		"tjurd_rekid":   "A102010100",
		"tjurd_pos":     "K",
		"tjurd_nominal": header.ARTCNHTotal,
		"tjurd_ket":     fmt.Sprintf("CN %s Pengurang Piutang Usaha", header.ARTCNHNo),
	}
	_ = tx.Table("public.gl_t_jurnald").Create(&jurD2)

	// Update Header Credit Note[cite: 6]
	tx.Table("public.art_t_creditnoteh").Where("artcnh_no = ?", noCN).Updates(map[string]interface{}{
		"artcnh_postingyn":  "Y",
		"artcnh_journalid":  jurnalNo,
		"artcnh_updateid":   fmt.Sprintf("%v", userID),
		"artcnh_updatetime": time.Now(),
	})

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    fmt.Sprintf("Credit Note %s berhasil diposting dengan No Jurnal %s", noCN, jurnalNo),
		"journal_id": jurnalNo,
	})
}

// 6. POST /api/piutang/credit-note/unposting (Unposting Jurnal)
func UnpostingCreditNoteHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	userID, _ := c.Get("username")

	var req struct {
		ARTCNHNo string `json:"artcnh_no" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	noCN := strings.TrimSpace(req.ARTCNHNo)

	var header CreditNoteHeaderRow
	if err := database.Table("public.art_t_creditnoteh").Where("artcnh_no = ?", noCN).Take(&header).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Credit Note tidak ditemukan"})
		return
	}

	if header.ARTCNHPostingYN != "Y" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Credit Note belum diposting."})
		return
	}

	tx := database.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Void Jurnal GL[cite: 6]
	if header.ARTCNHJournalID != "" {
		tx.Table("public.gl_t_jurnalh").Where("tjurh_no = ?", header.ARTCNHJournalID).Update("tjurh_deleteyn", "Y")
		tx.Table("public.gl_t_jurnald").Where("tjurd_tjurhno = ?", header.ARTCNHJournalID).Delete(map[string]interface{}{})
	}

	// Reset Flag Posting[cite: 6]
	tx.Table("public.art_t_creditnoteh").Where("artcnh_no = ?", noCN).Updates(map[string]interface{}{
		"artcnh_postingyn":  "N",
		"artcnh_journalid":  nil,
		"artcnh_updateid":   fmt.Sprintf("%v", userID),
		"artcnh_updatetime": time.Now(),
	})

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Credit Note %s berhasil di-unposting. Jurnal %s dibatalkan.", noCN, header.ARTCNHJournalID),
	})
}

// 7. DELETE /api/piutang/credit-note/:no (Soft Delete)
func DeleteCreditNoteHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	noCN := strings.TrimSpace(c.Param("no"))
	userID, _ := c.Get("username")

	var header CreditNoteHeaderRow
	if err := database.Table("public.art_t_creditnoteh").Where("artcnh_no = ?", noCN).Take(&header).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Credit Note tidak ditemukan"})
		return
	}

	if header.ARTCNHPostingYN == "Y" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Credit Note sudah diposting. Lakukan unposting terlebih dahulu sebelum menghapus."})
		return
	}

	if err := database.Table("public.art_t_creditnoteh").Where("artcnh_no = ?", noCN).Updates(map[string]interface{}{
		"artcnh_deleteyn":   "Y",
		"artcnh_updateid":   fmt.Sprintf("%v", userID),
		"artcnh_updatetime": time.Now(),
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Credit Note %s berhasil dihapus.", noCN),
	})
}
