package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type KaryawanListRow struct {
	KryNIP          string `json:"kry_nip" gorm:"column:kry_nip"`
	KryNama         string `json:"kry_nama" gorm:"column:kry_nama"`
	KryTelp1        string `json:"kry_telp1" gorm:"column:kry_telp1"`
	KryTelp2        string `json:"kry_telp2" gorm:"column:kry_telp2"`
	KryImei1        string `json:"kry_imei1" gorm:"column:kry_imei1"`
	KrySimcardID1   string `json:"kry_simcardid1" gorm:"column:kry_simcardid1"`
	KryActiveAgenID string `json:"kry_activeagenid" gorm:"column:kry_activeagenid"`
	AgenNama        string `json:"agen_nama" gorm:"column:agen_nama"`
	DivNama         string `json:"div_nama" gorm:"column:div_nama"`
	JabNama         string `json:"jab_nama" gorm:"column:jab_nama"`
	KryAktifYN      string `json:"kry_aktifyn" gorm:"column:kry_aktifyn"`
}

type KaryawanUpsertReq struct {
	KryNIP          string `json:"kry_nip" binding:"required"`
	KryNama         string `json:"kry_nama" binding:"required"`
	KryDDBID        string `json:"kry_ddbid"`
	KryJabCode      string `json:"kry_jabcode"`
	KryActiveAgenID string `json:"kry_activeagenid"`
	KryTelp1        string `json:"kry_telp1"`
	KryTelp2        string `json:"kry_telp2"`
	KryImei1        string `json:"kry_imei1"`
	KrySimcardID1   string `json:"kry_simcardid1"`
	KryNoID         string `json:"kry_noid"`
	KryAktifYN      string `json:"kry_aktifyn"`
}

// GET /hrd/karyawan
func GetKaryawanListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	search := strings.TrimSpace(c.Query("search"))
	agenID := strings.TrimSpace(c.Query("agen_id"))
	divCode := strings.TrimSpace(c.Query("div_code"))
	aktifYN := strings.TrimSpace(c.Query("aktif_yn"))

	query := database.Table("public.hrd_m_karyawan k").
		Select(`
			k.kry_nip,
			k.kry_nama,
			COALESCE(k.kry_telp1, '') AS kry_telp1,
			COALESCE(k.kry_telp2, '') AS kry_telp2,
			COALESCE(k.kry_imei1, '') AS kry_imei1,
			COALESCE(k.kry_simcardid1, '') AS kry_simcardid1,
			COALESCE(k.kry_activeagenid, '') AS kry_activeagenid,
			COALESCE(a.agen_nama, 'PUSAT') AS agen_nama,
			COALESCE(d.div_nama, '-') AS div_nama,
			COALESCE(j.jab_nama, '-') AS jab_nama,
			COALESCE(k.kry_aktifyn, 'Y') AS kry_aktifyn
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(k.kry_activeagenid::varchar)").
		Joins("LEFT JOIN public.hrd_m_divisi d ON TRIM(d.div_code::varchar) = TRIM(k.kry_ddbid::varchar)").
		Joins("LEFT JOIN public.hrd_m_jabatan j ON TRIM(j.jab_code::varchar) = TRIM(k.kry_jabcode::varchar)")

	if search != "" {
		s := "%" + search + "%"
		query = query.Where("(k.kry_nip ILIKE ? OR k.kry_nama ILIKE ? OR k.kry_telp1 ILIKE ?)", s, s, s)
	}

	if agenID != "" && agenID != "ALL" {
		query = query.Where("TRIM(k.kry_activeagenid::varchar) = TRIM(?::varchar)", agenID)
	}

	if divCode != "" && divCode != "ALL" {
		query = query.Where("TRIM(k.kry_ddbid::varchar) = TRIM(?::varchar)", divCode)
	}

	if aktifYN != "" && aktifYN != "ALL" {
		query = query.Where("COALESCE(k.kry_aktifyn, 'Y') = ?", aktifYN)
	}

	var results []KaryawanListRow
	if err := query.Order("k.kry_nama ASC").Limit(1000).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if results == nil {
		results = []KaryawanListRow{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// GET /hrd/karyawan/:nip
func GetKaryawanDetailHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	nip := strings.TrimSpace(c.Param("nip"))
	var row map[string]interface{}
	err := database.Table("public.hrd_m_karyawan k").
		Select(`
			k.*,
			COALESCE(a.agen_nama, '') AS agen_nama,
			COALESCE(d.div_nama, '') AS div_nama,
			COALESCE(j.jab_nama, '') AS jab_nama
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(k.kry_activeagenid::varchar)").
		Joins("LEFT JOIN public.hrd_m_divisi d ON TRIM(d.div_code::varchar) = TRIM(k.kry_ddbid::varchar)").
		Joins("LEFT JOIN public.hrd_m_jabatan j ON TRIM(j.jab_code::varchar) = TRIM(k.kry_jabcode::varchar)").
		Where("TRIM(k.kry_nip::varchar) = TRIM(?)", nip).
		Take(&row).Error

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data karyawan tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": row})
}

