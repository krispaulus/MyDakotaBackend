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

// GET /mkt/dalam-kota/list
func GetMasterDalamKota(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	search := strings.TrimSpace(c.Query("search"))
	agenIDStr := strings.TrimSpace(c.Query("agen_id"))

	query := database.Table("public.mkt_m_areakota a").
		Select(`
			a.area_id, 
			a.agen_id, 
			COALESCE(ag.agen_nama, 'CABANG: ' || a.agen_id::text) as agen_nama, 
			COALESCE(a.cust_id, '') as cust_id, 
			COALESCE(c.cust_name, 'SEMUA CUSTOMER (GLOBAL)') as cust_name, 
			a.kotakabupaten
		`).
		Joins("LEFT JOIN public.glb_m_agen ag ON ag.agen_id::text = a.agen_id::text").
		Joins("LEFT JOIN public.mkt_m_customer c ON c.cust_id::text = a.cust_id::text")

	if agenIDStr != "" && agenIDStr != "ALL" {
		query = query.Where("a.agen_id::text = ?", agenIDStr)
	}

	if search != "" {
		s := "%" + search + "%"
		query = query.Where("(a.kotakabupaten ILIKE ? OR ag.agen_nama ILIKE ? OR c.cust_name ILIKE ?)", s, s, s)
	}

	var results []AreaKotaRow
	if err := query.Order("a.area_id DESC").Limit(500).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if results == nil {
		results = []AreaKotaRow{}
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
