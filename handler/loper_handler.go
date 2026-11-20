package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// GetLoperList Handler untuk Menarik Daftar Manifes Loper
func GetLoperList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	loperID := strings.TrimSpace(c.Query("loper_id"))
	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))

	filterClause := ""
	if loperID != "" {
		filterClause += fmt.Sprintf(` AND UPPER(l.loper_eid) LIKE UPPER('%%%s%%') `, loperID)
	}
	if tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(l.loper_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			l.loper_eid,
			TO_CHAR(l.loper_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS loper_tanggal,
			COALESCE(l.loper_agenid, '-') AS loper_agenid,
			COALESCE(l.loper_nomobil, '-') AS loper_nomobil,
			COALESCE(l.loper_nipsopir, '-') AS loper_nipsopir,
			COALESCE(s.kry_nama, '-') AS nama_sopir,
			COALESCE(l.loper_keraniyn, 'N') AS loper_keraniyn,
			COALESCE(l.loper_nipkerani, '-') AS loper_nipkerani,
			COALESCE(k.kry_nama, '-') AS nama_kerani,
			COALESCE(l.loper_aktifyn, 'Y') AS loper_aktifyn,
			COALESCE(COUNT(d.loperd_bttid), 0) AS total_btt
		FROM public.opr_t_eloper l
		LEFT JOIN public.opr_t_eloperdetail d ON TRIM(l.loper_eid) = TRIM(d.loperd_eloperid)
		LEFT JOIN public.hrd_m_karyawan s ON TRIM(l.loper_nipsopir) = TRIM(s.kry_nip)
		LEFT JOIN public.hrd_m_karyawan k ON TRIM(l.loper_nipkerani) = TRIM(k.kry_nip)
		WHERE COALESCE(TRIM(l.loper_aktifyn), 'Y') = 'Y' %s
		GROUP BY l.loper_eid, l.loper_tanggal, l.loper_agenid, l.loper_nomobil, l.loper_nipsopir, s.kry_nama, l.loper_keraniyn, l.loper_nipkerani, k.kry_nama, l.loper_aktifyn
		ORDER BY l.loper_tanggal DESC, l.loper_eid DESC
	`, filterClause)

	type RawLoper struct {
		LoperEID       string `gorm:"column:loper_eid" json:"loper_eid"`
		LoperTanggal   string `gorm:"column:loper_tanggal" json:"loper_tanggal"`
		LoperAgenID    string `gorm:"column:loper_agenid" json:"loper_agenid"`
		LoperNoMobil   string `gorm:"column:loper_nomobil" json:"loper_nomobil"`
		LoperNIPSopir  string `gorm:"column:loper_nipsopir" json:"loper_nipsopir"`
		NamaSopir      string `gorm:"column:nama_sopir" json:"nama_sopir"`
		LoperKeraniYN  string `gorm:"column:loper_keraniyn" json:"loper_keraniyn"`
		LoperNIPKerani string `gorm:"column:loper_nipkerani" json:"loper_nipkerani"`
		NamaKerani     string `gorm:"column:nama_kerani" json:"nama_kerani"`
		LoperAktifYN   string `gorm:"column:loper_aktifyn" json:"loper_aktifyn"`
		TotalBTT       int    `gorm:"column:total_btt" json:"total_btt"`
	}

	var list []RawLoper
	if err := database.Raw(queryStr).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query list loper: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// CreateLoper Handler untuk Membuat Manifes Loper Baru
func CreateLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req struct {
		LoperEID       string `json:"loper_eid" binding:"required"`
		LoperAgenID    string `json:"loper_agenid"`
		LoperNoMobil   string `json:"loper_nomobil" binding:"required"`
		LoperNIPSopir  string `json:"loper_nipsopir" binding:"required"`
		LoperKeraniYN  string `json:"loper_keraniyn"`
		LoperNIPKerani string `json:"loper_nipkerani"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Data input tidak valid: " + err.Error()})
		return
	}

	insertSQL := `
		INSERT INTO public.opr_t_eloper (loper_eid, loper_agenid, loper_tanggal, loper_nomobil, loper_nipsopir, loper_keraniyn, loper_nipkerani, loper_aktifyn)
		VALUES (?, ?, NOW(), ?, ?, ?, ?, 'Y')
	`
	if err := database.Exec(insertSQL, req.LoperEID, req.LoperAgenID, req.LoperNoMobil, req.LoperNIPSopir, req.LoperKeraniYN, req.LoperNIPKerani).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan loper baru: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Manifes Loper berhasil dibuat"})
}

