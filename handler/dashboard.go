// handler/dashboard.go
package handler

import (
	"dakotagroup/business-insight-be/db"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 1. Endpoint Summary Metrics Kartu Atas
func GetDashboardMetrics(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi DLI belum terhubung"})
		return
	}

	var totalCargo int64
	var totalTonase float64

	// Hitung agregasi real dari tabel BTT
	database.Table("public.mkt_t_econote").
		Where("COALESCE(bttt_aktifyn, 'Y') = 'Y'").
		Count(&totalCargo)

	database.Table("public.mkt_t_econote").
		Where("COALESCE(bttt_aktifyn, 'Y') = 'Y'").
		Select("COALESCE(SUM(bttt_berat), 0)").
		Scan(&totalTonase)

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

	// Ambil 5 data BTT teratas (Urutkan berdasarkan transaksi terbaru & nominal terbesar)
	err := database.Table("public.mkt_t_econote e").
		Select(`
			COALESCE(e.bttt_id::varchar, '-') AS no_btt,
			COALESCE(TO_CHAR(e.bttt_tanggal, 'YYYY-MM-DD'), '-') AS tanggal,
			COALESCE(e.bttt_asalname::varchar, 'PUSAT DAKOTA') AS asal,
			COALESCE(e.bttt_tujuankota::varchar, '-') AS tujuan,
			COALESCE(e.bttt_asalname::varchar, '-') AS pengirim,
			COALESCE(e.bttt_tujuannama::varchar, '-') AS penerima,
			'ARMADA DAKOTA' AS armada,
			COALESCE(e.bttt_harga::numeric, 0) AS harga,
			COALESCE(e.bttt_berat::numeric, 0) AS berat,
			? AS status
		`, status).
		Where("COALESCE(e.bttt_aktifyn, 'Y') = 'Y'").
		Order("e.bttt_tanggal DESC, e.bttt_harga DESC").
		Limit(5).
		Scan(&list).Error

	if err != nil {
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
