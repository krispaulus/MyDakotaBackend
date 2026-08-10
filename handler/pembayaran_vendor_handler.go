package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PembayaranVendorListModel Struct Tampilan Tabel Pembayaran Vendor
type PembayaranVendorListModel struct {
	TPayHNo         string  `json:"tpayh_no" gorm:"column:tpayh_no"`
	TPayHCBID       string  `json:"tpayh_cbid" gorm:"column:tpayh_cbid"`
	NoInvoice       string  `json:"no_invoice" gorm:"column:no_invoice"`
	NoKwitansi      string  `json:"no_kwitansi" gorm:"column:no_kwitansi"`
	TPayHTanggal    string  `json:"tpayh_tanggal" gorm:"column:tpayh_tanggal"`
	TPayHVendName   string  `json:"tpayh_vendname" gorm:"column:tpayh_vendname"`
	TotalDPP        float64 `json:"total_dpp" gorm:"column:total_dpp"`
	TotalHarusBayar float64 `json:"total_harus_bayar" gorm:"column:total_harus_bayar"`
	TPayHKeterangan string  `json:"tpayh_keterangan" gorm:"column:tpayh_keterangan"`
	TPayHDeleteYN   string  `json:"tpayh_deleteyn" gorm:"column:tpayh_deleteyn"`
	TPayHTJurHNo    string  `json:"tpayh_tjurhno" gorm:"column:tpayh_tjurhno"`
	TPayHPostingYN  string  `json:"tpayh_postingyn" gorm:"column:tpayh_postingyn"`
}

// CreatePembayaranVendorReq DTO Request Input Pembayaran Vendor
type CreatePembayaranVendorReq struct {
	TPayHNo         string   `json:"tpayh_no"`
	TPayHCBID       string   `json:"tpayh_cbid"`
	TPayHTanggal    string   `json:"tpayh_tanggal" binding:"required"`
	TPayHVendName   string   `json:"tpayh_vendname" binding:"required"`
	TPayHKeterangan string   `json:"tpayh_keterangan"`
	TotalDPP        float64  `json:"total_dpp"`
	NilaiPPN        float64  `json:"nilai_ppn"`
	NilaiPPH        float64  `json:"nilai_pph"`
	InvoiceIDs      []string `json:"invoice_ids"`
}

// VendorOptionModel Struct Dropdown Vendor
type VendorOptionModel struct {
	VendID   string `json:"vend_id" gorm:"column:vend_id"`
	VendName string `json:"vend_name" gorm:"column:vend_name"`
}

func getPembayaranVendorDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/pembayaran-vendor (READ LIST PEMBAYARAN VENDOR)
// =========================================================================
func GetPembayaranVendorListHandler(c *gin.Context) {
	database := getPembayaranVendorDB(c)

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	vendName := c.Query("vend_name")
	noPayment := c.Query("no_payment")
	noKwitansi := c.Query("no_kwitansi")
	noInvoice := c.Query("no_invoice")
	postingYN := c.Query("posting_yn")
	showDeleted := c.Query("show_deleted") // Opsional: 'Y' / 'N'
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "500")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 500
	}
	offset := (page - 1) * limit

	query := database.Table("public.gl_t_pembayaranvendorh h").
		Select(`
			h.tpayh_no, 
			COALESCE(CAST(h.tpayh_cbid AS VARCHAR), '1') AS tpayh_cbid, 
			TO_CHAR(h.tpayh_tanggal, 'YYYY-MM-DD') AS tpayh_tanggal, 
			COALESCE(h.tpayh_vendname, '-') AS tpayh_vendname, 
			COALESCE(h.tpayh_keterangan, '-') AS tpayh_keterangan, 
			COALESCE(h.tpayh_deleteyn, 'N') AS tpayh_deleteyn, 
			COALESCE(h.tpayh_tjurhno, '-') AS tpayh_tjurhno, 
			COALESCE(h.tpayh_postingyn, 'N') AS tpayh_postingyn, 
			COALESCE(SUM(d.trectd_nilai), h.tpayh_total, 0) AS total_dpp,
			(COALESCE(SUM(d.trectd_nilai), h.tpayh_total, 0) + COALESCE(h.tpayh_ppn, 0) - COALESCE(h.tpayh_pph, 0)) AS total_harus_bayar,
			COALESCE(STRING_AGG(DISTINCT d.trectd_aptivhid, ', '), '') AS no_invoice,
			COALESCE(STRING_AGG(DISTINCT iv.aptivh_nokw, ', '), '-') AS no_kwitansi
		`).
		Joins("LEFT JOIN public.gl_t_pembayaranvendord d ON TRIM(BOTH FROM CAST(h.tpayh_no AS VARCHAR)) = TRIM(BOTH FROM CAST(d.trectd_tpayhno AS VARCHAR))").
		Joins("LEFT JOIN public.apt_t_invoicevendorh iv ON TRIM(BOTH FROM CAST(d.trectd_aptivhid AS VARCHAR)) = TRIM(BOTH FROM CAST(iv.aptivh_id AS VARCHAR))").
		Where("COALESCE(h.tpayh_no, '') <> ''")

	// 🌟 Sembunyikan transaksi BATAL / VOID secara default jika show_deleted != 'Y'
	if showDeleted != "Y" {
		query = query.Where("COALESCE(h.tpayh_deleteyn, 'N') = 'N'")
	}

	if startDate != "" && endDate != "" {
		query = query.Where("h.tpayh_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if vendName != "" {
		query = query.Where("h.tpayh_vendname ILIKE ?", "%"+vendName+"%")
	}
	if noPayment != "" {
		query = query.Where("h.tpayh_no ILIKE ?", "%"+noPayment+"%")
	}
	if postingYN != "" {
		query = query.Where("h.tpayh_postingyn ILIKE ?", "%"+postingYN+"%")
	}
	if noInvoice != "" {
		query = query.Where("d.trectd_aptivhid ILIKE ?", "%"+noInvoice+"%")
	}
	if noKwitansi != "" {
		query = query.Where("iv.aptivh_nokw ILIKE ?", "%"+noKwitansi+"%")
	}

	query = query.Group("h.tpayh_no, h.tpayh_cbid, h.tpayh_tanggal, h.tpayh_vendname, h.tpayh_keterangan, h.tpayh_deleteyn, h.tpayh_tjurhno, h.tpayh_postingyn, h.tpayh_total, h.tpayh_ppn, h.tpayh_pph")

	var totalRecords int64
	database.Table("(?) AS count_tbl", query).Count(&totalRecords)

	var list []PembayaranVendorListModel
	err := query.Order("h.tpayh_tanggal DESC, h.tpayh_no DESC").Limit(limit).Offset(offset).Scan(&list).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          list,
		"total_records": totalRecords,
	})
}

// =========================================================================
// 2. POST /api/gl/pembayaran-vendor/create (SAVE PEMBAYARAN VENDOR)
// =========================================================================
func CreatePembayaranVendorHandler(c *gin.Context) {
	database := getPembayaranVendorDB(c)
	userID, _ := c.Get("username")

	var req CreatePembayaranVendorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if req.TPayHNo == "" {
		req.TPayHNo = fmt.Sprintf("VP%s%04d", time.Now().Format("020121"), time.Now().Unix()%10000)
	}

	// 🌟 Parsing aman Kode Cabang agar tidak menyebabkan 'numeric field overflow'
	cbIDVal := strings.TrimSpace(req.TPayHCBID)
	if cbIDVal == "" {
		cbIDVal = "1"
	}

	insertHeader := map[string]interface{}{
		"tpayh_no":         strings.TrimSpace(req.TPayHNo),
		"tpayh_cbid":       cbIDVal,
		"tpayh_tanggal":    req.TPayHTanggal + " " + time.Now().Format("15:04:05"),
		"tpayh_vendname":   strings.TrimSpace(req.TPayHVendName),
		"tpayh_total":      req.TotalDPP,
		"tpayh_ppn":        req.NilaiPPN,
		"tpayh_pph":        req.NilaiPPH,
		"tpayh_keterangan": req.TPayHKeterangan,
		"tpayh_deleteyn":   "N",
		"tpayh_postingyn":  "N",
		"tpayh_printyn":    "N",
		"tpayh_updateid":   fmt.Sprintf("%v", userID),
		"tpayh_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_t_pembayaranvendorh").Create(insertHeader).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan Pembayaran Vendor: " + err.Error()})
		return
	}

	// Insert Detail Invoice jika ada
	for _, invID := range req.InvoiceIDs {
		if strings.TrimSpace(invID) != "" {
			insertDetail := map[string]interface{}{
				"trectd_tpayhno":  strings.TrimSpace(req.TPayHNo),
				"trectd_aptivhid": strings.TrimSpace(invID),
				"trectd_nilai":    req.TotalDPP,
			}
			database.Table("public.gl_t_pembayaranvendord").Create(insertDetail)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Pembayaran Vendor Berhasil Disimpan!",
	})
}

