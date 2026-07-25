package handler

import (
	"net/http"
	"strconv"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 1. GET LIST LOKASI TERAKHIR KARYAWAN (Filter Tanggal, NIP, & Nama)
func GetMonitoringLokasiList(c *gin.Context) {
	database := db.DB // atau db.DB / db.DBDLI sesuai koneksi DB HRD kamu

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "15"))
	offset := (page - 1) * limit

	tglStart := strings.TrimSpace(c.Query("tgl_start"))
	tglEnd := strings.TrimSpace(c.Query("tgl_end"))
	nip := strings.TrimSpace(c.Query("nip"))
	nama := strings.TrimSpace(c.Query("nama"))

	// Subquery untuk mengambil waktu update paling baru (MAX Time) per NIP
	subQuery := database.Table("hrd_t_karyawanabsenupdatelokasi").
		Select("kaul_nip, MAX(kaul_updatetime) as max_time").
		Group("kaul_nip")

	if tglStart != "" && tglEnd != "" {
		subQuery = subQuery.Where("kaul_updatetime BETWEEN ? AND ?", tglStart+" 00:00:00", tglEnd+" 23:59:59")
	}

	// Query Utama JOIN ke Master Karyawan
	query := database.Table("hrd_t_karyawanabsenupdatelokasi kaul").
		Select("COALESCE(k.kry_nama, '') as kry_nama, k.kry_nip, kaul.kaul_updatetime, kaul.kaul_lat, kaul.kaul_long").
		Joins("LEFT JOIN hrd_m_karyawan k ON kaul.kaul_nip = k.kry_nip").
		Joins("INNER JOIN (?) latest ON kaul.kaul_nip = latest.kaul_nip AND kaul.kaul_updatetime = latest.max_time", subQuery).
		Where("k.kry_nip IS NOT NULL")

	if nip != "" {
		query = query.Where("kaul.kaul_nip = ?", nip)
	}
	if nama != "" {
		query = query.Where("UPPER(k.kry_nama) LIKE UPPER(?)", "%"+nama+"%")
	}

	var totalRecords int64
	query.Count(&totalRecords)

	var result []models.KaryawanLokasiResponse
	err := query.Order("k.kry_nip ASC").Offset(offset).Limit(limit).Scan(&result).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data monitoring lokasi: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          result,
		"total_records": totalRecords,
		"page":          page,
		"limit":         limit,
	})
}

// 2. GET HISTORY GPS ROUTE UNTUK POP-UP PETA TRACKING
func GetHistoryGPSKaryawan(c *gin.Context) {
	database := db.DB

	nip := strings.TrimSpace(c.Query("nip"))
	tglStart := strings.TrimSpace(c.Query("tgl_start"))
	tglEnd := strings.TrimSpace(c.Query("tgl_end"))

	if nip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "NIP Karyawan wajib diisi"})
		return
	}

	query := database.Table("hrd_t_karyawanabsenupdatelokasi kaul").
		Select("kaul.kaul_nip, COALESCE(k.kry_nama, '') as kry_nama, kaul.kaul_updatetime, kaul.kaul_lat, kaul.kaul_long").
		Joins("LEFT JOIN hrd_m_karyawan k ON kaul.kaul_nip = k.kry_nip").
		Where("kaul.kaul_nip = ?", nip)

	if tglStart != "" && tglEnd != "" {
		query = query.Where("kaul.kaul_updatetime BETWEEN ? AND ?", tglStart+" 00:00:00", tglEnd+" 23:59:59")
	}

	var history []models.KaryawanLokasiResponse
	err := query.Order("kaul.kaul_updatetime ASC").Find(&history).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil history GPS karyawan: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   history,
	})
}

// 3. POST RECEIVER LOKASI DARIKARYAWAN / MOBILE APP
func PostUpdateLokasiKaryawan(c *gin.Context) {
	database := db.DB
	var req models.UpdateLokasiRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload tidak valid"})
		return
	}

	newLog := models.HrdTKaryawanAbsenUpdateLokasi{
		KaulNip:  req.Nip,
		KaulLat:  req.Lat,
		KaulLong: req.Lng,
	}

	if err := database.Create(&newLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan koordinat lokasi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Lokasi berhasil diperbarui"})
}