// 1. GetApprovedLoadingForLoper Mengambil daftar Loading ID yang sudah Approved & belum dipakai
func GetApprovedLoadingForLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	keyword := strings.TrimSpace(c.Query("q"))
	filterKeyword := ""
	var args []interface{}
	if keyword != "" {
		filterKeyword = " AND UPPER(h.loadh_id) LIKE UPPER(?) "
		args = append(args, "%"+keyword+"%")
	}

	query := fmt.Sprintf(`
		SELECT 
			TRIM(h.loadh_id) AS loadh_id,
			TO_CHAR(h.loadh_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS loadh_tanggal,
			COALESCE(TRIM(h.loadh_kendid), '') AS loadh_kendid,
			COALESCE(TRIM(s.sjh_sopir1_nip), '') AS sopir_nip,
			COALESCE(k.kry_nama, '') AS sopir_nama,
			COUNT(DISTINCT COALESCE(NULLIF(TRIM(e.bttt_id), ''), SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)))::int AS jml_btt,
			COUNT(d.loadd_id)::int AS jml_koli
		FROM public.opr_t_loadingh h
		LEFT JOIN public.opr_t_loadingd d ON TRIM(h.loadh_id) = TRIM(d.loadd_loadhid)
		LEFT JOIN public.mkt_t_econote e 
			ON TRIM(e.bttt_id) = SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)
			OR LEFT(TRIM(d.loadd_bttkoliid), LENGTH(TRIM(e.bttt_id))) = TRIM(e.bttt_id)
		LEFT JOIN public.opr_t_esuratjalan s ON TRIM(h.loadh_id) = TRIM(s.sjh_loadhid)
		LEFT JOIN public.hrd_m_karyawan k ON TRIM(s.sjh_sopir1_nip) = TRIM(k.kry_nip)
		WHERE COALESCE(TRIM(h.loadh_approveyn), 'N') = 'Y'
		  AND NOT EXISTS (SELECT 1 FROM public.opr_t_eloper l WHERE TRIM(l.loper_loadhid) = TRIM(h.loadh_id) AND COALESCE(l.loper_aktifyn, 'Y') = 'Y')
		  AND NOT EXISTS (SELECT 1 FROM public.opr_t_esp_terima sp WHERE TRIM(sp.spt_loadhid) = TRIM(h.loadh_id))
		  %s
		GROUP BY h.loadh_id, h.loadh_tanggal, h.loadh_kendid, s.sjh_sopir1_nip, k.kry_nama
		ORDER BY h.loadh_tanggal DESC, h.loadh_id DESC
		LIMIT 30
	`, filterKeyword)

	type ApprovedLoadingItem struct {
		LoadHID      string `gorm:"column:loadh_id" json:"loadh_id"`
		LoadHTanggal string `gorm:"column:loadh_tanggal" json:"loadh_tanggal"`
		LoadHKendID  string `gorm:"column:loadh_kendid" json:"loadh_kendid"`
		SopirNIP     string `gorm:"column:sopir_nip" json:"sopir_nip"`
		SopirNama    string `gorm:"column:sopir_nama" json:"sopir_nama"`
		JmlBTT       int    `gorm:"column:jml_btt" json:"jml_btt"`
		JmlKoli      int    `gorm:"column:jml_koli" json:"jml_koli"`
	}

	var list []ApprovedLoadingItem
	if err := database.Raw(query, args...).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengambil data loading approved: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// 2. CreateLoperFromLoading Membuat Manifes Loper & otomatis menarik BTT dari Loading ID
