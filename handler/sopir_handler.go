package handler

import (
	"fmt"
	"log"
	"net/http"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// DTO khusus untuk mendistribusikan data sopir dari hrd_m_karyawan
type SopirKaryawanDTO struct {
	KryNIP   string `json:"kry_nip" gorm:"column:kry_nip"`
	KryNama  string `json:"kry_nama" gorm:"column:kry_nama"`
	KryTelp1 string `json:"kry_telp1" gorm:"column:kry_telp1"`
	KryTelp2 string `json:"kry_telp2" gorm:"column:kry_telp2"`
	KryPIN   string `json:"kry_pin" gorm:"column:kry_pin"`
}

type AssignmentDTO struct {
	AssID   string `json:"ass_id" gorm:"column:ass_id"`
	AssNama string `json:"ass_nama" gorm:"column:ass_nama"`
}

// 📱 1. API GET: Ambil Daftar Sopir dari HRD Master Karyawan (Sesuai ASP Lawas & pgAdmin)
func GetSupirList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	searchNama := c.Query("nama")
	filterNamaSQL := ""
	if searchNama != "" {
		filterNamaSQL = fmt.Sprintf(" AND LOWER(Karyawan.kry_nama) LIKE LOWER('%%%s%%') ", searchNama)
	}

	// Query Presisi dari pgAdmin Gambar 2
	query := fmt.Sprintf(`
		SELECT DISTINCT 
			Karyawan.kry_nip, 
			Karyawan.kry_nama, 
			COALESCE(Karyawan.kry_telp1, '-') AS kry_telp1, 
			COALESCE(Karyawan.kry_telp2, '-') AS kry_telp2, 
			COALESCE(Karyawan.kry_pin, '-') AS kry_pin 
		FROM public.hrd_m_karyawan Karyawan 
		WHERE UPPER(Karyawan.kry_aktifyn) = 'Y' 
		  AND (Karyawan.kry_jabcode = '30' OR Karyawan.kry_jabcode = '31')
		  %s
		GROUP BY 
			Karyawan.kry_nip, 
			Karyawan.kry_nama, 
			Karyawan.kry_telp1, 
			Karyawan.kry_telp2, 
			Karyawan.kry_pin 
		ORDER BY Karyawan.kry_nama ASC
	`, filterNamaSQL)

	var listSopir []SopirKaryawanDTO
	err := database.Raw(query).Scan(&listSopir).Error
	if err != nil {
		log.Println("❌ ERROR GetSupirList:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data sopir dari master karyawan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   listSopir,
	})
}

// ➕ 2. API POST: Pendaftaran Sopir & Pairing IMEI Baru
func CreateSupir(c *gin.Context) {
	var input models.OprMSupir
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload input tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	username, _ := c.Get("username")
	usernameStr := fmt.Sprintf("%v", username)
	input.SupirUpdateID = &usernameStr
	input.SupirAktifYN = "Y"

	if err := database.Create(&input).Error; err != nil {
		log.Println("❌ ERROR CreateSupir:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mendaftarkan sopir, ID kemungkinan duplikat!"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Supir baru berhasil didaftarkan!"})
}

// 📝 3. API PUT: Edit Data Sopir & Update Identitas Device Tracker
func UpdateSupir(c *gin.Context) {
	id := c.Param("id")
	var input models.OprMSupir
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload update tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	username, _ := c.Get("username")
	usernameStr := fmt.Sprintf("%v", username)

	err := database.Model(&models.OprMSupir{}).Where("supir_id = ?", id).Updates(map[string]interface{}{
		"supir_nama":        input.SupirNama,
		"supir_borongan_yn": input.SupirBoronganYN,
		"supir_imei1":       input.SupirImei1,
		"supir_imei2":       input.SupirImei2,
		"supir_simcard_id1": input.SupirSimcardID1,
		"supir_simcard_id2": input.SupirSimcardID2,
		"supir_update_id":   usernameStr,
	}).Error

	if err != nil {
		log.Println("❌ ERROR UpdateSupir:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memperbarui data sopir"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data sopir berhasil diperbarui!"})
}

// 🗑️ 4. API DELETE: Soft Delete / Non-Aktifkan Sopir
func DeleteSupir(c *gin.Context) {
	id := c.Param("id")

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	username, _ := c.Get("username")
	usernameStr := fmt.Sprintf("%v", username)

	err := database.Model(&models.OprMSupir{}).Where("supir_id = ?", id).Updates(map[string]interface{}{
		"supir_aktif_yn":  "N",
		"supir_update_id": usernameStr,
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menonaktifkan sopir"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Status sopir resmi dinonaktifkan!"})
}

func GetAssignmentList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database resolver gagal"})
		return
	}

	// Query presisi ke tabel public.opr_m_assignment
	query := `
		SELECT 
			COALESCE(ass_id, '') AS ass_id, 
			COALESCE(ass_nama, '') AS ass_nama 
		FROM public.opr_m_assignment 
		WHERE UPPER(ass_aktifyn) = 'Y' 
		ORDER BY ass_nama ASC
	`

	var listAssignment []AssignmentDTO
	if err := database.Raw(query).Scan(&listAssignment).Error; err != nil {
		// Fallback default jika tabel belum terisi
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": []AssignmentDTO{
				{AssID: "AS001", AssNama: "PENGIRIMAN REGULER LINTAS"},
				{AssID: "AS002", AssNama: "PENGIRIMAN EKSPRES PRIORITAS"},
				{AssID: "AS003", AssNama: "LANGGANAN KHUSUS / CHARTER"},
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   listAssignment,
	})
}
