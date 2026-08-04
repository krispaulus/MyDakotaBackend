package handler

import (
	"fmt"
	"net/http"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

type SPPADHeaderDTO struct {
	NoSPPAD            string `json:"no_sp_pad" gorm:"column:no_sp_pad"`
	TanggalSP          string `json:"tanggal_sp" gorm:"column:tanggal_sp"`
	CabangAsal         string `json:"cabang_asal" gorm:"column:cabang_asal"`
	VendorExpedisiLain string `json:"vendor_expedisi_lain" gorm:"column:vendor_expedisi_lain"`
	StatusAktif        string `json:"status_aktif" gorm:"column:status_aktif"`
}

// GetListSPPAD Handler untuk menarik data histori SP PAD
func GetListSPPAD(c *gin.Context) {
	noSP := c.Query("noSP")
	noBTT := c.Query("noBTT")
	ekspedisiLain := c.Query("ekspedisiLain")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.GetDB()
	}

	query := `
		SELECT 
			TRIM(p.spp_eid) AS no_sp_pad,
			TO_CHAR(p.spp_tanggal, 'DD/MM/YYYY') AS tanggal_sp,
			COALESCE(a.agen_nama, '-') AS cabang_asal,
			COALESCE(p.spp_expedisilain, '-') AS vendor_expedisi_lain,
			CASE WHEN p.spp_aktifyn = 'Y' THEN 'Aktif' ELSE 'Non-Aktif' END AS status_aktif
		FROM public.opr_t_esp_pad p
		LEFT JOIN public.glb_m_agen a ON CAST(p.spp_asalagenid AS VARCHAR) = CAST(a.agen_id AS VARCHAR)
		WHERE p.spp_eid IS NOT NULL
	`

	if noSP != "" {
		query += fmt.Sprintf(" AND TRIM(p.spp_eid) ILIKE '%%%s%%'", noSP)
	}

	if noBTT != "" {
		query += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM public.opr_t_esp_paddetil pd 
			WHERE TRIM(pd.sppd_esptid) = TRIM(p.spp_eid) 
			AND TRIM(pd.sppd_bttid) ILIKE '%%%s%%'
		)`, noBTT)
	}

	if ekspedisiLain != "" {
		query += fmt.Sprintf(" AND TRIM(p.spp_expedisilain) ILIKE '%%%s%%'", ekspedisiLain)
	}

	query += " ORDER BY p.spp_tanggal DESC, p.spp_eid DESC LIMIT 100"

	var results []SPPADHeaderDTO
	if err := database.Raw(query).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}
