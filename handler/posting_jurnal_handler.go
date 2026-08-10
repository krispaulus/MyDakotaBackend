package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ProcessPostingReq DTO Request Posting Pembukuan
type ProcessPostingReq struct {
	Bulan string `json:"bulan" binding:"required"` // Format '01' - '12'
	Tahun string `json:"tahun" binding:"required"` // Format 'YYYY'
}

// UnpostedDetailModel Struct untuk Preview List Unposted
type UnpostedDetailModel struct {
	NoTrans   string  `json:"no_trans" gorm:"column:no_trans"`
	Tanggal   string  `json:"tanggal" gorm:"column:tanggal"`
	Modul     string  `json:"modul" gorm:"column:modul"`
	Deskripsi string  `json:"deskripsi" gorm:"column:deskripsi"`
	Nominal   float64 `json:"nominal" gorm:"column:nominal"`
}

// YearOptionModel Struct Dropdown Tahun
type YearOptionModel struct {
	Tahun string `json:"tahun" gorm:"column:tahun"`
}

func getPostingJurnalDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/posting-jurnal/status (CEK STATUS & LIST UNPOSTED)
// =========================================================================
func GetPostingJurnalStatusHandler(c *gin.Context) {
	database := getPostingJurnalDB(c)

	bulan := c.Query("bulan")
	tahun := c.Query("tahun")

	if bulan == "" || tahun == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Bulan dan Tahun harus diisi!"})
		return
	}

	bInt, _ := strconv.Atoi(bulan)
	tInt, _ := strconv.Atoi(tahun)
	periodeStr := fmt.Sprintf("%04d-%02d", tInt, bInt)

	// 1. Hitung Unposted Jurnal Umum
	var unpostedJurnal int64
	database.Table("public.gl_t_jurnalh").
		Where("TO_CHAR(tjurh_tanggal, 'YYYY-MM') = ? AND COALESCE(tjurh_postingyn, 'N') = 'N' AND COALESCE(tjurh_deleteyn, 'N') = 'N'", periodeStr).
		Count(&unpostedJurnal)

	// 2. Hitung Unposted Kas / Bank
	var unpostedCashBank int64
	database.Table("public.gl_t_cashbank").
		Where("TO_CHAR(cb_tanggal, 'YYYY-MM') = ? AND COALESCE(cb_postyn, 'N') = 'N' AND COALESCE(cb_aktifyn, 'Y') = 'Y'", periodeStr).
		Count(&unpostedCashBank)

	// 3. Hitung Unposted Pembayaran Vendor
	var unpostedVendor int64
	database.Table("public.gl_t_pembayaranvendorh").
		Where("TO_CHAR(tpayh_tanggal, 'YYYY-MM') = ? AND COALESCE(tpayh_postingyn, 'N') = 'N' AND COALESCE(tpayh_deleteyn, 'N') = 'N'", periodeStr).
		Count(&unpostedVendor)

	// 4. Tarik Preview Rincian Unposted (Gunakan JOIN ke gl_t_jurnald agar tidak error column tjurh_total)
	var listUnposted []UnpostedDetailModel
	queryUnion := `
		SELECT 
			h.tjurh_no AS no_trans, 
			TO_CHAR(h.tjurh_tanggal, 'YYYY-MM-DD') AS tanggal, 
			'JURNAL UMUM' AS modul, 
			COALESCE(h.tjurh_keterangan, '-') AS deskripsi, 
			COALESCE(SUM(d.tjurd_debet), 0) AS nominal
		FROM public.gl_t_jurnalh h
		LEFT JOIN public.gl_t_jurnald d ON h.tjurh_no = d.tjurd_tjurhno
		WHERE TO_CHAR(h.tjurh_tanggal, 'YYYY-MM') = ? 
		  AND COALESCE(h.tjurh_postingyn, 'N') = 'N' 
		  AND COALESCE(h.tjurh_deleteyn, 'N') = 'N'
		GROUP BY h.tjurh_no, h.tjurh_tanggal, h.tjurh_keterangan
		
		UNION ALL
		
		SELECT 
			cb_id AS no_trans, 
			TO_CHAR(cb_tanggal, 'YYYY-MM-DD') AS tanggal, 
			'KAS / BANK' AS modul, 
			COALESCE(cb_ket, '-') AS deskripsi, 
			COALESCE(cb_total, 0) AS nominal
		FROM public.gl_t_cashbank
		WHERE TO_CHAR(cb_tanggal, 'YYYY-MM') = ? 
		  AND COALESCE(cb_postyn, 'N') = 'N' 
		  AND COALESCE(cb_aktifyn, 'Y') = 'Y'
		
		UNION ALL
		
		SELECT 
			tpayh_no AS no_trans, 
			TO_CHAR(tpayh_tanggal, 'YYYY-MM-DD') AS tanggal, 
			'PEMBAYARAN VENDOR' AS modul, 
			COALESCE(tpayh_keterangan, '-') AS deskripsi, 
			COALESCE(tpayh_total, 0) AS nominal
		FROM public.gl_t_pembayaranvendorh
		WHERE TO_CHAR(tpayh_tanggal, 'YYYY-MM') = ? 
		  AND COALESCE(tpayh_postingyn, 'N') = 'N' 
		  AND COALESCE(tpayh_deleteyn, 'N') = 'N'
		
		ORDER BY tanggal ASC
	`
	database.Raw(queryUnion, periodeStr, periodeStr, periodeStr).Scan(&listUnposted)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"periode": periodeStr,
		"summary": gin.H{
			"unposted_jurnal":   unpostedJurnal,
			"unposted_cashbank": unpostedCashBank,
			"unposted_vendor":   unpostedVendor,
			"total_unposted":    unpostedJurnal + unpostedCashBank + unpostedVendor,
		},
		"unposted_list": listUnposted,
	})
}

