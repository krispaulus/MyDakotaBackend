package handler

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// =========================================================================
// 🔎 ENDPOINT HISTORY LISTER: MENAMPILKAN DAFTAR SURAT LOPER BARANG (DYNAMIC)
// =========================================================================
func GetHistoryLoper(c *gin.Context) {
	// 1. Ambil Identitas Tenant / Branch Active dari JWT Context
	ptID, _ := c.Get("pt_id")
	currentTenant := fmt.Sprintf("%v", ptID)
	currentDatabase, _ := db.ResolveDB(currentTenant)

	// 2. Tangkap Parameter Filter dari Frontend Query String
	tglAwal := c.Query("tgl_awal")
	tglAkhir := c.Query("tgl_akhir")
	noLoper := c.Query("no_loper")
	noMobil := c.Query("no_mobil")
	sopir := c.Query("sopir")
	noBTT := c.Query("no_btt")

	// 3. Rancang Query Utama dengan Pembersih Tipe Data ::text & Penjaga Duplikasi JOIN
	// Menghitung SUM COD dan COUNT Jumlah BTT per dokumen Loper
	baseQuery := currentDatabase.Table("public.opr_t_eloper AS l").
		Select(`
			l.loper_eid,
			TO_CHAR(l.loper_tanggal::timestamp, 'YYYY-MM-DD HH24:MI:SS') AS loper_tanggal,
			l.loper_nomobil,
			l.loper_nipsopir,
			l.loper_nipkerani,
			l.loper_updateid,
			l.loper_aktifyn,
			l.loper_keraniyn,
			COALESCE(k_sopir.kry_nama, 'TANPA SOPIR') AS sopir_nama,
			COALESCE(k_kerani.kry_nama, 'TANPA KERANI') AS kerani_nama,
			COALESCE(SUM(c.bttt_tagihtujuan::numeric), 0) AS total_cod,
			COUNT(DISTINCT d.loperd_bttid) AS jumlah_btt
		`).
		Joins("LEFT JOIN public.opr_t_eloperdetail d ON l.loper_eid::text = d.loperd_eloperid::text").
		Joins("LEFT JOIN public.mkt_t_econote c ON d.loperd_bttid::text = c.bttt_id::text").
		Joins("LEFT JOIN public.hrd_m_karyawan k_sopir ON l.loper_nipsopir::text = k_sopir.kry_nip::text").
		Joins("LEFT JOIN public.hrd_m_karyawan k_kerani ON l.loper_nipkerani::text = k_kerani.kry_nip::text")

	// 4. Injeksi Filter Dinamis Sesuai Checkbox yang Aktif di Layar
	if tglAwal != "" && tglAkhir != "" {
		baseQuery = baseQuery.Where("l.loper_tanggal::timestamp::date BETWEEN ? AND ?", tglAwal, tglAkhir)
	}
	if noLoper != "" {
		baseQuery = baseQuery.Where("l.loper_eid ILIKE ?", "%"+noLoper+"%")
	}
	if noMobil != "" {
		baseQuery = baseQuery.Where("l.loper_nomobil ILIKE ?", "%"+noMobil+"%")
	}
	if sopir != "" {
		baseQuery = baseQuery.Where("k_sopir.kry_nama ILIKE ?", "%"+sopir+"%")
	}
	if noBTT != "" {
		baseQuery = baseQuery.Where("d.loperd_bttid = ?", noBTT)
	}

	// 5. Eksekusi Pengelompokan (Group By) & Urutan Data Terbaru
	var results []map[string]interface{}
	err := baseQuery.Group("l.loper_eid, l.loper_tanggal, l.loper_nomobil, l.loper_nipsopir, l.loper_nipkerani, l.loper_updateid, l.loper_aktifyn, l.loper_keraniyn, k_sopir.kry_nama, k_kerani.kry_nama").
		Order("l.loper_tanggal DESC, l.loper_eid DESC").
		Find(&results).Error

	// 6. Response Handlers
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat history Loper: " + err.Error()})
		return
	}

	if len(results) == 0 {
		c.JSON(http.StatusOK, []map[string]interface{}{})
		return
	}

	c.JSON(http.StatusOK, results)
}
