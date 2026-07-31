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
func getTarifPaketDB(c *gin.Context) *gorm.DB {
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

// 1. GET LIST AGEN ASAL + JUMLAH RUTE TERDAFTAR (SUMMARY)
func GetTarifPaketSummaryList(c *gin.Context) {
	database := getTarifPaketDB(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset := (page - 1) * limit

	agenNama := strings.TrimSpace(c.Query("agen_nama"))
	activeAgen := strings.TrimSpace(c.Query("agen_id"))

	query := database.Table("public.glb_m_agen a").
		Select("a.agen_id, a.agen_nama, a.agen_alamat, a.agen_kota, a.agen_phone1, COUNT(e.area_agenid) as jml").
		Joins("LEFT JOIN public.opr_m_earea e ON a.agen_id = e.area_agenid").
		Where("a.agen_aktifyn = ?", "Y")

	if activeAgen != "" && activeAgen != "ALL" && !strings.Contains(strings.ToUpper(activeAgen), "PUSAT") {
		query = query.Where("a.agen_id = ? OR a.agen_id LIKE ?", activeAgen, activeAgen+"%")
	}

	if agenNama != "" {
		query = query.Where("UPPER(a.agen_nama) LIKE UPPER(?)", "%"+agenNama+"%")
	}

	query = query.Group("a.agen_id, a.agen_nama, a.agen_alamat, a.agen_kota, a.agen_phone1")

	var totalRecords int64
	database.Table("public.glb_m_agen a").Where("a.agen_aktifyn = ?", "Y").Count(&totalRecords)

	type AgenTarifSummary struct {
		AgenID     string `json:"agen_id"`
		AgenNama   string `json:"agen_nama"`
		AgenAlamat string `json:"agen_alamat"`
		AgenKota   string `json:"agen_kota"`
		AgenPhone1 string `json:"agen_phone1"`
		Jml        int64  `json:"jml"`
	}

	results := make([]AgenTarifSummary, 0)
	err := query.Order("a.agen_nama ASC").
		Offset(offset).
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data summary tarif paket: " + err.Error(),
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

// 2. GET DETAIL RUTE TARIF PAKET PER AGEN ASAL
func GetDetailTarifPaketByAgen(c *gin.Context) {
	database := getTarifPaketDB(c)
	agenID := strings.TrimSpace(c.Param("agen_id"))

	if agenID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Agen ID wajib diisi"})
		return
	}

	results := make([]models.OprMEarea, 0)
	err := database.Where("area_agenid = ?", agenID).
		Order("tujuan_propinsi ASC, tujuan_kabupaten ASC, tujuan_kecamatan ASC").
		Find(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengambil detail rute tarif paket"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

// 3. CREATE / UPDATE TARIF PAKET AREA (SAVE & UPSERT)
func SaveTarifPaketArea(c *gin.Context) {
	database := getTarifPaketDB(c)

	var input models.OprMEarea
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload input tidak valid: " + err.Error()})
		return
	}

	input.AreaAgenID = strings.ToUpper(strings.TrimSpace(input.AreaAgenID))
	input.TujuanPropinsi = strings.ToUpper(strings.TrimSpace(input.TujuanPropinsi))
	input.TujuanKabupaten = strings.ToUpper(strings.TrimSpace(input.TujuanKabupaten))
	input.TujuanKecamatan = strings.ToUpper(strings.TrimSpace(input.TujuanKecamatan))
	input.PickupAgenID = strings.ToUpper(strings.TrimSpace(input.PickupAgenID))

	if strings.ToUpper(input.PenerusYN) == "Y" {
		input.PenerusYN = "Y"
	} else {
		input.PenerusYN = "N"
	}

	if strings.ToUpper(input.ProsentaseByKirimYN) == "Y" {
		input.ProsentaseByKirimYN = "Y"
	} else {
		input.ProsentaseByKirimYN = "N"
	}

	// Jika ID > 0, jalankan UPDATE
	if input.ID > 0 {
		err := database.Model(&models.OprMEarea{}).Where("id = ?", input.ID).Updates(input).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate tarif paket area"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Tarif paket area berhasil diperbarui"})
		return
	}

	// Jika ID = 0, jalankan INSERT
	if err := database.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menambah rute tarif paket baru: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Rute tarif paket area baru berhasil disimpan",
		"data":    input,
	})
}

// 4. DELETE TARIF PAKET (SATUAN RUTE ATAU SELURUH RUTE PER AGEN)
func DeleteTarifPaketArea(c *gin.Context) {
	database := getTarifPaketDB(c)

	type DeleteReq struct {
		ID         uint   `json:"id"`
		AreaAgenID string `json:"area_agenid"`
	}

	var req DeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload input tidak valid"})
		return
	}

	// Delete Satuan Rute
	if req.ID > 0 {
		if err := database.Delete(&models.OprMEarea{}, req.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus rute tarif paket"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Rute tarif paket berhasil dihapus"})
		return
	}

	// Delete Seluruh Rute per Agen Asal
	if req.AreaAgenID != "" {
		cleanAgenID := strings.ToUpper(strings.TrimSpace(req.AreaAgenID))
		if err := database.Where("area_agenid = ?", cleanAgenID).Delete(&models.OprMEarea{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus seluruh rute tarif agen"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Seluruh rute tarif paket milik agen %s berhasil dihapus", cleanAgenID)})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID atau Area Agen ID wajib diisi"})
}
