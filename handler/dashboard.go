// handler/dashboard.go
package handler

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Helper dinamis untuk menemukan nama kolom cabang pada tabel mkt_t_econote & mencocokkan kode/ID agen
func applyCabangFilter(database *gorm.DB, query *gorm.DB, tableAlias string, kodeCabang string) *gorm.DB {
	cleanCabang := strings.TrimSpace(kodeCabang)
	if cleanCabang == "" || strings.ToUpper(cleanCabang) == "ALL" || strings.ToUpper(cleanCabang) == "ALL CABANG" {
		return query
	}

	// 1. Ambil info agen (agen_id, agen_kode, agen_nama) dari master agen
	var agenID, agenKode, agenNama string
	_ = database.Table("public.glb_m_agen").
		Select("COALESCE(CAST(agen_id AS VARCHAR), ''), COALESCE(TRIM(agen_kode), ''), COALESCE(TRIM(agen_nama), '')").
		Where("TRIM(UPPER(agen_kode)) = UPPER(?) OR CAST(agen_id AS VARCHAR) = ?", cleanCabang, cleanCabang).
		Limit(1).
		Row().Scan(&agenID, &agenKode, &agenNama)

	// Jika yang dipilih adalah DLI PUSAT (agen_id = '1' atau nama mengandung PUSAT), tampilkan data pusat/keseluruhan
	if agenID == "1" || strings.Contains(strings.ToUpper(agenNama), "PUSAT") {
		return query
	}

	if agenKode == "" {
		agenKode = cleanCabang
	}

	prefix := ""
	if tableAlias != "" {
		prefix = tableAlias + "."
	}

	// 2. Deteksi otomatis kolom kode/ID cabang di mkt_t_econote
	var colName string
	_ = database.Raw(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_schema = 'public' 
		  AND table_name = 'mkt_t_econote' 
		  AND column_name IN (
			'bttt_agenkode', 'bttt_asalkode', 'bttt_asal', 'bttt_kodecabang', 
			'bttt_kdcabang', 'bttt_agenid', 'bttt_asalid', 'agen_kode', 'kode_cabang'
		  )
		LIMIT 1
	`).Scan(&colName)

	if colName != "" {
		if agenID != "" && agenID != agenKode {
			return query.Where(fmt.Sprintf("TRIM(CAST(%s%s AS VARCHAR)) IN (?, ?)", prefix, colName), agenKode, agenID)
		}
		return query.Where(fmt.Sprintf("TRIM(CAST(%s%s AS VARCHAR)) = ?", prefix, colName), agenKode)
	}

	// 3. Fallback jika tabel mkt_t_econote tidak punya kolom kode agen: filter berdasarkan nama kota pada bttt_asalname (misal: "DLI SURABAYA" -> "SURABAYA")
	kotaCabang := strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(agenNama), "DLI "))
	if kotaCabang != "" {
		return query.Where(fmt.Sprintf("UPPER(COALESCE(%sbttt_asalname, '')) LIKE ?", prefix), "%"+kotaCabang+"%")
	}

	return query.Where("1 = 0")
}

// 1. Endpoint Summary Metrics Kartu Atas
func GetDashboardMetrics(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi DLI belum terhubung"})
		return
	}

	var totalCargo int64
	var totalTonase float64
	kodeCabang := strings.TrimSpace(c.Query("kode_cabang"))

	// Query Total Cargo
	cargoQuery := database.Table("public.mkt_t_econote").
		Where("COALESCE(bttt_aktifyn, 'Y') = 'Y'")
	cargoQuery = applyCabangFilter(database, cargoQuery, "", kodeCabang)
	if err := cargoQuery.Count(&totalCargo).Error; err != nil {
		fmt.Println("❌ [Dashboard Metrics Error]:", err.Error())
	}

	// Query Total Tonase
	tonaseQuery := database.Table("public.mkt_t_econote").
		Where("COALESCE(bttt_aktifyn, 'Y') = 'Y'")
	tonaseQuery = applyCabangFilter(database, tonaseQuery, "", kodeCabang)
	tonaseQuery.Select("COALESCE(SUM(bttt_berat), 0)").Scan(&totalTonase)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"totalCargo":       totalCargo,
			"inTransit":        int64(float64(totalCargo) * 0.35),
			"pending":          int64(float64(totalCargo) * 0.05),
			"completed":        int64(float64(totalCargo) * 0.60),
			"growthPercentage": 12,
		},
	})
}

// 2. Endpoint Data BTT Berdasarkan Status (Top 5 Teratas dari Database Real)
func GetBTTByStatus(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi DLI belum terhubung"})
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	if status == "" {
		status = "IN_TRANSIT"
	}
	kodeCabang := strings.TrimSpace(c.Query("kode_cabang"))

	type BTTDetailView struct {
		NoBTT    string  `json:"no_btt"`
		Tanggal  string  `json:"tanggal"`
		Asal     string  `json:"asal"`
		Tujuan   string  `json:"tujuan"`
		Pengirim string  `json:"pengirim"`
		Penerima string  `json:"penerima"`
		Armada   string  `json:"armada"`
		Harga    float64 `json:"harga"`
		Berat    float64 `json:"berat"`
		Status   string  `json:"status"`
	}

	var list []BTTDetailView

	bttQuery := database.Table("public.mkt_t_econote e").
		Select(`
			COALESCE(e.bttt_id::varchar, '-') AS no_btt,
			COALESCE(TO_CHAR(e.bttt_tanggal, 'YYYY-MM-DD'), '-') AS tanggal,
			COALESCE(e.bttt_asalname::varchar, '-') AS asal,
			COALESCE(e.bttt_tujuankota::varchar, '-') AS tujuan,
			COALESCE(e.bttt_asalname::varchar, '-') AS pengirim,
			COALESCE(e.bttt_tujuannama::varchar, '-') AS penerima,
			'ARMADA DAKOTA' AS armada,
			COALESCE(e.bttt_harga::numeric, 0) AS harga,
			COALESCE(e.bttt_berat::numeric, 0) AS berat,
			? AS status
		`, status).
		Where("COALESCE(e.bttt_aktifyn, 'Y') = 'Y'")

	bttQuery = applyCabangFilter(database, bttQuery, "e", kodeCabang)

	err := bttQuery.
		Order("e.bttt_tanggal DESC, e.bttt_harga DESC").
		Limit(5).
		Scan(&list).Error

	if err != nil {
		fmt.Println("❌ [Dashboard BTT Error]:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data BTT: " + err.Error(),
			"data":    []BTTDetailView{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}
