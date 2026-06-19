// SearchKotaHandler untuk autocomplete pencarian kota di tabel public.glb_m_kota
package handler

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func SearchKotaHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "PT ID tidak ditemukan"})
		return
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database gagal"})
		return
	}

	// Ambil parameter pencarian dari query url (?search=...)
	search := c.Query("search")
	searchKeyword := "%" + strings.ToUpper(search) + "%"

	type KotaResult struct {
		KotaID   string `json:"kota_id"`
		KotaNama string `json:"kota_nama"`
	}

	var listKota []KotaResult

	// Query mencari berdasarkan kota_id (JKT) atau kota_nama (JAKARTA)
	err := database.Table("public.glb_m_kota").
		Select("kota_id, kota_nama").
		Where("kota_id LIKE ? OR kota_nama LIKE ?", searchKeyword, searchKeyword).
		Order("kota_nama ASC").
		Limit(10). // Cukup batasi 10 data biar response super gesit
		Find(&listKota).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query tabel glb_m_kota"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   listKota,
	})
}
