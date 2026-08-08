package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// InsentifLoperModel Struct Tampilan Header Insentif Loper
type InsentifLoperModel struct {
	LopInsID           string  `json:"lopins_id" gorm:"column:lopins_id"`
	LopInsTanggal      string  `json:"lopins_tanggal" gorm:"column:lopins_tanggal"`
	LopInsNIP          string  `json:"lopins_nip" gorm:"column:lopins_nip"`
	KryNama            string  `json:"kry_nama" gorm:"column:kry_nama"`
	LopInsStartPeriode string  `json:"lopins_start_periode" gorm:"column:lopins_startperiode"`
	LopInsEndPeriode   string  `json:"lopins_end_periode" gorm:"column:lopins_endperiode"`
	Komisi             float64 `json:"komisi" gorm:"column:komisi"`
	LopInsCBID         string  `json:"lopins_cbid" gorm:"column:lopins_cbid"`
}

// DriverOptionModel Struct Dropdown Driver
type DriverOptionModel struct {
	NIPSopir string `json:"nip_sopir" gorm:"column:loper_nipsopir"`
	KryNama  string `json:"kry_nama" gorm:"column:kry_nama"`
	Display  string `json:"display" gorm:"column:display"`
}

func getInsentifLoperDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/insentif-loper (READ LIST INSENTIF & FILTER)
// =========================================================================
func GetInsentifLoperHandler(c *gin.Context) {
	database := getInsentifLoperDB(c)

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	nipDriver := c.Query("nip_driver")
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

	query := database.Table("public.opr_t_eloper_insentif i").
		Select(`
			i.lopins_id, 
			TO_CHAR(i.lopins_tanggal, 'YYYY-MM-DD') AS lopins_tanggal, 
			i.lopins_nip, 
			COALESCE(k.kry_nama, '-') AS kry_nama, 
			TO_CHAR(i.lopins_startperiode, 'YYYY-MM-DD') AS lopins_startperiode, 
			TO_CHAR(i.lopins_endperiode, 'YYYY-MM-DD') AS lopins_endperiode, 
			COALESCE(i.lopins_cbid, '-') AS lopins_cbid, 
			COALESCE(SUM(d.lopinsd_komisi), 0) AS komisi
		`).
		Joins("LEFT JOIN public.opr_t_eloper_insentifdetail d ON TRIM(BOTH FROM CAST(i.lopins_id AS VARCHAR)) = TRIM(BOTH FROM CAST(d.lopinsd_lopinsid AS VARCHAR))").
		Joins("LEFT JOIN public.hrd_m_karyawan k ON TRIM(BOTH FROM CAST(i.lopins_nip AS VARCHAR)) = TRIM(BOTH FROM CAST(k.kry_nip AS VARCHAR))").
		Where("COALESCE(i.lopins_id, '') <> ''")

	if startDate != "" && endDate != "" {
		query = query.Where("i.lopins_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}

	if nipDriver != "" {
		query = query.Where("i.lopins_nip = ?", nipDriver)
	}

	query = query.Group("i.lopins_id, i.lopins_tanggal, i.lopins_nip, k.kry_nama, i.lopins_startperiode, i.lopins_endperiode, i.lopins_cbid")

	var totalRecords int64
	// Hitung total unique record
	database.Table("(?) AS count_tbl", query).Count(&totalRecords)

	var list []InsentifLoperModel
	err := query.Order("i.lopins_tanggal DESC, i.lopins_id DESC").Limit(limit).Offset(offset).Scan(&list).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          list,
		"total_records": totalRecords,
	})
}

// =========================================================================
// 2. GET /api/gl/insentif-loper/driver-options (DROPDOWN DRIVER)
// =========================================================================
// =========================================================================
// 2. GET /api/gl/insentif-loper/driver-options (AMBIL DARI MASTER KARYAWAN)
// =========================================================================
func GetDriverOptionsHandler(c *gin.Context) {
	database := getInsentifLoperDB(c)

	var drivers []DriverOptionModel

	// Tarik data driver dari hrd_m_karyawan
	err := database.Table("public.hrd_m_karyawan k").
		Select("k.kry_nip AS loper_nipsopir, k.kry_nama, CONCAT(k.kry_nama, ' | ', k.kry_nip) AS display").
		Where("COALESCE(k.kry_aktifyn, 'Y') = 'Y'").
		Order("k.kry_nama ASC").
		Scan(&drivers).Error

	if err != nil || len(drivers) == 0 {
		// Fallback: Jika hrd_m_karyawan belum diimport, ambil dari opr_t_eloper tanpa filter agenid
		database.Table("public.opr_t_eloper l").
			Select("l.loper_nipsopir, COALESCE(k.kry_nama, l.loper_nipsopir) AS kry_nama, CONCAT(COALESCE(k.kry_nama, l.loper_nipsopir), ' | ', l.loper_nipsopir) AS display").
			Joins("LEFT JOIN public.hrd_m_karyawan k ON TRIM(BOTH FROM CAST(l.loper_nipsopir AS VARCHAR)) = TRIM(BOTH FROM CAST(k.kry_nip AS VARCHAR))").
			Group("l.loper_nipsopir, k.kry_nama").
			Order("kry_nama ASC").
			Scan(&drivers)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   drivers,
	})
}

// =========================================================================
// 3. DELETE /api/gl/insentif-loper/:id (DELETE HEADER & DETAIL)
// =========================================================================
func DeleteInsentifLoperHandler(c *gin.Context) {
	lopinsID := c.Param("id")
	database := getInsentifLoperDB(c)

	if err := database.Table("public.opr_t_eloper_insentif").Where("lopins_id = ?", lopinsID).Delete(map[string]interface{}{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus insentif: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Data Insentif %s berhasil dihapus!", lopinsID),
	})
}
