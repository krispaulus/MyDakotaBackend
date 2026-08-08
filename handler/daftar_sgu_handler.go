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

// SGUModel Struct Master Header SGU (public.gl_t_sgu)
type SGUModel struct {
	SGUID           string    `json:"sgu_id" gorm:"column:sgu_id;primaryKey"`
	SGUNama         string    `json:"sgu_nama" gorm:"column:sgu_nama"`
	SGUKeterangan   string    `json:"sgu_keterangan" gorm:"column:sgu_keterangan"`
	SGUTanggal      string    `json:"sgu_tanggal" gorm:"column:sgu_tanggal"`
	SGUPeriodeStart string    `json:"sgu_periode_start" gorm:"column:sgu_periodestart"`
	SGUPeriodeEnd   string    `json:"sgu_periode_end" gorm:"column:sgu_periodeend"`
	SGUCAID         string    `json:"sgu_caid" gorm:"column:sgu_caid"`
	SGUDeleteYN     string    `json:"sgu_delete_yn" gorm:"column:sgu_deleteyn"`
	SGUUpdateID     string    `json:"sgu_update_id" gorm:"column:sgu_updateid"`
	SGUUpdateTime   time.Time `json:"sgu_update_time" gorm:"column:sgu_updatetime"`

	// Join / Computed Fields
	JmlAngsuran int64  `json:"jml_angsuran" gorm:"-"`
	AktifJadi   string `json:"aktif_jadi" gorm:"-"`
}

func getSGUDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/daftar-sgu (READ LIST & FILTER)
// =========================================================================
func GetDaftarSGUHandler(c *gin.Context) {
	database := getSGUDB(c)

	nama := c.Query("nama")
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

	query := database.Table("public.gl_t_sgu s").
		Select(`
			s.sgu_id, 
			s.sgu_nama, 
			COALESCE(s.sgu_keterangan, '-') AS sgu_keterangan, 
			TO_CHAR(s.sgu_tanggal, 'YYYY-MM-DD') AS sgu_tanggal, 
			TO_CHAR(s.sgu_periodestart, 'YYYY-MM-DD') AS sgu_periodestart, 
			TO_CHAR(s.sgu_periodeend, 'YYYY-MM-DD') AS sgu_periodeend, 
			COALESCE(s.sgu_caid, '-') AS sgu_caid, 
			COALESCE(s.sgu_deleteyn, 'N') AS sgu_deleteyn,
			s.sgu_updatetime,
			COUNT(d.sgud_sguid) AS jml_angsuran
		`).
		Joins("LEFT JOIN public.gl_t_sgud d ON TRIM(BOTH FROM CAST(s.sgu_id AS VARCHAR)) = TRIM(BOTH FROM CAST(d.sgud_sguid AS VARCHAR))").
		Group("s.sgu_id, s.sgu_nama, s.sgu_keterangan, s.sgu_tanggal, s.sgu_periodestart, s.sgu_periodeend, s.sgu_caid, s.sgu_deleteyn, s.sgu_updatetime")

	if nama != "" {
		query = query.Where("LOWER(s.sgu_nama) LIKE ?", "%"+strings.ToLower(nama)+"%")
	}

	var totalRecords int64
	query.Count(&totalRecords)

	type QueryResult struct {
		SGUID           string    `gorm:"column:sgu_id"`
		SGUNama         string    `gorm:"column:sgu_nama"`
		SGUKeterangan   string    `gorm:"column:sgu_keterangan"`
		SGUTanggal      string    `gorm:"column:sgu_tanggal"`
		SGUPeriodeStart string    `gorm:"column:sgu_periodestart"`
		SGUPeriodeEnd   string    `gorm:"column:sgu_periodeend"`
		SGUCAID         string    `gorm:"column:sgu_caid"`
		SGUDeleteYN     string    `gorm:"column:sgu_deleteyn"`
		SGUUpdateTime   time.Time `gorm:"column:sgu_updatetime"`
		JmlAngsuran     int64     `gorm:"column:jml_angsuran"`
	}

	var rawList []QueryResult
	// Sorting: Data yang baru diupdate/di-add berada di paling atas
	err := query.Order("s.sgu_updatetime DESC, s.sgu_id DESC").Limit(limit).Offset(offset).Scan(&rawList).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	var list []SGUModel
	for _, item := range rawList {
		aktifStr := "YA"
		if strings.ToUpper(item.SGUDeleteYN) == "Y" {
			aktifStr = "TIDAK"
		}

		list = append(list, SGUModel{
			SGUID:           item.SGUID,
			SGUNama:         item.SGUNama,
			SGUKeterangan:   item.SGUKeterangan,
			SGUTanggal:      item.SGUTanggal,
			SGUPeriodeStart: item.SGUPeriodeStart,
			SGUPeriodeEnd:   item.SGUPeriodeEnd,
			SGUCAID:         item.SGUCAID,
			SGUDeleteYN:     item.SGUDeleteYN,
			SGUUpdateTime:   item.SGUUpdateTime,
			JmlAngsuran:     item.JmlAngsuran,
			AktifJadi:       aktifStr,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          list,
		"total_records": totalRecords,
	})
}

// =========================================================================
// 2. POST /api/gl/daftar-sgu (CREATE)
// =========================================================================
func CreateSGUHandler(c *gin.Context) {
	database := getSGUDB(c)
	userID, _ := c.Get("username")

	var req SGUModel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	req.SGUID = strings.ToUpper(strings.TrimSpace(req.SGUID))
	req.SGUNama = strings.ToUpper(strings.TrimSpace(req.SGUNama))

	if req.SGUID == "" || req.SGUNama == "" || req.SGUTanggal == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Kode SGU, Nama Leasing, dan Tanggal Kontrak wajib diisi!",
		})
		return
	}

	insertData := map[string]interface{}{
		"sgu_id":           req.SGUID,
		"sgu_nama":         req.SGUNama,
		"sgu_keterangan":   req.SGUKeterangan,
		"sgu_tanggal":      req.SGUTanggal,
		"sgu_periodestart": req.SGUPeriodeStart,
		"sgu_periodeend":   req.SGUPeriodeEnd,
		"sgu_caid":         req.SGUCAID,
		"sgu_deleteyn":     "N", // Default 'N' (Aktif)
		"sgu_updateid":     fmt.Sprintf("%v", userID),
		"sgu_updatetime":   time.Now(),
	}

	if err := database.Table("public.gl_t_sgu").Create(insertData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan data SGU: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data SGU berhasil ditambahkan!",
		"data":    req,
	})
}

