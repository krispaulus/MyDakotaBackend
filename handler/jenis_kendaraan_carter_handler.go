package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

type OprMKendjenis struct {
	JenisID        string  `gorm:"primaryKey;column:jenis_id" json:"jenis_id"`
	JenisMerk      string  `gorm:"column:jenis_merk;not null" json:"jenis_merk"`
	JenisModel     string  `gorm:"column:jenis_model;not null" json:"jenis_model"`
	JenisHargasewa float64 `gorm:"column:jenis_hargasewa;default:0" json:"jenis_hargasewa"`
	JenisAktifyn   string  `gorm:"column:jenis_aktifyn;default:'Y'" json:"jenis_aktifyn"`
}

func (OprMKendjenis) TableName() string {
	return "public.opr_m_kendjenis"
}

// 1. GET LIST JENIS KENDARAAN CARTER
func GetJenisKendaraanCarterList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	merk := strings.TrimSpace(c.Query("merk"))
	model := strings.TrimSpace(c.Query("model"))

	query := database.Model(&OprMKendjenis{})

	if merk != "" {
		query = query.Where("UPPER(jenis_merk) LIKE UPPER(?)", "%"+merk+"%")
	}
	if model != "" {
		query = query.Where("UPPER(jenis_model) LIKE UPPER(?)", "%"+model+"%")
	}

	var list []OprMKendjenis
	if err := query.Order("jenis_id ASC").Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat jenis kendaraan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// 2. SAVE / UPDATE JENIS KENDARAAN CARTER
// 2. SAVE / UPDATE JENIS KENDARAAN CARTER
func SaveJenisKendaraanCarter(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var input OprMKendjenis
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid"})
		return
	}

	input.JenisID = strings.ToUpper(strings.TrimSpace(input.JenisID))
	input.JenisMerk = strings.ToUpper(strings.TrimSpace(input.JenisMerk))
	input.JenisModel = strings.ToUpper(strings.TrimSpace(input.JenisModel))

	if input.JenisID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Kode Jenis ID wajib diisi"})
		return
	}

	// Cek apakah data sudah ada di DB
	var count int64
	database.Model(&OprMKendjenis{}).Where("jenis_id = ?", input.JenisID).Count(&count)

	if count > 0 {
		err := database.Model(&OprMKendjenis{}).Where("jenis_id = ?", input.JenisID).Updates(input).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update jenis kendaraan"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Jenis kendaraan berhasil diperbarui"})
		return
	}

	if err := database.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menambah jenis kendaraan baru"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Jenis kendaraan baru berhasil disimpan"})
}

// 3. DELETE JENIS KENDARAAN CARTER
func DeleteJenisKendaraanCarter(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	id := c.Param("id")
	if err := database.Where("jenis_id = ?", id).Delete(&OprMKendjenis{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus jenis kendaraan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Jenis kendaraan berhasil dihapus"})
}
