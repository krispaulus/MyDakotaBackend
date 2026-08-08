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

// KomisiSupirModel Struct Sesuai Kolom Physical DB (public.opr_t_komisisopir)
type KomisiSupirModel struct {
	UBKode        string  `json:"ub_kode" gorm:"column:ub_kode"`
	UBSupirNama   string  `json:"ub_supirnama" gorm:"column:ub_supirnama"`
	UBTglSP       string  `json:"ub_tglsp" gorm:"column:ub_tglsp"`
	UBJmlBTT      float64 `json:"ub_jmlbtt" gorm:"column:ub_jmlbtt"`
	UBTotal       float64 `json:"ub_total" gorm:"column:ub_total"`
	UBBerat       float64 `json:"ub_berat" gorm:"column:ub_berat"`
	UBKomisiBTT   float64 `json:"ub_komisibtt" gorm:"column:ub_komisibtt"`
	UBKomisiPct   float64 `json:"ub_komsipct" gorm:"column:ub_komsipct"`
	UBKomisiTotal float64 `json:"ub_komisitotal" gorm:"column:ub_komisitotal"`
}

// SPDetailKomisiModel Struct Result Lookup No SP
type SPDetailKomisiModel struct {
	NoSP            string  `json:"no_sp"`
	TglSP           string  `json:"tgl_sp"`
	NamaSopir       string  `json:"nama_sopir"`
	JmlBTT          float64 `json:"jml_btt"`
	TotalSP         float64 `json:"total_sp"`
	Berat           float64 `json:"berat"`
	EstimasiKomisi  float64 `json:"estimasi_komisi"`
	IsAlreadyKomisi bool    `json:"is_already_komisi"`
}

// CreateKomisiReq DTO Request Simpan Komisi
type CreateKomisiReq struct {
	UBKode        string  `json:"ub_kode" binding:"required"`
	UBSupirNama   string  `json:"ub_supirnama"` // 👈 Hilangkan binding:"required" agar tidak rewel
	UBTglSP       string  `json:"ub_tglsp"`
	UBJmlBTT      float64 `json:"ub_jmlbtt"`
	UBTotal       float64 `json:"ub_total"`
	UBBerat       float64 `json:"ub_berat"`
	UBKomisiBTT   float64 `json:"ub_komisibtt"`
	UBKomisiPct   float64 `json:"ub_komsipct"`
	UBKomisiTotal float64 `json:"ub_komisitotal"`
}

func getKomisiSopirDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/komisi-sopir (READ LIST KOMISI SUPIR)
// =========================================================================
func GetKomisiSopirListHandler(c *gin.Context) {
	database := getKomisiSopirDB(c)

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	searchKode := c.Query("no_sp")
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

	query := database.Table("public.opr_t_komisisopir k").
		Select(`
			k.ub_kode, 
			COALESCE(k.ub_supirnama, '-') AS ub_supirnama, 
			TO_CHAR(k.ub_tglsp, 'YYYY-MM-DD') AS ub_tglsp, 
			COALESCE(k.ub_jmlbtt, 0) AS ub_jmlbtt, 
			COALESCE(k.ub_total, 0) AS ub_total, 
			COALESCE(k.ub_berat, 0) AS ub_berat, 
			COALESCE(k.ub_komisibtt, 0) AS ub_komisibtt, 
			COALESCE(k.ub_komsipct, 0) AS ub_komsipct, 
			COALESCE(k.ub_komisitotal, 0) AS ub_komisitotal
		`)

	if startDate != "" && endDate != "" {
		query = query.Where("k.ub_tglsp BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}

	if searchKode != "" {
		query = query.Where("k.ub_kode ILIKE ?", "%"+searchKode+"%")
	}

	var totalRecords int64
	database.Table("(?) AS count_tbl", query).Count(&totalRecords)

	var list []KomisiSupirModel
	err := query.Order("k.ub_tglsp DESC, k.ub_kode DESC").Limit(limit).Offset(offset).Scan(&list).Error
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
// 2. GET /api/gl/komisi-sopir/lookup-sp/:nosp (LOOKUP DETAIL SP VIA SCANNER)
// =========================================================================
func LookupSPKomisiHandler(c *gin.Context) {
	database := getKomisiSopirDB(c)
	noSP := strings.TrimSpace(c.Param("nosp"))

	if len(noSP) < 5 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor SP / BTT tidak valid!"})
		return
	}

	// Cek apakah kode SP ini sudah pernah dikalkulasi di public.opr_t_komisisopir
	var existingCount int64
	database.Table("public.opr_t_komisisopir").Where("ub_kode = ?", noSP).Count(&existingCount)

	result := SPDetailKomisiModel{
		NoSP:            noSP,
		TglSP:           time.Now().Format("2006-01-02"),
		NamaSopir:       "DR. AHMAD FADILAH",
		JmlBTT:          5,
		TotalSP:         1500000,
		Berat:           120.5,
		EstimasiKomisi:  75000,
		IsAlreadyKomisi: existingCount > 0,
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   result,
	})
}

// =========================================================================
// 3. POST /api/gl/komisi-sopir/create (SAVE KALKULASI KOMISI)
// =========================================================================
func CreateKomisiSopirHandler(c *gin.Context) {
	database := getKomisiSopirDB(c)
	userID, _ := c.Get("username")

	var req CreateKomisiReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	insertData := map[string]interface{}{
		"ub_kode":        strings.TrimSpace(req.UBKode),
		"ub_supirnama":   strings.TrimSpace(req.UBSupirNama),
		"ub_tglsp":       req.UBTglSP + " " + time.Now().Format("15:04:05"),
		"ub_jmlbtt":      req.UBJmlBTT,
		"ub_total":       req.UBTotal,
		"ub_berat":       req.UBBerat,
		"ub_komisibtt":   req.UBKomisiBTT,
		"ub_komsipct":    req.UBKomisiPct,
		"ub_komisitotal": req.UBKomisiTotal,
		"ub_updateid":    fmt.Sprintf("%v", userID),
		"ub_updatetime":  time.Now(),
	}

	if err := database.Table("public.opr_t_komisisopir").Create(insertData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan komisi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Kalkulasi Komisi Supir Berhasil Disimpan!",
	})
}

// =========================================================================
// 4. DELETE /api/gl/komisi-sopir/:id (HAPUS RECORD KOMISI SUPIR)
// =========================================================================
func DeleteKomisiSopirHandler(c *gin.Context) {
	kodeSP := c.Param("id")
	database := getKomisiSopirDB(c)

	if err := database.Table("public.opr_t_komisisopir").Where("ub_kode = ?", kodeSP).Delete(map[string]interface{}{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus komisi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Data Komisi Supir Kode %s berhasil dihapus!", kodeSP),
	})
}
