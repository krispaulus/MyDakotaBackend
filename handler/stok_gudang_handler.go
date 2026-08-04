package handler

import (
	"net/http"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// GetStokBarangGudang Handler untuk menyajikan stok barang fisik di gudang
func GetStokBarangGudang(c *gin.Context) {
	noBTT := c.Query("noBTT")
	transitYN := c.Query("transitYN")

	// Gunakan DB utama
	database := db.GetDB()

	// 🔍 Query SQL 100% Murni Persis dari pgAdmin
	query := `
		SELECT 
			t.sp_agenid,
			COALESCE(CAST(c.bttt_tujuanagenid AS VARCHAR), '0') AS bttt_tujuanagenid,
			t.sp_bttid AS no_btt,
			CASE 
				WHEN t.sp_tanggal IS NULL THEN '-'
				ELSE TO_CHAR(t.sp_tanggal::timestamp, 'MM/DD/YYYY HH12:MI:SS AM') 
			END AS tgl_turun,
			COALESCE(a.agen_nama, 'DLI SURABAYA') AS asal_agen,
			COALESCE(c.bttt_jmlunit, 1) AS colly,
			COALESCE(c.bttt_berat, 0) AS berat_kg,
			COALESCE(c.bttt_beratvol, 0) AS volume_kg,
			CASE 
				WHEN TRIM(COALESCE(CAST(c.bttt_pembayaran AS VARCHAR), '0')) != '3' 
				THEN COALESCE(c.bttt_harga, 0) + COALESCE(c.bttt_biayapenerus, 0)
				ELSE 0 
			END AS biaya_tunai,
			CASE 
				WHEN TRIM(COALESCE(CAST(c.bttt_pembayaran AS VARCHAR), '0')) = '3' 
				THEN COALESCE(c.bttt_harga, 0) + COALESCE(c.bttt_biayapenerus, 0)
				ELSE 0 
			END AS biaya_tagih_cod,
			CASE 
				WHEN CAST(COALESCE(CAST(c.bttt_tujuanagenid AS VARCHAR), '0') AS VARCHAR) != CAST(t.sp_agenid AS VARCHAR) THEN 'YA' 
				ELSE '' 
			END AS is_transit,
			COALESCE(t.sp_keterangan, '-') AS keterangan
		FROM public.opr_t_esp_turun t
		LEFT JOIN public.mkt_t_econote c ON REGEXP_REPLACE(UPPER(t.sp_bttid), '\s+', '', 'g') = REGEXP_REPLACE(UPPER(c.bttt_id), '\s+', '', 'g')
		LEFT JOIN public.glb_m_agen a ON CAST(c.bttt_asalagenid AS VARCHAR) = CAST(a.agen_id AS VARCHAR)
		WHERE COALESCE(t.sp_kirimyn, 'N') = 'N'
		  AND COALESCE(t.sp_returbarangyn, 'N') = 'N'
		  AND NOT EXISTS (SELECT 1 FROM public.opr_t_esp_terimadetil WHERE REGEXP_REPLACE(UPPER(sptd_bttid), '\s+', '', 'g') = REGEXP_REPLACE(UPPER(t.sp_bttid), '\s+', '', 'g'))
		  AND NOT EXISTS (SELECT 1 FROM public.opr_t_ereturbttdetil WHERE REGEXP_REPLACE(UPPER(rbd_bttid), '\s+', '', 'g') = REGEXP_REPLACE(UPPER(t.sp_bttid), '\s+', '', 'g'))
		  AND NOT EXISTS (SELECT 1 FROM public.opr_t_esp_paddetil WHERE REGEXP_REPLACE(UPPER(sppd_bttid), '\s+', '', 'g') = REGEXP_REPLACE(UPPER(t.sp_bttid), '\s+', '', 'g'))
		  AND NOT EXISTS (SELECT 1 FROM public.opr_t_ebbl WHERE REGEXP_REPLACE(UPPER(bbl_bttid), '\s+', '', 'g') = REGEXP_REPLACE(UPPER(t.sp_bttid), '\s+', '', 'g') AND bbl_terimayn = 'Y')
		  AND NOT EXISTS (SELECT 1 FROM public.opr_t_eambil WHERE REGEXP_REPLACE(UPPER(ambil_bttid), '\s+', '', 'g') = REGEXP_REPLACE(UPPER(t.sp_bttid), '\s+', '', 'g') AND ambil_aktifyn = 'Y')
	`

	if noBTT != "" {
		query += " AND REGEXP_REPLACE(UPPER(t.sp_bttid), '\\s+', '', 'g') ILIKE '%" + noBTT + "%'"
	}

	if transitYN == "Y" {
		query += " AND CAST(COALESCE(CAST(c.bttt_tujuanagenid AS VARCHAR), '0') AS VARCHAR) != CAST(t.sp_agenid AS VARCHAR)"
	} else if transitYN == "N" {
		query += " AND CAST(COALESCE(CAST(c.bttt_tujuanagenid AS VARCHAR), '0') AS VARCHAR) = CAST(t.sp_agenid AS VARCHAR)"
	}

	query += " ORDER BY t.sp_tanggal DESC, t.sp_bttid ASC LIMIT 200"

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
