package handler

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Model tabel tarif multidrop
type TarifCustomerMultidrop struct {
	ID              int64   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CustomerID      string  `gorm:"column:customer_id" json:"customer_id"`
	TujuanPropinsi  string  `gorm:"column:tujuan_propinsi" json:"tujuan_propinsi"`
	TujuanKabupaten string  `gorm:"column:tujuan_kabupaten" json:"tujuan_kabupaten"`
	TujuanKecamatan string  `gorm:"column:tujuan_kecamatan" json:"tujuan_kecamatan"`
	HargaKg         float64 `gorm:"column:harga_kg" json:"harga_kg"`
	KgMin           float64 `gorm:"column:kg_min" json:"kg_min"`
	KgMax           float64 `gorm:"column:kg_max" json:"kg_max"`
	HargaOverKgMax  float64 `gorm:"column:harga_over_kg_max" json:"harga_over_kg_max"`
	HargaKgDG       float64 `gorm:"column:harga_kg_dg" json:"harga_kg_dg"`
}

func (TarifCustomerMultidrop) TableName() string {
	return "mkt_t_tarif_customer_multidrop"
}

// 1. GET DETAIL TARIF MULTIDROP PER CUSTOMER
func GetTarifMultidropDetail(c *gin.Context) {
	customerID := c.Query("customer_id")
	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "customer_id wajib diisi"})
		return
	}

	ptID, _ := c.Get("pt_id")
	ptKey := fmt.Sprintf("%v", ptID)
	if ptKey == "" || ptKey == "<nil>" {
		ptKey = "C" // Default ke DLI
	}

	database, ok := db.ResolveDB(ptKey)
	if !ok || database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak ditemukan"})
		return
	}

	var list []TarifCustomerMultidrop
	err := database.Where("customer_id = ?", customerID).
		Order("tujuan_propinsi ASC, tujuan_kabupaten ASC, tujuan_kecamatan ASC").
		Find(&list).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error(), "data": []any{}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// 2. INLINE AUTOSAVE SATU FIELD (onBlur)
func UpdateInlineMultidrop(c *gin.Context) {
	var req struct {
		ID    int64   `json:"id" binding:"required"`
		Field string  `json:"field" binding:"required"`
		Value float64 `json:"value"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload tidak valid"})
		return
	}

	// Whitelist kolom numerik yang boleh diupdate inline
	allowedFields := map[string]string{
		"harga_kg":          "harga_kg",
		"kg_min":            "kg_min",
		"kg_max":            "kg_max",
		"harga_over_kg_max": "harga_over_kg_max",
		"harga_kg_dg":       "harga_kg_dg",
	}

	colName, allowed := allowedFields[req.Field]
	if !allowed {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Field tidak diizinkan untuk diubah"})
		return
	}

	ptID, _ := c.Get("pt_id")
	ptKey := fmt.Sprintf("%v", ptID)
	if ptKey == "" || ptKey == "<nil>" {
		ptKey = "C"
	}

	database, ok := db.ResolveDB(ptKey)
	if !ok || database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak ditemukan"})
		return
	}

	err := database.Model(&TarifCustomerMultidrop{}).Where("id = ?", req.ID).Update(colName, req.Value).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Berhasil memperbarui nilai"})
}

// 3. HAPUS BARIS RUTE MULTIDROP
func DeleteTarifMultidropItem(c *gin.Context) {
	var req struct {
		ID int64 `json:"id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID rute tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	ptKey := fmt.Sprintf("%v", ptID)
	if ptKey == "" || ptKey == "<nil>" {
		ptKey = "C"
	}

	database, ok := db.ResolveDB(ptKey)
	if !ok || database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak ditemukan"})
		return
	}

	err := database.Where("id = ?", req.ID).Delete(&TarifCustomerMultidrop{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Rute multidrop berhasil dihapus"})
}

// 4. SIMPAN RUTE BARU DARI MODAL
func SaveTarifMultidrop(c *gin.Context) {
	var req struct {
		CustomerID     string  `json:"customer_id" binding:"required"`
		Propinsi       string  `json:"propinsi" binding:"required"`
		Kabupaten      string  `json:"kabupaten" binding:"required"`
		Kecamatan      string  `json:"kecamatan"`
		HargaKg        float64 `json:"harga_kg"`
		KgMin          float64 `json:"kg_min"`
		KgMax          float64 `json:"kg_max"`
		HargaOverKgMax float64 `json:"harga_over_kg_max"`
		HargaKgDG      float64 `json:"harga_kg_dg"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Data rute tidak lengkap"})
		return
	}

	ptID, _ := c.Get("pt_id")
	ptKey := fmt.Sprintf("%v", ptID)
	if ptKey == "" || ptKey == "<nil>" {
		ptKey = "C"
	}

	database, ok := db.ResolveDB(ptKey)
	if !ok || database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak ditemukan"})
		return
	}

	item := TarifCustomerMultidrop{
		CustomerID:      req.CustomerID,
		TujuanPropinsi:  req.Propinsi,
		TujuanKabupaten: req.Kabupaten,
		TujuanKecamatan: req.Kecamatan,
		HargaKg:         req.HargaKg,
		KgMin:           req.KgMin,
		KgMax:           req.KgMax,
		HargaOverKgMax:  req.HargaOverKgMax,
		HargaKgDG:       req.HargaKgDG,
	}

	// Auto-create tabel jika belum dibuat sebelumnya di PostgreSQL
	if err := database.AutoMigrate(&TarifCustomerMultidrop{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal sinkronisasi skema tabel: " + err.Error()})
		return
	}

	if err := database.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Tarif multidrop berhasil disimpan",
		"data":    item,
	})
}
