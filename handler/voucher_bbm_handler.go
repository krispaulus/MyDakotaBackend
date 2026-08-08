package handler

import (
	"fmt"
	"net/http"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// GetVoucherBBM Handler menyajikan data list Voucher BBM (Versi Standard Huruf Kecil)
func GetVoucherBBM(c *gin.Context) {
	tanggalStart := c.Query("tanggalStart")
	tanggalEnd := c.Query("tanggalEnd")
	agenID := c.Query("agenID")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.GetDB()
	}

	// 🔍 Query SQL Murni Huruf Kecil murni tanpa Double Quotes ("")
	query := `
		SELECT 
			v.vb_id AS no_voucher,
			COALESCE(g.agen_nama, 'PUSAT DAKOTA') AS cabang,
			CASE 
				WHEN v.vb_tanggal IS NULL THEN '-'
				ELSE TO_CHAR(v.vb_tanggal::timestamp, 'MM/DD/YYYY') 
			END AS tanggal,
			COALESCE(v.vb_nomobil, sj.sjh_kendid, '-') AS no_kend,
			COALESCE(v.vb_sopir, k1.kry_nama, '-') AS driver,
			COALESCE(v.vb_keterangan, sj.sjh_keterangan, '-') AS keterangan,
			COALESCE(v.vb_sjid, '-') AS no_st,
			REPLACE(REPLACE(COALESCE(b.bplk_nama, 'SOLAR'), 'BBM (', ''), ')', '') AS jns_bbm,
			COALESCE(v.vb_liter, 0) AS liter,
			COALESCE(v.vb_harga, 0) AS harga,
			CASE 
				WHEN COALESCE(v.vb_aktifyn, 'Y') = 'Y' THEN 'Ya' 
				ELSE 'Tidak' 
			END AS aktif
		FROM public.opr_t_voucherbbm v
		LEFT JOIN public.glb_m_agen g ON CAST(v.vb_agenid AS VARCHAR) = CAST(g.agen_id AS VARCHAR)
		LEFT JOIN public.opr_m_bplk b ON CAST(v.vb_bplkid AS VARCHAR) = CAST(b.bplk_id AS VARCHAR)
		LEFT JOIN public.opr_t_esuratjalan sj ON REGEXP_REPLACE(UPPER(v.vb_sjid), '\s+', '', 'g') = REGEXP_REPLACE(UPPER(sj.sjh_id), '\s+', '', 'g')
		LEFT JOIN public.hrd_m_karyawan k1 ON TRIM(CAST(sj.sjh_sopir1_nip AS VARCHAR)) = TRIM(CAST(k1.kry_nip AS VARCHAR))
		WHERE COALESCE(v.vb_id, '') != ''
	`

	if tanggalStart != "" && tanggalEnd != "" {
		query += fmt.Sprintf(" AND v.vb_tanggal BETWEEN '%s 00:00:00' AND '%s 23:59:59'", tanggalStart, tanggalEnd)
	}

	if agenID != "" && agenID != "PUSAT DAKOTA" && agenID != "HOLDING" && agenID != "1" && agenID != "PUSAT DAKOTA (HOLDING)" {
		query += fmt.Sprintf(" AND CAST(v.vb_agenid AS VARCHAR) = '%s'", agenID)
	}

	query += " ORDER BY v.vb_tanggal DESC, v.vb_id ASC LIMIT 200"

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
