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

	// Parsing Tanggal
	parsedDate, err := time.Parse("2006-01-02", p.ValidDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format tanggal wajib YYYY-MM-DD!"})
		return
	}

	// Normalisasi String Kosong menjadi Pointer Nil (NULL di DB)
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

	// dim conn -> Kita buka transaksi database biar aman bray!
	tx := db.DB.Begin()

	// ==========================================
	// 🔁 JALUR 1: UPDATE MODE
	// ==========================================
	if p.EditMode == "True" {
		err := tx.Model(&MktMUncoveredArea{}).Where("generated_id = ?", p.GeneratedID).Updates(map[string]interface{}{
			"AsalKota":         asalKotaPtr,
			"servID":           p.ServID,
			"Tujuan_Propinsi":  propPtr,
			"Tujuan_Kabupaten": kabPtr,
			"Tujuan_Kecamatan": kecPtr,
			"BlockYN":          p.BlockYN,
			"Valid_Date":       parsedDate,
		}).Error

		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update data: " + err.Error()})
			return
		}
		tx.Commit()
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data berhasil diupdate!"})
		return
	}

	// ==========================================
	// 🆕 JALUR 2: INSERT MODE
	// ==========================================
	if p.ConfirmReplace != "YES" {
		// 🛡️ LAPIS 1: Cek Redundansi (Aturan yang Lebih Umum)
		var redundantID string
		checkSQL := `
			SELECT generated_id FROM public.mkt_m_uncoveredareas 
			WHERE servid = ? AND blockyn = 'Y'
			AND (? IS NULL OR asalkota IS NULL OR asalkota = ?)
			AND (? IS NULL OR tujuan_propinsi IS NULL OR tujuan_propinsi = ?)
			AND (? IS NULL OR tujuan_kabupaten IS NULL OR tujuan_kabupaten = ?)
			AND (? IS NULL OR tujuan_kecamatan IS NULL OR tujuan_kecamatan = ?)
			AND NOT (
				COALESCE(asalkota,'') = COALESCE(?,'') AND 
				COALESCE(tujuan_propinsi,'') = COALESCE(?,'') AND 
				COALESCE(tujuan_kabupaten,'') = COALESCE(?,'') AND 
				COALESCE(tujuan_kecamatan,'') = COALESCE(?,'')
			)
			LIMIT 1`

		tx.Raw(checkSQL,
			p.ServID,
			asalKotaPtr, asalKotaPtr,
			propPtr, propPtr,
			kabPtr, kabPtr,
			kecPtr, kecPtr,
			asalKotaPtr, propPtr, kabPtr, kecPtr,
		).Scan(&redundantID)

		if redundantID != "" {
			tx.Rollback()
			c.JSON(http.StatusConflict, gin.H{
				"status":  "redundant",
				"message": "DATA REDUNDAN! Sudah ada aturan yang lebih umum yang mencakup rute logistik ini bray.",
			})
			return
		}

		// 🛡️ LAPIS 2: Deteksi Konflik Dua Arah (Minta Konfirmasi User)
		var conflictCount int64
		var conflictType string

		if p.AsalKota != "" {
			// Kasus A: Aturan spesifik menimpa aturan AsalKota NULL
			tx.Model(&MktMUncoveredArea{}).
				Where(`"servID" = ? AND "AsalKota" IS NULL 
					AND ("Tujuan_Propinsi" = ? OR ("Tujuan_Propinsi" IS NULL AND ? IS NULL))
					AND ("Tujuan_Kabupaten" = ? OR ("Tujuan_Kabupaten" IS NULL AND ? IS NULL))
					AND ("Tujuan_Kecamatan" = ? OR ("Tujuan_Kecamatan" IS NULL AND ? IS NULL))`,
					p.ServID, propPtr, propPtr, kabPtr, kabPtr, kecPtr, kecPtr,
				).Count(&conflictCount)
			if conflictCount > 0 {
				conflictType = "null_to_specific"
			}
		} else {
			// Kasus B: Aturan umum (NULL) menimpa aturan-aturan AsalKota spesifik
			tx.Model(&MktMUncoveredArea{}).
				Where(`"servID" = ? AND "AsalKota" IS NOT NULL
					AND ("Tujuan_Propinsi" = ? OR ("Tujuan_Propinsi" IS NULL AND ? IS NULL))
					AND ("Tujuan_Kabupaten" = ? OR ("Tujuan_Kabupaten" IS NULL AND ? IS NULL))
					AND ("Tujuan_Kecamatan" = ? OR ("Tujuan_Kecamatan" IS NULL AND ? IS NULL))`,
					p.ServID, propPtr, propPtr, kabPtr, kabPtr, kecPtr, kecPtr,
				).Count(&conflictCount)
			if conflictCount > 0 {
				conflictType = "specific_to_null"
			}
		}

		if conflictCount > 0 {
			tx.Rollback()
			c.JSON(http.StatusAccepted, gin.H{
				"status":       "conflict_detected",
				"conflictType": conflictType,
				"message":      "Konflik hierarki aturan terdeteksi bray!",
			})
			return
		}
	}

	// 🔥 JIKA USER SUDAH KLIK OKE (confirmReplace = YES), BERSIHKAN KONFLIKNYA BRAY!
	if p.ConfirmReplace == "YES" {
		if p.AsalKota != "" {
			tx.Where(`"servID" = ? AND "AsalKota" IS NULL
				AND ("Tujuan_Propinsi" = ? OR ("Tujuan_Propinsi" IS NULL AND ? IS NULL))
				AND ("Tujuan_Kabupaten" = ? OR ("Tujuan_Kabupaten" IS NULL AND ? IS NULL))
				AND ("Tujuan_Kecamatan" = ? OR ("Tujuan_Kecamatan" IS NULL AND ? IS NULL))`,
				p.ServID, propPtr, propPtr, kabPtr, kabPtr, kecPtr, kecPtr,
			).Delete(&MktMUncoveredArea{})
		} else {
			tx.Where(`"servID" = ? AND "AsalKota" IS NOT NULL
				AND ("Tujuan_Propinsi" = ? OR ("Tujuan_Propinsi" IS NULL AND ? IS NULL))
				AND ("Tujuan_Kabupaten" = ? OR ("Tujuan_Kabupaten" IS NULL AND ? IS NULL))
				AND ("Tujuan_Kecamatan" = ? OR ("Tujuan_Kecamatan" IS NULL AND ? IS NULL))`,
				p.ServID, propPtr, propPtr, kabPtr, kabPtr, kecPtr, kecPtr,
			).Delete(&MktMUncoveredArea{})
		}
	}

	// 💾 EKSEKUSI PENYIMPANAN DATA BARU
	// Memanggil Stored Procedure sp_AddMKT_M_UncoveredAreas atau GORM Native Insert
	// Demi keamanan dan efisiensi PostgreSQL, kita bisa generate ID & Insert langsung bray
	newID := fmt.Sprintf("UNC-%d", time.Now().UnixNano()/1e6)

	newData := MktMUncoveredArea{
		GeneratedID:     newID,
		AsalKota:        asalKotaPtr,
		ServID:          p.ServID,
		TujuanPropinsi:  propPtr,
		TujuanKabupaten: kabPtr,
		TujuanKecamatan: kecPtr,
		BlockYN:         p.BlockYN,
		ValidDate:       parsedDate,
	}

	if err := tx.Create(&newData).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan data: " + err.Error()})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data berhasil disimpan bray!",
		"id":      newID,
	})
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
