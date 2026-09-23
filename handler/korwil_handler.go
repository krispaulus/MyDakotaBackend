package handler

import (
	"log"
	"net/http"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// Model sesuai tabel riil weblogin_groupkorwil
type WebloginGroupKorwil struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Username    string    `gorm:"column:username" json:"username"`
	KodeWilayah string    `gorm:"column:kode_wilayah" json:"kode_wilayah"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (WebloginGroupKorwil) TableName() string {
	return "weblogin_groupkorwil"
}

// 1. GET: Ambil Daftar Group Korwil
func GetKorwilList(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database DLI gagal"})
		return
	}

	searchNama := c.Query("nama")
	searchKode := c.Query("kode")

	var list []WebloginGroupKorwil
	query := database.Model(&WebloginGroupKorwil{})

	if searchNama != "" {
		query = query.Where("username ILIKE ?", "%"+searchNama+"%")
	}
	if searchKode != "" {
		query = query.Where("kode_wilayah ILIKE ?", "%"+searchKode+"%")
	}

	if err := query.Order("id DESC").Find(&list).Error; err != nil {
		log.Println("❌ ERROR GetKorwilList:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data korwil"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// 2. POST: Tambah Data Baru
func CreateKorwil(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database DLI tidak tersedia"})
		return
	}

	var input struct {
		Username    string `json:"username" binding:"required"`
		KodeWilayah string `json:"kode_wilayah" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Username dan Kode Wilayah wajib diisi"})
		return
	}

	newEntry := WebloginGroupKorwil{
		Username:    input.Username,
		KodeWilayah: input.KodeWilayah,
		UpdatedAt:   time.Now(),
	}

	if err := database.Create(&newEntry).Error; err != nil {
		log.Println("❌ ERROR CreateKorwil:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan data korwil"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data Korwil berhasil disimpan!",
		"data":    newEntry,
	})
}

// 3. PUT: Update Data Korwil
func UpdateKorwil(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database DLI tidak tersedia"})
		return
	}

	id := c.Param("id")
	var input struct {
		Username    string `json:"username"`
		KodeWilayah string `json:"kode_wilayah"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Data update tidak valid"})
		return
	}

	updates := map[string]interface{}{
		"username":     input.Username,
		"kode_wilayah": input.KodeWilayah,
		"updated_at":   time.Now(),
	}

	if err := database.Model(&WebloginGroupKorwil{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		log.Println("❌ ERROR UpdateKorwil:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui data korwil"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data Korwil berhasil diperbarui!"})
}

// 4. DELETE: Hapus Data Korwil
func DeleteKorwil(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database DLI tidak tersedia"})
		return
	}

	id := c.Param("id")
	if err := database.Where("id = ?", id).Delete(&WebloginGroupKorwil{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus data korwil"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data Korwil berhasil dihapus!"})
}

// 5. GET Detail Korwil (Placeholder agar compile tidak error)
func GetKorwilDetail(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"data":    []interface{}{},
		"message": "Detail agen tidak digunakan pada struktur weblogin_groupkorwil",
	})
}

// 6. POST Add Agen ke Korwil (Placeholder)
func AddAgenToKorwil(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Fitur relasi agen belum diterapkan",
	})
}

// 7. DELETE Remove Agen dari Korwil (Placeholder)
func RemoveAgenFromKorwil(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Fitur relasi agen belum diterapkan",
	})
}
