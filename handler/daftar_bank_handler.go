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

// KotaModel Struct untuk Master Kota (public.glb_m_kota)
type KotaModel struct {
	KotaID   string `json:"kota_id" gorm:"column:kota_id"`
	KotaNama string `json:"kota_nama" gorm:"column:kota_nama"`
}

// GetKotaListHandler Mengambil daftar master kota dari tabel public.glb_m_kota
func GetKotaListHandler(c *gin.Context) {
	database := getDB(c)

	var listKota []KotaModel
	err := database.Table("public.glb_m_kota").
		Select("kota_id, kota_nama").
		Where("COALESCE(kota_aktifyn, 'Y') = 'Y'").
		Order("kota_nama ASC").
		Scan(&listKota).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   listKota,
	})
}

// BankModel Struktur Data Master Bank (public.gl_m_bank)
type BankModel struct {
	BankID         string    `json:"bank_id" gorm:"column:bank_id;primaryKey"`
	BankName       string    `json:"bank_name" gorm:"column:bank_name"`
	BankAddress    string    `json:"bank_address" gorm:"column:bank_address"`
	BankKotaID     string    `json:"bank_kota_id" gorm:"column:bank_kotaid"`
	BankZip        string    `json:"bank_zip" gorm:"column:bank_zip"`
	BankPhone      string    `json:"bank_phone" gorm:"column:bank_phone"`
	BankFax        string    `json:"bank_fax" gorm:"column:bank_fax"`
	BankAccCode    string    `json:"bank_acc_code" gorm:"column:bank_acccode"`
	BankAktifYN    string    `json:"bank_aktif_yn" gorm:"column:bank_aktifyn"`
	BankUpdateID   string    `json:"bank_update_id" gorm:"column:bank_updateid"`
	BankUpdateTime time.Time `json:"bank_update_time" gorm:"column:bank_updatetime"`

	// 🎯 TAMBAHKAN tag gorm:"-" AGAR DIBUANG DARI QUERY INSERT/UPDATE
	KotaNama string `json:"kota_nama" gorm:"-"`
}

// getDB Context Helper
func getDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/daftar-bank (READ LIST & FILTER)
// =========================================================================
func GetDaftarBankHandler(c *gin.Context) {
	database := getDB(c)

	// Query Parameters
	kotaID := c.Query("kota")
	nama := c.Query("nama")
	status := c.Query("status")
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	// Query Base
	query := database.Table("public.gl_m_bank b").
		Select(`
			b.bank_id, 
			b.bank_name, 
			b.bank_address, 
			b.bank_kotaid, 
			b.bank_zip, 
			b.bank_phone, 
			b.bank_fax, 
			b.bank_acccode, 
			b.bank_aktifyn, 
			COALESCE(k.kota_nama, b.bank_kotaid) AS kota_nama
		`).
		Joins("LEFT JOIN public.glb_m_kota k ON CAST(b.bank_kotaid AS VARCHAR) = CAST(k.kota_id AS VARCHAR)")

	// Apply Filters
	if kotaID != "" {
		query = query.Where("CAST(b.bank_kotaid AS VARCHAR) = ?", kotaID)
	}
	if nama != "" {
		query = query.Where("LOWER(b.bank_name) LIKE ?", "%"+strings.ToLower(nama)+"%")
	}
	if status != "" {
		query = query.Where("b.bank_aktifyn = ?", status)
	}

	var totalRecords int64
	query.Count(&totalRecords)

	var listBank []BankModel
	err := query.Order("b.bank_id ASC").Limit(limit).Offset(offset).Scan(&listBank).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	totalPages := int((totalRecords + int64(limit) - 1) / int64(limit))

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          listBank,
		"total_records": totalRecords,
		"current_page":  page,
		"total_pages":   totalPages,
		"limit":         limit,
	})
}

// =========================================================================
// 2. POST /api/gl/bank (CREATE)
// =========================================================================
// 2. POST /api/gl/bank (CREATE)
func CreateBankHandler(c *gin.Context) {
	database := getDB(c)
	userID, _ := c.Get("username")

	var req BankModel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	req.BankID = strings.ToUpper(strings.TrimSpace(req.BankID))
	req.BankName = strings.ToUpper(strings.TrimSpace(req.BankName))
	req.BankAccCode = strings.TrimSpace(req.BankAccCode)
	req.BankKotaID = strings.TrimSpace(req.BankKotaID)

	// 🎯 VALIDASI MANDATORY: Kode, Nama, No Rekening, & Kota
	if req.BankID == "" || req.BankName == "" || req.BankAccCode == "" || req.BankKotaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Kode Bank, Nama Bank, No. Rekening, dan Kota wajib diisi!",
		})
		return
	}

	req.BankAktifYN = "Y"
	req.BankUpdateID = fmt.Sprintf("%v", userID)
	req.BankUpdateTime = time.Now()

	if err := database.Table("public.gl_m_bank").Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan bank: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Bank berhasil ditambahkan!",
		"data":    req,
	})
}

// 3. PUT /api/gl/bank/:id (UPDATE)
func UpdateBankHandler(c *gin.Context) {
	bankID := c.Param("id")
	database := getDB(c)
	userID, _ := c.Get("username")

	var req BankModel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	req.BankName = strings.ToUpper(strings.TrimSpace(req.BankName))
	req.BankAccCode = strings.TrimSpace(req.BankAccCode)
	req.BankKotaID = strings.TrimSpace(req.BankKotaID)

	// 🎯 VALIDASI MANDATORY SAAT UPDATE
	if req.BankName == "" || req.BankAccCode == "" || req.BankKotaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Nama Bank, No. Rekening, dan Kota wajib diisi!",
		})
		return
	}

	updateData := map[string]interface{}{
		"bank_name":       req.BankName,
		"bank_address":    req.BankAddress,
		"bank_kotaid":     req.BankKotaID,
		"bank_zip":        req.BankZip,
		"bank_phone":      req.BankPhone,
		"bank_fax":        req.BankFax,
		"bank_acccode":    req.BankAccCode,
		"bank_updateid":   fmt.Sprintf("%v", userID),
		"bank_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_m_bank").Where("bank_id = ?", bankID).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update bank: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data bank berhasil diperbarui!",
	})
}

// =========================================================================
// 3. PUT /api/gl/bank/:id (UPDATE)
// =========================================================================

// =========================================================================
// 4. PUT /api/gl/toggle-bank/:id (TOGGLE STATUS SOFT DELETE)
// =========================================================================
func ToggleStatusBankHandler(c *gin.Context) {
	bankID := c.Param("id")
	database := getDB(c)

	var bank BankModel
	if err := database.Table("public.gl_m_bank").Where("bank_id = ?", bankID).First(&bank).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Bank tidak ditemukan"})
		return
	}

	newStatus := "N"
	if bank.BankAktifYN == "N" {
		newStatus = "Y"
	}

	if err := database.Table("public.gl_m_bank").Where("bank_id = ?", bankID).Update("bank_aktifyn", newStatus).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    fmt.Sprintf("Status Bank %s berhasil diubah menjadi %s", bankID, newStatus),
		"new_status": newStatus,
	})
}

// =========================================================================
// 5. DELETE /api/gl/bank/:id (HARD DELETE)
// =========================================================================
func DeleteBankHandler(c *gin.Context) {
	bankID := c.Param("id")
	database := getDB(c)

	if err := database.Table("public.gl_m_bank").Where("bank_id = ?", bankID).Delete(&BankModel{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus bank: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Bank %s berhasil dihapus permanen!", bankID),
	})
}
