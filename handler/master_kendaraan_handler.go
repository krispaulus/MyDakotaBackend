package handler

import (
	"net/http"
	"strconv"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// Helper kecil untuk mengubah string biasa ke *string pointer
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// 1. GET LIST DAFTAR KENDARAAN
func GetDaftarKendaraanList(c *gin.Context) {
	database := db.DB // atau db.DB / db.DBDLI

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	nopol := strings.TrimSpace(c.Query("nopol"))
	aktif := strings.TrimSpace(c.Query("aktif"))

	query := database.Model(&models.GlbMKendaraan{})

	if nopol != "" {
		query = query.Where("UPPER(kend_id) LIKE UPPER(?)", "%"+nopol+"%")
	}
	if aktif != "" {
		query = query.Where("kend_aktifyn = ?", aktif)
	}

	var totalRecords int64
	query.Count(&totalRecords)

	var listData []models.GlbMKendaraan
	err := query.Order("kend_id ASC").Offset(offset).Limit(limit).Find(&listData).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data kendaraan: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          listData,
		"total_records": totalRecords,
		"page":          page,
		"limit":         limit,
	})
}

// Struct Payload Request Frontend
type KendaraanInputReq struct {
	KendID          string `json:"kend_id" binding:"required"`
	KendPemilik     string `json:"kend_pemilik"`
	KendAlamat      string `json:"kend_alamat"`
	KendMerk        string `json:"kend_merk"`
	KendJenis       string `json:"kend_jenis"`
	KendBerlakuSTNK string `json:"kend_berlakustnk"`
	KendAktifYN     string `json:"kend_aktifyn"`
	KendGpsImei     string `json:"kend_gps_imei"`
}

// 2. CREATE KENDARAAN BARU
func CreateKendaraanBaru(c *gin.Context) {
	database := db.DB
	var req KendaraanInputReq

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	req.KendID = strings.ToUpper(strings.TrimSpace(req.KendID))

	// Cek Duplikasi No Polisi
	var count int64
	database.Model(&models.GlbMKendaraan{}).Where("kend_id = ?", req.KendID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "No. Polisi " + req.KendID + " sudah terdaftar!"})
		return
	}

	if req.KendAktifYN == "" {
		req.KendAktifYN = "Y"
	}

	newKendaraan := models.GlbMKendaraan{
		KendID:          req.KendID,
		KendPemilik:     strPtr(req.KendPemilik),
		KendAlamat:      strPtr(req.KendAlamat),
		KendMerk:        strPtr(req.KendMerk),
		KendJenis:       strPtr(req.KendJenis),
		KendBerlakuSTNK: strPtr(req.KendBerlakuSTNK),
		KendAktifYN:     req.KendAktifYN,
		KendGpsImei:     strPtr(req.KendGpsImei),
	}

	if err := database.Create(&newKendaraan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan kendaraan: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success", "message": "Data kendaraan berhasil disimpan", "data": newKendaraan})
}

// 3. UPDATE KENDARAAN
func UpdateKendaraanBaru(c *gin.Context) {
	database := db.DB
	kendID := strings.ToUpper(strings.TrimSpace(c.Param("id")))

	var req KendaraanInputReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid"})
		return
	}

	var existing models.GlbMKendaraan
	if err := database.First(&existing, "kend_id = ?", kendID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data kendaraan tidak ditemukan"})
		return
	}

	existing.KendPemilik = strPtr(req.KendPemilik)
	existing.KendAlamat = strPtr(req.KendAlamat)
	existing.KendMerk = strPtr(req.KendMerk)
	existing.KendJenis = strPtr(req.KendJenis)
	existing.KendBerlakuSTNK = strPtr(req.KendBerlakuSTNK)
	existing.KendAktifYN = req.KendAktifYN
	existing.KendGpsImei = strPtr(req.KendGpsImei)

	if err := database.Save(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate kendaraan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data kendaraan berhasil diperbarui"})
}

// 4. DELETE KENDARAAN
func DeleteKendaraanBaru(c *gin.Context) {
	database := db.DB
	kendID := strings.ToUpper(strings.TrimSpace(c.Param("id")))

	if err := database.Delete(&models.GlbMKendaraan{}, "kend_id = ?", kendID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus kendaraan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data kendaraan berhasil dihapus"})
}