// =========================================================================
// 2. POST /api/gl/posting-jurnal/process (EKSEKUSI POSTING AKHIR BULAN)
// =========================================================================
func ProcessPostingJurnalHandler(c *gin.Context) {
	database := getPostingJurnalDB(c)
	userID, _ := c.Get("username")

	var req ProcessPostingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	bInt, _ := strconv.Atoi(req.Bulan)
	tInt, _ := strconv.Atoi(req.Tahun)
	periodeStr := fmt.Sprintf("%04d-%02d", tInt, bInt)

	tx := database.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	now := time.Now()

	// 1. Update Status Posting Jurnal Umum
	if err := tx.Table("public.gl_t_jurnalh").
		Where("TO_CHAR(tjurh_tanggal, 'YYYY-MM') = ? AND COALESCE(tjurh_deleteyn, 'N') = 'N'", periodeStr).
		Updates(map[string]interface{}{
			"tjurh_postingyn":  "Y",
			"tjurh_updateid":   fmt.Sprintf("%v", userID),
			"tjurh_updatetime": now,
		}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal posting Jurnal Umum: " + err.Error()})
		return
	}

	// 2. Update Status Posting Kas / Bank
	if err := tx.Table("public.gl_t_cashbank").
		Where("TO_CHAR(cb_tanggal, 'YYYY-MM') = ? AND COALESCE(cb_aktifyn, 'Y') = 'Y'", periodeStr).
		Updates(map[string]interface{}{
			"cb_postyn":     "Y",
			"cb_updateid":   fmt.Sprintf("%v", userID),
			"cb_updatetime": now,
		}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal posting Kas Bank: " + err.Error()})
		return
	}

	// 3. Update Status Posting Pembayaran Vendor
	if err := tx.Table("public.gl_t_pembayaranvendorh").
		Where("TO_CHAR(tpayh_tanggal, 'YYYY-MM') = ? AND COALESCE(tpayh_deleteyn, 'N') = 'N'", periodeStr).
		Updates(map[string]interface{}{
			"tpayh_postingyn":  "Y",
			"tpayh_updateid":   fmt.Sprintf("%v", userID),
			"tpayh_updatetime": now,
		}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal posting Pembayaran Vendor: " + err.Error()})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Proses Posting Pembukuan Akhir Bulan Periode %s BERHASIL Diselesaikan!", periodeStr),
	})
}

// =========================================================================
// 3. GET /api/gl/posting-jurnal/year-options (LIST DROPDOWN TAHUN DINAMIS)
// =========================================================================
func GetYearOptionsHandler(c *gin.Context) {
	database := getPostingJurnalDB(c)

	var years []YearOptionModel

	querySQL := `
		SELECT DISTINCT tahun FROM (
			SELECT TO_CHAR(tjurh_tanggal, 'YYYY') AS tahun FROM public.gl_t_jurnalh WHERE tjurh_tanggal IS NOT NULL
			UNION
			SELECT TO_CHAR(cb_tanggal, 'YYYY') AS tahun FROM public.gl_t_cashbank WHERE cb_tanggal IS NOT NULL
			UNION
			SELECT TO_CHAR(tbb_tjurhtanggal, 'YYYY') AS tahun FROM public.gl_t_bukubesar WHERE tbb_tjurhtanggal IS NOT NULL
		) AS combined_years
		WHERE tahun <> '' AND tahun IS NOT NULL
		ORDER BY tahun DESC
	`

	err := database.Raw(querySQL).Scan(&years).Error
	if err != nil || len(years) == 0 {
		currentYear := time.Now().Year()
		years = []YearOptionModel{}
		for y := currentYear; y >= currentYear-15; y-- {
			years = append(years, YearOptionModel{Tahun: fmt.Sprintf("%d", y)})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   years,
	})
}
