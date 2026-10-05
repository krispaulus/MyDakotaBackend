package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GET /api/setting/operasional
func GetSettingOperasionalHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var val string
	database.Table("public.glb_m_param").
		Select("COALESCE(set_varvalue, 'N')").
		Where("TRIM(LOWER(set_varname)) = 'allow_pusat_create_btt'").
		Limit(1).
		Scan(&val)

	allowPusat := strings.ToUpper(strings.TrimSpace(val)) == "Y"

	c.JSON(http.StatusOK, gin.H{
		"status":                 "success",
		"allow_pusat_create_btt": allowPusat,
	})
}

// POST /api/settings/operasional
func SaveSettingOperasionalHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		AllowPusatCreateBTT bool `json:"allow_pusat_create_btt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	valStr := "N"
	if req.AllowPusatCreateBTT {
		valStr = "Y"
	}

	// 1. Coba Update baris yang sudah ada terlebih dahulu
	res := database.Exec(`
		UPDATE public.glb_m_param 
		SET set_varvalue = ?, set_updatetime = NOW() 
		WHERE TRIM(LOWER(set_varname)) = 'allow_pusat_create_btt'
	`, valStr)

	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update setting: " + res.Error.Error()})
		return
	}

	// 2. Jika baris belum pernah ada (RowsAffected == 0), lakukan INSERT baru
	if res.RowsAffected == 0 {
		err := database.Exec(`
			INSERT INTO public.glb_m_param (set_varname, set_varvalue, set_updatetime) 
			VALUES ('allow_pusat_create_btt', ?, NOW())
		`, valStr).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal insert setting: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":                 "success",
		"message":                "Pengaturan izin input BTT Pusat Dakota berhasil diubah!",
		"allow_pusat_create_btt": req.AllowPusatCreateBTT,
	})
}
