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

// ItemModel Struct Master Item Pemasukan & Pengeluaran (public.gl_m_item)
type ItemModel struct {
	ItemID         string    `json:"item_id" gorm:"column:item_id;primaryKey"`
	ItemCatID      string    `json:"item_cat_id" gorm:"column:item_catid"` // Match JSON dengan React
	ItemName       string    `json:"item_name" gorm:"column:item_name"`
	ItemStatus     string    `json:"item_status" gorm:"column:item_status"` // 'A' / 'L'
	ItemOwnCAID    string    `json:"item_own_caid" gorm:"column:item_owncaid"`
	ItemDepCAID    string    `json:"item_dep_caid" gorm:"column:item_depcaid"`
	ItemCashCAID   string    `json:"item_cash_caid" gorm:"column:item_cashcaid"`
	ItemAktifYN    string    `json:"item_aktif_yn" gorm:"column:item_aktifyn"`
	ItemUpdateID   string    `json:"item_update_id" gorm:"column:item_updateid"`
	ItemUpdateTime time.Time `json:"item_update_time" gorm:"column:item_updatetime"`

	// Join Field (Harus gorm:"-" agar dibuang saat Create/Update)
	CatName string `json:"cat_name" gorm:"-"`
}

// CategoryItemModel Struct Kategori (public.gl_m_categoryitem)
type CategoryItemModel struct {
	CatID   string `json:"cat_id" gorm:"column:cat_id"`
	CatName string `json:"cat_name" gorm:"column:cat_name"`
}

// Helper Get DB
func getItemDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/pemasukan-pengeluaran (READ LIST & FILTER)
// =========================================================================
func GetDaftarItemHandler(c *gin.Context) {
	database := getItemDB(c)

	// Mendukung parameter dari React (cat_id, q, status, aktif_yn) maupun parameter lama (kategori, nama)
	catID := c.Query("cat_id")
	if catID == "" {
		catID = c.Query("kategori")
	}

	nama := c.Query("q")
	if nama == "" {
		nama = c.Query("nama")
	}

	status := c.Query("status")
	aktifYN := c.Query("aktif_yn")

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "2000")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 2000
	}
	offset := (page - 1) * limit

	query := database.Table("public.gl_m_item i").
		Select(`
			TRIM(BOTH FROM i.item_id) AS item_id, 
			TRIM(BOTH FROM CAST(i.item_catid AS VARCHAR)) AS item_catid, 
			TRIM(BOTH FROM i.item_name) AS item_name, 
			COALESCE(NULLIF(TRIM(BOTH FROM i.item_status), ''), 'L') AS item_status, 
			COALESCE(NULLIF(TRIM(BOTH FROM i.item_owncaid), ''), '-') AS item_owncaid, 
			COALESCE(NULLIF(TRIM(BOTH FROM i.item_depcaid), ''), '-') AS item_depcaid, 
			COALESCE(NULLIF(TRIM(BOTH FROM i.item_cashcaid), ''), '-') AS item_cashcaid, 
			COALESCE(NULLIF(TRIM(BOTH FROM i.item_aktifyn), ''), 'N') AS item_aktifyn, 
			COALESCE(NULLIF(TRIM(BOTH FROM c.cat_name), ''), NULLIF(TRIM(BOTH FROM CAST(i.item_catid AS VARCHAR)), ''), '-') AS cat_name
		`).
		Joins("LEFT JOIN public.gl_m_categoryitem c ON TRIM(BOTH FROM CAST(i.item_catid AS VARCHAR)) = TRIM(BOTH FROM CAST(c.cat_id AS VARCHAR))")

	if catID != "" {
		query = query.Where("TRIM(BOTH FROM CAST(i.item_catid AS VARCHAR)) = TRIM(BOTH FROM ?)", catID)
	}
	if nama != "" {
		searchPattern := "%" + strings.ToLower(strings.TrimSpace(nama)) + "%"
		query = query.Where("(LOWER(i.item_name) LIKE ? OR LOWER(i.item_id) LIKE ?)", searchPattern, searchPattern)
	}
	if status != "" {
		query = query.Where("TRIM(BOTH FROM i.item_status) = TRIM(BOTH FROM ?)", status)
	}
	if aktifYN != "" {
		query = query.Where("COALESCE(NULLIF(TRIM(BOTH FROM i.item_aktifyn), ''), 'N') = TRIM(BOTH FROM ?)", aktifYN)
	}

	var totalRecords int64
	query.Count(&totalRecords)

	var list []ItemModel
	// Urutkan berdasarkan item_id ASC (sama seperti GL_M_Item.asp)
	err := query.Order("i.item_id ASC").Limit(limit).Offset(offset).Scan(&list).Error
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
// 2. GET /api/gl/master-category-item (DROPDOWN KATEGORI)
// =========================================================================
func GetCategoryItemListHandler(c *gin.Context) {
	database := getItemDB(c)

	var list []CategoryItemModel
	err := database.Table("public.gl_m_categoryitem").
		Select("cat_id, cat_name").
		Where("COALESCE(cat_aktifyn, 'Y') = 'Y'").
		Order("cat_name ASC").
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
// 3. POST /api/gl/pemasukan-pengeluaran (CREATE)
// =========================================================================
func CreateItemHandler(c *gin.Context) {
	database := getItemDB(c)
	userID, _ := c.Get("username")

	var req ItemModel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	req.ItemID = strings.ToUpper(strings.TrimSpace(req.ItemID))
	req.ItemName = strings.ToUpper(strings.TrimSpace(req.ItemName))
	req.ItemCatID = strings.TrimSpace(req.ItemCatID)

	if req.ItemID == "" || req.ItemName == "" || req.ItemCatID == "" || req.ItemOwnCAID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Kode Item, Nama Item, Kategori, dan No. Acc Utama wajib diisi!",
		})
		return
	}

	if req.ItemStatus == "" {
		req.ItemStatus = "L"
	}

	// 🎯 FIX STATUS AKTIF: Dipaksa bernilai "Y" saat diproses
	aktifYN := "Y"
	if req.ItemAktifYN != "" {
		aktifYN = req.ItemAktifYN
	}

	insertData := map[string]interface{}{
		"item_id":         req.ItemID,
		"item_catid":      req.ItemCatID,
		"item_name":       req.ItemName,
		"item_status":     req.ItemStatus,
		"item_owncaid":    req.ItemOwnCAID,
		"item_depcaid":    req.ItemDepCAID,
		"item_cashcaid":   req.ItemCashCAID,
		"item_aktifyn":    aktifYN, // 👈 Tersimpan 'Y' Presisi
		"item_updateid":   fmt.Sprintf("%v", userID),
		"item_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_m_item").Create(insertData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan item: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Item berhasil ditambahkan!",
		"data":    req,
	})
}

