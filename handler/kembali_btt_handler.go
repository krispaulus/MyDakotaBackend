package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 🔍 1. GET LIST HISTORY PENGEMBALIAN BTT (SESUAI DENGAN API REACT `/operasional/kembali-btt/history`)
func GetListKembaliBTT(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	tglAwal := strings.TrimSpace(c.Query("tgl_awal"))
	tglAkhir := strings.TrimSpace(c.Query("tgl_akhir"))
	noKembali := strings.TrimSpace(c.Query("no_kembali"))
	agenNama := strings.TrimSpace(c.Query("agen_nama"))
	noBtt := strings.TrimSpace(c.Query("no_btt"))

	filterClause := ""

	if tglAwal != "" && tglAkhir != "" {
		filterClause += fmt.Sprintf(` AND CAST(k.kb_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tglAwal, tglAkhir)
	}

	if noKembali != "" {
		filterClause += fmt.Sprintf(` AND UPPER(k.kb_eid) LIKE UPPER('%%%s%%') `, noKembali)
	}

	if agenNama != "" {
		filterClause += fmt.Sprintf(` AND (UPPER(a.agen_nama) LIKE UPPER('%%%s%%') OR UPPER(k.kb_tujuanagenid) LIKE UPPER('%%%s%%')) `, agenNama, agenNama)
	}

	if noBtt != "" {
		filterClause += fmt.Sprintf(` AND EXISTS (SELECT 1 FROM public.opr_t_ekembalibttdetil d WHERE (d.kbd_eid = k.kb_eid OR d.kbd_kbeid = k.kb_eid) AND UPPER(d.kbd_bttid) LIKE UPPER('%%%s%%')) `, noBtt)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			COALESCE(k.kb_eid, '-') AS kb_eid,
			TO_CHAR(k.kb_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS kb_tanggal,
			COALESCE(k.kb_agenid, '-') AS kb_agen_id,
			COALESCE(k.kb_tujuanagenid, '-') AS kb_tujuan_agen_id,
			COALESCE(a.agen_nama, k.kb_tujuanagenid, '-') AS agen_nama_tujuan,
			COALESCE(k.kb_bdbid, '') AS kb_bdbid,
			COALESCE(k.kb_updateid, '-') AS kb_updateid,
			COALESCE(k.kb_aktifyn, 'Y') AS kb_aktifyn,
			COUNT(d.kbd_bttid) AS jumlah_btt_retur
		FROM public.opr_t_ekembalibtt k
		LEFT OUTER JOIN public.glb_m_agen a ON CAST(k.kb_tujuanagenid AS VARCHAR) = CAST(a.agen_id AS VARCHAR)
		LEFT OUTER JOIN public.opr_t_ekembalibttdetil d ON (k.kb_eid = d.kbd_eid OR k.kb_eid = d.kbd_kbeid)
		WHERE 1=1 %s
		GROUP BY k.kb_eid, k.kb_tanggal, k.kb_agenid, k.kb_tujuanagenid, a.agen_nama, k.kb_bdbid, k.kb_updateid, k.kb_aktifyn
		ORDER BY k.kb_tanggal DESC, k.kb_eid DESC
	`, filterClause)

	var results []map[string]interface{}
	if err := database.Raw(queryStr).Scan(&results).Error; err != nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	c.JSON(http.StatusOK, results)
}

// 📊 2. MONITOR BTT TERIMA BELUM DIAJUKAN RETUR (`/operasional/kembali-btt/monitor-belum-kembali`)
func GetMonitorBelumKembali(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database gagal"})
		return
	}

	queryStr := `
		SELECT 
			c.bttt_id AS btt_id,
			TO_CHAR(c.bttt_tanggal, 'YYYY-MM-DD') AS tanggal_terima,
			COALESCE(s.sptd_esptid, '-') AS no_sp,
			'RETUR / GAGAL SERAH' AS keterangan_bongkar
		FROM public.mkt_t_econote c
		LEFT OUTER JOIN public.opr_t_esp_terimadetil s ON c.bttt_id = s.sptd_bttid
		WHERE c.bttt_id NOT IN (
			SELECT kbd_bttid FROM public.opr_t_ekembalibttdetil WHERE kbd_bttid IS NOT NULL
		)
		ORDER BY c.bttt_tanggal DESC
		LIMIT 100
	`

	var results []map[string]interface{}
	database.Raw(queryStr).Scan(&results)
	c.JSON(http.StatusOK, results)
}

