package handler

import (
	"fmt"
	"net/http"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// GetVoucherBBM Handler untuk menyajikan data list Voucher BBM
func GetVoucherBBM(c *gin.Context) {
	tanggalStart := c.Query("tanggalStart")
	tanggalEnd := c.Query("tanggalEnd")
	agenID := c.Query("agenID")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.GetDB()
	}

	// 🔍 Query SQL Utama Tahan Banting (Menggunakan nama kolom bawaan tabel VoucherBBM)
	query := `
		SELECT 
			v."VB_ID" AS no_voucher,
			COALESCE(g.agen_nama, 'PUSAT DAKOTA') AS cabang,
			CASE 
				WHEN v."VB_Tanggal" IS NULL THEN '-'
				ELSE TO_CHAR(v."VB_Tanggal"::timestamp, 'MM/DD/YYYY') 
			END AS tanggal,
			COALESCE(v."VB_NoMobil", '-') AS no_kend,
			COALESCE(v."VB_Sopir", '-') AS driver,
			COALESCE(v."VB_Keterangan", '-') AS keterangan,
			COALESCE(v."VB_SJID", '-') AS no_st,
			'SOLAR' AS jns_bbm,
			COALESCE(v."VB_Liter", 0) AS liter,
			COALESCE(v."VB_Harga", 0) AS harga,
			CASE 
				WHEN COALESCE(v."VB_AktifYN", 'Y') = 'Y' THEN 'Ya' 
				ELSE 'Tidak' 
			END AS aktif
		FROM public."OPR_T_VoucherBBM" v
		LEFT JOIN public.glb_m_agen g ON CAST(v."VB_AgenID" AS VARCHAR) = CAST(g.agen_id AS VARCHAR)
		WHERE COALESCE(v."VB_ID", '') != ''
	`

	if tanggalStart != "" && tanggalEnd != "" {
		query += fmt.Sprintf(" AND v.\"VB_Tanggal\" BETWEEN '%s 00:00:00' AND '%s 23:59:59'", tanggalStart, tanggalEnd)
	}

	if agenID != "" && agenID != "PUSAT DAKOTA" && agenID != "HOLDING" && agenID != "1" && agenID != "PUSAT DAKOTA (HOLDING)" {
		query += fmt.Sprintf(" AND CAST(v.\"VB_AgenID\" AS VARCHAR) = '%s'", agenID)
	}

	query += " ORDER BY v.\"VB_Tanggal\" DESC, v.\"VB_ID\" ASC LIMIT 200"

	var results []map[string]interface{}
	if err := database.Raw(query).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}
