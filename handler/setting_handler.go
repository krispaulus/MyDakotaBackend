package handler

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func UploadLoginBannerHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	// 1. Ambil file dari form-data
	file, err := c.FormFile("banner_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File banner wajib diunggah!"})
		return
	}

	// Validasi ekstensi
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format gambar harus JPG, JPEG, PNG, atau WEBP"})
		return
	}

	// 2. Buat direktori penyimpanan jika belum ada
	uploadDir := "./uploads/banners"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyiapkan folder penyimpanan"})
		return
	}

	// 3. Buat nama file unik
	filename := fmt.Sprintf("login_banner_%d%s", time.Now().Unix(), ext)
	dst := filepath.Join(uploadDir, filename)

	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan file: " + err.Error()})
		return
	}

	// URL path publik yang akan disimpan ke database
	publicURL := fmt.Sprintf("/uploads/banners/%s", filename)

	// 4. Simpan / Perbarui nilai ke tabel glb_m_param (tanpa ubah struktur tabel)
	res := database.Exec(`
		UPDATE public.glb_m_param 
		SET set_varvalue = ?, set_updatetime = NOW() 
		WHERE TRIM(LOWER(set_varname)) = 'login_banner_url'
	`, publicURL)

	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan konfigurasi: " + res.Error.Error()})
		return
	}

	if res.RowsAffected == 0 {
		_ = database.Exec(`
			INSERT INTO public.glb_m_param (set_varname, set_varvalue, set_updatetime) 
			VALUES ('login_banner_url', ?, NOW())
		`, publicURL).Error
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "Banner login berhasil diunggah dan diterapkan!",
		"banner_url": publicURL,
	})
}

// GET /api/setting/operasional
func GetSettingOperasionalHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var val string
	database.Table("public.glb_m_param").
		Select("COALESCE(set_varvalue, 'N')").
		Where("TRIM(LOWER(set_varname)) = 'allow_pusat_create_btt'").
		Limit(1).
		Scan(&val)

	allowPusat := strings.ToUpper(strings.TrimSpace(val)) == "Y"

	c.JSON(http.StatusOK, gin.H{
		"status":                 "success",
		"allow_pusat_create_btt": allowPusat,
	})
}

// POST /api/settings/operasional
func SaveSettingOperasionalHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		AllowPusatCreateBTT bool `json:"allow_pusat_create_btt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	valStr := "N"
	if req.AllowPusatCreateBTT {
		valStr = "Y"
	}

	// 1. Coba Update baris yang sudah ada terlebih dahulu
	res := database.Exec(`
		UPDATE public.glb_m_param 
		SET set_varvalue = ?, set_updatetime = NOW() 
		WHERE TRIM(LOWER(set_varname)) = 'allow_pusat_create_btt'
	`, valStr)

	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update setting: " + res.Error.Error()})
		return
	}

	// 2. Jika baris belum pernah ada (RowsAffected == 0), lakukan INSERT baru
	if res.RowsAffected == 0 {
		err := database.Exec(`
			INSERT INTO public.glb_m_param (set_varname, set_varvalue, set_updatetime) 
			VALUES ('allow_pusat_create_btt', ?, NOW())
		`, valStr).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal insert setting: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":                 "success",
		"message":                "Pengaturan izin input BTT Pusat Dakota berhasil diubah!",
		"allow_pusat_create_btt": req.AllowPusatCreateBTT,
	})
}

// =========================================================================
// 🖼️ 1. GET BANNER LOGIN (PUBLIC - BISA DIAKSES SEBELUM LOGIN)
// Endpoint: GET /api/public/login-banner
// =========================================================================
func GetLoginBannerHandler(c *gin.Context) {
	// Ambil koneksi database (fallback ke "A" atau "C" jika public/belum login)
	database, ok := db.ResolveDB("C") // Atau db.ResolveDB("A")
	if !ok {
		database = getJurnalDB(c)
	}

	if database == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":     "success",
			"banner_url": "",
		})
		return
	}

	var bannerURL string
	database.Table("public.glb_m_param").
		Select("COALESCE(set_varvalue, '')").
		Where("TRIM(LOWER(set_varname)) = 'login_banner_url'").
		Limit(1).
		Scan(&bannerURL)

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"banner_url": strings.TrimSpace(bannerURL),
	})
}

// =========================================================================
// ⚙️ 2. SAVE BANNER LOGIN (ADMIN SETTINGS)
// Endpoint: POST /api/settings/login-banner
// =========================================================================
func SaveLoginBannerHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		BannerURL string `json:"banner_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	cleanURL := strings.TrimSpace(req.BannerURL)

	// 1. Coba UPDATE param login_banner_url
	res := database.Exec(`
		UPDATE public.glb_m_param 
		SET set_varvalue = ?, set_updatetime = NOW() 
		WHERE TRIM(LOWER(set_varname)) = 'login_banner_url'
	`, cleanURL)

	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update banner: " + res.Error.Error()})
		return
	}

	// 2. Jika belum ada, INSERT baris baru
	if res.RowsAffected == 0 {
		err := database.Exec(`
			INSERT INTO public.glb_m_param (set_varname, set_varvalue, set_updatetime) 
			VALUES ('login_banner_url', ?, NOW())
		`, cleanURL).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal insert banner: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "Banner login berhasil disimpan!",
		"banner_url": cleanURL,
	})
}
