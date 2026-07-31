package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// GetBTTBelumKembaliDetail Handler untuk Detail Laporan BTT Belum Kembali
func GetBTTBelumKembaliDetail(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	ckpic := strings.TrimSpace(c.Query("ckpic"))
	pic := strings.TrimSpace(c.Query("pic"))
	ckagen := strings.TrimSpace(c.Query("ckagen"))
	agen := strings.TrimSpace(c.Query("agen"))
	ckinv := strings.TrimSpace(c.Query("ckinv"))
	inv := strings.TrimSpace(c.Query("inv"))

	if tgla == "" || tgle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Periode Tanggal wajib diisi"})
		return
	}

	filterClause := ""
	if (ckagen == "true" || ckagen == "on") && agen != "" {
		if agen == "1" {
			filterClause += ` AND (e.bttt_tujuanagenid = '1' OR e.bttt_tujuanagenid = '0') `
		} else {
			filterClause += fmt.Sprintf(` AND e.bttt_tujuanagenid = '%s' `, agen)
		}
	}

	if (ckinv == "true" || ckinv == "on") && inv != "" {
		if strings.ToUpper(inv) == "Y" {
			filterClause += ` AND invd.artid_artihid IS NOT NULL `
		} else {
			filterClause += ` AND invd.artid_artihid IS NULL `
		}
	}

	if (ckpic == "true" || ckpic == "on") && pic != "" {
		filterClause += fmt.Sprintf(` AND cust.cust_pic = '%s' `, pic)
	}

	// Query PostgreSQL dengan perhitungan Umur Hari (CURRENT_DATE - CAST(bttt_tanggal AS DATE))
	queryStr := fmt.Sprintf(`
		SELECT 
			TO_CHAR(e.bttt_tanggal, 'YYYY-MM-DD') AS bttt_tanggal,
			e.bttt_id,
			COALESCE(e.bttt_nosuratjalan, '') AS bttt_nosuratjalan,
			COALESCE(cust.cust_name, '') AS cust_name,
			COALESCE(e.bttt_harga, 0) + COALESCE(e.bttt_biayapenerus, 0) + COALESCE(pck.pck_biaya, 0) AS jharga,
			COALESCE(e.bttt_jmlunit, 0) AS bttt_jml_unit,
			CASE WHEN COALESCE(e.bttt_berat, 0) >= COALESCE(e.bttt_beratvol, 0) THEN COALESCE(e.bttt_berat, 0) ELSE COALESCE(e.bttt_beratvol, 0) END AS jberat,
			COALESCE(e.bttt_tujuannama, '') AS bttt_tujuan_nama,
			COALESCE(e.bttt_tujuankota, '') AS bttt_tujuan_kota,
			COALESCE(a.agen_nama, '') AS agen_nama,
			COALESCE(cust.cust_pic, '') AS cust_pic,
			COALESCE(invd.artid_artihid, '') AS invoice_no,
			(CURRENT_DATE - CAST(e.bttt_tanggal AS DATE)) AS umur
		FROM public.mkt_t_econote e
		LEFT JOIN public.glb_m_agen a ON (CASE WHEN e.bttt_tujuanagenid = '0' THEN '1' ELSE e.bttt_tujuanagenid END) = CAST(a.agen_id AS VARCHAR)
		LEFT JOIN public.opr_t_eloperdetail loper ON TRIM(UPPER(e.bttt_id)) = TRIM(UPPER(loper.loperd_bttid))
		LEFT JOIN public.mkt_t_eterimabtt tb ON TRIM(UPPER(e.bttt_id)) = TRIM(UPPER(tb.tb_bttid))
		LEFT JOIN public.opr_t_esp_paddetil pad ON TRIM(UPPER(e.bttt_id)) = TRIM(UPPER(pad.sppd_bttid))
		LEFT JOIN public.opr_t_ebbl bbl ON TRIM(UPPER(e.bttt_id)) = TRIM(UPPER(bbl.bbl_bttid))
		LEFT JOIN public.art_t_invoiced invd ON TRIM(UPPER(e.bttt_id)) = TRIM(UPPER(invd.artid_bttid))
		LEFT JOIN public.pck_t_packing pck ON e.bttt_packingid = pck.pck_id
		LEFT JOIN public.mkt_m_customer cust ON e.bttt_asalcustid = cust.cust_id
		WHERE e.bttt_aktifyn = 'Y'
		  AND CAST(e.bttt_tanggal AS DATE) BETWEEN ? AND ?
		  AND (
			(loper.loperd_bttid IS NOT NULL AND bbl.bbl_bttid IS NULL)
			OR
			(pad.sppd_bttid IS NOT NULL AND tb.tb_tglkembali IS NULL)
		  ) %s
		ORDER BY e.bttt_tanggal, cust.cust_name
	`, filterClause)

	var list []models.BTTBelumKembaliDetailDTO
	if err := database.Raw(queryStr, tgla, tgle).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query detail BTT belum kembali: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
		"count":  len(list),
	})
}

