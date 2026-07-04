package handler

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// =========================================================================
// 🔎 ENDPOINT HISTORY LISTER: MENAMPILKAN DAFTAR DOKUMEN PENGEMBALIAN BTT
// =========================================================================
func GetHistoryKembaliBTT(c *gin.Context) {
	// 1. Ambil Identitas Tenant Active dari JWT Context Token
	ptID, _ := c.Get("pt_id")
	currentTenant := fmt.Sprintf("%v", ptID)
	currentDatabase, _ := db.ResolveDB(currentTenant)

	// 2. Tangkap Parameter Filter dari Frontend Query String
	tglAwal := c.Query("tgl_awal")
	tglAkhir := c.Query("tgl_akhir")
	noKembali := c.Query("no_kembali")
	agenTujuan := c.Query("agen_tujuan_id")
	noBTT := c.Query("no_btt")

	// 🌟 ATURAN BISNIS 10.5: Default tanggal jika kosong adalah HARI INI saja!
	if tglAwal == "" && tglAkhir == "" && noKembali == "" && noBTT == "" {
		hariIni := time.Now().Format("2006-01-02")
		tglAwal = hariIni
		tglAkhir = hariIni
	}

	// 3. Rancang Query Utama dengan Proteksi Casting String dan COALESCE BDB
	baseQuery := currentDatabase.Table("public.opr_t_ekembalibtt AS kb").
		Select(`
			kb.kb_eid,
			COALESCE(kb.kb_bdbid, '') AS kb_bdbid,
			TO_CHAR(kb.kb_tanggal::timestamp, 'YYYY-MM-DD HH24:MI:SS') AS kb_tanggal,
			kb.kb_tujuanagenid,
			kb.kb_updateid,
			kb.kb_aktifyn,
			COALESCE(a.agen_nama, 'CABANG RETUR UNDEFINED') AS agen_nama_tujuan,
			COUNT(DISTINCT d.kbd_bttid) AS jumlah_btt_retur
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON kb.kb_tujuanagenid::text = a.agen_id::text").
		Joins("LEFT JOIN public.opr_t_ekembalibttdetil d ON kb.kb_eid::text = d.kbd_kbeid::text")

	// 4. Injeksi Parameter Filter Dinamis
	if tglAwal != "" && tglAkhir != "" {
		baseQuery = baseQuery.Where("kb.kb_tanggal::timestamp::date BETWEEN ? AND ?", tglAwal, tglAkhir)
	}
	if noKembali != "" {
		baseQuery = baseQuery.Where("kb.kb_eid ILIKE ?", "%"+noKembali+"%")
	}
	if agenTujuan != "" {
		baseQuery = baseQuery.Where("kb.kb_tujuanagenid::text = ?", agenTujuan)
	}
	if noBTT != "" {
		baseQuery = baseQuery.Where("d.kbd_bttid ILIKE ?", "%"+noBTT+"%")
	}

	// 5. Eksekusi Group By Sesuai Aturan JOIN Berulang
	var results []map[string]interface{}
	err := baseQuery.Group("kb.kb_eid, kb.kb_bdbid, kb.kb_tanggal, kb.kb_tujuanagenid, kb.kb_updateid, kb.kb_aktifyn, a.agen_nama").
		Order("kb.kb_tanggal DESC, kb.kb_eid DESC").
		Find(&results).Error

	// 6. Response Router Handlers
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat log Pengembalian BTT: " + err.Error()})
		return
	}

	if len(results) == 0 {
		c.JSON(http.StatusOK, []map[string]interface{}{})
		return
	}

	c.JSON(http.StatusOK, results)
}

// =========================================================================
// 📊 MONITORING 1: BTT YANG SUDAH TERIMA TAPI BELUM DIAJUKAN RETUR (SAFE MODE)
// =========================================================================
func GetBTTBelumKembali(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	currentTenant := fmt.Sprintf("%v", ptID)
	currentDatabase, _ := db.ResolveDB(currentTenant)

	// 💡 AMANKAN KEY CONTEXT DENGAN FALLBACK CADANGAN AGAR TIDAK PANIC
	var activeAgenID string
	if val, exists := c.Get("active_agen_id"); exists {
		activeAgenID = fmt.Sprintf("%v", val)
	} else if valHeader, existsHeader := c.Get("agen_id"); existsHeader {
		activeAgenID = fmt.Sprintf("%v", valHeader)
	} else {
		// Jika benar-benar kosong, ambil fallback string kosong agar SQL tidak crash panic
		activeAgenID = ""
	}

	query := `
		SELECT 
			t.sp_bttid AS btt_id,
			TO_CHAR(t.sp_tanggal::timestamp, 'YYYY-MM-DD HH24:MI:SS') AS tanggal_terima,
			t.sp_eid AS no_sp,
			COALESCE(t.sp_keterangan, '-') AS keterangan_bongkar
		FROM public.opr_t_esp_turun t
		LEFT JOIN public.opr_t_ekembalibttdetil kbd ON t.sp_bttid::text = kbd.kbd_bttid::text
		WHERE t.sp_agenid::text = $1 
		  AND t.sp_aktifyn = 'Y'
		  AND kbd.kbd_bttid IS NULL
		ORDER BY t.sp_tanggal DESC`

	var results []map[string]interface{}
	err := currentDatabase.Raw(query, activeAgenID).Scan(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

// =========================================================================
// 🚨 MONITORING 2: DAFTAR DOKUMEN RETUR YANG OUTSTANDING / BELUM BDB (SAFE MODE)
// =========================================================================
func GetReturOutstandingBDB(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	currentTenant := fmt.Sprintf("%v", ptID)
	currentDatabase, _ := db.ResolveDB(currentTenant)

	// 💡 AMANKAN KEY CONTEXT DENGAN FALLBACK CADANGAN AGAR TIDAK PANIC
	var activeAgenID string
	if val, exists := c.Get("active_agen_id"); exists {
		activeAgenID = fmt.Sprintf("%v", val)
	} else if valHeader, existsHeader := c.Get("agen_id"); existsHeader {
		activeAgenID = fmt.Sprintf("%v", valHeader)
	} else {
		activeAgenID = ""
	}

	query := `
		SELECT 
			kb.kb_eid AS no_pengembalian,
			TO_CHAR(kb.kb_tanggal::timestamp, 'YYYY-MM-DD HH24:MI:SS') AS tanggal_retur,
			COALESCE(a.agen_nama, 'CABANG RETUR') AS agen_tujuan_nama,
			kb.kb_updateid AS pembuat,
			COUNT(DISTINCT d.kbd_bttid) AS jumlah_btt
		FROM public.opr_t_ekembalibtt kb
		LEFT JOIN public.glb_m_agen a ON kb.kb_tujuanagenid::text = a.agen_id::text
		LEFT JOIN public.opr_t_ekembalibttdetil d ON kb.kb_eid::text = d.kbd_kbeid::text
		WHERE kb.kb_agenid::text = $1 
		  AND kb.kb_aktifyn = 'Y' 
		  AND (kb.kb_bdbid IS NULL OR kb.kb_bdbid = '')
		GROUP BY kb.kb_eid, kb.kb_tanggal, a.agen_nama, kb.kb_updateid
		ORDER BY kb.kb_tanggal DESC`

	var results []map[string]interface{}
	err := currentDatabase.Raw(query, activeAgenID).Scan(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}
