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

// 📱 1. API GET: List Data Sewa Kendaraan dengan Multi Filter
func GetSewaKendaraanList(c *gin.Context) {
	var result []map[string]interface{}

	nopol := c.Query("nopol")
	tgla := c.Query("tgla")
	tgle := c.Query("tgle")

	// Pake Query Builder Raw selection biar join data aman tanpa sengketa struct bray!
	query := db.DB.Table("opr_t_kendaraan_sewa AS s").
		Select(`s.*, j.jnskend_nama`).
		Joins(`LEFT JOIN glb_m_jnskend j ON s.jns_kend_id = j.jnskend_id`).
		Where(`s.aktif_yn = 'Y'`) // Menyaring record aktif

	// Filter No. Kendaraan jika dicentang di frontend
	if nopol != "" {
		query = query.Where(`s.no_kendaraan LIKE ?`, "%"+nopol+"%")
	}
	// Filter Range Tanggal Mulai Sewa
	if tgla != "" {
		query = query.Where(`s.tgl_start_sewa >= ?`, tgla+" 00:00:00")
	}
	// Filter Range Tanggal Berakhir Sewa
	if tgle != "" {
		query = query.Where(`s.tgl_end_sewa <= ?`, tgle+" 23:59:59")
	}

	err := query.Order("s.sewa_id DESC").Find(&result).Error
	if err != nil {
		log.Println("❌ ERROR GetSewaKendaraanList:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memuat list kendaraan sewa"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": result})
}

// ➕ 2. API POST: Simpan Pendaftaran Sewa Kendaraan Baru
func CreateSewaKendaraan(c *gin.Context) {
	var input models.OprTKendaraanSewa
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload input sewa tidak valid"})
		return
	}

	// Generate Sewa ID unik berbasis timestamp lokalan logistik
	now := time.Now()
	input.SewaID = fmt.Sprintf("SEWA-%d", now.UnixNano()/1e6)

	// Bind log operator pembuat data
	username := "admin_lokal"
	input.UpdateIDSewa = &username
	input.AktifYN = "Y" // Otomatis aktif

	if err := db.DB.Create(&input).Error; err != nil {
		log.Println("❌ ERROR CreateSewaKendaraan:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan data sewa kendaraan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Armada sewa berhasil disimpan!"})
}

// 📝 3. API PUT: Perbarui Kontrak Sewa Vendor
func UpdateSewaKendaraan(c *gin.Context) {
	id := c.Param("id")
	var input models.OprTKendaraanSewa
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload update tidak valid"})
		return
	}

	err := db.DB.Model(&models.OprTKendaraanSewa{}).Where(`sewa_id = ?`, id).Updates(map[string]interface{}{
		"vendor_sewa_id": input.VendorSewaID,
		"cabang_id":      input.CabangID,
		"jns_kend_id":    input.JnsKendID,
		"biaya_sewa":     input.BiayaSewa,
		"tgl_start_sewa": input.TglStartSewa,
		"tgl_end_sewa":   input.TglEndSewa,
		"kend_sopir1":    input.KendSopir1,
		"kend_sopir2":    input.KendSopir2,
	}).Error

	if err != nil {
		log.Println("❌ ERROR UpdateSewaKendaraan:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal meng-update kontrak sewa"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Kontrak sewa berhasil diperbarui!"})
}

// 🗑️ 4. API DELETE: Soft Delete ubah AktifYN menjadi 'N'
func DeleteSewaKendaraan(c *gin.Context) {
	id := c.Param("id")
	err := db.DB.Model(&models.OprTKendaraanSewa{}).Where(`sewa_id = ?`, id).Update("aktif_yn", "N").Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menonaktifkan sewa"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Armada sewa dinonaktifkan"})
}
