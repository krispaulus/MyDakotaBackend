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

// KelompokPerkiraanModel Struct Master Kelompok Perkiraan (public.gl_m_kelompok)
type KelompokPerkiraanModel struct {
	KID         string    `json:"k_id" gorm:"column:k_id;primaryKey"`
	KName       string    `json:"k_name" gorm:"column:k_name"`
	KAktifYN    string    `json:"k_aktif_yn" gorm:"column:k_aktifyn"`
	KUpdateID   string    `json:"k_update_id" gorm:"column:k_updateid"`
	KUpdateTime time.Time `json:"k_update_time" gorm:"column:k_updatetime"`
}

func getKelompokDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/kelompok-perkiraan (READ LIST & FILTER)
// =========================================================================
func GetDaftarKelompokPerkiraanHandler(c *gin.Context) {
	database := getKelompokDB(c)

	nama := c.Query("nama")
	status := c.Query("status")
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

	query := database.Table("public.gl_m_kelompok").
		Select("k_id, k_name, COALESCE(k_aktifyn, 'Y') AS k_aktifyn, k_updateid, k_updatetime")

	if nama != "" {
		query = query.Where("LOWER(k_name) LIKE ?", "%"+strings.ToLower(nama)+"%")
	}
	if status != "" {
		query = query.Where("k_aktifyn = ?", status)
	}

	var totalRecords int64
	query.Count(&totalRecords)

	var list []KelompokPerkiraanModel
	// Sorting: Data yang baru diupdate/di-add berada di baris teratas
	err := query.Order("k_updatetime DESC, k_id DESC").Limit(limit).Offset(offset).Scan(&list).Error
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
// 2. POST /api/gl/kelompok-perkiraan (CREATE)
// =========================================================================
func CreateKelompokPerkiraanHandler(c *gin.Context) {
	database := getKelompokDB(c)
	userID, _ := c.Get("username")

	var req KelompokPerkiraanModel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	req.KID = strings.ToUpper(strings.TrimSpace(req.KID))
	req.KName = strings.ToUpper(strings.TrimSpace(req.KName))

	if req.KID == "" || req.KName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Kode Kelompok dan Nama Kelompok wajib diisi!",
		})
		return
	}

	aktifYN := "Y"
	if req.KAktifYN != "" {
		aktifYN = req.KAktifYN
	}

	insertData := map[string]interface{}{
		"k_id":         req.KID,
		"k_name":       req.KName,
		"k_aktifyn":    aktifYN,
		"k_updateid":   fmt.Sprintf("%v", userID),
		"k_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_m_kelompok").Create(insertData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan kelompok perkiraan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Kelompok Perkiraan berhasil ditambahkan!",
		"data":    req,
	})
}

// =========================================================================
// 3. PUT /api/gl/kelompok-perkiraan/:id (UPDATE)
// =========================================================================
func UpdateKelompokPerkiraanHandler(c *gin.Context) {
	kID := c.Param("id")
	database := getKelompokDB(c)
	userID, _ := c.Get("username")

	var req KelompokPerkiraanModel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	req.KName = strings.ToUpper(strings.TrimSpace(req.KName))

	if req.KName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Nama Kelompok wajib diisi!",
		})
		return
	}

	updateData := map[string]interface{}{
		"k_name":       req.KName,
		"k_aktifyn":    req.KAktifYN,
		"k_updateid":   fmt.Sprintf("%v", userID),
		"k_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_m_kelompok").Where("k_id = ?", kID).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update kelompok perkiraan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Kelompok Perkiraan berhasil diperbarui!",
	})
}

// =========================================================================
// 4. DELETE /api/gl/kelompok-perkiraan/:id (HARD DELETE)
// =========================================================================
func DeleteKelompokPerkiraanHandler(c *gin.Context) {
	kID := c.Param("id")
	database := getKelompokDB(c)

	if err := database.Table("public.gl_m_kelompok").Where("k_id = ?", kID).Delete(&KelompokPerkiraanModel{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus kelompok perkiraan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Kelompok Perkiraan %s berhasil dihapus permanen!", kID),
	})
}