// POST /hrd/karyawan/save
func SaveKaryawanHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	username, _ := c.Get("username")
	var req KaryawanUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	nip := strings.TrimSpace(req.KryNIP)
	var count int64
	database.Table("public.hrd_m_karyawan").Where("TRIM(kry_nip::varchar) = TRIM(?)", nip).Count(&count)

	dataMap := map[string]interface{}{
		"kry_nama":         req.KryNama,
		"kry_ddbid":        req.KryDDBID,
		"kry_jabcode":      req.KryJabCode,
		"kry_activeagenid": req.KryActiveAgenID,
		"kry_telp1":        req.KryTelp1,
		"kry_telp2":        req.KryTelp2,
		"kry_imei1":        req.KryImei1,
		"kry_simcardid1":   req.KrySimcardID1,
		"kry_noid":         req.KryNoID,
		"kry_aktifyn":      req.KryAktifYN,
		"kry_updateid":     fmt.Sprintf("%v", username),
		"kry_updatetime":   time.Now(),
	}

	if count > 0 {
		if err := database.Table("public.hrd_m_karyawan").Where("TRIM(kry_nip::varchar) = TRIM(?)", nip).Updates(dataMap).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update: " + err.Error()})
			return
		}
	} else {
		dataMap["kry_nip"] = nip
		dataMap["kry_tglmasuk"] = time.Now()
		if err := database.Table("public.hrd_m_karyawan").Create(&dataMap).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Karyawan %s berhasil disimpan", nip)})
}

// DELETE /hrd/karyawan/:nip (Soft Delete)
func DeleteKaryawanHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	nip := strings.TrimSpace(c.Param("nip"))
	username, _ := c.Get("username")

	if err := database.Table("public.hrd_m_karyawan").
		Where("TRIM(kry_nip::varchar) = TRIM(?)", nip).
		Updates(map[string]interface{}{
			"kry_aktifyn":    "N",
			"kry_updateid":   fmt.Sprintf("%v", username),
			"kry_updatetime": time.Now(),
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menonaktifkan karyawan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Karyawan %s berhasil dinonaktifkan", nip)})
}

// GET /hrd/divisi-options
func GetDivisiOptionsHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var list []map[string]interface{}
	database.Table("public.hrd_m_divisi").
		Select("div_code, div_nama").
		Where("COALESCE(div_aktifyn, 'Y') = 'Y'").
		Order("div_nama ASC").
		Scan(&list)

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// GET /hrd/jabatan-options
func GetJabatanOptionsHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var list []map[string]interface{}
	database.Table("public.hrd_m_jabatan").
		Select("jab_code, jab_nama").
		Where("COALESCE(jab_aktifyn, 'Y') = 'Y'").
		Order("jab_nama ASC").
		Scan(&list)

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}
