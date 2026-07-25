package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GET /api/agens - DYNAMIC MULTI-TENANT
func GetAgens(c *gin.Context) {
	var agens []models.Agen
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung bray"})
		return
	}

	err := database.Table("public.glb_m_agen").Select(`
		agen_id, agen_kode, agen_nama, agen_alamat, agen_kotaid, agen_kota, 
		agen_kecamatan, agen_propinsi, agen_aktifyn, agen_cabangid, agen_contactperson, 
		agen_stt, agen_phone1, agen_phone2, agen_phone3, agen_dialstring, agen_kaakunting, agen_kacontact
	`).Order("agen_id DESC").Find(&agens).Error

	if err != nil {
		log.Printf("❌ ERROR SQL SELECT ALL AGENS: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengambil data fisik agen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": agens})
}

// POST /api/agens - DYNAMIC MULTI-TENANT
func CreateAgen(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung bray"})
		return
	}

	var input models.Agen
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	prefix := input.AgenKotaID
	if prefix == "" {
		prefix = "PST"
	}

	var lastAgen models.Agen
	database.Table("public.glb_m_agen").Where("agen_id LIKE ?", prefix+"%").Order("agen_id DESC").First(&lastAgen)

	var nextNumber = 1
	if lastAgen.AgenID != "" && len(lastAgen.AgenID) >= 6 {
		fmt.Sscanf(lastAgen.AgenID[3:], "%d", &nextNumber)
		nextNumber++
	}

	input.AgenID = fmt.Sprintf("%s%03d", prefix, nextNumber)
	now := time.Now()
	input.AgenUpdateTime = &now
	input.AgenAktifTanggal = &now

	if err := database.Table("public.glb_m_agen").Create(&input).Error; err != nil {
		log.Printf("❌ ERROR CREATE AGEN: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan agen baru"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Agen berhasil ditambahkan", "id": input.AgenID})
}

// PUT /api/agens/:id - DYNAMIC MULTI-TENANT
func UpdateAgen(c *gin.Context) {
	id := c.Param("id")
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var agen models.Agen
	if err := database.Table("public.glb_m_agen").First(&agen, "agen_id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data agen tidak ditemukan"})
		return
	}

	if err := c.ShouldBindJSON(&agen); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	now := time.Now()
	agen.AgenUpdateTime = &now

	if err := database.Table("public.glb_m_agen").Where("agen_id = ?", id).Save(&agen).Error; err != nil {
		log.Printf("❌ GAGAL UPDATE AGEN SQL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui data agen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data agen berhasil diperbarui"})
}

// DELETE /api/agens/:id - DYNAMIC MULTI-TENANT
func DeleteAgen(c *gin.Context) {
	id := c.Param("id")
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	now := time.Now()
	err := database.Table("public.glb_m_agen").Where("agen_id = ?", id).Updates(map[string]interface{}{
		"agen_aktifyn":         "N",
		"agen_nonaktiftanggal": &now,
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menonaktifkan agen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Agen berhasil dinonaktifkan"})
}

// GET /api/agens/detail/:kode - DYNAMIC MULTI-TENANT
func GetDetailAgen(c *gin.Context) {
	kode := c.Param("kode")
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung bray"})
		return
	}

	var agen struct {
		AgenID            string `json:"agen_id"`
		AgenKotaID        string `json:"agen_kotaid"`
		AgenCabangID      string `json:"agen_cabangid"`
		AgenTlc           string `json:"agen_tlc"`
		AgenKode          string `json:"agen_kode"`
		AgenNama          string `json:"agen_nama"`
		AgenAlamat        string `json:"agen_alamat"`
		AgenKota          string `json:"agen_kota"`
		AgenKecamatan     string `json:"agen_kecamatan"`
		AgenPropinsi      string `json:"agen_propinsi"`
		AgenContactPerson string `json:"agen_contactperson"`
		AgenStt           string `json:"agen_stt"`
		BronzePhone1      string `json:"agen_phone1"`
	}

	queryRaw := `
		SELECT 
			COALESCE(agen_id, '') as agen_id, COALESCE(agen_kotaid, '') as agen_kotaid, 
			COALESCE(agen_cabangid, '') as agen_cabangid, COALESCE(agen_tlc, '') as agen_tlc, 
			COALESCE(agen_kode, '') as agen_kode, COALESCE(agen_nama, '') as agen_nama, 
			COALESCE(agen_alamat, '') as agen_alamat, COALESCE(agen_kota, '') as agen_kota, 
			COALESCE(agen_kecamatan, '') as agen_kecamatan, COALESCE(agen_propinsi, '') as agen_propinsi, 
			COALESCE(agen_contactperson, '') as agen_contactperson, COALESCE(agen_stt, '') as agen_stt, 
			COALESCE(agen_phone1, '') as agen_phone1 
		FROM public.glb_m_agen 
		WHERE UPPER(TRIM(agen_kode)) = UPPER(TRIM(?))`

	if err := database.Raw(queryRaw, kode).Scan(&agen).Error; err != nil {
		log.Printf("❌ GAGAL SCAN EXPLICIT DETAIL AGEN: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat detail data fisik agen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": agen})
}

// GET /api/agens/search-name/:nama - DYNAMIC MULTI-TENANT COMPLETE
func GetDetailAgenByName(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	namaSearch := c.Param("nama")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database failed"})
		return
	}

	var agen models.GlbMAgen // Menggunakan struct glb_m_agen asli Dakota bray
	err := database.Table("public.glb_m_agen").
		Where("agen_nama ILIKE ?", "%"+namaSearch+"%").
		First(&agen).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Agen tidak ditemukan di Database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   agen,
	})
}