func CreateLoperFromLoading(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		LoperAgenID    string   `json:"loper_agenid"`
		LoperTanggal   string   `json:"loper_tanggal"`
		LoperLoadHID   string   `json:"loper_loadhid"`
		LoperNoMobil   string   `json:"loper_nomobil" binding:"required"`
		LoperNIPSopir  string   `json:"loper_nipsopir" binding:"required"`
		LoperKeraniYN  string   `json:"loper_keraniyn"`
		LoperNIPKerani string   `json:"loper_nipkerani"`
		ManualBTTList  []string `json:"manual_btt_list"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Data input tidak lengkap: " + err.Error()})
		return
	}

	userLogin, _ := c.Get("username")
	userNameStr := fmt.Sprintf("%v", userLogin)
	if userNameStr == "" || userNameStr == "<nil>" {
		userNameStr = "ADMIN"
	}

	agenID := strings.TrimSpace(req.LoperAgenID)
	if agenID == "" {
		agenID = "001"
	}

	now := time.Now()
	tglLoper := strings.TrimSpace(req.LoperTanggal)
	if tglLoper == "" {
		tglLoper = now.Format("2006-01-02 15:04:05")
	}

	// Generate Nomor Loper (Format: LP/YYYY/MM/0001)
	prefix := fmt.Sprintf("LP/%s/", now.Format("2006/01"))
	var lastLoperID string
	database.Raw(`SELECT loper_eid FROM public.opr_t_eloper WHERE loper_eid LIKE ? ORDER BY loper_eid DESC LIMIT 1`, prefix+"%").Scan(&lastLoperID)

	urut := 1
	if len(lastLoperID) > len(prefix) {
		fmt.Sscanf(lastLoperID[len(prefix):], "%d", &urut)
		urut++
	}
	newLoperID := fmt.Sprintf("%s%04d", prefix, urut)

	// Generate 6 digit OTP Loper
	otpCode := fmt.Sprintf("%06d", now.UnixNano()%1000000)

	keraniYN := "N"
	nipKerani := ""
	if strings.ToUpper(req.LoperKeraniYN) == "Y" || strings.ToUpper(req.LoperKeraniYN) == "YA" {
		keraniYN = "Y"
		nipKerani = strings.TrimSpace(req.LoperNIPKerani)
	}

	if strings.TrimSpace(req.LoperLoadHID) != "" {
		var appStatus string
		database.Raw(`SELECT COALESCE(TRIM(loadh_approveyn), 'N') FROM public.opr_t_loadingh WHERE UPPER(TRIM(loadh_id)) = UPPER(TRIM(?)) LIMIT 1`, req.LoperLoadHID).Scan(&appStatus)
		if appStatus == "" {
			c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Nomor Loading tidak ditemukan!"})
			return
		}
		if strings.ToUpper(appStatus) != "Y" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Nomor Loading Ini Belum Dapat Diproses, Dikarenakan Belum DiApprove. Silahkan Koordinasikan Dengan Penanggung Jawab Operasional.",
			})
			return
		}
	}

	tx := database.Begin()

	// A. Insert Header ke opr_t_eloper
	insertHeaderSQL := `
		INSERT INTO public.opr_t_eloper (
			loper_eid, loper_agenid, loper_tanggal, loper_nipsopir,
			loper_keraniyn, loper_nipkerani, loper_nomobil, loper_aktifyn,
			loper_updateid, loper_updatetime, loper_otp, loper_loadhid
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'Y', ?, CURRENT_TIMESTAMP, ?, NULLIF(TRIM(?), ''))
	`
	if err := tx.Exec(
		insertHeaderSQL,
		newLoperID, agenID, tglLoper, strings.TrimSpace(req.LoperNIPSopir),
		keraniYN, nipKerani, strings.ToUpper(strings.TrimSpace(req.LoperNoMobil)),
		userNameStr, otpCode, req.LoperLoadHID,
	).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membuat header loper: " + err.Error()})
		return
	}

	// B. Jika dibuat dari Loading ID, tarik semua BTT unik dari opr_t_loadingd ke opr_t_eloperdetail
	totalInsertedBTT := 0
	if strings.TrimSpace(req.LoperLoadHID) != "" {
		var bttIDs []string
		tx.Raw(`
			SELECT DISTINCT COALESCE(NULLIF(TRIM(e.bttt_id), ''), SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)) AS btt_id
			FROM public.opr_t_loadingd d
			LEFT JOIN public.mkt_t_econote e 
				ON TRIM(e.bttt_id) = SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)
				OR LEFT(TRIM(d.loadd_bttkoliid), LENGTH(TRIM(e.bttt_id))) = TRIM(e.bttt_id)
			WHERE TRIM(d.loadd_loadhid) = TRIM(?)
		`, req.LoperLoadHID).Scan(&bttIDs)

		for _, btt := range bttIDs {
			bttClean := strings.TrimSpace(btt)
			if bttClean == "" {
				continue
			}
			if err := tx.Exec(`
				INSERT INTO public.opr_t_eloperdetail (loperd_eloperid, loperd_bttid, loperd_updatetime)
				VALUES (?, ?, CURRENT_TIMESTAMP)
			`, newLoperID, bttClean).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memasukkan BTT " + bttClean + ": " + err.Error()})
				return
			}
			totalInsertedBTT++
		}
	}

	// C. Jika ada input BTT manual tambahan
	for _, btt := range req.ManualBTTList {
		bttClean := strings.ToUpper(strings.TrimSpace(btt))
		if bttClean == "" {
			continue
		}
		tx.Exec(`
			INSERT INTO public.opr_t_eloperdetail (loperd_eloperid, loperd_bttid, loperd_updatetime)
			VALUES (?, ?, CURRENT_TIMESTAMP)
		`, newLoperID, bttClean)
		totalInsertedBTT++
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"loper_eid": newLoperID,
		"otp":       otpCode,
		"total_btt": totalInsertedBTT,
		"message":   fmt.Sprintf("Manifes Loper %s berhasil dibuat dengan %d BTT!", newLoperID, totalInsertedBTT),
	})
}

// 3. GetLoperDetail Mengambil Detail Header & Daftar BTT untuk Edit & Print Loper
func GetLoperDetail(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	loperID := strings.TrimSpace(c.Query("loper_id"))
	if loperID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter loper_id wajib diisi"})
		return
	}

	var header map[string]interface{}
	if err := database.Raw(`
		SELECT 
			l.loper_eid,
			TO_CHAR(l.loper_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS loper_tanggal,
			COALESCE(l.loper_nomobil, '-') AS loper_nomobil,
			COALESCE(l.loper_nipsopir, '-') AS loper_nipsopir,
			COALESCE(s.kry_nama, '-') AS nama_sopir,
			COALESCE(l.loper_keraniyn, 'N') AS loper_keraniyn,
			COALESCE(l.loper_nipkerani, '-') AS loper_nipkerani,
			COALESCE(k.kry_nama, '-') AS nama_kerani,
			COALESCE(CAST(l.loper_otp AS VARCHAR), '-') AS loper_otp,
			COALESCE(l.loper_loadhid, '-') AS loper_loadhid,
			COALESCE(l.loper_updateid, '-') AS loper_updateid
		FROM public.opr_t_eloper l
		LEFT JOIN public.hrd_m_karyawan s ON TRIM(l.loper_nipsopir) = TRIM(s.kry_nip)
		LEFT JOIN public.hrd_m_karyawan k ON TRIM(l.loper_nipkerani) = TRIM(k.kry_nip)
		WHERE TRIM(l.loper_eid) = TRIM(?)
		LIMIT 1
	`, loperID).Scan(&header).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query header loper: " + err.Error()})
		return
	}

	var details []map[string]interface{}
	if errDet := database.Raw(`
		SELECT 
			TRIM(d.loperd_bttid) AS no_btt,
			COALESCE(e.bttt_nosuratjalan, '-') AS surat_jalan,
			COALESCE(TO_CHAR(e.bttt_tanggal, 'MM/DD/YYYY'), '-') AS tgl_btt,
			COALESCE(e.bttt_asalname, '-') AS pengirim,
			COALESCE(e.bttt_tujuannama, '-') AS penerima,
			COALESCE(e.bttt_up, '-') AS up_penerima,
			COALESCE(e.bttt_tujuanalamat, '-') AS alamat,
			COALESCE(e.bttt_tujuankota, '-') AS kota,
			COALESCE(e.bttt_ket, '') AS keterangan,
			COALESCE(e.bttt_jmlunit, 1) AS jml_koli,
			COALESCE(e.bttt_berat, 0) AS berat,
			COALESCE(e.bttt_beratvol, 0) AS berat_vol,
			COALESCE(NULLIF(TRIM(e.bttt_ukuran), ''), '0') AS ukuran,
			COALESCE(e.bttt_harga, 0) AS harga,
			COALESCE(e.bttt_tagihtujuan, 0) AS cod,
			'' AS status_bbl,
			'' AS jam_bbl,
			'' AS input_oleh
		FROM public.opr_t_eloperdetail d
		LEFT JOIN public.mkt_t_econote e ON TRIM(d.loperd_bttid) = TRIM(e.bttt_id)
		WHERE TRIM(d.loperd_eloperid) = TRIM(?)
		ORDER BY d.loperd_bttid DESC
	`, loperID).Scan(&details).Error; errDet != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query detail BTT: " + errDet.Error()})
		return
	}

	for idx, row := range details {
		noBtt := fmt.Sprintf("%v", row["no_btt"])
		var bblInfo struct {
			StatusBBL string `gorm:"column:status_bbl"`
			JamBBL    string `gorm:"column:jam_bbl"`
			InputOleh string `gorm:"column:input_oleh"`
		}
		database.Raw(`
			SELECT 
				CASE WHEN bbl_terimayn = 'Y' THEN 'Diterima' WHEN bbl_terimayn = 'N' THEN 'Gagal' ELSE '' END AS status_bbl,
				COALESCE(TO_CHAR(bbl_tanggal, 'HH24:MI'), '') AS jam_bbl,
				COALESCE(bbl_updateid, '') AS input_oleh
			FROM public.opr_t_ebbl
			WHERE TRIM(bbl_bttid) = TRIM(?)
			ORDER BY bbl_tanggal DESC
			LIMIT 1
		`, noBtt).Scan(&bblInfo)

		details[idx]["status_bbl"] = bblInfo.StatusBBL
		details[idx]["jam_bbl"] = bblInfo.JamBBL
		details[idx]["input_oleh"] = bblInfo.InputOleh
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"header":  header,
		"details": details,
	})
}

// 4. DeleteLoper Membatalkan/Menonaktifkan Manifes Loper (loper_aktifyn = 'N')
func DeleteLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	loperID := strings.TrimSpace(c.Query("loper_id"))
	if loperID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter loper_id wajib diisi"})
		return
	}

	database.Exec(`DELETE FROM public.opr_t_eloperdetail WHERE TRIM(loperd_eloperid) = TRIM(?)`, loperID)
	if err := database.Exec(`DELETE FROM public.opr_t_eloper WHERE TRIM(loper_eid) = TRIM(?)`, loperID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus Manifes Loper: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Manifes Loper %s berhasil dihapus permanen.", loperID)})
}


// 5. UpdateLoperHeader Menyimpan perubahan header Manifes Loper (Tanggal, Mobil, Sopir, Kerani)
func UpdateLoperHeader(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		LoperEID       string `json:"loper_eid" binding:"required"`
		LoperTanggal   string `json:"loper_tanggal" binding:"required"`
		LoperNoMobil   string `json:"loper_nomobil" binding:"required"`
		LoperNIPSopir  string `json:"loper_nipsopir" binding:"required"`
		LoperKeraniYN  string `json:"loper_keraniyn"`
		LoperNIPKerani string `json:"loper_nipkerani"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	userLogin, _ := c.Get("username")
	userNameStr := fmt.Sprintf("%v", userLogin)
	if userNameStr == "" || userNameStr == "<nil>" {
		userNameStr = "ADMIN"
	}

	keraniYN := "N"
	nipKerani := ""
	if strings.ToUpper(req.LoperKeraniYN) == "Y" || strings.ToUpper(req.LoperKeraniYN) == "YA" {
		keraniYN = "Y"
		nipKerani = strings.TrimSpace(req.LoperNIPKerani)
	}

	err := database.Exec(`
		UPDATE public.opr_t_eloper
		SET loper_tanggal = ?,
		    loper_nomobil = ?,
		    loper_nipsopir = ?,
		    loper_keraniyn = ?,
		    loper_nipkerani = ?,
		    loper_updateid = ?,
		    loper_updatetime = CURRENT_TIMESTAMP
		WHERE TRIM(loper_eid) = TRIM(?)
	`, req.LoperTanggal, strings.ToUpper(strings.TrimSpace(req.LoperNoMobil)),
		strings.TrimSpace(req.LoperNIPSopir), keraniYN, nipKerani, userNameStr, req.LoperEID).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update header loper: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data header Manifes Loper berhasil diperbarui!"})
}

