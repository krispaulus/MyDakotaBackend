package handler

import (
	"fmt"
	"net/http"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// GetJurnalTidakSeimbang Handler untuk menyajikan daftar jurnal tidak seimbang (Debet != Kredit)
func GetJurnalTidakSeimbang(c *gin.Context) {
	tanggalStart := c.Query("tanggalStart")
	tanggalEnd := c.Query("tanggalEnd")
	cabang := c.Query("cabang")
	noJurnal := c.Query("noJurnal")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.GetDB()
	}

	// 🔍 Query SQL Murni ke Tabel Huruf Kecil (Clean tanpa Double Quotes)
	query := `
		SELECT 
			h.tjurh_no AS no_jurnal,
			CASE 
				WHEN h.tjurh_tanggal IS NULL THEN '-'
				ELSE TO_CHAR(h.tjurh_tanggal::timestamp, 'MM/DD/YYYY') 
			END AS tanggal,
			COALESCE(h.tjurh_type, '-') AS tipe,
			COALESCE(h.tjurh_keterangan, '-') AS keterangan,
			CASE WHEN h.tjurh_deleteyn = 'Y' THEN 'BATAL' ELSE '' END AS status,
			CASE WHEN h.tjurh_postingyn = 'Y' THEN 'POSTING' ELSE '' END AS posting,
			COALESCE(SUM(d.tjurd_debet), 0) AS debet,
			COALESCE(SUM(d.tjurd_kredit), 0) AS kredit,
			ABS(COALESCE(SUM(d.tjurd_debet), 0) - COALESCE(SUM(d.tjurd_kredit), 0)) AS selisih
		FROM public.gl_t_jurnalh h
		LEFT JOIN public.gl_t_jurnald d ON h.tjurh_no::text = d.tjurd_tjurhno::text
		LEFT JOIN public.glb_m_agen g ON CAST(d.tjurd_agenid AS VARCHAR) = CAST(g.agen_id AS VARCHAR)
		WHERE COALESCE(h.tjurh_no, '') != ''
	`

	if tanggalStart != "" && tanggalEnd != "" {
		query += fmt.Sprintf(" AND h.tjurh_tanggal BETWEEN '%s 00:00:00' AND '%s 23:59:59'", tanggalStart, tanggalEnd)
	}

	if cabang != "" && cabang != "PUSAT DAKOTA" && cabang != "HOLDING" && cabang != "PUSAT DAKOTA (HOLDING)" {
		query += fmt.Sprintf(" AND g.agen_nama = '%s'", cabang)
	}

	if noJurnal != "" {
		query += fmt.Sprintf(" AND h.tjurh_no ILIKE '%%%s%%'", noJurnal)
	}

	query += `
		GROUP BY h.tjurh_no, h.tjurh_tanggal, h.tjurh_type, h.tjurh_keterangan, h.tjurh_deleteyn, h.tjurh_postingyn
		HAVING (COALESCE(SUM(d.tjurd_debet), 0) - COALESCE(SUM(d.tjurd_kredit), 0)) != 0
		ORDER BY h.tjurh_tanggal DESC, h.tjurh_no ASC LIMIT 200
	`

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
