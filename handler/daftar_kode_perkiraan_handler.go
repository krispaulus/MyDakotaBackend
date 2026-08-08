package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ChartAccountModel Struct Master Kode Perkiraan (public.gl_m_chartaccount)
type ChartAccountModel struct {
	CAID         string    `json:"ca_id" gorm:"column:ca_id;primaryKey"`
	CAName       string    `json:"ca_name" gorm:"column:ca_name"`
	CAUpID       string    `json:"ca_up_id" gorm:"column:ca_upid"`
	CAJenis      string    `json:"ca_jenis" gorm:"column:ca_jenis"`
	CAType       string    `json:"ca_type" gorm:"column:ca_type"`
	CAGolongan   string    `json:"ca_golongan" gorm:"column:ca_golongan"`
	CAKelompok   string    `json:"ca_kelompok" gorm:"column:ca_kelompok"`
	CAAktifYN    string    `json:"ca_aktif_yn" gorm:"column:ca_aktifyn"`
	CAUpdateID   string    `json:"ca_update_id" gorm:"column:ca_updateid"`
	CAUpdateTime time.Time `json:"ca_update_time" gorm:"column:ca_updatetime"`

	// Field Hasil Formatter
	GolonganJadi string `json:"golongan_jadi" gorm:"-"`
	AktifJadi    string `json:"aktif_jadi" gorm:"-"`
}

func getChartAccountDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/kode-perkiraan (READ LIST & FILTER)
// =========================================================================
func GetDaftarKodePerkiraanHandler(c *gin.Context) {
	database := getChartAccountDB(c)

	nama := c.Query("nama")
	status := c.Query("status")
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "500")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 500
	}
	offset := (page - 1) * limit

	query := database.Table("public.gl_m_chartaccount").
		Select(`
			ca_id, 
			ca_name, 
			COALESCE(ca_upid, '-') AS ca_upid, 
			COALESCE(ca_jenis, 'D') AS ca_jenis, 
			COALESCE(ca_type, 'D') AS ca_type, 
			COALESCE(ca_golongan, 'N') AS ca_golongan, 
			COALESCE(ca_kelompok, '-') AS ca_kelompok, 
			COALESCE(ca_aktifyn, 'Y') AS ca_aktifyn,
			ca_updatetime
		`)

	if nama != "" {
		query = query.Where("LOWER(ca_name) LIKE ?", "%"+strings.ToLower(nama)+"%")
	}
	if status != "" {
		query = query.Where("ca_aktifyn = ?", status)
	}

	var totalRecords int64
	query.Count(&totalRecords)

	var list []ChartAccountModel
	// Sorting: Data yang baru diupdate/di-add berada di paling atas
	err := query.Order("ca_updatetime DESC, ca_id ASC").Limit(limit).Offset(offset).Scan(&list).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Format text tampilan Golongan dan Aktif
	for i := range list {
		if strings.ToUpper(list[i].CAGolongan) == "N" {
			list[i].GolonganJadi = "NERACA"
		} else {
			list[i].GolonganJadi = "RUGI LABA"
		}

		if strings.ToUpper(list[i].CAAktifYN) == "Y" {
			list[i].AktifJadi = "Ya"
		} else {
			list[i].AktifJadi = "Tidak"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          list,
		"total_records": totalRecords,
	})
}

// =========================================================================
// 2. POST /api/gl/kode-perkiraan (CREATE)
// =========================================================================
func CreateKodePerkiraanHandler(c *gin.Context) {
	database := getChartAccountDB(c)
	userID, _ := c.Get("username")

	var req ChartAccountModel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	req.CAID = strings.ToUpper(strings.TrimSpace(req.CAID))
	req.CAName = strings.ToUpper(strings.TrimSpace(req.CAName))

	if req.CAID == "" || req.CAName == "" || req.CAKelompok == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Kode Akun, Nama Perkiraan, dan Kelompok wajib diisi!",
		})
		return
	}

	if req.CAJenis == "" {
		req.CAJenis = "D"
	}
	if req.CAType == "" {
		req.CAType = "D"
	}
	if req.CAGolongan == "" {
		req.CAGolongan = "N"
	}

	aktifYN := "Y"
	if req.CAAktifYN != "" {
		aktifYN = req.CAAktifYN
	}

	insertData := map[string]interface{}{
		"ca_id":         req.CAID,
		"ca_name":       req.CAName,
		"ca_upid":       req.CAUpID,
		"ca_jenis":      req.CAJenis,
		"ca_type":       req.CAType,
		"ca_golongan":   req.CAGolongan,
		"ca_kelompok":   req.CAKelompok,
		"ca_aktifyn":    aktifYN,
		"ca_updateid":   fmt.Sprintf("%v", userID),
		"ca_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_m_chartaccount").Create(insertData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan kode perkiraan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Kode Perkiraan berhasil ditambahkan!",
		"data":    req,
	})
}

// =========================================================================
// 3. PUT /api/gl/kode-perkiraan/:id (UPDATE)
// =========================================================================
func UpdateKodePerkiraanHandler(c *gin.Context) {
	caID := c.Param("id")
	database := getChartAccountDB(c)
	userID, _ := c.Get("username")

	var req ChartAccountModel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	req.CAName = strings.ToUpper(strings.TrimSpace(req.CAName))

	if req.CAName == "" || req.CAKelompok == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Nama Perkiraan dan Kelompok wajib diisi!",
		})
		return
	}

	updateData := map[string]interface{}{
		"ca_name":       req.CAName,
		"ca_upid":       req.CAUpID,
		"ca_jenis":      req.CAJenis,
		"ca_type":       req.CAType,
		"ca_golongan":   req.CAGolongan,
		"ca_kelompok":   req.CAKelompok,
		"ca_aktifyn":    req.CAAktifYN,
		"ca_updateid":   fmt.Sprintf("%v", userID),
		"ca_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_m_chartaccount").Where("ca_id = ?", caID).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update kode perkiraan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Kode Perkiraan berhasil diperbarui!",
	})
}

// =========================================================================
// 4. DELETE /api/gl/kode-perkiraan/:id (HARD DELETE)
// =========================================================================
func DeleteKodePerkiraanHandler(c *gin.Context) {
	caID := c.Param("id")
	database := getChartAccountDB(c)

	if err := database.Table("public.gl_m_chartaccount").Where("ca_id = ?", caID).Delete(&ChartAccountModel{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus kode perkiraan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Kode Perkiraan %s berhasil dihapus permanen!", caID),
	})
}
