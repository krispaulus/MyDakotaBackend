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

	customerID := strings.TrimSpace(c.Query("customer_id"))
	kotaAsal := strings.TrimSpace(c.Query("kota_asal"))
	kotaTujuan := strings.TrimSpace(c.Query("kota_tujuan"))

	type LeadTimeRow struct {
		CustomerID string `gorm:"column:customer_id" json:"customer_id"`
		CustName   string `gorm:"column:cust_name" json:"cust_name"`
		KotaAsal   string `gorm:"column:kota_asal" json:"kota_asal"`
		KotaTujuan string `gorm:"column:kota_tujuan" json:"kota_tujuan"`
		Kabupaten  string `gorm:"column:kabupaten" json:"kabupaten"`
		Darat      int    `gorm:"column:darat" json:"darat"`
		Laut       int    `gorm:"column:laut" json:"laut"`
		Udara      int    `gorm:"column:udara" json:"udara"`
	}

	var results []LeadTimeRow

	sqlQuery := `
		SELECT 
			COALESCE(lt.customer_id, '') AS customer_id,
			COALESCE(c.cust_name, lt.customer_id, '') AS cust_name,
			COALESCE(lt.kota_asal, '') AS kota_asal,
			COALESCE(lt.kota_tujuan, '') AS kota_tujuan,
			COALESCE(lt.kabupaten, '') AS kabupaten,
			COALESCE(lt.darat, 0) AS darat,
			COALESCE(lt.laut, 0) AS laut,
			COALESCE(lt.udara, 0) AS udara
		FROM public.opr_t_transportplanningleadtime lt
		LEFT JOIN public.mkt_m_customer c ON TRIM(UPPER(c.cust_id)) = TRIM(UPPER(lt.customer_id))
		WHERE 1=1
	`
	var args []interface{}

	if len(customerID) > 0 {
		sqlQuery += " AND UPPER(lt.customer_id) LIKE UPPER(?)"
		args = append(args, "%"+customerID+"%")
	}
	if len(kotaAsal) > 0 {
		sqlQuery += " AND UPPER(lt.kota_asal) LIKE UPPER(?)"
		args = append(args, "%"+kotaAsal+"%")
	}
	if len(kotaTujuan) > 0 {
		sqlQuery += " AND UPPER(lt.kota_tujuan) LIKE UPPER(?)"
		args = append(args, "%"+kotaTujuan+"%")
	}

	sqlQuery += " ORDER BY lt.customer_id ASC, lt.kota_asal ASC, lt.kota_tujuan ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	if err := database.Raw(sqlQuery, args...).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data lead time: " + err.Error(),
		})
		return
	}

	// Cetak sampel 1 baris data asli di terminal
	if len(results) > 0 {
		fmt.Printf("DATA ASLI DB -> CustID: [%s] | CustName: [%s] | Asal: [%s] | Tujuan: [%s] | Kab: [%s]\n",
			results[0].CustomerID, results[0].CustName, results[0].KotaAsal, results[0].KotaTujuan, results[0].Kabupaten)
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

func GetActiveCustomerList(c *gin.Context) {
	database := getDynamicDB(c)

	type CustOption struct {
		CustID   string `gorm:"column:cust_id" json:"cust_id"`
		CustName string `gorm:"column:cust_name" json:"cust_name"`
	}

	var customers []CustOption
	err := database.Table("public.mkt_m_customer").
		Select("cust_id, cust_name").
		Where("cust_aktifyn = 'Y'").
		Order("cust_name ASC").
		Scan(&customers).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   customers,
	})
}
