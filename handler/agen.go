package handler

import (
	"log"
	"net/http"

	"dakotagroup/business-insight-be/db" // sesuaikan dengan path project kamu
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

func GetAgens(c *gin.Context) {
	var agens []models.Agen

	// Coba pisahkan penulisan NOT LIKE atau pastikan urutannya benar
	// Query menggunakan GORM
	err := db.DB.Where("agen_aktifyn = ?", "Y").
		Where("agen_nama NOT LIKE ?", "%XXX%"). // % di depan dan belakang biar lebih aman filter-nya
		Order("agen_nama ASC").
		Find(&agens).Error

	if err != nil {
		// Ini akan membantu kita melihat detail error aslinya di log terminal
		log.Printf("❌ ERROR SQL AGENS: %v", err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data agen",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   agens,
	})
}
