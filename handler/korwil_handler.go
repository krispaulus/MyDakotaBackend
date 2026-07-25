package handler

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 📱 1. API GET: List Data Induk Korwil dengan Filter Multi Parameter
func GetKorwilList(c *gin.Context) {
	var result []map[string]interface{}
	searchNama := c.Query("nama")
	searchPJ := c.Query("namapj")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	// Raw join builder kasta tertinggi tembus master karyawan
	query := database.Table("glb_m_korwil AS k").
		Select("k.*, emp.kry_nama, emp.kry_telp1, emp.kry_telp2").
		Joins("LEFT JOIN hrd_m_karyawan emp ON k.nip_karyawan = emp.kry_nip").
		Where("k.korwil_aktif_yn = 'Y'")

	if searchNama != "" {
		query = query.Where("k.nama_wilayah ILIKE ?", "%"+searchNama+"%")
	}
	if searchPJ != "" {
		query = query.Where("emp.kry_nama ILIKE ?", "%"+searchPJ+"%")
	}

	err := query.Order("k.kd_korwil DESC").Find(&result).Error
	if err != nil {
		log.Println("❌ ERROR GetKorwilList:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memuat daftar korwil"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": result})
}

// ➕ 2. API POST: Simpan Data Induk Korwil Baru
func CreateKorwil(c *gin.Context) {
	var input models.GlbMKorwil
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload input tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	// Generate ID unik korwil berbasis waktu lokal logistik
	input.KdKorwil = fmt.Sprintf("KORWIL-%d", time.Now().Unix()/100)
	input.KorwilAktifYN = "Y"

	username := "admin_korwil"
	input.UpdateID = &username

	if err := database.Create(&input).Error; err != nil {
		log.Println("❌ ERROR CreateKorwil:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan wilayah baru"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Koordinator wilayah berhasil didaftarkan!"})
}

// 📝 3. API PUT: Perbarui Info Induk Korwil
func UpdateKorwil(c *gin.Context) {
	id := c.Param("id")
	var input models.GlbMKorwil
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload update tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	err := database.Model(&models.GlbMKorwil{}).Where("kd_korwil = ?", id).Updates(map[string]interface{}{
		"nama_wilayah": input.NamaWilayah,
		"nip_karyawan": input.NipKaryawan,
		"ket_korwil":   input.KetKorwil,
	}).Error

	if err != nil {
		log.Println("❌ ERROR UpdateKorwil:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memperbarui data induk korwil"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Master wilayah berhasil di-update!"})
}

// 🗺️ 4. API GET: List Detail Cakupan Agen Berdasarkan Kode Korwil
func GetKorwilDetail(c *gin.Context) {
	id := c.Param("id")
	var result []map[string]interface{}

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	// Menarik data detail join master agen logistik
	err := database.Table("glb_m_korwil_d AS d").
		Select("d.kd_wilayah_d, d.kd_wilayah_agen_id, a.agen_nama").
		Joins("LEFT JOIN glb_m_agen a ON d.kd_wilayah_agen_id = a.agen_id").
		Where("d.kd_wilayah_d = ?", id).
		Find(&result).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memuat detail cakupan agen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": result})
}

// 🗺️ 5. API POST: Tambah Cakupan Agen ke Dalam Wilayah Korwil
func AddAgenToKorwil(c *gin.Context) {
	var input models.GlbMKorwilD
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload detail tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	if err := database.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Agen sudah terdaftar di cakupan wilayah ini bray!"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Agen berhasil dimasukkan ke cakupan wilayah!"})
}

// 🗑️ 6. API DELETE: Hapus Cakupan Agen dari Korwil
func RemoveAgenFromKorwil(c *gin.Context) {
	kdKorwil := c.Query("kd_korwil")
	agenID := c.Query("agen_id")

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	err := database.Where("kd_wilayah_d = ? AND kd_wilayah_agen_id = ?", kdKorwil, agenID).Delete(&models.GlbMKorwilD{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mencabut cakupan agen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Cakupan agen berhasil dicabut!"})
}
