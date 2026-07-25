package handler

import (
	"fmt"
	"net/http"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 🔍 1. GET LIST DATA AKUN USER DLI & PENCARIAN
func GetUserDLIList(c *gin.Context) {
	var users []models.WebLogin
	searchKeyword := c.Query("keyword")

	// Switch database dinamis sesuai context aktif tenant (dli / dbs)
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung bray"})
		return
	}

	query := database.Model(&models.WebLogin{})

	// Jika ada keyword pencarian (cari berdasarkan username atau nama asli)
	if searchKeyword != "" {
		query = query.Where("username ILIKE ? OR real_name ILIKE ?", "%"+searchKeyword+"%", "%"+searchKeyword+"%")
	}

	err := query.Order("id ASC").Find(&users).Error
	if err != nil {
		fmt.Println("❌ [CRASH GET USER DLI]:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data pengguna DLI"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   users,
	})
}

// 💾 2. POST ENTRI USER BARU (ADD USER INFO)
func CreateUserDLI(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung bray"})
		return
	}

	var req models.WebLogin
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid"})
		return
	}

	// Set default untuk sistem DLI
	req.PT_ID = "DLI"
	if req.User_aktifYN == "" {
		req.User_aktifYN = "Y"
	}
	if req.All_cabangYN == "" {
		req.All_cabangYN = "N"
	}

	// NOTE: Untuk password legacy, lu bisa simpan MD5 dulu sesuai data lama,
	// atau kalau mau modern pakai bcrypt tinggal diganti bray!
	if err := database.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan user baru, username mungkin sudah ada!"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Akun pengguna baru DLI berhasil didaftarkan!",
	})
}