// 6. AddBTTToLoper Menambahkan BTT via Scan Barcode (BTT Naik/Turun, SJ Naik/Turun) atau Checklist
func AddBTTToLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		LoperEID string   `json:"loper_eid" binding:"required"`
		Mode     string   `json:"mode"` // "BTT_NAIK", "SJ_NAIK", "BTT_TURUN", "SJ_TURUN", "CHECKLIST"
		KodeScan string   `json:"kode_scan"`
		BTTList  []string `json:"btt_list"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid"})
		return
	}

	loperID := strings.TrimSpace(req.LoperEID)
	kode := strings.ToUpper(strings.TrimSpace(req.KodeScan))
	mode := strings.ToUpper(strings.TrimSpace(req.Mode))

	var targetBTTs []string

	if len(req.BTTList) > 0 {
		for _, b := range req.BTTList {
			if clean := strings.ToUpper(strings.TrimSpace(b)); clean != "" {
				targetBTTs = append(targetBTTs, clean)
			}
		}
	} else if kode != "" {
		if strings.HasPrefix(mode, "SJ_") {
			// Scan Surat Jalan -> Tarik semua BTT yang memiliki bttt_nosuratjalan sesuai kode scan
			database.Raw(`
				SELECT DISTINCT TRIM(bttt_id)
				FROM public.mkt_t_econote
				WHERE UPPER(TRIM(bttt_nosuratjalan)) LIKE ? AND COALESCE(bttt_aktifyn, 'Y') = 'Y'
			`, "%"+kode+"%").Scan(&targetBTTs)

			if len(targetBTTs) == 0 {
				c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": fmt.Sprintf("Tidak ditemukan BTT pada Surat Jalan [%s]!", kode)})
				return
			}
		} else {
			// Scan BTT (mendukung kode BTT 12 digit maupun stiker koli 16/20 digit)
			noBTT := kode
			if strings.Contains(kode, "-") {
				noBTT = strings.Split(kode, "-")[0]
			} else if len(kode) == 20 {
				noBTT = kode[:16]
			} else if len(kode) > 16 {
				noBTT = kode[:len(kode)-4]
			}

			var validID string
			database.Raw(`SELECT TRIM(bttt_id) FROM public.mkt_t_econote WHERE TRIM(bttt_id) = ? LIMIT 1`, noBTT).Scan(&validID)
			if validID == "" {
				c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": fmt.Sprintf("Nomor BTT [%s] tidak terdaftar dalam sistem!", kode)})
				return
			}
			targetBTTs = append(targetBTTs, validID)
		}
	}

	insertedCount := 0
	for _, btt := range targetBTTs {
		var existsCount int64
		database.Raw(`
			SELECT COUNT(*) FROM public.opr_t_eloperdetail
			WHERE TRIM(loperd_eloperid) = TRIM(?) AND TRIM(loperd_bttid) = TRIM(?)
		`, loperID, btt).Scan(&existsCount)

		if existsCount == 0 {
			if err := database.Exec(`
				INSERT INTO public.opr_t_eloperdetail (loperd_eloperid, loperd_bttid, loperd_updatetime)
				VALUES (?, ?, CURRENT_TIMESTAMP)
			`, loperID, btt).Error; err == nil {
				insertedCount++
			}
		}
	}

	if insertedCount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "warning", "message": "BTT tersebut sudah terdaftar di Manifes Loper ini!"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("%d BTT berhasil ditambahkan ke Manifes Loper!", insertedCount),
	})
}

// 7. RemoveBTTFromLoper Menghapus 1 atau beberapa BTT dari Manifes Loper
func RemoveBTTFromLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		LoperEID string   `json:"loper_eid" binding:"required"`
		BTTList  []string `json:"btt_list" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || len(req.BTTList) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Pilih minimal 1 BTT yang akan dihapus"})
		return
	}

	for _, btt := range req.BTTList {
		database.Exec(`
			DELETE FROM public.opr_t_eloperdetail
			WHERE TRIM(loperd_eloperid) = TRIM(?) AND TRIM(loperd_bttid) = TRIM(?)
		`, req.LoperEID, btt)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("%d BTT berhasil dihapus dari Manifes Loper.", len(req.BTTList))})
}

