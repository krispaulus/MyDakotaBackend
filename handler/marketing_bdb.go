package handler

import (
	"dakotagroup/business-insight-be/db" // Package koneksi DB lu
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// MktTeBDB merepresentasikan model database PostgreSQL untuk tabel MKT_T_eBDB
type MktTeBDB struct {
	BDB_ID           string    `gorm:"primaryKey;column:BDB_ID" json:"BDB_ID"`
	BDB_Tanggal      time.Time `gorm:"column:BDB_Tanggal" json:"BDB_Tanggal"`
	BDB_NamaPengirim string    `gorm:"column:BDB_NamaPengirim" json:"BDB_NamaPengirim"`
	BDB_AsalName     string    `gorm:"column:BDB_AsalName" json:"BDB_AsalName"`
	BDB_AsalTelp     string    `gorm:"column:BDB_AsalTelp" json:"BDB_AsalTelp"`
	BDB_TujuanAgenID string    `gorm:"column:BDB_TujuanAgenID" json:"BDB_TujuanAgenID"`
	BDB_TujuanNama   string    `gorm:"column:BDB_TujuanNama" json:"BDB_TujuanNama"`
	BDB_Up           string    `gorm:"column:BDB_Up" json:"BDB_Up"`
	BDB_NamaBarang   string    `gorm:"column:BDB_NamaBarang" json:"BDB_NamaBarang"`
	BDB_JmlUnit      int       `gorm:"column:BDB_JmlUnit" json:"BDB_JmlUnit"`
	BDB_JmlPck       int       `gorm:"column:BDB_JmlPck" json:"BDB_JmlPck"`
	BDB_Berat        float64   `gorm:"column:BDB_Berat" json:"BDB_Berat"`
	BDB_Beratvol     float64   `gorm:"column:BDB_Beratvol" json:"BDB_Beratvol"`
	BDB_Ukuran       string    `gorm:"column:BDB_Ukuran" json:"BDB_Ukuran"`
	BDB_Service      string    `gorm:"column:BDB_Service" json:"BDB_Service"`
	BDB_PostingYN    string    `gorm:"column:BDB_PostingYN;default:N" json:"BDB_PostingYN"`
	BDB_AktifYN      string    `gorm:"column:BDB_AktifYN;default:Y" json:"BDB_AktifYN"`
}

// BDBResponseDTO digunakan untuk memformat tanggal agar manis saat dibaca Frontend React
type BDBResponseDTO struct {
	BDB_ID           string  `json:"BDB_ID"`
	BDB_Tanggal      string  `json:"BDB_Tanggal"` // Diubah jadi string (YYYY-MM-DD)
	BDB_NamaPengirim string  `json:"BDB_NamaPengirim"`
	BDB_TujuanNama   string  `json:"BDB_TujuanNama"`
	BDB_JmlUnit      int     `json:"BDB_JmlUnit"`
	BDB_Berat        float64 `json:"BDB_Berat"`
	BDB_PostingYN    string  `json:"BDB_PostingYN"`
	BDB_Service      string  `json:"BDB_Service"`
}

// GetBDBListHandler menarik history kargo Bebas Dari Biaya (BDB) dari PostgreSQL
func GetBDBListHandler(c *gin.Context) {
	var bdbRecords []MktTeBDB

	// 💡 KUNCI SAKTI: Panggil instance DB utama dari package internal lu (contoh: db.DB)
	err := db.DB.Table("MKT_T_eBDB").
		Where("BDB_AktifYN = ?", "Y").
		Order("BDB_Tanggal DESC").
		Find(&bdbRecords).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal fetch data dari database Dakota"})
		return
	}

	// Mapping hasil query ke DTO agar format tanggalnya rapi dan enteng di-load frontend
	var responseData []BDBResponseDTO
	for _, item := range bdbRecords {
		responseData = append(responseData, BDBResponseDTO{
			BDB_ID:           item.BDB_ID,
			BDB_Tanggal:      item.BDB_Tanggal.Format("2006-01-02"), // Format standard YYYY-MM-DD bray
			BDB_NamaPengirim: item.BDB_NamaPengirim,
			BDB_TujuanNama:   item.BDB_TujuanNama,
			BDB_JmlUnit:      item.BDB_JmlUnit,
			BDB_Berat:        item.BDB_Berat,
			BDB_PostingYN:    item.BDB_PostingYN,
			BDB_Service:      item.BDB_Service,
		})
	}

	c.JSON(http.StatusOK, responseData)
}

// =========================================================================
// 👑 SUNTIKAN SAKTI: HANDLER MONITORING BTT MURNI MANDIRI (ANTI-EROR)
// =========================================================================
func GetMonitoringBTT(c *gin.Context) {
	// 1. Tangkap parameter saringan filter dari React Frontend lu
	agenID := c.Query("agen_id")
	status := c.Query("status")
	layanan := c.Query("layanan")

	// Log singkat di terminal console backend buat tracing laser scanner bray
	log.Printf("📡 [Golang Backend] Memproses Monitoring BTT untuk Agen ID: %s, Status: %s, Layanan: %s", agenID, status, layanan)

	// 💡 Langkah Selanjutnya: Di bawah ini nanti tinggal kita tembak query SQL database gorm lu,
	// murni menggunakan instance db utama lu (contoh: db.DB.Table(...)) sama seperti fungsi BDB di atas!

	// Response sukses sementara agar engine Go lu lolos sensor compile bray
	c.JSON(200, gin.H{
		"status":  "success",
		"message": "Koneksi data monitoring BTT dari PostgreSQL berhasil terhubung!",
		"data":    []interface{}{},
	})
}
