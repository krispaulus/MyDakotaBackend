package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Helper internal pengambil DB dinamis
func getTarifHandlingDB(c *gin.Context) *gorm.DB {
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

// 1. GET LIST TARIF HANDLING PROPINSI
func GetTarifHandlingPropinsiList(c *gin.Context) {
	database := getTarifHandlingDB(c)

	propinsi := strings.TrimSpace(c.Query("propinsi"))

	query := database.Model(&models.OprMTarifHandlingByPropinsi{})

	if propinsi != "" {
		query = query.Where("UPPER(propinsi) LIKE UPPER(?)", "%"+propinsi+"%")
	}

	results := make([]models.OprMTarifHandlingByPropinsi, 0)
	err := query.Order("propinsi ASC").Find(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data tarif handling propinsi: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

// 2. CREATE / UPDATE TARIF HANDLING PROPINSI (SAVE & UPSERT)
func SaveTarifHandlingPropinsi(c *gin.Context) {
	database := getTarifHandlingDB(c)

	var input models.OprMTarifHandlingByPropinsi
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload input tidak valid: " + err.Error()})
		return
	}

	input.Propinsi = strings.ToUpper(strings.TrimSpace(input.Propinsi))
	if input.Propinsi == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nama Propinsi tidak boleh kosong"})
		return
	}

	// Normalisasi Flag Y/N
	if strings.ToUpper(input.ProsentaseYN) == "Y" {
		input.ProsentaseYN = "Y"
	} else {
		input.ProsentaseYN = "N"
	}

	if strings.ToUpper(input.TarifYN) == "Y" {
		input.TarifYN = "Y"
	} else {
		input.TarifYN = "N"
	}

	// Cek apakah data provinsi sudah ada (Upsert Process)
	var count int64
	database.Model(&models.OprMTarifHandlingByPropinsi{}).Where("propinsi = ?", input.Propinsi).Count(&count)

	if count > 0 {
		// UPDATE
		err := database.Model(&models.OprMTarifHandlingByPropinsi{}).
			Where("propinsi = ?", input.Propinsi).
			Updates(map[string]interface{}{
				"prosentaseyn":  input.ProsentaseYN,
				"prosentaseval": input.ProsentaseVal,
				"tarifyn":       input.TarifYN,
				"tarifval":      input.TarifVal,
			}).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui data tarif handling"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Tarif handling propinsi berhasil diperbarui"})
		return
	}

	// INSERT BARU
	if err := database.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menambah tarif handling propinsi baru"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Tarif handling propinsi baru berhasil disimpan",
		"data":    input,
	})
}

// 3. DELETE TARIF HANDLING PROPINSI
func DeleteTarifHandlingPropinsi(c *gin.Context) {
	database := getTarifHandlingDB(c)

	type DeleteReq struct {
		Propinsi string `json:"propinsi" binding:"required"`
	}

	var req DeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nama Propinsi wajib diisi"})
		return
	}

	cleanPropinsi := strings.ToUpper(strings.TrimSpace(req.Propinsi))

	err := database.Where("propinsi = ?", cleanPropinsi).Delete(&models.OprMTarifHandlingByPropinsi{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus tarif handling propinsi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Tarif handling propinsi %s berhasil dihapus", cleanPropinsi),
	})
}