// =========================================================================
// 4. PUT /api/gl/pemasukan-pengeluaran/:id (UPDATE)
// =========================================================================
func UpdateItemHandler(c *gin.Context) {
	itemID := c.Param("id")
	database := getItemDB(c)
	userID, _ := c.Get("username")

	var req ItemModel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	req.ItemName = strings.ToUpper(strings.TrimSpace(req.ItemName))

	if req.ItemName == "" || req.ItemCatID == "" || req.ItemOwnCAID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Nama Item, Kategori, dan No. Acc Utama wajib diisi!",
		})
		return
	}

	updateData := map[string]interface{}{
		"item_catid":      req.ItemCatID,
		"item_name":       req.ItemName,
		"item_status":     req.ItemStatus,
		"item_owncaid":    req.ItemOwnCAID,
		"item_depcaid":    req.ItemDepCAID,
		"item_cashcaid":   req.ItemCashCAID,
		"item_updateid":   fmt.Sprintf("%v", userID),
		"item_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_m_item").Where("item_id = ?", itemID).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update item: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data item berhasil diperbarui!",
	})
}

// =========================================================================
// 5. DELETE /api/gl/pemasukan-pengeluaran/:id (HARD DELETE)
// =========================================================================
func DeleteItemHandler(c *gin.Context) {
	itemID := c.Param("id")
	database := getItemDB(c)

	if err := database.Table("public.gl_m_item").Where("item_id = ?", itemID).Delete(&ItemModel{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus item: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Item %s berhasil dihapus permanen!", itemID),
	})
}

// GET /api/gl/cashbank/detail?cb_id=...
func GetCashBankDetailHandler(c *gin.Context) {
	database := getCashBankDB(c)
	cbID := strings.TrimSpace(c.Query("cb_id"))
	if cbID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter cb_id wajib diisi!"})
		return
	}

	type DetailRow struct {
		CbdItemID      string  `json:"cbd_itemid" gorm:"column:cbd_itemid"`
		ItemName       string  `json:"item_name" gorm:"column:item_name"`
		ItemOwnCAID    string  `json:"item_owncaid" gorm:"column:item_owncaid"`
		CbdKet         string  `json:"cbd_ket" gorm:"column:cbd_ket"`
		CbdQuantity    float64 `json:"cbd_quantity" gorm:"column:cbd_quantity"`
		CbdHargaSatuan float64 `json:"cbd_hargasatuan" gorm:"column:cbd_hargasatuan"`
		CbdAgenID      string  `json:"cbd_agenid" gorm:"column:cbd_agenid"`
	}

	var details []DetailRow
	err := database.Table("public.gl_t_cashbankdetil d").
		Select(`
			TRIM(BOTH FROM d.cbd_itemid) AS cbd_itemid,
			TRIM(BOTH FROM COALESCE(i.item_name, '')) AS item_name,
			TRIM(BOTH FROM COALESCE(i.item_owncaid, '')) AS item_owncaid,
			TRIM(BOTH FROM COALESCE(d.cbd_ket, '')) AS cbd_ket,
			d.cbd_quantity,
			d.cbd_hargasatuan,
			TRIM(BOTH FROM COALESCE(d.cbd_agenid, '')) AS cbd_agenid
		`).
		Joins("LEFT JOIN public.gl_m_item i ON TRIM(BOTH FROM d.cbd_itemid) = TRIM(BOTH FROM i.item_id)").
		Where("TRIM(BOTH FROM d.cbd_cbid) = ?", cbID).
		Scan(&details).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"details": details,
	})
}
