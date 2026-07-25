package handler

import (
	"fmt"
	"net/http"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 🔍 1. PENCARIAN & HISTORY LIST DATA PENGAMBILAN
func GetAmbilList(c *gin.Context) {
	var ambilList []models.OprTEambil
	searchID := c.Query("ambil_id")
	searchBtt := c.Query("btt_id")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	// Sesuai query ASP: SELECT ... LEFT OUTER JOIN HRD_M_Karyawan ON ...[cite: 9]
	query := database.Table("opr_t_eambil").
		Select("opr_t_eambil.*, hrd_m_karyawan.kry_nama").
		Joins("LEFT OUTER JOIN hrd_m_karyawan ON opr_t_eambil.ambil_checkernip = hrd_m_karyawan.kry_nip")

	// Filter pencarian taktis bray
	if searchID != "" {
		query = query.Where("opr_t_eambil.ambil_id ILIKE ?", "%"+searchID+"%")
	}
	if searchBtt != "" {
		query = query.Where("opr_t_eambil.ambil_bttid ILIKE ?", "%"+searchBtt+"%")
	}

	// Eksekusi sorting data terbaru
	err := query.Order("opr_t_eambil.id DESC").Find(&ambilList).Error
	if err != nil {
		fmt.Println("❌ [CRASH QUERY AMBIL BARANG]:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data pengambilan barang"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   ambilList,
	})
}

// 💾 2. ENTRI / PEMBUATAN TRANSAKSI PENGAMBILAN BARANG BARU
func CreateAmbil(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTEambil
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid"})
		return
	}

	// Generate Waktu Sinkron server log bray
	now := time.Now()
	req.AmbilTanggal = now.Format("2006-01-02")
	req.AmbilJam = now.Format("15")
	req.AmbilMenit = now.Format("04")
	req.AmbilUpdateTime = now.Format("2006-01-02 15:04:05")
	req.AmbilAktifYN = "Y"
	if req.AmbilSKYN == "" {
		req.AmbilSKYN = "N"
	}

	// Simpan transaksi baru langsung ke Postgres
	if err := database.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memproses penyerahan barang"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Transaksi penyerahan barang berhasil direkam!",
		"data":    req,
	})
}