// =========================================================================
// 3. DELETE /api/gl/pembayaran-vendor/:id (SOFT DELETE / VOID PEMBAYARAN)
// =========================================================================
func DeletePembayaranVendorHandler(c *gin.Context) {
	noPayment := c.Param("id")
	database := getPembayaranVendorDB(c)

	var checkHeader struct {
		PostingYN string `gorm:"column:tpayh_postingyn"`
		TJurHNo   string `gorm:"column:tpayh_tjurhno"`
	}
	database.Table("public.gl_t_pembayaranvendorh").
		Select("COALESCE(tpayh_postingyn, 'N') AS tpayh_postingyn, COALESCE(tpayh_tjurhno, '') AS tpayh_tjurhno").
		Where("tpayh_no = ?", noPayment).
		Scan(&checkHeader)

	if checkHeader.PostingYN == "Y" || strings.TrimSpace(checkHeader.TJurHNo) != "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Transaksi tidak dapat dibatalkan karena sudah di-posting atau memiliki No. Jurnal!"})
		return
	}

	err := database.Table("public.gl_t_pembayaranvendorh").
		Where("tpayh_no = ?", noPayment).
		Update("tpayh_deleteyn", "Y").Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membatalkan transaksi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Transaksi Pembayaran Vendor No %s berhasil dibatalkan!", noPayment),
	})
}

// =========================================================================
// 4. GET /api/gl/pembayaran-vendor/vendor-options (LIST DROPDOWN VENDOR)
// =========================================================================
func GetVendorOptionsHandler(c *gin.Context) {
	database := getPembayaranVendorDB(c)

	var list []VendorOptionModel
	err := database.Table("public.apt_t_invoicevendorh").
		Select("DISTINCT COALESCE(aptivh_vendid, '') AS vend_id, TRIM(aptivh_vendname) AS vend_name").
		Where("COALESCE(aptivh_vendname, '') <> '' AND COALESCE(aptivh_delete, 'N') = 'N'").
		Order("vend_name ASC").
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

// =========================================================================
// 5. PUT /api/gl/pembayaran-vendor/update (UPDATE PEMBAYARAN VENDOR)
// =========================================================================
func UpdatePembayaranVendorHandler(c *gin.Context) {
	database := getPembayaranVendorDB(c)
	userID, _ := c.Get("username")

	var req CreatePembayaranVendorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if strings.TrimSpace(req.TPayHNo) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Pembayaran tidak boleh kosong saat update!"})
		return
	}

	// Parsing aman Kode Cabang
	cbIDVal := strings.TrimSpace(req.TPayHCBID)
	if cbIDVal == "" {
		cbIDVal = "1"
	}

	// 1. Update Header
	updateHeader := map[string]interface{}{
		"tpayh_cbid":       cbIDVal,
		"tpayh_tanggal":    req.TPayHTanggal + " " + time.Now().Format("15:04:05"),
		"tpayh_vendname":   strings.TrimSpace(req.TPayHVendName),
		"tpayh_total":      req.TotalDPP,
		"tpayh_ppn":        req.NilaiPPN,
		"tpayh_pph":        req.NilaiPPH,
		"tpayh_keterangan": req.TPayHKeterangan,
		"tpayh_updateid":   fmt.Sprintf("%v", userID),
		"tpayh_updatetime": time.Now(),
	}

	err := database.Table("public.gl_t_pembayaranvendorh").
		Where("tpayh_no = ?", strings.TrimSpace(req.TPayHNo)).
		Updates(updateHeader).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate Pembayaran Vendor: " + err.Error()})
		return
	}

	// 2. Hapus detail lama & Insert ulang detail baru
	database.Table("public.gl_t_pembayaranvendord").
		Where("trectd_tpayhno = ?", strings.TrimSpace(req.TPayHNo)).
		Delete(nil)

	for _, invID := range req.InvoiceIDs {
		if strings.TrimSpace(invID) != "" {
			insertDetail := map[string]interface{}{
				"trectd_tpayhno":  strings.TrimSpace(req.TPayHNo),
				"trectd_aptivhid": strings.TrimSpace(invID),
				"trectd_nilai":    req.TotalDPP,
			}
			database.Table("public.gl_t_pembayaranvendord").Create(insertDetail)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Pembayaran Vendor Berhasil Diperbarui!",
	})
}
