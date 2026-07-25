package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetKodePos(c *gin.Context) {
	var data []models.KodePos
	ptID, _ := c.Get("pt_id")
	search := c.Query("search")

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB tidak ditemukan"})
		return
	}

	query := database.Model(&models.KodePos{})

	if search != "" {
		query = query.Where("kodepos ILIKE ? OR desakelurahan ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Data kodepos Indonesia itu 80ribu lebih, wajib LIMIT!
	if err := query.Limit(100).Order("kodepos asc").Find(&data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// ➕ 2. API POST: Tambah Data Wilayah / Kode Pos Baru
func CreateKodePos(c *gin.Context) {
	var input models.KodePos
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload input tidak valid bray"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB tidak ditemukan"})
		return
	}

	// Simpan data baru langsung ke database operasional terkait
	if err := database.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan kode pos, kemungkinan duplikat!"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Wilayah baru sukses didaftarkan!"})
}

// 📝 3. API PUT: Update / Edit Atribut Wilayah Kode Pos
func UpdateKodePos(c *gin.Context) {
	id := c.Param("id") // Menggunakan parameter kode pos lama sebagai kunci bray
	var input models.KodePos
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload update tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB tidak ditemukan"})
		return
	}

	err := database.Model(&models.KodePos{}).Where("kodepos = ?", id).Updates(map[string]interface{}{
		"desakelurahan":    input.DesaKelurahan,    // 💡 K diganti kapital
		"kecamatandistrik": input.KecamatanDistrik, // 💡 D diganti kapital
		"kotakabupaten":    input.KotaKabupaten,    // 💡 K diganti kapital
		"propinsi":         input.Propinsi,
		"area":             input.Area,
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui data wilayah"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data wilayah berhasil diperbarui!"})
}

// 🗑️ 4. API DELETE: Hapus Data Kode Pos Permanen Dari Sistem
func DeleteKodePos(c *gin.Context) {
	id := c.Param("id")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB tidak ditemukan"})
		return
	}

	if err := database.Where("kodepos = ?", id).Delete(&models.KodePos{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus data wilayah"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Kode pos resmi dihapus dari sistem"})
}
