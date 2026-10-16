package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

type GpsHistoryRow struct {
	SjlSjhid      string  `json:"sjl_sjhid" gorm:"column:SJL_SJHID"`
	SjlUpdateTime string  `json:"sjl_updatetime" gorm:"column:SJL_UpdateTime"`
	SjlLat        float64 `json:"sjl_lat" gorm:"column:SJL_Lat"`
	SjlLong       float64 `json:"sjl_long" gorm:"column:SJL_Long"`
}

// GET /operasional/surattugas/gps-history?sjh_id=001JC2025020228
func GetSuratTugasGpsHistory(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	sjhID := strings.TrimSpace(c.Query("sjh_id"))
	if sjhID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Surat Tugas wajib diisi"})
		return
	}

	var list []GpsHistoryRow

	// Gunakan RTRIM/LTRIM dan nama tabel langsung tanpa public.
	query := `
		SELECT 
			SJL_SJHID, 
			CONVERT(VARCHAR(19), SJL_UpdateTime, 120) AS SJL_UpdateTime, 
			SJL_Lat, 
			SJL_Long 
		FROM OPR_T_eSuratJalanUpdateLokasi 
		WHERE RTRIM(LTRIM(SJL_SJHID)) = RTRIM(LTRIM(?)) 
		ORDER BY SJL_UpdateTime DESC
	`

	if err := database.Raw(query, sjhID).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}
