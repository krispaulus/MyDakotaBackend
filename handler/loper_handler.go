package handler

import (
	"fmt"
	"net/http"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

func GetLoperList(c *gin.Context) {
	var loperList []models.OprTEloper
	searchID := c.Query("loper_id")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database tidak terhubung"})
		return
	}

	query := database.Model(&models.OprTEloper{})

	if searchID != "" {
		query = query.Where("loper_eid ILIKE ?", "%"+searchID+"%")
	}

	// 🟩 URUTKAN BERDASARKAN ID DESC AGAR PROSES SCANNING STR MURNI AMAN DAN CEPAT BRAY!
	err := query.Order("id DESC").Find(&loperList).Error
	if err != nil {
		fmt.Println("❌ [CRASH QUERY LOPER]:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memuat manifes loper"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   loperList,
	})
}

// 🟩 TAMBAHKAN FUNGSI BARU INI DI loper_handler.go LU BRAY!
func CreateLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTEloper
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid"})
		return
	}

	// Buat nomor surat jalan otomatis/logika penomoran loper bray (Contoh format: LP/TAHUN-BULAN/RANDOM)
	currentTime := time.Now()
	formattedDate := currentTime.Format("2006-01-02 15:04:05")

	// Set default values untuk data baru bray
	req.LoperTanggal = formattedDate[:10] // Simpan tanggal saja YYYY-MM-DD
	req.LoperUpdateTime = formattedDate
	req.LoperAktifYN = "Y"

	if req.LoperKeraniYN == "" {
		req.LoperKeraniYN = "N"
	}

	// Simpan data manifes baru ke PostgreSQL pgAdmin bray!
	if err := database.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan manifes loper baru"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Manifes Loper Baru Berhasil Dibuat!",
		"data":    req,
	})
}
