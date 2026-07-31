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

// Helper internal pengambil koneksi DB dinamis
func getTarifCustomerDB(c *gin.Context) *gorm.DB {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" || fmt.Sprintf("%v", ptID) == "<nil>" {
		ptID = "A"
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		if db.DB != nil {
			return db.DB
		}
		return db.DB
	}
	return database
}

// 1. GET LIST CUSTOMER + JUMLAH TARIF DITERAPKAN (SESUAI CLASSIC ASP MKT_M_EHARGA_PELANGGAN.ASP)
func GetTarifCustomerList(c *gin.Context) {
	database := getTarifCustomerDB(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset := (page - 1) * limit

	customerName := strings.TrimSpace(c.Query("customer_name"))
	agenID := strings.TrimSpace(c.Query("agen_id"))

	// Query LEFT JOIN untuk menghitung jumlah rute/tarif khusus yang dimiliki customer
	query := database.Table("public.mkt_m_customer c").
		Select("c.cust_id, c.cust_name, c.cust_alamat1, c.cust_alamat2, c.cust_telp1, c.cust_telp2, COUNT(p.customerid) as jml").
		Joins("LEFT JOIN public.mkt_m_eharga_pelanggan p ON c.cust_id = p.customerid").
		Where("c.cust_aktifyn = ?", "Y")

	if agenID != "" && agenID != "ALL" && !strings.Contains(strings.ToUpper(agenID), "PUSAT") {
		query = query.Where("c.cust_id LIKE ?", agenID+"%")
	}

	if customerName != "" {
		query = query.Where("UPPER(c.cust_name) LIKE UPPER(?)", "%"+customerName+"%")
	}

	query = query.Group("c.cust_id, c.cust_name, c.cust_alamat1, c.cust_alamat2, c.cust_telp1, c.cust_telp2")

	// Hitung total records
	var totalRecords int64
	database.Table("public.mkt_m_customer c").Where("c.cust_aktifyn = ?", "Y").Count(&totalRecords)

	type CustomerTarifSummary struct {
		CustID      string `json:"cust_id"`
		CustName    string `json:"cust_name"`
		CustAlamat1 string `json:"cust_alamat1"`
		CustAlamat2 string `json:"cust_alamat2"`
		CustTelp1   string `json:"cust_telp1"`
		CustTelp2   string `json:"cust_telp2"`
		Jml         int64  `json:"jml"`
	}

	results := make([]CustomerTarifSummary, 0)
	err := query.Order("c.cust_name ASC").
		Offset(offset).
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data tarif customer: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          results,
		"total_records": totalRecords,
		"page":          page,
		"limit":         limit,
	})
}

// 2. GET DETAIL RUTE TARIF MILIK 1 CUSTOMER SPESIFIK
func GetDetailTarifByCustomer(c *gin.Context) {
	database := getTarifCustomerDB(c)
	customerID := strings.TrimSpace(c.Param("customer_id"))

	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Customer ID wajib diisi"})
		return
	}

	results := make([]models.MktMEHargaPelanggan, 0)
	err := database.Where("customerid = ?", customerID).Order("kota_asal ASC, kota_tujuan ASC").Find(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengambil detail tarif customer"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

// 3. TAMBAH / UPDATE TARIF KHUSUS CUSTOMER (SAVE & UPSERT)
func SaveTarifCustomer(c *gin.Context) {
	database := getTarifCustomerDB(c)

	var input models.MktMEHargaPelanggan
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input data tidak valid: " + err.Error()})
		return
	}

	input.CustomerID = strings.ToUpper(strings.TrimSpace(input.CustomerID))
	input.KotaAsal = strings.ToUpper(strings.TrimSpace(input.KotaAsal))
	input.KotaTujuan = strings.ToUpper(strings.TrimSpace(input.KotaTujuan))
	input.Kabupaten = strings.ToUpper(strings.TrimSpace(input.Kabupaten))

	// Jika ID ada, lakukan update
	if input.ID > 0 {
		err := database.Model(&models.MktMEHargaPelanggan{}).Where("id = ?", input.ID).Updates(input).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate tarif customer"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Tarif customer berhasil diperbarui"})
		return
	}

	// Jika ID tidak ada, lakukan INSERT baru
	if err := database.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan tarif customer baru: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Tarif khusus customer berhasil ditambahkan",
		"data":    input,
	})
}

// 4. DELETE TARIF KHUSUS CUSTOMER (SATUAN ATAU ALL PER CUSTOMER)
func DeleteTarifCustomer(c *gin.Context) {
	database := getTarifCustomerDB(c)

	type DeleteTarifInput struct {
		ID         uint   `json:"id"`
		CustomerID string `json:"customer_id"`
	}

	var input DeleteTarifInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload input ID tidak valid"})
		return
	}

	// Jika ID Rute spesifik dikirim
	if input.ID > 0 {
		if err := database.Delete(&models.MktMEHargaPelanggan{}, input.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus rute tarif"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Rute tarif customer berhasil dihapus"})
		return
	}

	// Jika Menghapus seluruh tarif milik customer
	if input.CustomerID != "" {
		if err := database.Where("customerid = ?", input.CustomerID).Delete(&models.MktMEHargaPelanggan{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus seluruh tarif customer"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Seluruh tarif khusus customer berhasil dihapus"})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID atau Customer ID wajib diisi"})
}
