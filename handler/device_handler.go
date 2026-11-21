package handler

import (
	"dakotagroup/business-insight-be/db"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func resolveDeviceDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		return db.DB
	}
	return database
}

// Struct response master device karyawan (Sesuai SELECT di hrd_m_kry_device.asp)
type EmployeeDeviceRes struct {
	KryNIP        string  `gorm:"column:kry_nip;primaryKey" json:"kry_nip"`
	KryNama       string  `gorm:"column:kry_nama" json:"kry_nama"`
	KryImei1      *string `gorm:"column:kry_imei1" json:"kry_imei1"`
	KryImei2      *string `gorm:"column:kry_imei2" json:"kry_imei2"`
	KrySimcardID1 *string `gorm:"column:kry_simcardid1" json:"kry_simcard_id1"`
	KrySimcardID2 *string `gorm:"column:kry_simcardid2" json:"kry_simcard_id2"`
}

// 1. GET LIST DEVICE KARYAWAN (hrd_m_kry_device.asp)
func GetDeviceKaryawanList(c *gin.Context) {
	database := resolveDeviceDB(c)
	results := make([]EmployeeDeviceRes, 0)

	namaFilter := strings.TrimSpace(c.Query("nama"))
	typeKry := strings.TrimSpace(c.Query("typekry")) // H = Harian, KT = Kontrak/Tetap

	query := database.Table("public.hrd_m_karyawan").
		Select("TRIM(kry_nip) AS kry_nip, TRIM(kry_nama) AS kry_nama, kry_imei1, kry_imei2, kry_simcardid1, kry_simcardid2").
		Where("UPPER(COALESCE(kry_aktifyn, 'Y')) = 'Y'")

	if namaFilter != "" {
		query = query.Where("(kry_nama ILIKE ? OR kry_nip ILIKE ?)", "%"+namaFilter+"%", "%"+namaFilter+"%")
	}

	if typeKry == "H" {
		query = query.Where("kry_nip ILIKE '%H%'")
	} else if typeKry == "KT" {
		query = query.Where("SUBSTRING(TRIM(kry_nip), 4, 1) <> 'H'")
	}

	err := query.Order("kry_updatetime DESC NULLS LAST, kry_nama ASC").Find(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal muat device karyawan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 2. UPDATE ATRIBUT DEVICE MANUAL (p-hrd_m_kry_device_e.asp)
func UpdateDeviceKaryawan(c *gin.Context) {
	database := resolveDeviceDB(c)
	nip := strings.TrimSpace(c.Param("nip"))

	var input struct {
		Imei1      string `json:"kry_imei1" binding:"required"`
		Imei2      string `json:"kry_imei2"`
		SimcardID1 string `json:"kry_simcard_id1" binding:"required"`
		SimcardID2 string `json:"kry_simcard_id2"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "IMEI 1 dan Simcard 1 wajib diisi"})
		return
	}

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)

	err := database.Table("public.hrd_m_karyawan").Where("TRIM(kry_nip) = TRIM(?)", nip).Updates(map[string]interface{}{
		"kry_imei1":      strings.ToUpper(strings.TrimSpace(input.Imei1)),
		"kry_imei2":      gorm.Expr("NULLIF(?, '')", strings.ToUpper(strings.TrimSpace(input.Imei2))),
		"kry_simcardid1": strings.ToUpper(strings.TrimSpace(input.SimcardID1)),
		"kry_simcardid2": gorm.Expr("NULLIF(?, '')", strings.ToUpper(strings.TrimSpace(input.SimcardID2))),
		"kry_updateid":   userStr,
		"kry_updatetime": time.Now(),
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui device: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data device karyawan berhasil diperbarui!"})
}

// 3. RESET / HAPUS DATA DEVICE KARYAWAN (p-hrd_m_kry_device.asp)
func DeleteDeviceKaryawan(c *gin.Context) {
	database := resolveDeviceDB(c)
	nip := strings.TrimSpace(c.Param("nip"))

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)

	err := database.Table("public.hrd_m_karyawan").Where("TRIM(kry_nip) = TRIM(?)", nip).Updates(map[string]interface{}{
		"kry_imei1":      nil,
		"kry_imei2":      nil,
		"kry_simcardid1": nil,
		"kry_simcardid2": nil,
		"kry_updateid":   userStr,
		"kry_updatetime": time.Now(),
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus otorisasi device: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Otorisasi IMEI & Simcard karyawan berhasil dihapus!"})
}

// 4. PARSING & IMPORT AUTO LINK WHATSAPP (p-hrd_m_kry_device_a.asp)
func ImportDeviceViaWhatsApp(c *gin.Context) {
	database := resolveDeviceDB(c)
	var input struct {
		LinkRaw string `json:"link_raw"`
	}

	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.LinkRaw) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Link WhatsApp tidak boleh kosong"})
		return
	}

	linkTarget := strings.TrimSpace(input.LinkRaw)

	// Jika berupa URL penuh (misal https://.../register?d=BASE64), ambil parameter terakhirnya
	if idx := strings.LastIndex(linkTarget, "="); idx != -1 && strings.HasPrefix(strings.ToLower(linkTarget), "http") {
		linkTarget = strings.TrimSpace(linkTarget[idx+1:])
	}

	// Decode Base64 jika dikirim terenkripsi
	if !strings.Contains(linkTarget, ",") && len(linkTarget) > 12 {
		if decodedByte, err := base64.StdEncoding.DecodeString(linkTarget); err == nil {
			linkTarget = string(decodedByte)
		}
	}

	parts := strings.Split(linkTarget, ",")
	if len(parts) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format link WhatsApp tidak sesuai standar (Wajib: NIP,IMEI1,SIMCARD1[,IMEI2,SIMCARD2])"})
		return
	}

	nip := strings.TrimSpace(parts[0])
	imei1 := strings.ToUpper(strings.TrimSpace(parts[1]))
	simcard1 := strings.ToUpper(strings.TrimSpace(parts[2]))
	imei2 := ""
	simcard2 := ""
	if len(parts) >= 4 {
		imei2 = strings.ToUpper(strings.TrimSpace(parts[3]))
	}
	if len(parts) >= 5 {
		simcard2 = strings.ToUpper(strings.TrimSpace(parts[4]))
	}

	var exists int64
	database.Table("public.hrd_m_karyawan").Where("TRIM(kry_nip) = TRIM(?)", nip).Count(&exists)
	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "NIP Karyawan (" + nip + ") Tidak Terdaftar di Sistem!"})
		return
	}

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)

	err := database.Table("public.hrd_m_karyawan").Where("TRIM(kry_nip) = TRIM(?)", nip).Updates(map[string]interface{}{
		"kry_imei1":      imei1,
		"kry_simcardid1": simcard1,
		"kry_imei2":      gorm.Expr("NULLIF(?, '')", imei2),
		"kry_simcardid2": gorm.Expr("NULLIF(?, '')", simcard2),
		"kry_updateid":   userStr,
		"kry_updatetime": time.Now(),
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal inject data via WhatsApp: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Sukses! Device NIP %s Berhasil Terdaftar via WhatsApp Link", nip),
	})
}

// 5. UPLOAD CSV REGISTER HP KARYAWAN (hrd_m_kry_device_upload.asp)
func UploadCSVDeviceKaryawan(c *gin.Context) {
	database := resolveDeviceDB(c)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV wajib dipilih!"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membaca file CSV"})
		return
	}
	defer file.Close()

	// Deteksi delimiter otomatis (; atau ,)
	rawBytes, _ := io.ReadAll(file)
	content := strings.TrimPrefix(string(rawBytes), "\xef\xbb\xbf")
	reader := csv.NewReader(strings.NewReader(content))
	if strings.Contains(strings.Split(content, "\n")[0], ";") {
		reader.Comma = ';'
	}
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil || len(records) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Isi file CSV kosong atau format rusak"})
		return
	}

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)
	updatedCount := 0

	for idx, row := range records {
		if len(row) < 3 {
			continue
		}
		nip := strings.TrimSpace(row[0])
		if idx == 0 && (strings.EqualFold(nip, "nip") || strings.EqualFold(nip, "kry_nip")) {
			continue
		}
		if nip == "" {
			continue
		}

		imei1 := strings.ToUpper(strings.TrimSpace(row[1]))
		sim1 := strings.ToUpper(strings.TrimSpace(row[2]))
		imei2 := ""
		sim2 := ""
		if len(row) >= 4 {
			imei2 = strings.ToUpper(strings.TrimSpace(row[3]))
		}
		if len(row) >= 5 {
			sim2 = strings.ToUpper(strings.TrimSpace(row[4]))
		}

		res := database.Table("public.hrd_m_karyawan").Where("TRIM(kry_nip) = TRIM(?)", nip).Updates(map[string]interface{}{
			"kry_imei1":      gorm.Expr("NULLIF(?, '')", imei1),
			"kry_simcardid1": gorm.Expr("NULLIF(?, '')", sim1),
			"kry_imei2":      gorm.Expr("NULLIF(?, '')", imei2),
			"kry_simcardid2": gorm.Expr("NULLIF(?, '')", sim2),
			"kry_updateid":   userStr,
			"kry_updatetime": time.Now(),
		})
		if res.Error == nil && res.RowsAffected > 0 {
			updatedCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Import CSV selesai! %d data perangkat karyawan berhasil diperbarui.", updatedCount),
	})
}

// 6. GET RAW DATA ABSENSI GEO TRACKING (hrd_t_absensi_raw.asp)
func GetRawAbsensiList(c *gin.Context) {
	database := resolveDeviceDB(c)
	nip := strings.TrimSpace(c.Query("nip"))
	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	kdagen := strings.TrimSpace(c.Query("kdagen"))
	div := strings.TrimSpace(c.Query("div"))

	// var results []map[string]interface{}
	results := make([]map[string]interface{}, 0)

	query := database.Table("public.hrd_t_absensi a").
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
		Joins("LEFT JOIN public.hrd_m_karyawan k ON TRIM(a.abs_nip) = TRIM(k.kry_nip)").
		Joins("LEFT JOIN public.hrd_m_divisi d ON TRIM(k.kry_ddbid) = TRIM(d.div_code)").
		Joins("LEFT JOIN public.glb_m_agen g ON TRIM(a.abs_agenid) = TRIM(g.agen_id)").
		Where("COALESCE(a.abs_nip, '') <> ''")

	if tgla != "" && tgle != "" {
		query = query.Where("a.abs_datetime::date BETWEEN ? AND ?", tgla, tgle)
	} else {
		query = query.Where("a.abs_datetime::date = CURRENT_DATE")
	}

	if nip != "" {
		query = query.Where("TRIM(a.abs_nip) = ?", nip)
	}
	if kdagen != "" {
		query = query.Where("TRIM(a.abs_agenid) = ?", kdagen)
	}
	if div != "" {
		query = query.Where("TRIM(k.kry_ddbid) = ?", div)
	}

	err := query.Order("a.abs_nip ASC, a.abs_datetime DESC").Find(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal muat raw data absensi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}