// =========================================================================
// 3. PUT /api/gl/daftar-sgu/:id (UPDATE)
// =========================================================================
func UpdateSGUHandler(c *gin.Context) {
	sguID := c.Param("id")
	database := getSGUDB(c)
	userID, _ := c.Get("username")

	var req SGUModel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	req.SGUNama = strings.ToUpper(strings.TrimSpace(req.SGUNama))

	if req.SGUNama == "" || req.SGUTanggal == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Nama Leasing dan Tanggal Kontrak wajib diisi!",
		})
		return
	}

	deleteYN := "N"
	if req.SGUDeleteYN == "Y" {
		deleteYN = "Y"
	}

	updateData := map[string]interface{}{
		"sgu_nama":         req.SGUNama,
		"sgu_keterangan":   req.SGUKeterangan,
		"sgu_tanggal":      req.SGUTanggal,
		"sgu_periodestart": req.SGUPeriodeStart,
		"sgu_periodeend":   req.SGUPeriodeEnd,
		"sgu_caid":         req.SGUCAID,
		"sgu_deleteyn":     deleteYN,
		"sgu_updateid":     fmt.Sprintf("%v", userID),
		"sgu_updatetime":   time.Now(),
	}

	if err := database.Table("public.gl_t_sgu").Where("sgu_id = ?", sguID).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update data SGU: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data SGU berhasil diperbarui!",
	})
}

// =========================================================================
// 4. DELETE /api/gl/daftar-sgu/:id (HARD DELETE)
// =========================================================================
func DeleteSGUHandler(c *gin.Context) {
	sguID := c.Param("id")
	database := getSGUDB(c)

	if err := database.Table("public.gl_t_sgu").Where("sgu_id = ?", sguID).Delete(&SGUModel{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus data SGU: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Data SGU %s berhasil dihapus permanen!", sguID),
	})
}
