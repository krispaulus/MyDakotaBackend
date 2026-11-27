package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type HariLiburRow struct {
	ID         int    `gorm:"column:id" json:"id"`
	Tanggal    string `gorm:"column:tanggal" json:"tanggal"`
	HariNama   string `gorm:"column:hari_nama" json:"hari_nama"`
	Keterangan string `gorm:"column:keterangan" json:"keterangan"`
	CreatedBy  string `gorm:"column:created_by" json:"created_by"`
}

// 1. GET LIST HARI LIBUR (Filter Tahun & Bulan)
func GetListHariLibur(c *gin.Context) {
	database := getDynamicDB(c)

	// Pastikan tabel & index sudah tersedia
	database.Exec(`
		CREATE TABLE IF NOT EXISTS public.opr_m_harilibur (
			id SERIAL PRIMARY KEY,
			tanggal DATE UNIQUE NOT NULL,
			keterangan VARCHAR(150) NOT NULL,
			created_by VARCHAR(50),
			created_at TIMESTAMP DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_harilibur_tanggal ON public.opr_m_harilibur (tanggal);
	`)

	tahun := strings.TrimSpace(c.Query("tahun"))
	bulan := strings.TrimSpace(c.Query("bulan"))

	sqlQuery := `
		SELECT 
			id,
			TO_CHAR(tanggal, 'YYYY-MM-DD') AS tanggal,
			CASE EXTRACT(ISODOW FROM tanggal)
				WHEN 1 THEN 'SENIN'
				WHEN 2 THEN 'SELASA'
				WHEN 3 THEN 'RABU'
				WHEN 4 THEN 'KAMIS'
				WHEN 5 THEN 'JUMAT'
				WHEN 6 THEN 'SABTU'
				WHEN 7 THEN 'MINGGU'
			END AS hari_nama,
			COALESCE(keterangan, '') AS keterangan,
			COALESCE(created_by, 'SYSTEM') AS created_by
		FROM public.opr_m_harilibur
		WHERE 1=1
	`
	var args []interface{}

	if tahun != "" {
		if yr, err := strconv.Atoi(tahun); err == nil {
			sqlQuery += " AND EXTRACT(YEAR FROM tanggal) = ?"
			args = append(args, yr)
		}
	}

	if bulan != "" {
		if mo, err := strconv.Atoi(bulan); err == nil && mo >= 1 && mo <= 12 {
			sqlQuery += " AND EXTRACT(MONTH FROM tanggal) = ?"
			args = append(args, mo)
		}
	}

	sqlQuery += " ORDER BY tanggal ASC"

	var results []HariLiburRow
	if err := database.Raw(sqlQuery, args...).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memuat data hari libur: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

// 2. CREATE HARI LIBUR (Mendukung 1 Tanggal atau Rentang Tanggal / Cuti Bersama)
func CreateHariLibur(c *gin.Context) {
	database := getDynamicDB(c)

	userID, _ := c.Get("username")
	createdBy := "SYSTEM"
	if userID != nil && fmt.Sprintf("%v", userID) != "" {
		createdBy = strings.ToUpper(fmt.Sprintf("%v", userID))
	}

	var input struct {
		TanggalAwal  string `json:"tanggal_awal"`
		TanggalAkhir string `json:"tanggal_akhir"`
		Keterangan   string `json:"keterangan"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	tglAwalStr := strings.TrimSpace(input.TanggalAwal)
	tglAkhirStr := strings.TrimSpace(input.TanggalAkhir)
	if tglAkhirStr == "" {
		tglAkhirStr = tglAwalStr
	}
	ket := strings.ToUpper(strings.TrimSpace(input.Keterangan))

	if tglAwalStr == "" || ket == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Tanggal dan Keterangan wajib diisi!"})
		return
	}

	startDate, err := time.Parse("2006-01-02", tglAwalStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format tanggal awal tidak valid"})
		return
	}

	endDate, err := time.Parse("2006-01-02", tglAkhirStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format tanggal akhir tidak valid"})
		return
	}

	if endDate.Before(startDate) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Tanggal akhir tidak boleh lebih kecil dari tanggal awal"})
		return
	}

	// Batasi maksimal 31 hari sekali input untuk keamanan
	if endDate.Sub(startDate).Hours()/24 > 31 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Maksimal rentang input adalah 31 hari sekaligus"})
		return
	}

	insertedCount := 0
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		tglSQL := d.Format("2006-01-02")
		err := database.Exec(`
			INSERT INTO public.opr_m_harilibur (tanggal, keterangan, created_by)
			VALUES (?::date, ?, ?)
			ON CONFLICT (tanggal) DO UPDATE 
			SET keterangan = EXCLUDED.keterangan, created_by = EXCLUDED.created_by
		`, tglSQL, ket, createdBy).Error

		if err == nil {
			insertedCount++
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Berhasil menyimpan %d hari libur!", insertedCount),
	})
}

// 3. UPDATE HARI LIBUR
func UpdateHariLibur(c *gin.Context) {
	database := getDynamicDB(c)

	var input struct {
		ID         int    `json:"id"`
		Tanggal    string `json:"tanggal"`
		Keterangan string `json:"keterangan"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	tgl := strings.TrimSpace(input.Tanggal)
	ket := strings.ToUpper(strings.TrimSpace(input.Keterangan))

	if input.ID <= 0 || tgl == "" || ket == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID, Tanggal, dan Keterangan wajib diisi!"})
		return
	}

	err := database.Exec(`
		UPDATE public.opr_m_harilibur
		SET tanggal = ?::date, keterangan = ?
		WHERE id = ?
	`, tgl, ket, input.ID).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate hari libur (pastikan tanggal tidak duplikat): " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data hari libur berhasil diperbarui!",
	})
}

// 4. DELETE HARI LIBUR
func DeleteHariLibur(c *gin.Context) {
	database := getDynamicDB(c)

	idStr := strings.TrimSpace(c.Param("id"))
	if idStr == "" {
		idStr = strings.TrimSpace(c.Query("id"))
	}

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID hari libur tidak valid"})
		return
	}

	if err := database.Exec(`DELETE FROM public.opr_m_harilibur WHERE id = ?`, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus hari libur: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data hari libur berhasil dihapus!",
	})
}
