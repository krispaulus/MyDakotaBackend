package handler

import (
	"dakotagroup/business-insight-be/db"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Struct response master device karyawan
type EmployeeDeviceRes struct {
	KryNIP        string  `gorm:"column:kry_nip;primaryKey" json:"kry_nip"`
	KryNama       string  `gorm:"column:kry_nama" json:"kry_nama"`
	KryImei1      *string `gorm:"column:kry_imei1" json:"kry_imei1"`
	KryImei2      *string `gorm:"column:kry_imei2" json:"kry_imei2"`
	KrySimcardID1 *string `gorm:"column:kry_simcardid1" json:"kry_simcard_id1"`
	KrySimcardID2 *string `gorm:"column:kry_simcardid2" json:"kry_simcard_id2"`
}

// 📱 1. GET LIST DEVICE KARYAWAN (Mengadopsi index.asp)
func GetDeviceKaryawanList(c *gin.Context) {
	var results []EmployeeDeviceRes

	namaFilter := c.Query("nama")
	typeKry := c.Query("typekry") // H = Harian, KT = Kontrak/Tetap

	// Base query aman mengarah ke skema public tabel karyawan aktif
	query := db.DB.Table("public.hrd_m_karyawan").
		Select("kry_nip, kry_nama, kry_imei1, kry_imei2, kry_simcardid1, kry_simcardid2").
		Where("kry_aktifyn = 'Y'")

	// Filter Nama Karyawan/Sopir
	if namaFilter != "" {
		query = query.Where("kry_nama ILIKE ?", "%"+namaFilter+"%")
	}

	// Filter Jenis Karyawan SOP Dakota Cargo Jul
	if typeKry == "H" {
		query = query.Where("kry_nip ILIKE '%H%'")
	} else if typeKry == "KT" {
		query = query.Where("SUBSTRING(kry_nip, 4, 1) <> 'H'")
	}

	err := query.Order("kry_nip ASC, kry_nama ASC").Find(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal muat device karyawan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 💾 2. UPDATE ATRIBUT DEVICE MANUAL (Mengadopsi p-hrd_m_kry_device_e.asp)
func UpdateDeviceKaryawan(c *gin.Context) {
	nip := c.Param("nip")
	var input struct {
		Imei1      string `json:"kry_imei1" binding:"required"`
		Imei2      string `json:"kry_imei2"`
		SimcardID1 string `json:"kry_simcard_id1" binding:"required"`
		SimcardID2 string `json:"kry_simcard_id2"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload tidak valid bray"})
		return
	}

	username, _ := c.Get("username") // Identitas updater dari JWT Token

	err := db.DB.Table("public.hrd_m_karyawan").Where("kry_nip = ?", nip).Updates(map[string]interface{}{
		"kry_imei1":      strings.ToUpper(input.Imei1),
		"kry_imei2":      strings.ToUpper(input.Imei2),
		"kry_simcardid1": strings.ToUpper(input.SimcardID1),
		"kry_simcardid2": strings.ToUpper(input.SimcardID2),
		"kry_updateid":   username,
		"kry_updatetime": time.Now(),
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui device bray"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data device karyawan berhasil diperbarui!"})
}

// 🚀 3. PARSING & IMPORT AUTO LINK WHATSAPP (Mengadopsi p-hrd_m_kry_device_a.asp)
func ImportDeviceViaWhatsApp(c *gin.Context) {
	var input struct {
		LinkRaw string `json:"link_raw"` // Mendukung plain text maupun raw Base64 bray
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak boleh kosong"})
		return
	}

	linkTarget := strings.TrimSpace(input.LinkRaw)
	if linkTarget == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Link kosong bray"})
		return
	}

	// Deteksi jika link dikirim dalam bentuk enkripsi Base64 (Re-engineering ASP lama lu!)
	if !strings.Contains(linkTarget, ",") && len(linkTarget) > 15 {
		// Mengembalikan struktur kompresi manipulasi string ASP lama lu bray
		decodedByte, err := base64.StdEncoding.DecodeString(linkTarget)
		if err == nil {
			linkTarget = string(decodedByte)
		}
	}

	// Proses Split Pemecah Koma String Logistik: nip,imei1,simcard1
	parts := strings.Split(linkTarget, ",")
	if len(parts) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format link WhatsApp tidak sesuai standar (Wajib: NIP,IMEI,SIMCARD)"})
		return
	}

	nip := strings.TrimSpace(parts[0])
	imei1 := strings.TrimSpace(parts[1])
	simcard1 := strings.TrimSpace(parts[2])

	// Cek Keberadaan NIP Fisik di DB Karyawan bray
	var exists int64
	db.DB.Table("public.hrd_m_karyawan").Where("kry_nip = ?", nip).Count(&exists)
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "NIP Karyawan (" + nip + ") Tidak Terdaftar di Sistem!"})
		return
	}

	// Jalankan Update Inject Device Kilat
	err := db.DB.Table("public.hrd_m_karyawan").Where("kry_nip = ?", nip).Updates(map[string]interface{}{
		"kry_imei1":      imei1,
		"kry_simcardid1": simcard1,
		"kry_updatetime": time.Now(),
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal inject data via WhatsApp"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Sukses! Device NIP %s Berhasil Terdaftar via WhatsApp Link", nip),
	})
}

// 🗺️ 4. GET RAW DATA ABSENSI GEO TRACKING (Mengadopsi RawDataAbsensi/index.asp)
func GetRawAbsensiList(c *gin.Context) {
	nip := c.Query("nip")
	tgla := c.Query("tgla") // Format: YYYY-MM-DD
	tgle := c.Query("tgle")
	kdagen := c.Query("kdagen")
	div := c.Query("div")

	var results []map[string]interface{}

	query := db.DB.Table("public.hrd_t_absensi a").
		Select(`
			a.abs_nip, 
			k.kry_nama, 
			k.kry_ddbid, 
			d.div_nama, 
			g.agen_nama, 
			a.abs_lat, 
			a.abs_lon, 
			TO_CHAR(a.abs_datetime, 'YYYY-MM-DD HH24:MI:SS') as abs_datetime
		`).
		Joins("LEFT JOIN public.hrd_m_karyawan k ON a.abs_nip = k.kry_nip").
		Joins("LEFT JOIN public.hrd_m_divisi d ON k.kry_ddbid = d.div_code").
		Joins("LEFT JOIN public.glb_m_agen g ON a.abs_agenid = g.agen_id").
		Where("COALESCE(a.abs_nip, '') <> ''")

	// Filter Rentang Tanggal Absensi Lapangan
	if tgla != "" && tgle != "" {
		query = query.Where("a.abs_datetime::date BETWEEN ? AND ?", tgla, tgle)
	} else {
		// Default ke hari ini jika filter kosong agar server tidak jebol bray!
		query = query.Where("a.abs_datetime::date = CURRENT_DATE")
	}

	if nip != "" {
		query = query.Where("a.abs_nip = ?", nip)
	}
	if kdagen != "" {
		query = query.Where("a.abs_agenid = ?", kdagen)
	}
	if div != "" {
		query = query.Where("k.kry_ddbid = ?", div)
	}

	err := query.Order("a.abs_nip ASC, a.abs_datetime DESC").Find(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal muat raw data absensi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}
