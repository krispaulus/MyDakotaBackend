package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type AreaKotaRow struct {
	AreaID        int64  `json:"area_id" gorm:"column:area_id"`
	AgenID        string `json:"agen_id" gorm:"column:agen_id"`
	AgenNama      string `json:"agen_nama" gorm:"column:agen_nama"`
	CustID        string `json:"cust_id" gorm:"column:cust_id"`
	CustName      string `json:"cust_name" gorm:"column:cust_name"`
	KotaKabupaten string `json:"kotakabupaten" gorm:"column:kotakabupaten"`
}

type SaveMasterDalamKotaPayload struct {
	AgenID   string   `json:"agen_id" binding:"required"`
	CustID   string   `json:"cust_id"`
	KotaList []string `json:"kota_list" binding:"required"`
}

type CustomerDalamKotaRow struct {
	CustID        string `json:"cust_id" gorm:"column:cust_id"`
	CustName      string `json:"cust_name" gorm:"column:cust_name"`
	Provinsi      string `json:"provinsi" gorm:"column:provinsi"`
	KotaKabupaten string `json:"kotakabupaten" gorm:"column:kotakabupaten"`
	Kecamatan     string `json:"kecamatan" gorm:"column:kecamatan"`
	Kelurahan     string `json:"kelurahan" gorm:"column:kelurahan"`
	CustAktifYN   string `json:"cust_aktifyn" gorm:"column:cust_aktifyn"`
}

// GET /mkt/dalam-kota/list
func GetMasterDalamKota(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	custID := strings.TrimSpace(c.Query("cust_id"))
	custName := strings.TrimSpace(c.Query("cust_name"))
	kota := strings.TrimSpace(c.Query("kota"))
	provinsi := strings.TrimSpace(c.Query("provinsi"))
	kecamatan := strings.TrimSpace(c.Query("kecamatan"))
	kelurahan := strings.TrimSpace(c.Query("kelurahan"))
	aktifYN := strings.TrimSpace(c.Query("aktif_yn"))

	query := database.Table("public.mkt_m_customer c").
		Select(`
			c.cust_id,
			c.cust_name,
			COALESCE(c.cust_alamat1, '') AS provinsi,
			COALESCE(c.cust_kotaid, '') AS kotakabupaten,
			COALESCE(c.cust_alamat2, '') AS kecamatan,
			COALESCE(c.cust_alamat2, '') AS kelurahan,
			COALESCE(c.cust_aktifyn, 'Y') AS cust_aktifyn
		`).
		Where("c.cust_id IS NOT NULL")

	if custID != "" {
		query = query.Where("c.cust_id ILIKE ?", "%"+custID+"%")
	}
	if custName != "" {
		query = query.Where("c.cust_name ILIKE ?", "%"+custName+"%")
	}
	if kota != "" {
		query = query.Where("c.cust_kotaid ILIKE ?", "%"+kota+"%")
	}
	if provinsi != "" {
		query = query.Where("c.cust_alamat1 ILIKE ?", "%"+provinsi+"%")
	}
	if kecamatan != "" || kelurahan != "" {
		pat := "%" + kecamatan + "%"
		if kelurahan != "" {
			pat = "%" + kelurahan + "%"
		}
		query = query.Where("c.cust_alamat2 ILIKE ?", pat)
	}
	if aktifYN != "" {
		query = query.Where("c.cust_aktifyn = ?", aktifYN)
	}

	var results []CustomerDalamKotaRow
	if err := query.Order("c.cust_id ASC").Limit(500).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if results == nil {
		results = []CustomerDalamKotaRow{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// GET /mkt/dalam-kota/suggest?type=agen|customer|kota&q=...
func GetMasterDalamKotaSuggest(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	suggestType := c.Query("type")
	q := strings.TrimSpace(c.Query("q"))
	pattern := "%" + q + "%"

	switch suggestType {
	case "agen":
		type AgenItem struct {
			AgenID   string `json:"agen_id" gorm:"column:agen_id"`
			AgenNama string `json:"agen_nama" gorm:"column:agen_nama"`
		}
		var list []AgenItem
		database.Table("public.glb_m_agen").
			Select("DISTINCT agen_id, agen_nama").
			Where("agen_nama ILIKE ? OR agen_id ILIKE ?", pattern, pattern).
			Order("agen_nama ASC").
			Limit(20).
			Scan(&list)
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})

	case "customer":
		type CustItem struct {
			CustID   string `json:"cust_id" gorm:"column:cust_id"`
			CustName string `json:"cust_name" gorm:"column:cust_name"`
		}
		var list []CustItem
		database.Table("public.mkt_m_customer").
			Select("DISTINCT cust_id, cust_name").
			Where("cust_name ILIKE ? OR cust_id ILIKE ?", pattern, pattern).
			Order("cust_name ASC").
			Limit(20).
			Scan(&list)
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})

	case "kota":
		var list []string
		database.Table("public.glb_m_agen").
			Select("DISTINCT UPPER(TRIM(agen_kota))").
			Where("agen_kota IS NOT NULL AND agen_kota <> '' AND agen_kota ILIKE ?", pattern).
			Order("UPPER(TRIM(agen_kota)) ASC").
			Limit(30).
			Pluck("UPPER(TRIM(agen_kota))", &list)
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})

	default:
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Tipe suggest tidak dikenali"})
	}
}

// POST /mkt/dalam-kota/save
func SaveMasterDalamKota(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var payload SaveMasterDalamKotaPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Data formulir tidak lengkap"})
		return
	}

	if len(payload.KotaList) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Minimal pilih satu kota"})
		return
	}

	agenID := strings.TrimSpace(payload.AgenID)
	custID := strings.TrimSpace(payload.CustID)
	sukses := 0

	for _, kota := range payload.KotaList {
		k := strings.TrimSpace(strings.ToUpper(kota))
		if k == "" {
			continue
		}

		sql := `INSERT INTO public.mkt_m_areakota (agen_id, cust_id, kotakabupaten, created_at, updated_at) 
		        VALUES (?, ?, ?, NOW(), NOW()) 
		        ON CONFLICT (agen_id, cust_id, kotakabupaten) DO NOTHING`

		res := database.Exec(sql, agenID, custID, k)
		if res.Error == nil && res.RowsAffected > 0 {
			sukses++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Berhasil menyimpan %d kota baru ke master dalam kota.", sukses),
	})
}

// DELETE /mkt/dalam-kota/delete/:id
func DeleteMasterDalamKota(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	id := c.Param("id")
	if err := database.Table("public.mkt_m_areakota").Where("area_id = ?", id).Delete(nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus kota"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data kota berhasil dihapus"})
}