// 8. GetAvailableBTTForLoper Mengambil daftar BTT Naik / BTT Turun yang belum masuk Loper
func GetAvailableBTTForLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	tipe := strings.ToUpper(strings.TrimSpace(c.Query("tipe"))) // "NAIK" atau "TURUN"
	agenID := strings.TrimSpace(c.Query("agen_id"))
	if agenID == "" {
		agenID = "001"
	}
	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	bypassTgl := strings.TrimSpace(c.Query("bypass_tgl")) == "true"

	whereClause := " WHERE COALESCE(e.bttt_aktifyn, 'Y') = 'Y' "
	var args []interface{}

	if tipe == "NAIK" {
		whereClause += " AND TRIM(e.bttt_asalagenid) = TRIM(?) "
		args = append(args, agenID)
	} else {
		whereClause += " AND TRIM(e.bttt_tujuanagenid) = TRIM(?) "
		args = append(args, agenID)
	}

	if !bypassTgl && tgla != "" && tgle != "" {
		whereClause += " AND CAST(e.bttt_tanggal AS DATE) BETWEEN ? AND ? "
		args = append(args, tgla, tgle)
	}

	// Hanya tampilkan BTT yang belum masuk manifes loper aktif
	whereClause += `
		AND NOT EXISTS (
			SELECT 1 FROM public.opr_t_eloperdetail d
			INNER JOIN public.opr_t_eloper l ON TRIM(l.loper_eid) = TRIM(d.loperd_eloperid)
			WHERE TRIM(d.loperd_bttid) = TRIM(e.bttt_id) AND COALESCE(l.loper_aktifyn, 'Y') = 'Y'
		)
	`

	query := fmt.Sprintf(`
		SELECT 
			TRIM(e.bttt_id) AS no_btt,
			COALESCE(e.bttt_nosuratjalan, '-') AS surat_jalan,
			TO_CHAR(e.bttt_tanggal, 'DD/MM/YYYY') AS tgl_btt,
			COALESCE(e.bttt_asalname, '-') AS pengirim,
			COALESCE(e.bttt_tujuannama, '-') AS penerima,
			COALESCE(e.bttt_tujuanalamat, '-') AS alamat,
			COALESCE(e.bttt_tujuankota, '-') AS kota,
			COALESCE(e.bttt_jmlunit, 1) AS jml_koli,
			COALESCE(e.bttt_berat, 0) AS berat,
			COALESCE(e.bttt_tagihtujuan, 0) AS cod
		FROM public.mkt_t_econote e
		%s
		ORDER BY e.bttt_tanggal DESC, e.bttt_id DESC
		LIMIT 100
	`, whereClause)

	var list []map[string]interface{}
	if err := database.Raw(query, args...).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat daftar BTT: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// CheckLoadingForLoper Validasi Nomor Loading & Ambil Summary + Detail BTT (Sesuai alur ASP p-opr_t_loper_a -> d -> f)
func CheckLoadingForLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	loID := strings.ToUpper(strings.TrimSpace(c.Query("loadh_id")))
	if loID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Loading Barang wajib diisi!"})
		return
	}

	// 1. Cek Header Loading + Relasi ke Surat Tugas + Cek apakah sudah pernah dibuatkan Loper / SP
	var info struct {
		LoadHID        string `gorm:"column:loadh_id" json:"loadh_id"`
		LoadHTanggal   string `gorm:"column:loadh_tanggal" json:"loadh_tanggal"`
		LoadHKendID    string `gorm:"column:loadh_kendid" json:"loadh_kendid"`
		LoadHApproveYN string `gorm:"column:loadh_approveyn" json:"loadh_approveyn"`
		SjhID          string `gorm:"column:sjh_id" json:"sjh_id"`
		Sopir1NIP      string `gorm:"column:sopir1_nip" json:"sopir1_nip"`
		Sopir1Nama     string `gorm:"column:sopir1_nama" json:"sopir1_nama"`
		Sopir2NIP      string `gorm:"column:sopir2_nip" json:"sopir2_nip"`
		Sopir2Nama     string `gorm:"column:sopir2_nama" json:"sopir2_nama"`
		ExistingLoper  string `gorm:"column:existing_loper" json:"existing_loper"`
	}

	err := database.Raw(`
		SELECT 
			TRIM(h.loadh_id) AS loadh_id,
			TO_CHAR(h.loadh_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS loadh_tanggal,
			COALESCE(NULLIF(TRIM(s.sjh_kendid), ''), TRIM(h.loadh_kendid), '') AS loadh_kendid,
			COALESCE(TRIM(h.loadh_approveyn), 'N') AS loadh_approveyn,
			COALESCE(TRIM(s.sjh_id), '-') AS sjh_id,
			COALESCE(TRIM(s.sjh_sopir1_nip), '') AS sopir1_nip,
			COALESCE(k1.kry_nama, '') AS sopir1_nama,
			COALESCE(TRIM(s.sjh_sopir2_nip), '') AS sopir2_nip,
			COALESCE(k2.kry_nama, '') AS sopir2_nama,
			COALESCE((
				SELECT TRIM(l.loper_eid) FROM public.opr_t_eloper l 
				WHERE TRIM(l.loper_loadhid) = TRIM(h.loadh_id) AND COALESCE(l.loper_aktifyn, 'Y') = 'Y' 
				LIMIT 1
			), '') AS existing_loper
		FROM public.opr_t_loadingh h
		LEFT JOIN public.opr_t_esuratjalan s ON TRIM(h.loadh_id) = TRIM(s.sjh_loadhid)
		LEFT JOIN public.hrd_m_karyawan k1 ON TRIM(s.sjh_sopir1_nip) = TRIM(k1.kry_nip)
		LEFT JOIN public.hrd_m_karyawan k2 ON TRIM(s.sjh_sopir2_nip) = TRIM(k2.kry_nip)
		WHERE UPPER(TRIM(h.loadh_id)) = ?
		LIMIT 1
	`, loID).Scan(&info).Error

	if err != nil || info.LoadHID == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Nomor Loading [%s] tidak ditemukan dalam sistem!", loID),
		})
		return
	}

	// 2. VALIDASI PERSIS SEPERTI IMAGE 2 (p-opr_t_loper_a.asp)
	if strings.ToUpper(info.LoadHApproveYN) != "Y" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "not_approved",
			"message": "Nomor Loading Ini Belum Dapat Diproses, Dikarenakan Belum DiApprove. Silahkan Koordinasikan Dengan Penanggung Jawab Operasional.",
		})
		return
	}

	// 3. Ambil Rincian BTT yang ada di Loading ID tersebut (persis seperti opr_t_loper_f.asp)
	var bttDetails []map[string]interface{}
	database.Raw(`
		SELECT 
			COALESCE(NULLIF(TRIM(e.bttt_id), ''), SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)) AS no_btt,
			COALESCE(e.bttt_nosuratjalan, '-') AS surat_jalan,
			COALESCE(TO_CHAR(e.bttt_tanggal, 'DD/MM/YYYY'), '-') AS tgl_btt,
			COALESCE(e.bttt_asalname, '-') AS pengirim,
			COALESCE(e.bttt_tujuannama, '-') AS penerima,
			COALESCE(e.bttt_tujuanalamat, '-') AS alamat,
			COALESCE(e.bttt_tujuankota, '-') AS kota,
			COALESCE(e.bttt_tujuantelp, '-') AS telp,
			COUNT(d.loadd_id)::int AS jml_koli,
			GREATEST(COALESCE(e.bttt_berat, 0), COALESCE(e.bttt_beratvol, 0)) AS berat
		FROM public.opr_t_loadingd d
		LEFT JOIN public.mkt_t_econote e 
			ON TRIM(e.bttt_id) = SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)
			OR LEFT(TRIM(d.loadd_bttkoliid), LENGTH(TRIM(e.bttt_id))) = TRIM(e.bttt_id)
		WHERE UPPER(TRIM(d.loadd_loadhid)) = ?
		GROUP BY 
			COALESCE(NULLIF(TRIM(e.bttt_id), ''), SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)),
			e.bttt_nosuratjalan, e.bttt_tanggal, e.bttt_asalname, e.bttt_tujuannama,
			e.bttt_tujuanalamat, e.bttt_tujuankota, e.bttt_tujuantelp, e.bttt_berat, e.bttt_beratvol
		ORDER BY no_btt ASC
	`, loID).Scan(&bttDetails)

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"summary":     info,
		"btt_details": bttDetails,
		"jml_btt":     len(bttDetails),
	})
}