// GetBTTBelumKembaliRekap Handler untuk Rekap By Cabang / PIC
func GetBTTBelumKembaliRekap(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	groupBy := strings.ToLower(strings.TrimSpace(c.Query("group_by"))) // 'cabang' | 'pic'

	ckpic := strings.TrimSpace(c.Query("ckpic"))
	pic := strings.TrimSpace(c.Query("pic"))
	ckagen := strings.TrimSpace(c.Query("ckagen"))
	agen := strings.TrimSpace(c.Query("agen"))
	ckinv := strings.TrimSpace(c.Query("ckinv"))
	inv := strings.TrimSpace(c.Query("inv"))

	filterClause := ""
	if (ckagen == "true" || ckagen == "on") && agen != "" {
		if agen == "1" {
			filterClause += ` AND (e.bttt_tujuanagenid = '1' OR e.bttt_tujuanagenid = '0') `
		} else {
			filterClause += fmt.Sprintf(` AND e.bttt_tujuanagenid = '%s' `, agen)
		}
	}

	if (ckinv == "true" || ckinv == "on") && inv != "" {
		if strings.ToUpper(inv) == "Y" {
			filterClause += ` AND invd.artid_artihid IS NOT NULL `
		} else {
			filterClause += ` AND invd.artid_artihid IS NULL `
		}
	}

	if (ckpic == "true" || ckpic == "on") && pic != "" {
		filterClause += fmt.Sprintf(` AND cust.cust_pic = '%s' `, pic)
	}

	groupField := "a.agen_nama"
	if groupBy == "pic" {
		groupField = "COALESCE(cust.cust_pic, 'LAINNYA')"
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			%s AS group_key,
			COUNT(DISTINCT e.bttt_id) AS jml_btt,
			SUM(COALESCE(e.bttt_harga, 0) + COALESCE(e.bttt_biayapenerus, 0) + COALESCE(pck.pck_biaya, 0)) AS jharga
		FROM public.mkt_t_econote e
		LEFT JOIN public.glb_m_agen a ON (CASE WHEN e.bttt_tujuanagenid = '0' THEN '1' ELSE e.bttt_tujuanagenid END) = CAST(a.agen_id AS VARCHAR)
		LEFT JOIN public.opr_t_eloperdetail loper ON TRIM(UPPER(e.bttt_id)) = TRIM(UPPER(loper.loperd_bttid))
		LEFT JOIN public.mkt_t_eterimabtt tb ON TRIM(UPPER(e.bttt_id)) = TRIM(UPPER(tb.tb_bttid))
		LEFT JOIN public.opr_t_esp_paddetil pad ON TRIM(UPPER(e.bttt_id)) = TRIM(UPPER(pad.sppd_bttid))
		LEFT JOIN public.opr_t_ebbl bbl ON TRIM(UPPER(e.bttt_id)) = TRIM(UPPER(bbl.bbl_bttid))
		LEFT JOIN public.art_t_invoiced invd ON TRIM(UPPER(e.bttt_id)) = TRIM(UPPER(invd.artid_bttid))
		LEFT JOIN public.pck_t_packing pck ON e.bttt_packingid = pck.pck_id
		LEFT JOIN public.mkt_m_customer cust ON e.bttt_asalcustid = cust.cust_id
		WHERE e.bttt_aktifyn = 'Y'
		  AND CAST(e.bttt_tanggal AS DATE) BETWEEN ? AND ?
		  AND (
			(loper.loperd_bttid IS NOT NULL AND bbl.bbl_bttid IS NULL)
			OR
			(pad.sppd_bttid IS NOT NULL AND tb.tb_tglkembali IS NULL)
		  ) %s
		GROUP BY %s
		ORDER BY %s
	`, groupField, filterClause, groupField, groupField)

	var list []models.BTTBelumKembaliRekapDTO
	if err := database.Raw(queryStr, tgla, tgle).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query rekap BTT belum kembali: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
		"count":  len(list),
	})
}
