package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AssignmentDTO struct {
	AssID   string `json:"ass_id" gorm:"column:ass_id"`
	AssNama string `json:"ass_nama" gorm:"column:ass_nama"`
}

func resolveSopirDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		return db.DB
	}
	return database
}

// 1. API GET: Ambil Daftar Sopir Lengkap (Gabungan hrd_m_karyawan & opr_m_supir)
func GetSupirList(c *gin.Context) {
	database := resolveSopirDB(c)

	searchNama := strings.TrimSpace(c.Query("nama"))
	statusAktif := strings.TrimSpace(c.Query("aktif"))

	filterSQL := ""
	var args []interface{}

	if searchNama != "" {
		filterSQL += " AND (UPPER(k.kry_nama) LIKE UPPER(?) OR UPPER(k.kry_nip) LIKE UPPER(?)) "
		args = append(args, "%"+searchNama+"%", "%"+searchNama+"%")
	}

	if statusAktif != "" {
		filterSQL += " AND COALESCE(NULLIF(TRIM(s.supir_aktifyn), ''), COALESCE(TRIM(k.kry_aktifyn), 'Y')) = ? "
		args = append(args, strings.ToUpper(statusAktif))
	} else {
		filterSQL += " AND COALESCE(NULLIF(TRIM(s.supir_aktifyn), ''), COALESCE(TRIM(k.kry_aktifyn), 'Y')) = 'Y' "
	}

	query := fmt.Sprintf(`
		SELECT * FROM (
			SELECT DISTINCT ON (TRIM(k.kry_nip))
				TRIM(k.kry_nip) AS supir_id,
				TRIM(k.kry_nip) AS kry_nip,
				COALESCE(NULLIF(TRIM(s.supir_nama), ''), TRIM(k.kry_nama), '-') AS supir_nama,
				COALESCE(NULLIF(TRIM(s.supir_nama), ''), TRIM(k.kry_nama), '-') AS kry_nama,
				COALESCE(k.kry_telp1, '-') AS kry_telp1,
				COALESCE(k.kry_telp2, '-') AS kry_telp2,
				COALESCE(k.kry_pin, '-') AS kry_pin,
				COALESCE(NULLIF(TRIM(s.supir_boronganyn), ''), 'N') AS supir_borongan_yn,
				COALESCE(NULLIF(TRIM(s.supir_aktifyn), ''), COALESCE(TRIM(k.kry_aktifyn), 'Y')) AS supir_aktif_yn,
				COALESCE(TRIM(k.kry_imei1), '') AS supir_imei1,
				COALESCE(TRIM(k.kry_imei2), '') AS supir_imei2,
				COALESCE(TRIM(k.kry_simcardid1), '') AS supir_simcard_id1,
				COALESCE(TRIM(k.kry_simcardid2), '') AS supir_simcard_id2,
				COALESCE(s.supir_updatetime, k.kry_updatetime) AS last_updated
			FROM public.hrd_m_karyawan k
			LEFT JOIN public.opr_m_supir s ON TRIM(s.supir_id) = TRIM(k.kry_nip)
			WHERE (TRIM(k.kry_jabcode) IN ('30', '31') OR s.supir_id IS NOT NULL)
			  AND COALESCE(TRIM(k.kry_nama), '') <> ''
			  %s
			ORDER BY TRIM(k.kry_nip), COALESCE(s.supir_updatetime, k.kry_updatetime) DESC NULLS LAST
		) sub
		ORDER BY last_updated DESC NULLS LAST, supir_nama ASC
	`, filterSQL)

	listSopir := make([]map[string]interface{}, 0)
	if err := database.Raw(query, args...).Scan(&listSopir).Error; err != nil {
		log.Println("❌ ERROR GetSupirList:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data sopir: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   listSopir,
	})
}

