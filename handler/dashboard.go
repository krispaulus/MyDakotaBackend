// handler/dashboard.go
package handler

import (
	"dakotagroup/business-insight-be/db"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 1. Endpoint Summary Metrics Kartu Atas
func GetDashboardMetrics(c *gin.Context) {
	// Koneksi ke database DLI
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi DLI belum terhubung"})
		return
	}

	// Response default / agregasi data real
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"totalCargo":       1240,
			"inTransit":        452,
			"pending":          12,
			"completed":        776,
			"growthPercentage": 12,
		},
	})
}

// 2. Endpoint Data BTT Berdasarkan Status (Drill-down Modal)
func GetBTTByStatus(c *gin.Context) {
	status := c.Query("status")

	type BTTDetailView struct {
		NoBTT    string `json:"no_btt"`
		Tanggal  string `json:"tanggal"`
		Asal     string `json:"asal"`
		Tujuan   string `json:"tujuan"`
		Pengirim string `json:"pengirim"`
		Penerima string `json:"penerima"`
		Armada   string `json:"armada"`
		Status   string `json:"status"`
	}

	// Data operasional sementara (nanti bisa diganti query SELECT real dari tabel BTT dli)
	list := []BTTDetailView{
		{
			NoBTT:    "BTT-260901-0012",
			Tanggal:  "2026-09-21",
			Asal:     "JAKARTA",
			Tujuan:   "SURABAYA",
			Pengirim: "PT INDO FOOD",
			Penerima: "CV BERKAH JAYA",
			Armada:   "B 9281 UXT (Budi Santoso)",
			Status:   status,
		},
		{
			NoBTT:    "BTT-260901-0045",
			Tanggal:  "2026-09-21",
			Asal:     "BEKASI",
			Tujuan:   "SEMARANG",
			Pengirim: "PT ASTRA HONDA",
			Penerima: "PT MAJU MOTOR",
			Armada:   "B 9112 KLO (Agus Prayitno)",
			Status:   status,
		},
		{
			NoBTT:    "BTT-260901-0078",
			Tanggal:  "2026-09-21",
			Asal:     "TANGERANG",
			Tujuan:   "MEDAN",
			Pengirim: "PT SAMUDRA ABADI",
			Penerima: "TOKO BINTANG",
			Armada:   "Kapal Laut Express",
			Status:   status,
		},
		{
			NoBTT:    "BTT-260901-0091",
			Tanggal:  "2026-09-21",
			Asal:     "BANDUNG",
			Tujuan:   "DENPASAR",
			Pengirim: "CV TEXTILE JAYA",
			Penerima: "BALI BOUTIQUE",
			Armada:   "D 8812 AB (Rian Hidayat)",
			Status:   status,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}
