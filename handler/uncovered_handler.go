package handler

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Request payload untuk input/update data
type UncoveredAreaPayload struct {
	EditMode        string `json:"editMode"`
	GeneratedID     string `json:"generatedID"`
	AsalKota        string `json:"asalKota"`
	ServID          int    `json:"servID" binding:"required"`
	TujuanPropinsi  string `json:"tujuanPropinsi"`
	TujuanKabupaten string `json:"tujuanKabupaten"`
	TujuanKecamatan string `json:"tujuanKecamatan"`
	BlockYN         string `json:"blockYN" binding:"required,oneof=Y N"`
	ValidDate       string `json:"validDate" binding:"required"`
	ConfirmReplace  string `json:"confirmReplace"`
}

// Struct untuk mapping tabel mkt_m_uncoveredareas dengan JSON tag terarah bray!
type MktMUncoveredArea struct {
	GeneratedID     string    `gorm:"column:generated_id;primaryKey" json:"generated_id"`
	AsalKota        *string   `gorm:"column:asalkota" json:"asal_kota"`
	ServID          int       `gorm:"column:servid" json:"serv_id"`
	TujuanPropinsi  *string   `gorm:"column:tujuan_propinsi" json:"tujuan_propinsi"`
	TujuanKabupaten *string   `gorm:"column:tujuan_kabupaten" json:"tujuan_kabupaten"`
	TujuanKecamatan *string   `gorm:"column:tujuan_kecamatan" json:"tujuan_kecamatan"`
	BlockYN         string    `gorm:"column:blockyn" json:"block_yn"`
	ValidDate       time.Time `gorm:"column:valid_date" json:"valid_date"`
}

func (MktMUncoveredArea) TableName() string {
	return "public.mkt_m_uncoveredareas"
}

// ProcessUncoveredArea menangani validasi berlapis & simpan data
func ProcessUncoveredArea(c *gin.Context) {
	var p UncoveredAreaPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid bray: " + err.Error()})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)) // 🟩 DIBUAT DINAMIS DETIK INI JUGA!
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database corporate gagal resolved"})
		return
	}

	parsedDate, _ := time.Parse("2006-01-02", p.ValidDate)
	var asalKotaPtr, propPtr, kabPtr, kecPtr *string
	if p.AsalKota != "" {
		asalKotaPtr = &p.AsalKota
	}
	if p.TujuanPropinsi != "" {
		propPtr = &p.TujuanPropinsi
	}
	if p.TujuanKabupaten != "" {
		kabPtr = &p.TujuanKabupaten
	}
	if p.TujuanKecamatan != "" {
		kecPtr = &p.TujuanKecamatan
	}

	tx := database.Begin() // 🟩 Gunakan database dinamis bray!

	if p.EditMode == "True" {
		err := tx.Table("public.mkt_m_uncoveredareas").Where("generated_id = ?", p.GeneratedID).Updates(map[string]interface{}{
			"asalkota":         asalKotaPtr,
			"servid":           p.ServID,
			"tujuan_propinsi":  propPtr,
			"tujuan_kabupaten": kabPtr,
			"tujuan_kecamatan": kecPtr,
			"blockyn":          p.BlockYN,
			"valid_date":       parsedDate,
		}).Error
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update data"})
			return
		}
		tx.Commit()
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data berhasil diupdate!"})
		return
	}

	// ... Sisa proses insert di bawahnya tinggal ganti semua "tx.Raw" / "tx.Create" mengarah ke "database" dinamis ini bray!
}

// GetUncoveredAreas menarik seluruh list aturan dari database
func GetUncoveredAreas(c *gin.Context) {
	var results []MktMUncoveredArea

	// Ambil semua data aturan dan urutkan berdasarkan ID terbaru
	err := db.DB.Order("generated_id DESC").Find(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengambil data dari database bray: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}