// 2. API POST: Pendaftaran Sopir & Pairing IMEI Baru
func CreateSupir(c *gin.Context) {
	database := resolveSopirDB(c)

	var req struct {
		SupirID         string `json:"supir_id" binding:"required"`
		SupirNama       string `json:"supir_nama" binding:"required"`
		SupirBoronganYN string `json:"supir_borongan_yn"`
		SupirImei1      string `json:"supir_imei1"`
		SupirImei2      string `json:"supir_imei2"`
		SupirSimcard1   string `json:"supir_simcard_id1"`
		SupirSimcard2   string `json:"supir_simcard_id2"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID Sopir dan Nama wajib diisi!"})
		return
	}

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)
	if userStr == "" || userStr == "<nil>" {
		userStr = "ADMIN"
	}

	supirID := strings.ToUpper(strings.TrimSpace(req.SupirID))
	supirNama := strings.ToUpper(strings.TrimSpace(req.SupirNama))
	borongan := "N"
	if strings.ToUpper(req.SupirBoronganYN) == "Y" {
		borongan = "Y"
	}

	// 1. Simpan ke public.opr_m_supir
	err := database.Exec(`
		INSERT INTO public.opr_m_supir (supir_id, supir_nama, supir_boronganyn, supir_aktifyn, supir_updateid, supir_updatetime)
		VALUES (?, ?, ?, 'Y', ?, CURRENT_TIMESTAMP)
		ON CONFLICT (supir_id) DO UPDATE SET
			supir_nama = EXCLUDED.supir_nama,
			supir_boronganyn = EXCLUDED.supir_boronganyn,
			supir_aktifyn = 'Y',
			supir_updateid = EXCLUDED.supir_updateid,
			supir_updatetime = CURRENT_TIMESTAMP
	`, supirID, supirNama, borongan, userStr).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mendaftarkan sopir: " + err.Error()})
		return
	}

	// 2. Sinkronkan ke public.hrd_m_karyawan beserta IMEI & Simcard
	database.Exec(`
		INSERT INTO public.hrd_m_karyawan (
			kry_nip, kry_nama, kry_jabcode, kry_aktifyn,
			kry_imei1, kry_imei2, kry_simcardid1, kry_simcardid2,
			kry_updateid, kry_updatetime
		) VALUES (
			?, ?, '30', 'Y',
			NULLIF(TRIM(?), ''), NULLIF(TRIM(?), ''), NULLIF(TRIM(?), ''), NULLIF(TRIM(?), ''),
			?, CURRENT_TIMESTAMP
		)
		ON CONFLICT (kry_nip) DO UPDATE SET
			kry_nama = EXCLUDED.kry_nama,
			kry_aktifyn = 'Y',
			kry_imei1 = COALESCE(EXCLUDED.kry_imei1, hrd_m_karyawan.kry_imei1),
			kry_imei2 = COALESCE(EXCLUDED.kry_imei2, hrd_m_karyawan.kry_imei2),
			kry_simcardid1 = COALESCE(EXCLUDED.kry_simcardid1, hrd_m_karyawan.kry_simcardid1),
			kry_simcardid2 = COALESCE(EXCLUDED.kry_simcardid2, hrd_m_karyawan.kry_simcardid2),
			kry_updateid = EXCLUDED.kry_updateid,
			kry_updatetime = CURRENT_TIMESTAMP
	`, supirID, supirNama, req.SupirImei1, req.SupirImei2, req.SupirSimcard1, req.SupirSimcard2, userStr)

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Supir baru berhasil didaftarkan!"})
}

// 3. API PUT: Edit Data Sopir, Device Tracker, & Status Aktif/Resign
func UpdateSupir(c *gin.Context) {
	database := resolveSopirDB(c)
	id := strings.TrimSpace(c.Param("id"))

	var req struct {
		SupirNama       string `json:"supir_nama" binding:"required"`
		SupirBoronganYN string `json:"supir_borongan_yn"`
		SupirAktifYN    string `json:"supir_aktif_yn"`
		SupirImei1      string `json:"supir_imei1"`
		SupirImei2      string `json:"supir_imei2"`
		SupirSimcard1   string `json:"supir_simcard_id1"`
		SupirSimcard2   string `json:"supir_simcard_id2"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload update tidak valid"})
		return
	}

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)
	if userStr == "" || userStr == "<nil>" {
		userStr = "ADMIN"
	}

	supirNama := strings.ToUpper(strings.TrimSpace(req.SupirNama))
	borongan := "N"
	if strings.ToUpper(req.SupirBoronganYN) == "Y" {
		borongan = "Y"
	}
	aktif := "Y"
	if strings.ToUpper(req.SupirAktifYN) == "N" {
		aktif = "N"
	}

	err := database.Exec(`
		INSERT INTO public.opr_m_supir (supir_id, supir_nama, supir_boronganyn, supir_aktifyn, supir_updateid, supir_updatetime)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (supir_id) DO UPDATE SET
			supir_nama = EXCLUDED.supir_nama,
			supir_boronganyn = EXCLUDED.supir_boronganyn,
			supir_aktifyn = EXCLUDED.supir_aktifyn,
			supir_updateid = EXCLUDED.supir_updateid,
			supir_updatetime = CURRENT_TIMESTAMP
	`, id, supirNama, borongan, aktif, userStr).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui data sopir: " + err.Error()})
		return
	}

	database.Exec(`
		UPDATE public.hrd_m_karyawan
		SET kry_nama = ?,
		    kry_aktifyn = ?,
		    kry_imei1 = NULLIF(TRIM(?), ''),
		    kry_imei2 = NULLIF(TRIM(?), ''),
		    kry_simcardid1 = NULLIF(TRIM(?), ''),
		    kry_simcardid2 = NULLIF(TRIM(?), ''),
		    kry_updateid = ?,
		    kry_updatetime = CURRENT_TIMESTAMP
		WHERE TRIM(kry_nip) = TRIM(?)
	`, supirNama, aktif, req.SupirImei1, req.SupirImei2, req.SupirSimcard1, req.SupirSimcard2, userStr, id)

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data sopir berhasil diperbarui!"})
}

// 4. API DELETE: Non-Aktifkan Sopir (Resign)
func DeleteSupir(c *gin.Context) {
	database := resolveSopirDB(c)
	id := strings.TrimSpace(c.Param("id"))

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)
	if userStr == "" || userStr == "<nil>" {
		userStr = "ADMIN"
	}

	database.Exec(`
		INSERT INTO public.opr_m_supir (supir_id, supir_nama, supir_boronganyn, supir_aktifyn, supir_updateid, supir_updatetime)
		SELECT TRIM(kry_nip), COALESCE(TRIM(kry_nama), '-'), 'N', 'N', ?, CURRENT_TIMESTAMP
		FROM public.hrd_m_karyawan WHERE TRIM(kry_nip) = TRIM(?)
		ON CONFLICT (supir_id) DO UPDATE SET
			supir_aktifyn = 'N', supir_updateid = EXCLUDED.supir_updateid, supir_updatetime = CURRENT_TIMESTAMP
	`, userStr, id)

	database.Exec(`
		UPDATE public.hrd_m_karyawan
		SET kry_aktifyn = 'N', kry_updateid = ?, kry_updatetime = CURRENT_TIMESTAMP
		WHERE TRIM(kry_nip) = TRIM(?)
	`, userStr, id)

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Status sopir resmi dinonaktifkan (Resign)!"})
}

func GetAssignmentList(c *gin.Context) {
	database := resolveSopirDB(c)

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
