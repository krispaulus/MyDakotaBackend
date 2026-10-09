package handler

import (
	"dakotagroup/business-insight-be/db"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func getBannerListFromDB(database any) []string {
	type DBExec interface {
		Table(name string) any
	}

	var rawValue string
	// Asumsikan database bertipe *gorm.DB dari getJurnalDB / ResolveDB
	if dbConn, ok := database.(interface {
		Table(name string) interface {
			Select(query any, args ...any) interface {
				Where(query any, args ...any) interface {
					Limit(limit int) interface {
						Scan(dest any) interface{ Error() string }
					}
				}
			}
		}
	}); ok {
		_ = dbConn.Table("public.glb_m_param").
			Select("COALESCE(set_varvalue, '')").
			Where("TRIM(LOWER(set_varname)) = 'login_banner_url'").
			Limit(1).
			Scan(&rawValue)
	}

	banners := make([]string, 5)
	rawValue = strings.TrimSpace(rawValue)

	if rawValue != "" {
		// Cek apakah data tersimpan sebagai JSON Array
		var parsed []string
		if err := json.Unmarshal([]byte(rawValue), &parsed); err == nil {
			for i := 0; i < len(parsed) && i < 5; i++ {
				banners[i] = parsed[i]
			}
		} else {
			// Jika format lama (hanya string 1 link url)
			banners[0] = rawValue
		}
	}

	return banners
}

// =========================================================================
// 🚀 2. UPLOAD BANNER LOGIN PER SLOT (0 - 4)
// Endpoint: POST /api/settings/upload-login-banner
// =========================================================================

func UploadLoginBannerHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	// 1. Ambil slot_index (default 0 jika tidak dikirim)
	slotIndexStr := c.DefaultPostForm("slot_index", "0")
	slotIndex, err := strconv.Atoi(slotIndexStr)
	if err != nil || slotIndex < 0 || slotIndex >= 5 {
		slotIndex = 0
	}

	// 2. Ambil file gambar
	file, err := c.FormFile("banner_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File banner wajib diunggah!"})
		return
	}

	// Validasi format
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format gambar harus JPG, JPEG, PNG, atau WEBP"})
		return
	}

	// Siapkan folder
	uploadDir := "./uploads/banners"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyiapkan folder penyimpanan"})
		return
	}

	// Buat nama file unik dengan prefix nomor slot
	filename := fmt.Sprintf("login_banner_slot%d_%d%s", slotIndex+1, time.Now().Unix(), ext)
	dst := filepath.Join(uploadDir, filename)

	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan file: " + err.Error()})
		return
	}

	publicURL := fmt.Sprintf("/uploads/banners/%s", filename)

	// 3. Baca list banner yang sudah ada
	var rawValue string
	database.Table("public.glb_m_param").
		Select("COALESCE(set_varvalue, '')").
		Where("TRIM(LOWER(set_varname)) = 'login_banner_url'").
		Limit(1).
		Scan(&rawValue)

	banners := make([]string, 5)
	rawValue = strings.TrimSpace(rawValue)
	if rawValue != "" {
		var parsed []string
		if err := json.Unmarshal([]byte(rawValue), &parsed); err == nil {
			for i := 0; i < len(parsed) && i < 5; i++ {
				banners[i] = parsed[i]
			}
		} else {
			banners[0] = rawValue
		}
	}

	// Timpa gambar di slot yang dipilih
	banners[slotIndex] = publicURL

	// Serialize ke JSON String
	jsonBytes, err := json.Marshal(banners)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal serialize data banner"})
		return
	}
	jsonString := string(jsonBytes)

	// 4. Update ke database
	res := database.Exec(`
		UPDATE public.glb_m_param 
		SET set_varvalue = ?, set_updatetime = NOW() 
		WHERE TRIM(LOWER(set_varname)) = 'login_banner_url'
	`, jsonString)

	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan konfigurasi: " + res.Error.Error()})
		return
	}

	if res.RowsAffected == 0 {
		_ = database.Exec(`
			INSERT INTO public.glb_m_param (set_varname, set_varvalue, set_updatetime) 
			VALUES ('login_banner_url', ?, NOW())
		`, jsonString).Error
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    fmt.Sprintf("Banner Slot #%d berhasil diunggah!", slotIndex+1),
		"banners":    banners,
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
// 🖼️ 1. GET BANNER LOGIN (PUBLIC & SETTINGS)
// Endpoint: GET /api/public/login-banner & GET /api/settings/login-banner
// =========================================================================
func GetLoginBannerHandler(c *gin.Context) {
	database, ok := db.ResolveDB("C")
	if !ok {
		database = getJurnalDB(c)
	}

	if database == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":     "success",
			"banners":    make([]string, 5),
			"banner_url": "",
		})
		return
	}

	var rawValue string
	database.Table("public.glb_m_param").
		Select("COALESCE(set_varvalue, '')").
		Where("TRIM(LOWER(set_varname)) = 'login_banner_url'").
		Limit(1).
		Scan(&rawValue)

	banners := make([]string, 5)
	rawValue = strings.TrimSpace(rawValue)

	if rawValue != "" {
		var parsed []string
		if err := json.Unmarshal([]byte(rawValue), &parsed); err == nil {
			for i := 0; i < len(parsed) && i < 5; i++ {
				banners[i] = parsed[i]
			}
		} else {
			banners[0] = rawValue
		}
	}

	// Cari banner pertama yang aktif untuk fallback `banner_url`
	firstActive := ""
	for _, b := range banners {
		if strings.TrimSpace(b) != "" {
			firstActive = b
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"banners":    banners,     // List 5 slot banner untuk frontend baru
		"banner_url": firstActive, // Tetap sediakan untuk kompatibilitas frontend lama
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

// =========================================================================
// 🗑️ 3. DELETE BANNER LOGIN PER SLOT
// Endpoint: DELETE /api/settings/login-banner/:slotIndex
// =========================================================================
func DeleteLoginBannerHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	slotIndexParam := c.Param("slotIndex")
	slotIndex, err := strconv.Atoi(slotIndexParam)
	if err != nil || slotIndex < 0 || slotIndex >= 5 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Indeks slot tidak valid (0-4)"})
		return
	}

	var rawValue string
	database.Table("public.glb_m_param").
		Select("COALESCE(set_varvalue, '')").
		Where("TRIM(LOWER(set_varname)) = 'login_banner_url'").
		Limit(1).
		Scan(&rawValue)

	banners := make([]string, 5)
	rawValue = strings.TrimSpace(rawValue)
	if rawValue != "" {
		var parsed []string
		if err := json.Unmarshal([]byte(rawValue), &parsed); err == nil {
			for i := 0; i < len(parsed) && i < 5; i++ {
				banners[i] = parsed[i]
			}
		} else {
			banners[0] = rawValue
		}
	}

	// Kosongkan slot yang dihapus
	banners[slotIndex] = ""

	jsonBytes, _ := json.Marshal(banners)
	jsonString := string(jsonBytes)

	res := database.Exec(`
		UPDATE public.glb_m_param 
		SET set_varvalue = ?, set_updatetime = NOW() 
		WHERE TRIM(LOWER(set_varname)) = 'login_banner_url'
	`, jsonString)

	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus banner: " + res.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Banner Slot #%d berhasil dihapus!", slotIndex+1),
		"banners": banners,
	})
}
