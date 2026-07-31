package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 1. GET LIST VENDOR (Dengan Filter Nama, Status Aktif, dan Cabang/ServerID)
func GetVendorList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	nama := strings.TrimSpace(c.Query("nama"))
	aktif := strings.TrimSpace(c.Query("aktif")) // Y / N
	agenID := strings.TrimSpace(c.Query("agen_id"))

	query := database.Model(&models.MktMVendor{})

	// Filter Nama Vendor
	if nama != "" {
		query = query.Where("UPPER(vend_name) LIKE UPPER(?)", "%"+nama+"%")
	}

	// Filter Status Aktif (Default 'Y' jika tidak diisi)
	if aktif != "" {
		query = query.Where("vend_aktifyn = ?", aktif)
	}

	// Filter Agen / Cabang ID jika spesifik
	if agenID != "" && agenID != "ALL" && agenID != "PUSAT DAKOTA" {
		query = query.Where("vend_agenid = ? OR LEFT(vend_id, 3) = ?", agenID, agenID)
	}

	var vendors []models.MktMVendor
	if err := query.Order("vend_id ASC").Find(&vendors).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat daftar vendor"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": vendors})
}

// 2. SAVE / UPDATE VENDOR
func SaveVendor(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)

	var input models.MktMVendor
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload vendor tidak valid"})
		return
	}

	input.VendID = strings.ToUpper(strings.TrimSpace(input.VendID))
	input.VendName = strings.ToUpper(strings.TrimSpace(input.VendName))
	input.VendUpdateID = userStr
	input.VendUpdateTime = time.Now()

	if input.VendID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Kode Vendor wajib diisi"})
		return
	}

	// Pengecekan Upsert (Apakah ID Vendor sudah ada?)
	var count int64
	database.Model(&models.MktMVendor{}).Where("vend_id = ?", input.VendID).Count(&count)

	if count > 0 {
		// Update Data Vendor
		err := database.Model(&models.MktMVendor{}).Where("vend_id = ?", input.VendID).Updates(input).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui data vendor"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data vendor berhasil diperbarui"})
		return
	}

	// Create Data Vendor Baru
	if err := database.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menambah vendor baru"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Vendor baru berhasil disimpan"})
}

// 3. DELETE VENDOR
func DeleteVendor(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID Vendor wajib disertakan"})
		return
	}

	if err := database.Where("vend_id = ?", id).Delete(&models.MktMVendor{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus vendor"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Vendor berhasil dihapus"})
}
