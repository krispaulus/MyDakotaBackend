package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Helper Dinamis: Mengambil DB sesuai context request user login
func getDynamicDB(c *gin.Context) *gorm.DB {
	// 1. Ambil PT ID dari context GIN (dari AuthMiddleware)
	if ptID, exists := c.Get("pt_id"); exists {
		if conn, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
			return conn
		}
	}

	// 2. Cek apakah ada instance *gorm.DB di Gin Context (dari AuthMiddleware)
	if val, exists := c.Get("db"); exists {
		if conn, ok := val.(*gorm.DB); ok {
			return conn
		}
	}

	// 3. Fallback utama ke variabel db.DB standar
	return db.DB
}

func GetListLeadTime(c *gin.Context) {
	database := getDynamicDB(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 {
		limit = 100
	}
	offset := (page - 1) * limit

	// Trim whitespace
	customerID := strings.TrimSpace(c.Query("customer_id"))
	kotaAsal := strings.TrimSpace(c.Query("kota_asal"))
	kotaTujuan := strings.TrimSpace(c.Query("kota_tujuan"))

	// Base Query
	dbQuery := database.Table("public.opr_t_transportplanningleadtime lt").
		Select("lt.customer_id, COALESCE(c.cust_name, lt.customer_id) as cust_name, lt.kota_asal, lt.kota_tujuan, lt.kabupaten, lt.darat, lt.laut, lt.udara").
		Joins("LEFT JOIN public.mkt_m_customer c ON c.cust_id = lt.customer_id")

	// 🟢 Filter hanya berjalan jika string TIDAK kosong
	if len(customerID) > 0 {
		dbQuery = dbQuery.Where("UPPER(lt.customer_id) LIKE UPPER(?)", "%"+customerID+"%")
	}
	if len(kotaAsal) > 0 {
		dbQuery = dbQuery.Where("UPPER(lt.kota_asal) LIKE UPPER(?)", "%"+kotaAsal+"%")
	}
	if len(kotaTujuan) > 0 {
		dbQuery = dbQuery.Where("UPPER(lt.kota_tujuan) LIKE UPPER(?)", "%"+kotaTujuan+"%")
	}

	// Inisialisasi slice
	results := make([]models.TransportPlanningLeadTime, 0)

	// Fetch Data
	err := dbQuery.Order("lt.customer_id ASC, lt.kota_asal ASC, lt.kota_tujuan ASC").
		Offset(offset).
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data lead time: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
		"page":   page,
		"limit":  limit,
	})
}

// 2. CREATE LEAD TIME BARU
func CreateLeadTime(c *gin.Context) {
	database := getDynamicDB(c)
	var input models.TransportPlanningLeadTime

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	input.CustomerID = strings.ToUpper(strings.TrimSpace(input.CustomerID))
	input.KotaAsal = strings.TrimSpace(input.KotaAsal)
	input.KotaTujuan = strings.TrimSpace(input.KotaTujuan)
	input.Kabupaten = strings.TrimSpace(input.Kabupaten)

	if err := database.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan data lead time: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success", "message": "Data lead time berhasil disimpan", "data": input})
}

// 3. UPDATE LEAD TIME
func UpdateLeadTime(c *gin.Context) {
	database := getDynamicDB(c)
	var input models.TransportPlanningLeadTime

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	err := database.Model(&models.TransportPlanningLeadTime{}).
		Where("customer_id = ? AND kota_asal = ? AND kota_tujuan = ? AND kabupaten = ?",
			input.CustomerID, input.KotaAsal, input.KotaTujuan, input.Kabupaten).
		Updates(map[string]interface{}{
			"darat": input.Darat,
			"laut":  input.Laut,
			"udara": input.Udara,
		}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate data lead time: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data lead time berhasil diperbarui"})
}

// 4. DELETE LEAD TIME
func DeleteLeadTime(c *gin.Context) {
	database := getDynamicDB(c)

	cid := strings.TrimSpace(c.Query("customer_id"))
	asal := strings.TrimSpace(c.Query("kota_asal"))
	tujuan := strings.TrimSpace(c.Query("kota_tujuan"))
	kab := strings.TrimSpace(c.Query("kabupaten"))

	if cid == "" || asal == "" || tujuan == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter tidak lengkap"})
		return
	}

	err := database.Where("customer_id = ? AND kota_asal = ? AND kota_tujuan = ? AND kabupaten = ?", cid, asal, tujuan, kab).
		Delete(&models.TransportPlanningLeadTime{}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus data lead time: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data lead time berhasil dihapus"})
}
