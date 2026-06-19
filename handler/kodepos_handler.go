package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetKodePos(c *gin.Context) {
	var data []models.KodePos
	ptID, _ := c.Get("pt_id")
	search := c.Query("search")

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB tidak ditemukan"})
		return
	}

	query := database.Model(&models.KodePos{})

	if search != "" {
		query = query.Where("kodepos ILIKE ? OR desakelurahan ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Data kodepos Indonesia itu 80ribu lebih, wajib LIMIT!
	if err := query.Limit(100).Order("kodepos asc").Find(&data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}
