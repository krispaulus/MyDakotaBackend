package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// GetListLoadingBarang Handler untuk Mengambil Daftar Loading Barang
func GetListLoadingBarang(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	noLoad := strings.TrimSpace(c.Query("noload"))
	noMobil := strings.TrimSpace(c.Query("nomobil"))
	ckTgl := strings.TrimSpace(c.Query("cktgl"))
	ckNoLoad := strings.TrimSpace(c.Query("cknoload"))
	ckNoMobil := strings.TrimSpace(c.Query("cknomobil"))

	filterClause := ""

	if (ckTgl == "true" || ckTgl == "on") && tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(h.loadh_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	if (ckNoLoad == "true" || ckNoLoad == "on") && noLoad != "" {
		filterClause += fmt.Sprintf(` AND UPPER(h.loadh_id) LIKE UPPER('%%%s%%') `, noLoad)
	}

	if (ckNoMobil == "true" || ckNoMobil == "on") && noMobil != "" {
		filterClause += fmt.Sprintf(` AND UPPER(h.loadh_kendid) LIKE UPPER('%%%s%%') `, noMobil)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			h.loadh_id,
			TO_CHAR(h.loadh_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS loadh_tanggal,
			COALESCE(h.loadh_kendid, '-') AS loadh_kendid,
			COALESCE(h.loadh_updateid, '-') AS loadh_updateid,
			COALESCE(h.loadh_approveyn, 'N') AS loadh_approveyn,
			COUNT(d.loadd_bttkoliid) AS jml_barang
		FROM public.opr_t_loadingh h
		LEFT JOIN public.opr_t_loadingd d ON h.loadh_id = d.loadd_loadhid
		WHERE 1=1 %s
		GROUP BY h.loadh_id, h.loadh_tanggal, h.loadh_kendid, h.loadh_updateid, h.loadh_approveyn
		ORDER BY h.loadh_tanggal DESC, h.loadh_id DESC
	`, filterClause)

	type RawResult struct {
		LoadHID        string `gorm:"column:loadh_id"`
		LoadHTanggal   string `gorm:"column:loadh_tanggal"`
		LoadHKendID    string `gorm:"column:loadh_kendid"`
		LoadHUpdateID  string `gorm:"column:loadh_updateid"`
		LoadHApproveYN string `gorm:"column:loadh_approveyn"`
		JmlBarang      int    `gorm:"column:jml_barang"`
	}

	var rawList []RawResult
	if err := database.Raw(queryStr).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query loading barang: " + err.Error()})
		return
	}

	var resultList []models.LoadingBarangDTO
	for idx, r := range rawList {
		resultList = append(resultList, models.LoadingBarangDTO{
			No:             idx + 1,
			LoadHID:        r.LoadHID,
			LoadHTanggal:   r.LoadHTanggal,
			LoadHKendID:    r.LoadHKendID,
			LoadHUpdateID:  r.LoadHUpdateID,
			LoadHApproveYN: r.LoadHApproveYN,
			JmlBarang:      r.JmlBarang,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resultList,
		"count":  len(resultList),
	})
}

// GetDetailLoadingBarang Mengambil rincian BTT & Summary Layanan (Darat, Laut, Udara)
func GetDetailLoadingBarang(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	loID := strings.TrimSpace(c.Query("loadh_id"))
	if loID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter loadh_id wajib diisi"})
		return
	}

	// 1. Ambil Header + Cek apakah sudah ditarik ke SP Terima atau SP Loper (sesuai query ASP)
	var header struct {
		LoadHID        string `gorm:"column:loadh_id" json:"loadh_id"`
		LoadHTanggal   string `gorm:"column:loadh_tanggal" json:"loadh_tanggal"`
		LoadHKendID    string `gorm:"column:loadh_kendid" json:"loadh_kendid"`
		LoadHUpdateID  string `gorm:"column:loadh_updateid" json:"loadh_updateid"`
		LoadHApproveYN string `gorm:"column:loadh_approveyn" json:"loadh_approveyn"`
		SptEID         string `gorm:"column:spt_eid" json:"spt_eid"`
		LoperEID       string `gorm:"column:loper_eid" json:"loper_eid"`
	}

	err := database.Raw(`
		SELECT 
			h.loadh_id,
			TO_CHAR(h.loadh_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS loadh_tanggal,
			COALESCE(h.loadh_kendid, '-') AS loadh_kendid,
			COALESCE(h.loadh_updateid, '-') AS loadh_updateid,
			COALESCE(TRIM(h.loadh_approveyn), 'N') AS loadh_approveyn,
			COALESCE(spt.spt_eid, '') AS spt_eid,
			COALESCE(lop.loper_eid, '') AS loper_eid
		FROM public.opr_t_loadingh h
		LEFT JOIN public.opr_t_esp_terima spt ON TRIM(h.loadh_id) = TRIM(spt.spt_loadhid)
		LEFT JOIN public.opr_t_eloper lop ON TRIM(h.loadh_id) = TRIM(lop.loper_loadhid)
		WHERE TRIM(h.loadh_id) = TRIM(?)
		LIMIT 1
	`, loID).Scan(&header).Error

	if err != nil || header.LoadHID == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data Loading tidak ditemukan"})
		return
	}

	// 2. Ambil Daftar BTT Terdaftar (Di BTT vs Terdaftar)
	type BTTDetailRow struct {
		NoBTT      string `gorm:"column:nobtt" json:"no_btt"`
		Kecamatan  string `gorm:"column:kecamatan" json:"kecamatan"`
		Kota       string `gorm:"column:kota" json:"kota"`
		Propinsi   string `gorm:"column:propinsi" json:"propinsi"`
		CabangNama string `gorm:"column:cabang_nama" json:"cabang_nama"`
		ServID     string `gorm:"column:serv_id" json:"serv_id"`
		DiBTT      int    `gorm:"column:di_btt" json:"di_btt"`
		Terdaftar  int    `gorm:"column:terdaftar" json:"terdaftar"`
	}

	var bttList []BTTDetailRow
	database.Raw(`
	SELECT 
		COALESCE(NULLIF(TRIM(e.bttt_id), ''), SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)) AS nobtt,
		COALESCE(e.bttt_tujuankecamatan, '-') AS kecamatan,
		COALESCE(e.bttt_tujuankota, '-') AS kota,
		COALESCE(e.bttt_tujuanpulau, '-') AS propinsi,
		COALESCE(a.agen_nama, '-') AS cabang_nama,
		COALESCE(TRIM(e.bttt_servid), '1') AS serv_id,
		COALESCE(NULLIF(e.bttt_jmlunit, 0), COUNT(d.loadd_id)::int) AS di_btt,
		COUNT(d.loadd_id)::int AS terdaftar
	FROM public.opr_t_loadingd d
	LEFT JOIN public.mkt_t_econote e 
		ON TRIM(e.bttt_id) = SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)
		OR LEFT(TRIM(d.loadd_bttkoliid), LENGTH(TRIM(e.bttt_id))) = TRIM(e.bttt_id)
	LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(e.bttt_tujuanagenid::varchar)
	WHERE TRIM(d.loadd_loadhid) = TRIM(?)
	GROUP BY COALESCE(NULLIF(TRIM(e.bttt_id), ''), SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)), 
	         e.bttt_tujuankecamatan, e.bttt_tujuankota, e.bttt_tujuanpulau, a.agen_nama, e.bttt_servid, e.bttt_jmlunit
	ORDER BY a.agen_nama ASC, nobtt ASC
`, loID).Scan(&bttList)

	// 3. Summary SP Darat ('1'), Laut ('2'), Udara ('3') sesuai file ASP
	type SummaryRow struct {
		ServID     string  `gorm:"column:serv_id" json:"serv_id"`
		AgenID     string  `gorm:"column:agen_id" json:"agen_id"`
		CabangNama string  `gorm:"column:cabang_nama" json:"cabang_nama"`
		JmlBTT     int     `gorm:"column:jml_btt" json:"jml_btt"`
		TotalBerat float64 `gorm:"column:total_berat" json:"total_berat"`
	}

	var summaryList []SummaryRow
	database.Raw(`
		SELECT 
			CASE 
				WHEN UPPER(TRIM(COALESCE(e.bttt_servid, '1'))) IN ('2', 'LAUT') THEN '2'
				WHEN UPPER(TRIM(COALESCE(e.bttt_servid, '1'))) IN ('3', 'UDARA') THEN '3'
				ELSE '1'
			END AS serv_id,
			COALESCE(NULLIF(TRIM(e.bttt_tujuanagenid), ''), '-') AS agen_id,
			COALESCE(NULLIF(TRIM(a.agen_nama), ''), NULLIF(TRIM(e.bttt_tujuankota), ''), 'TANPA CABANG') AS cabang_nama,
			COUNT(d.loadd_id)::int AS jml_btt,
			COALESCE(SUM(
				CASE WHEN COALESCE(e.bttt_berat, 0) >= COALESCE(e.bttt_beratvol, 0) 
				     THEN COALESCE(e.bttt_berat, 0) 
				     ELSE COALESCE(e.bttt_beratvol, 0) 
				END
			), 0) AS total_berat
		FROM public.opr_t_loadingd d
		LEFT JOIN public.mkt_t_econote e 
			ON TRIM(e.bttt_id) = SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)
			OR LEFT(TRIM(d.loadd_bttkoliid), LENGTH(TRIM(e.bttt_id))) = TRIM(e.bttt_id)
		LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(e.bttt_tujuanagenid::varchar)
		WHERE TRIM(d.loadd_loadhid) = TRIM(?)
		GROUP BY 
			CASE 
				WHEN UPPER(TRIM(COALESCE(e.bttt_servid, '1'))) IN ('2', 'LAUT') THEN '2'
				WHEN UPPER(TRIM(COALESCE(e.bttt_servid, '1'))) IN ('3', 'UDARA') THEN '3'
				ELSE '1'
			END,
			COALESCE(NULLIF(TRIM(e.bttt_tujuanagenid), ''), '-'),
			COALESCE(NULLIF(TRIM(a.agen_nama), ''), NULLIF(TRIM(e.bttt_tujuankota), ''), 'TANPA CABANG')
		ORDER BY serv_id ASC, cabang_nama ASC
	`, loID).Scan(&summaryList)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"header":  header,
		"details": bttList,
		"summary": summaryList,
	})
}

// UpdateAndApproveLoadingBarang Menyimpan edit atau melakukan Approve / Batalkan Approve
func UpdateAndApproveLoadingBarang(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		LoadHID        string `json:"loadh_id" binding:"required"`
		LoadHKendID    string `json:"loadh_kend_id"`
		LoadHApproveYN string `json:"loadh_approve_yn" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid"})
		return
	}

	// Cek penguncian: Jika mau batalkan approve (Y -> N), pastikan belum dipakai di SP Terima / SP Loper
	if strings.ToUpper(strings.TrimSpace(req.LoadHApproveYN)) == "N" {
		var usedCount int64
		database.Raw(`
			SELECT (
				(SELECT COUNT(*) FROM public.opr_t_esp_terima WHERE TRIM(spt_loadhid) = TRIM(?)) +
				(SELECT COUNT(*) FROM public.opr_t_eloper WHERE TRIM(loper_loadhid) = TRIM(?))
			)
		`, req.LoadHID, req.LoadHID).Scan(&usedCount)

		if usedCount > 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Approval tidak dapat dibatalkan karena Loading ID ini sudah dibuatkan Surat Pengantar (SP Terima / SP Loper)!",
			})
			return
		}
	}

	userLogin, _ := c.Get("username")
	userNameStr := fmt.Sprintf("%v", userLogin)
	if userNameStr == "" || userNameStr == "<nil>" {
		userNameStr = "ADMIN"
	}

	err := database.Exec(`
		UPDATE public.opr_t_loadingh
		SET loadh_kendid = COALESCE(NULLIF(TRIM(?), ''), loadh_kendid),
		    loadh_approveyn = ?,
		    loadh_updateid = ?,
		    loadh_updatetime = CURRENT_TIMESTAMP
		WHERE TRIM(loadh_id) = TRIM(?)
	`, req.LoadHKendID, strings.ToUpper(req.LoadHApproveYN), userNameStr, req.LoadHID).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan perubahan: " + err.Error()})
		return
	}

	// Ikut perbarui sjh_kendid pada Surat Tugas yang terhubung dengan loadh_id ini
	database.Exec(`
		UPDATE public.opr_t_esuratjalan
		SET sjh_kendid = COALESCE(NULLIF(TRIM(?), ''), sjh_kendid),
		    sjh_updateid = ?,
		    sjh_updatetime = CURRENT_TIMESTAMP
		WHERE TRIM(sjh_loadhid) = TRIM(?)
	`, req.LoadHKendID, userNameStr, req.LoadHID)

	msg := "Manifest Loading berhasil disimpan!"
	if strings.ToUpper(req.LoadHApproveYN) == "Y" {
		msg = fmt.Sprintf("Loading %s berhasil di-APPROVE! Siap dibuatkan Surat Pengantar (SP).", req.LoadHID)
	} else {
		msg = fmt.Sprintf("Approval Loading %s berhasil dibatalkan (Status: PENDING).", req.LoadHID)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": msg})
}

// DeleteLoadingBarang Menghapus manifest loading jika belum dipakai SP
func DeleteLoadingBarang(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	loID := strings.TrimSpace(c.Query("loadh_id"))
	if loID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter loadh_id wajib diisi"})
		return
	}

	var usedCount int64
	database.Raw(`
		SELECT (
			(SELECT COUNT(*) FROM public.opr_t_esp_terima WHERE TRIM(spt_loadhid) = TRIM(?)) +
			(SELECT COUNT(*) FROM public.opr_t_eloper WHERE TRIM(loper_loadhid) = TRIM(?))
		)
	`, loID, loID).Scan(&usedCount)

	if usedCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Loading tidak dapat dihapus karena sudah terhubung ke SP!"})
		return
	}

	database.Exec(`DELETE FROM public.opr_t_loadingd WHERE TRIM(loadd_loadhid) = TRIM(?)`, loID)
	database.Exec(`DELETE FROM public.opr_t_loadingh WHERE TRIM(loadh_id) = TRIM(?)`, loID)
	database.Exec(`UPDATE public.opr_t_esuratjalan SET sjh_loadhid = NULL WHERE TRIM(sjh_loadhid) = TRIM(?)`, loID)

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Manifest Loading berhasil dihapus."})
}
