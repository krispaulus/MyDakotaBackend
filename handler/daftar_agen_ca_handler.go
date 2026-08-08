package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AgenCAModel Struct Tampilan Mapping COA Agen/Cabang
type AgenCAModel struct {
	AgenID           string `json:"agen_id" gorm:"column:agen_id"`
	AgenNama         string `json:"agen_nama" gorm:"column:agen_nama"`
	AgenAlamat       string `json:"agen_alamat" gorm:"column:agen_alamat"`
	AgenCCAID        string `json:"agenc_caid" gorm:"column:agenc_caid"`
	AgenCCAIDSetoran string `json:"agenc_caid_setoran" gorm:"column:agenc_caidsetoran"`
	AgenCItemID      string `json:"agenc_item_id" gorm:"column:agenc_itemid"`
}

// UpdateCAMappingReq DTO Request Update COA
type UpdateCAMappingReq struct {
	AgenID string `json:"agen_id" binding:"required"`
	Target string `json:"target" binding:"required"` // 'P' = Piutang, 'S' = Setoran, 'I' = Item
	Code   string `json:"code"`
}

func getAgenCADB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/agen-ca (READ LIST CABANG / AGEN FIX NULL FILTER)
// =========================================================================
func GetDaftarAgenCAHandler(c *gin.Context) {
	database := getAgenCADB(c)

	stt := c.DefaultQuery("stt", "2") // '2' = Cabang, '3' = Agen

	query := database.Table("public.glb_m_agen a").
		Select(`
			a.agen_id, 
			a.agen_nama, 
			COALESCE(a.agen_alamat, '-') AS agen_alamat, 
			COALESCE(ac.agenc_caid, '') AS agenc_caid, 
			COALESCE(ac.agenc_caidsetoran, '') AS agenc_caidsetoran, 
			COALESCE(ac.agenc_itemid, '') AS agenc_itemid
		`).
		Joins("LEFT JOIN public.glb_m_agenca ac ON TRIM(BOTH FROM CAST(a.agen_id AS VARCHAR)) = TRIM(BOTH FROM CAST(ac.agenc_id AS VARCHAR))").
		Where("COALESCE(a.agen_aktifyn, 'Y') = 'Y'").
		Where("a.agen_nama NOT LIKE '%XXX%'")

	// 🎯 HANDLE NULL / FILTER LONGGAR
	// Jika agen_stt di DB bernilai NULL, kita izinkan tetap muncul saat filter Cabang ('2')
	if stt == "2" {
		query = query.Where("(a.agen_stt = '2' OR a.agen_stt IS NULL OR a.agen_stt = '')")
	} else {
		query = query.Where("a.agen_stt = '3'")
	}

	// Jangan kunci agen_cabangid = '1' karena di DB nilainya [null]
	query = query.Order("CAST(a.agen_id AS VARCHAR) ASC")

	var list []AgenCAModel
	err := query.Scan(&list).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// =========================================================================
// 2. POST /api/gl/agen-ca/update (UPSERT MAPPING COA / ITEM)
// =========================================================================
func UpdateAgenCAMappingHandler(c *gin.Context) {
	database := getAgenCADB(c)
	userID, _ := c.Get("username")

	var req UpdateCAMappingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	req.AgenID = strings.TrimSpace(req.AgenID)
	req.Code = strings.TrimSpace(req.Code)
	req.Target = strings.ToUpper(strings.TrimSpace(req.Target))

	// Cek apakah record sudah ada di public.glb_m_agenca
	var count int64
	database.Table("public.glb_m_agenca").Where("agenc_id = ?", req.AgenID).Count(&count)

	fieldToUpdate := ""
	switch req.Target {
	case "P":
		fieldToUpdate = "agenc_caid"
	case "S":
		fieldToUpdate = "agenc_caidsetoran"
	case "I":
		fieldToUpdate = "agenc_itemid"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Target tidak valid! Gunakan P, S, atau I."})
		return
	}

	if count == 0 {
		// Insert Record Baru
		insertData := map[string]interface{}{
			"agenc_id":         req.AgenID,
			fieldToUpdate:      req.Code,
			"agenc_updateid":   fmt.Sprintf("%v", userID),
			"agenc_updatetime": time.Now(),
		}
		if err := database.Table("public.glb_m_agenca").Create(insertData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
	} else {
		// Update Record
		updateData := map[string]interface{}{
			fieldToUpdate:      req.Code,
			"agenc_updateid":   fmt.Sprintf("%v", userID),
			"agenc_updatetime": time.Now(),
		}
		if err := database.Table("public.glb_m_agenca").Where("agenc_id = ?", req.AgenID).Updates(updateData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Mapping Akun berhasil diperbarui!",
	})
}

// =========================================================================
// 3. DELETE /api/gl/agen-ca/:id (DELETE / RESET MAPPING COA)
// =========================================================================
func DeleteAgenCAMappingHandler(c *gin.Context) {
	agenID := c.Param("id")
	database := getAgenCADB(c)

	// Hapus mapping di public.glb_m_agenca berdasarkan agenc_id
	if err := database.Table("public.glb_m_agenca").Where("agenc_id = ?", agenID).Delete(map[string]interface{}{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mereset mapping: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Mapping COA untuk Cabang/Agen ID %s berhasil dihapus!", agenID),
	})
}
