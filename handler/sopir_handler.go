package handler

import (
	"fmt"
	"log"
	"net/http"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 📱 1. API GET: Ambil Daftar Sopir dengan Filter Nama
func GetSupirList(c *gin.Context) {
	var supir []models.OprMSupir
	searchNama := c.Query("nama")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	query := database.Model(&models.OprMSupir{})

	// Filter pencarian nama sopir se-Nusantara bray
	if searchNama != "" {
		query = query.Where("supir_nama ILIKE ?", "%"+searchNama+"%")
	}

	err := query.Order("supir_id DESC").Find(&supir).Error
	if err != nil {
		log.Println("❌ ERROR GetSupirList:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memuat data sopir"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": supir})
}

// ➕ 2. API POST: Pendaftaran Sopir & Pairing IMEI Baru
func CreateSupir(c *gin.Context) {
	var input models.OprMSupir
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload input tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	// Mengambil username dari token JWT untuk audit log supir_update_id
	username, _ := c.Get("username")
	usernameStr := fmt.Sprintf("%v", username)
	input.SupirUpdateID = &usernameStr
	input.SupirAktifYN = "Y"

	if err := database.Create(&input).Error; err != nil {
		log.Println("❌ ERROR CreateSupir:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mendaftarkan sopir, ID kemungkinan duplikat!"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Supir baru berhasil didaftarkan!"})
}

// 📝 3. API PUT: Edit Data Sopir & Update Identitas Device Tracker
func UpdateSupir(c *gin.Context) {
	id := c.Param("id")
	var input models.OprMSupir
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload update tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	username, _ := c.Get("username")
	usernameStr := fmt.Sprintf("%v", username)

	err := database.Model(&models.OprMSupir{}).Where("supir_id = ?", id).Updates(map[string]interface{}{
		"supir_nama":        input.SupirNama,
		"supir_borongan_yn": input.SupirBoronganYN,
		"supir_imei1":       input.SupirImei1,
		"supir_imei2":       input.SupirImei2,
		"supir_simcard_id1": input.SupirSimcardID1,
		"supir_simcard_id2": input.SupirSimcardID2,
		"supir_update_id":   usernameStr,
	}).Error

	if err != nil {
		log.Println("❌ ERROR UpdateSupir:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memperbarui data sopir"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data sopir berhasil diperbarui bray!"})
}

// 🗑️ 4. API DELETE: Soft Delete / Non-Aktifkan Sopir (Ubah Flag Aktif ke 'N')
func DeleteSupir(c *gin.Context) {
	id := c.Param("id")

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	username, _ := c.Get("username")
	usernameStr := fmt.Sprintf("%v", username)

	// Sesuai pakem logistik, kita main soft delete pakai flag supir_aktif_yn = 'N' bray!
	err := database.Model(&models.OprMSupir{}).Where("supir_id = ?", id).Updates(map[string]interface{}{
		"supir_aktif_yn":  "N",
		"supir_update_id": usernameStr,
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menonaktifkan sopir"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Status sopir resmi dinonaktifkan!"})
}
