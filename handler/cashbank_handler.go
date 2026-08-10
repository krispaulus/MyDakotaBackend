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

// CashBankListModel Struct Tampilan Tabel Kas Masuk/Keluar
type CashBankListModel struct {
	CBID        string  `json:"cb_id" gorm:"column:cb_id"`
	CBTanggal   string  `json:"cb_tanggal" gorm:"column:cb_tanggal"`
	CBTipe      string  `json:"cb_tipe" gorm:"column:cb_tipe"`
	CBKet       string  `json:"cb_ket" gorm:"column:cb_ket"`
	CBAktifYN   string  `json:"cb_aktifyn" gorm:"column:cb_aktifyn"`
	CBNoJurnal  string  `json:"cb_nojurnal" gorm:"column:cb_nojurnal"`
	CBPostYN    string  `json:"cb_postyn" gorm:"column:cb_postyn"`
	AgenNama    string  `json:"agen_nama" gorm:"column:agen_nama"`
	TotalAmount float64 `json:"total_amount" gorm:"column:total_amount"`
}

// CreateCashBankReq DTO Request Input Kas Masuk/Keluar
type CreateCashBankReq struct {
	CBID        string  `json:"cb_id"`
	CBTanggal   string  `json:"cb_tanggal" binding:"required"`
	CBTipe      string  `json:"cb_tipe" binding:"required"` // 'T' / 'K'
	CBTransAgen string  `json:"cb_transagenid"`
	CBKet       string  `json:"cb_ket"`
	Nominal     float64 `json:"nominal"`
}

func getCashBankDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/cashbank (READ LIST TRANS KAS / BANK)
// =========================================================================
func GetCashBankListHandler(c *gin.Context) {
	database := getCashBankDB(c)

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tipe := c.Query("tipe") // 'T' / 'K'
	noTrans := c.Query("no_trans")
	cabangNama := c.Query("cabang_nama")
	aktifYN := c.Query("aktif_yn")
	postingYN := c.Query("posting_yn")
	noJurnal := c.Query("no_jurnal")
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

	// Query disesuaikan dengan public.gl_t_cashbankdetil (kata 'detil')
	query := database.Table("public.gl_t_cashbank cb").
		Select(`
			cb.cb_id, 
			TO_CHAR(cb.cb_tanggal, 'YYYY-MM-DD') AS cb_tanggal, 
			cb.cb_tipe, 
			COALESCE(cb.cb_ket, '-') AS cb_ket, 
			COALESCE(cb.cb_aktifyn, 'Y') AS cb_aktifyn, 
			COALESCE(cb.cb_nojurnal, '-') AS cb_nojurnal, 
			COALESCE(cb.cb_postyn, 'N') AS cb_postyn, 
			COALESCE(a.agen_nama, '-') AS agen_nama, 
			COALESCE(SUM(cbd.cbd_quantity * cbd.cbd_hargasatuan), cb.cb_total, 0) AS total_amount
		`).
		Joins("LEFT JOIN public.gl_t_cashbankdetil cbd ON TRIM(BOTH FROM CAST(cb.cb_id AS VARCHAR)) = TRIM(BOTH FROM CAST(cbd.cbd_cbid AS VARCHAR))").
		Joins("LEFT JOIN public.glb_m_agen a ON CAST(COALESCE(cb.cb_transagenid, '1') AS INTEGER) = CAST(a.agen_id AS INTEGER)").
		Where("COALESCE(cb.cb_id, '') <> ''")

	if startDate != "" && endDate != "" {
		query = query.Where("cb.cb_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if tipe != "" {
		query = query.Where("cb.cb_tipe = ?", tipe)
	}
	if noTrans != "" {
		query = query.Where("cb.cb_id ILIKE ?", "%"+noTrans+"%")
	}
	if cabangNama != "" {
		query = query.Where("a.agen_nama = ?", cabangNama)
	}
	if aktifYN != "" {
		query = query.Where("cb.cb_aktifyn = ?", aktifYN)
	}
	if postingYN != "" {
		query = query.Where("cb.cb_postyn = ?", postingYN)
	}
	if noJurnal != "" {
		query = query.Where("cb.cb_nojurnal ILIKE ?", "%"+noJurnal+"%")
	}

	query = query.Group("cb.cb_id, cb.cb_tanggal, cb.cb_tipe, cb.cb_ket, cb.cb_total, cb.cb_aktifyn, cb.cb_nojurnal, cb.cb_postyn, a.agen_nama")

	var totalRecords int64
	database.Table("(?) AS count_tbl", query).Count(&totalRecords)

	var list []CashBankListModel
	err := query.Order("cb.cb_tanggal DESC, cb.cb_id DESC").Limit(limit).Offset(offset).Scan(&list).Error
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
// 2. POST /api/gl/cashbank/create (INPUT TRANS KAS / BANK)
// =========================================================================
func CreateCashBankHandler(c *gin.Context) {
	database := getCashBankDB(c)
	userID, _ := c.Get("username")

	var req CreateCashBankReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Auto-generate No Transaksi Kas jika kosong
	if req.CBID == "" {
		prefix := "CBK"
		if req.CBTipe == "T" {
			prefix = "CBT"
		}
		req.CBID = fmt.Sprintf("%s%s%04d", prefix, time.Now().Format("020121"), time.Now().Unix()%10000)
	}

	insertHeader := map[string]interface{}{
		"cb_id":          strings.TrimSpace(req.CBID),
		"cb_tanggal":     req.CBTanggal + " " + time.Now().Format("15:04:05"),
		"cb_tipe":        req.CBTipe,
		"cb_agenid":      "1",
		"cb_transagenid": "1",
		"cb_ket":         req.CBKet,
		"cb_total":       req.Nominal,
		"cb_postyn":      "N",
		"cb_aktifyn":     "Y",
		"cb_updateid":    fmt.Sprintf("%v", userID),
		"cb_updatetime":  time.Now(),
	}

	if err := database.Table("public.gl_t_cashbank").Create(insertHeader).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan Kas/Bank: " + err.Error()})
		return
	}

	// Insert Detail jika ada nominal (Ke tabel public.gl_t_cashbankdetil)
	if req.Nominal > 0 {
		insertDetail := map[string]interface{}{
			"cbd_cbid":        strings.TrimSpace(req.CBID),
			"cbd_ket":         req.CBKet,
			"cbd_quantity":    1,
			"cbd_hargasatuan": req.Nominal,
			"cbd_agenid":      "1",
			"cbd_susutyn":     "N",
		}
		database.Table("public.gl_t_cashbankdetil").Create(insertDetail)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Pencatatan Kas/Bank Berhasil Disimpan!",
	})
}

// =========================================================================
// 3. DELETE /api/gl/cashbank/:id (BATALKAN / VOID TRANS KAS)
// =========================================================================
func DeleteCashBankHandler(c *gin.Context) {
	cbID := c.Param("id")
	database := getCashBankDB(c)

	err := database.Table("public.gl_t_cashbank").
		Where("cb_id = ?", cbID).
		Update("cb_aktifyn", "N").Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus transaksi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Transaksi Kas/Bank No %s berhasil dibatalkan!", cbID),
	})
}