// 🚨 3. MONITOR RETUR OUTSTANDING BELUM TERBIT BDB (`/operasional/kembali-btt/monitor-outstanding-bdb`)
func GetMonitorOutstandingBDB(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database gagal"})
		return
	}

	queryStr := `
		SELECT 
			k.kb_eid AS no_pengembalian,
			TO_CHAR(k.kb_tanggal, 'YYYY-MM-DD') AS tanggal_retur,
			COALESCE(a.agen_nama, k.kb_tujuanagenid, '-') AS agen_tujuan_nama,
			COALESCE(k.kb_updateid, '-') AS pembuat,
			COUNT(d.kbd_bttid) AS jumlah_btt
		FROM public.opr_t_ekembalibtt k
		LEFT OUTER JOIN public.opr_t_ekembalibttdetil d ON (k.kb_eid = d.kbd_eid OR k.kb_eid = d.kbd_kbeid)
		LEFT OUTER JOIN public.glb_m_agen a ON CAST(k.kb_tujuanagenid AS VARCHAR) = CAST(a.agen_id AS VARCHAR)
		WHERE (k.kb_bdbid IS NULL OR k.kb_bdbid = '') AND COALESCE(k.kb_aktifyn, 'Y') = 'Y'
		GROUP BY k.kb_eid, k.kb_tanggal, a.agen_nama, k.kb_tujuanagenid, k.kb_updateid
		ORDER BY k.kb_tanggal DESC
	`

	var results []map[string]interface{}
	database.Raw(queryStr).Scan(&results)
	c.JSON(http.StatusOK, results)
}

// 💾 4. CREATE KEMBALI BTT
func CreateKembaliBTT(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.KembaliBTTReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	insertHeader := `
		INSERT INTO public.opr_t_ekembalibtt (kb_eid, kb_tanggal, kb_agenid, kb_tujuanagenid, kb_bdbid, kb_updateid, kb_aktifyn)
		VALUES (?, NOW(), 'PUSAT', ?, ?, ?, 'Y')
	`
	if err := database.Exec(insertHeader, req.KBEid, req.KBTujuanAgenID, req.KBBdbID, fmt.Sprintf("%v", username)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan header pengembalian BTT: " + err.Error()})
		return
	}

	insertDetail := `INSERT INTO public.opr_t_ekembalibttdetil (kbd_eid, kbd_bttid, kbd_updateid) VALUES (?, ?, ?)`
	for _, btt := range req.ListBttID {
		if strings.TrimSpace(btt) != "" {
			database.Exec(insertDetail, req.KBEid, strings.TrimSpace(btt), fmt.Sprintf("%v", username))
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Pengembalian BTT berhasil dibuat!"})
}

// ✏️ 5. UPDATE KEMBALI BTT
func UpdateKembaliBTT(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.KembaliBTTReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	updateHeader := `
		UPDATE public.opr_t_ekembalibtt 
		SET kb_tujuanagenid = ?, kb_bdbid = ?, kb_updateid = ?, kb_updatetime = NOW() 
		WHERE kb_eid = ?
	`
	if err := database.Exec(updateHeader, req.KBTujuanAgenID, req.KBBdbID, fmt.Sprintf("%v", username), req.KBEid).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui pengembalian BTT: " + err.Error()})
		return
	}

	database.Exec(`DELETE FROM public.opr_t_ekembalibttdetil WHERE kbd_eid = ? OR kbd_kbeid = ?`, req.KBEid, req.KBEid)
	insertDetail := `INSERT INTO public.opr_t_ekembalibttdetil (kbd_eid, kbd_bttid, kbd_updateid) VALUES (?, ?, ?)`
	for _, btt := range req.ListBttID {
		if strings.TrimSpace(btt) != "" {
			database.Exec(insertDetail, req.KBEid, strings.TrimSpace(btt), fmt.Sprintf("%v", username))
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data Pengembalian BTT berhasil diperbarui!"})
}

// 🗑️ 6. DELETE KEMBALI BTT
func DeleteKembaliBTT(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	kbEid := c.Query("kb_eid")
	if kbEid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter kb_eid wajib diisi"})
		return
	}

	database.Exec(`DELETE FROM public.opr_t_ekembalibttdetil WHERE kbd_eid = ? OR kbd_kbeid = ?`, kbEid, kbEid)
	if err := database.Exec(`DELETE FROM public.opr_t_ekembalibtt WHERE kb_eid = ?`, kbEid).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus pengembalian BTT: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Pengembalian BTT berhasil dihapus!"})
}
