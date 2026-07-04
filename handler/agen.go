package handler

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"dakotagroup/business-insight-be/db" // sesuaikan dengan path project kamu
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

func GetAgens(c *gin.Context) {
	var agens []models.Agen

	// 🚀 SUNTIKAN NUSANTARA ELEGAN: Tambahkan agen_kaakunting dan agen_kacontact ke baris select utama
	err := db.DB.Select(`
		agen_id, agen_kode, agen_nama, agen_alamat, agen_kotaid, agen_kota, 
		agen_kecamatan, agen_propinsi, agen_aktifyn, agen_cabangid,
		agen_contactperson, agen_stt, agen_phone1, agen_phone2, agen_phone3, agen_dialstring,
		agen_kaakunting, agen_kacontact
	`).Order("agen_id DESC").Find(&agens).Error

	if err != nil {
		log.Printf("❌ ERROR SQL SELECT ALL AGENS: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data fisik agen dari database",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   agens,
	})
}

func CreateAgen(c *gin.Context) {
	var input models.Agen
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Otomasi ID Generator (Jika kota JAT, cari JAT001, dst)
	prefix := input.AgenKotaID
	if prefix == "" {
		prefix = "PST"
	}
	var lastAgen models.Agen
	db.DB.Where("agen_id LIKE ?", prefix+"%").Order("agen_id DESC").First(&lastAgen)

	var nextNumber = 1
	if lastAgen.AgenID != "" && len(lastAgen.AgenID) >= 6 {
		fmt.Sscanf(lastAgen.AgenID[3:], "%d", &nextNumber)
		nextNumber++
	}
	input.AgenID = fmt.Sprintf("%s%03d", prefix, nextNumber)
	now := time.Now()
	input.AgenUpdateTime = &now
	input.AgenAktifTanggal = &now

	if err := db.DB.Create(&input).Error; err != nil {
		log.Printf("❌ ERROR CREATE AGEN: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan agen baru"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Agen berhasil ditambahkan", "id": input.AgenID})
}

// UpdateAgen memproses pembaharuan data fisik glb_m_agen berdasarkan agen_id
func UpdateAgen(c *gin.Context) {
	id := c.Param("id") // 🟢 WAJIB "id" karena di main.go ditulis "/agens/:id"
	var agen models.Agen

	// 1. Cari data berdasarkan agen_id fisik di database
	if err := db.DB.First(&agen, "agen_id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data agen tidak ditemukan"})
		return
	}

	// 2. Bind payload JSON baru dari frontend
	if err := c.ShouldBindJSON(&agen); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	now := time.Now()
	agen.AgenUpdateTime = &now

	// 3. Simpan perubahan ke database Postgres
	if err := db.DB.Save(&agen).Error; err != nil {
		log.Printf("❌ GAGAL UPDATE AGEN SQL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui data agen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data agen berhasil diperbarui"})
}

// DeleteAgen mengubah status agen menjadi Non-Aktif (Soft Delete Aman untuk Akuntansi)
func DeleteAgen(c *gin.Context) {
	id := c.Param("id")
	now := time.Now()
	err := db.DB.Model(&models.Agen{}).Where("agen_id = ?", id).Updates(map[string]interface{}{
		"agen_aktifyn":         "N",
		"agen_nonaktiftanggal": &now,
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menonaktifkan agen"})
		return
	}

	var agen models.Agen

	if err := db.DB.First(&agen, "agen_id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data agen tidak ditemukan"})
		return
	}

	// SOP Akuntansi Dakota Cargo: Ubah agen_aktifyn menjadi 'N' agar history data transaksi masa lalu tidak rusak!
	if err := db.DB.Model(&agen).Update("agen_aktifyn", "N").Error; err != nil {
		log.Printf("❌ GAGAL SOFT DELETE AGEN SQL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus status agen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Agen berhasil dinonaktifkan"})
}

// GetDetailAgen memproses scanning data terarah, mengamankan seluruh kolom geografi se-Nusantara
func GetDetailAgen(c *gin.Context) {
	kode := c.Param("kode")

	// 🚀 DEKLARASI STRUCT KHUSUS DETAIL: Wajib daftarkan properti json secara lengkap!
	var agen struct {
		AgenID            string `json:"agen_id"`
		AgenKotaID        string `json:"agen_kotaid"` // 🟢 KUNCI AMAN 1
		AgenCabangID      string `json:"agen_cabangid"`
		AgenTlc           string `json:"agen_tlc"`
		AgenKode          string `json:"agen_kode"`
		AgenNama          string `json:"agen_nama"`
		AgenAlamat        string `json:"agen_alamat"`
		AgenKota          string `json:"agen_kota"`
		AgenKecamatan     string `json:"agen_kecamatan"` // 🟢 KUNCI AMAN 2
		AgenPropinsi      string `json:"agen_propinsi"`  // 🟢 KUNCI AMAN 3
		AgenContactPerson string `json:"agen_contactperson"`
		AgenStt           string `json:"agen_stt"`
		AgenPhone1        string `json:"agen_phone1"`
	}

	// 🚀 SQL EXPLICIT LAPIS BAJA: Panggil nama kolom fisiknya satu per satu secara tegas dari schema public!
	queryRaw := `
		SELECT 
			COALESCE(agen_id, '') as agen_id,
			COALESCE(agen_kotaid, '') as agen_kotaid,
			COALESCE(agen_cabangid, '') as agen_cabangid,
			COALESCE(agen_tlc, '') as agen_tlc,
			COALESCE(agen_kode, '') as agen_kode,
			COALESCE(agen_nama, '') as agen_nama,
			COALESCE(agen_alamat, '') as agen_alamat,
			COALESCE(agen_kota, '') as agen_kota,
			COALESCE(agen_kecamatan, '') as agen_kecamatan,
			COALESCE(agen_propinsi, '') as agen_propinsi,
			COALESCE(agen_contactperson, '') as agen_contactperson,
			COALESCE(agen_stt, '') as agen_stt,
			COALESCE(agen_phone1, '') as agen_phone1
		FROM public.glb_m_agen 
		WHERE UPPER(TRIM(agen_kode)) = UPPER(TRIM(?))`

	if err := db.DB.Raw(queryRaw, kode).Scan(&agen).Error; err != nil {
		log.Printf("❌ GAGAL SCAN EXPLICIT DETAIL AGEN: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat detail data fisik agen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   agen,
	})
}
