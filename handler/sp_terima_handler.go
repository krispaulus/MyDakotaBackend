package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// GetSuratPengantarFetch menarik daftar data SP untuk data table React
func GetSuratPengantarFetch(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database tenant gagal resolved"})
		return
	}

	// 🔍 Tangkap Seluruh parameter saringan filter
	tglDari := c.Query("tanggal_awal")
	tglSampai := c.Query("tanggal_akhir")
	agenID := c.Query("agen_id")
	tujuan := c.Query("tujuan")
	noSP := c.Query("no_sp")
	noBTT := c.Query("no_btt")
	transit := c.Query("transit")
	noMobil := c.Query("no_mobil")

	filterClause := ""

	if tglDari != "" && tglSampai != "" {
		filterClause += fmt.Sprintf(` AND CAST(s.spt_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tglDari, tglSampai)
	}

	if noSP != "" {
		filterClause += fmt.Sprintf(` AND UPPER(s.spt_eid) LIKE UPPER('%%%s%%') `, noSP)
	}

	if noMobil != "" {
		filterClause += fmt.Sprintf(` AND UPPER(s.spt_nomobil) LIKE UPPER('%%%s%%') `, noMobil)
	}

	// if agenID != "" && agenID != "PUSAT DAKOTA" {
	// 	filterClause += fmt.Sprintf(` AND (UPPER(a1.agen_nama) LIKE UPPER('%%%s%%') OR UPPER(s.spt_asalagenid) LIKE UPPER('%%%s%%')) `, agenID, agenID)
	// }

	// Filter agen hanya aktif jika bukan Pusat / Holding / Empty
	if agenID != "" && agenID != "PUSAT DAKOTA" && !strings.Contains(strings.ToUpper(agenID), "PUSAT") && !strings.Contains(strings.ToUpper(agenID), "HOLDING") {
		filterClause += fmt.Sprintf(` AND (UPPER(a1.agen_nama) LIKE UPPER('%%%s%%') OR CAST(s.spt_asalagenid AS VARCHAR) = '%s') `, agenID, agenID)
	}

	if tujuan != "" {
		filterClause += fmt.Sprintf(` AND (UPPER(a2.agen_nama) LIKE UPPER('%%%s%%') OR UPPER(s.spt_tujuanagenid) LIKE UPPER('%%%s%%')) `, tujuan, tujuan)
	}

	if transit != "" {
		filterClause += fmt.Sprintf(` AND s.spt_transityn = '%s' `, transit)
	}

	if noBTT != "" {
		filterClause += fmt.Sprintf(` AND EXISTS (SELECT 1 FROM public.opr_t_esp_terimadetil d WHERE d.sptd_esptid = s.spt_eid AND UPPER(d.sptd_bttid) LIKE UPPER('%%%s%%')) `, noBTT)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			COALESCE(s.spt_eid, '-') AS no_sp,
			TO_CHAR(s.spt_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS tanggal,
			COALESCE(a1.agen_nama, s.spt_asalagenid, '-') AS asal,
			COALESCE(a2.agen_nama, s.spt_tujuanagenid, '-') AS tujuan,
			COALESCE(s.spt_transityn, 'N') AS spt_transityn,
			COALESCE(s.spt_nomobil, '-') AS no_mobil,
			COALESCE(s.spt_namasopir, '-') AS sopir,
			COALESCE(s.spt_aktifyn, 'Y') AS aktif,
			COUNT(d.sptd_bttid) AS jmlbtt,
			COALESCE(SUM(c.bttt_tagihtujuan), 0) AS jmlcod
		FROM public.opr_t_esp_terima s
		LEFT OUTER JOIN public.opr_t_esp_terimadetil d ON s.spt_eid = d.sptd_esptid
		LEFT OUTER JOIN public.mkt_t_econote c ON d.sptd_bttid = c.bttt_id
		LEFT OUTER JOIN public.glb_m_agen a1 ON CAST(s.spt_asalagenid AS VARCHAR) = CAST(a1.agen_id AS VARCHAR)
		LEFT OUTER JOIN public.glb_m_agen a2 ON CAST(s.spt_tujuanagenid AS VARCHAR) = CAST(a2.agen_id AS VARCHAR)
		WHERE 1=1 %s
		GROUP BY s.spt_eid, s.spt_tanggal, a1.agen_nama, s.spt_asalagenid, a2.agen_nama, s.spt_tujuanagenid, s.spt_transityn, s.spt_nomobil, s.spt_namasopir, s.spt_aktifyn
		ORDER BY s.spt_tanggal DESC, s.spt_eid DESC
	`, filterClause)

	var results []map[string]interface{}
	if err := database.Raw(queryStr).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal fetch data SP: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// CreateSuratPengantar menyimpan data SP baru
func CreateSuratPengantar(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	var req models.SPNaikReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload data tidak valid: " + err.Error()})
		return
	}

	now := time.Now()
	sptEID := fmt.Sprintf("SP/%s/%s", now.Format("20060102"), now.Format("150405"))

	// Cari ID Agen Asal dan Tujuan
	var asalID, tujuanID string = "1", "1"
	database.Raw("SELECT agen_id FROM public.glb_m_agen WHERE UPPER(agen_nama) LIKE UPPER(?) LIMIT 1", "%"+req.SptAsalAgenNama+"%").Scan(&asalID)
	database.Raw("SELECT agen_id FROM public.glb_m_agen WHERE UPPER(agen_nama) LIKE UPPER(?) LIMIT 1", "%"+req.SptTujuanAgenNama+"%").Scan(&tujuanID)

	if asalID == "" {
		asalID = "1"
	}
	if tujuanID == "" {
		tujuanID = "1"
	}

	insertHeader := `
		INSERT INTO public.opr_t_esp_terima (
			spt_eid, spt_tanggal, spt_asalagenid, spt_tujuanagenid, spt_transityn, 
			spt_nomobil, spt_namasopir, spt_aktifyn, spt_updateid, spt_updatetime
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'Y', ?, ?)
	`

	if err := database.Exec(
		insertHeader,
		sptEID, now.Format("2006-01-02 15:04:05"), asalID, tujuanID, req.SptTransitYN,
		req.SptNoMobil, req.SptNamaSopir, fmt.Sprintf("%v", username), now.Format("2006-01-02 15:04:05"),
	).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengunci Surat Pengantar: " + err.Error()})
		return
	}

	// Insert Detail BTT
	for _, bttID := range req.DaftarBTT {
		if strings.TrimSpace(bttID) != "" {
			database.Exec(`INSERT INTO public.opr_t_esp_terimadetil (sptd_esptid, sptd_bttid) VALUES (?, ?)`, sptEID, strings.TrimSpace(bttID))
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "spt_eid": sptEID})
}
