package handler

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 🔍 1. GET LIST SURAT TUGAS / SURAT JALAN
func GetSuratTugasList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	noSt := strings.TrimSpace(c.Query("no_st"))
	noMobil := strings.TrimSpace(c.Query("no_mobil"))
	sopirNama := strings.TrimSpace(c.Query("sopir_nama"))
	chkTgl := strings.TrimSpace(c.Query("chktgl"))

	filterClause := ""

	if (chkTgl == "true" || chkTgl == "on") && tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(s.sjh_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	if noSt != "" {
		filterClause += fmt.Sprintf(` AND UPPER(s.sjh_id) LIKE UPPER('%%%s%%') `, noSt)
	}

	if noMobil != "" {
		filterClause += fmt.Sprintf(` AND UPPER(s.sjh_kendid) LIKE UPPER('%%%s%%') `, noMobil)
	}

	if sopirNama != "" {
		filterClause += fmt.Sprintf(` AND (UPPER(sp1.kry_nama) LIKE UPPER('%%%s%%') OR UPPER(sp2.kry_nama) LIKE UPPER('%%%s%%') OR UPPER(s.sjh_sopir1_nip) LIKE UPPER('%%%s%%')) `, sopirNama, sopirNama, sopirNama)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			COALESCE(s.sjh_id, '-') AS sjh_id,
			TO_CHAR(s.sjh_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS sjh_tanggal,
			TO_CHAR(s.sjh_tanggalkembali, 'YYYY-MM-DD HH24:MI:SS') AS sjh_tanggalkembali,
			COALESCE(s.sjh_kendid, '-') AS sjh_kendid,
			COALESCE(s.sjh_sopir1_nip, '-') AS sjh_sopir1_nip,
			COALESCE(s.sjh_sopir2_nip, '-') AS sjh_sopir2_nip,
			COALESCE(sp1.kry_nama, s.sjh_sopir1_nip, '-') AS nmsopir1,
			COALESCE(sp2.kry_nama, s.sjh_sopir2_nip, '-') AS nmsopir2,
			COALESCE(s.sjh_assid, '-') AS sjh_assid,
			COALESCE(a.ass_nama, s.sjh_assid, '-') AS ass_nama,
			COALESCE(cStart.agen_nama, s.sjh_startagenid, '-') AS cbgmulai,
			COALESCE(cEnd.agen_nama, s.sjh_endagenid, '-') AS cbgselesai,
			COALESCE(s.sjh_keterangan, '-') AS sjh_keterangan,
			COALESCE(s.sjh_completeyn, 'N') AS sjh_completeyn,
			COALESCE(s.sjh_approve, '') AS sjh_approve,
			COALESCE(s.sjh_aktifyn, 'Y') AS sjh_aktifyn,
			COALESCE(r.sjr_nominalum, r.sjr_nominal, 0) AS nominal_um
		FROM public.opr_t_esuratjalan s
		LEFT OUTER JOIN public.hrd_m_karyawan sp1 ON CAST(s.sjh_sopir1_nip AS VARCHAR) = CAST(sp1.kry_nip AS VARCHAR)
		LEFT OUTER JOIN public.hrd_m_karyawan sp2 ON CAST(s.sjh_sopir2_nip AS VARCHAR) = CAST(sp2.kry_nip AS VARCHAR)
		LEFT OUTER JOIN public.glb_m_agen cStart ON CAST(s.sjh_startagenid AS VARCHAR) = CAST(cStart.agen_id AS VARCHAR)
		LEFT OUTER JOIN public.glb_m_agen cEnd ON CAST(s.sjh_endagenid AS VARCHAR) = CAST(cEnd.agen_id AS VARCHAR)
		LEFT OUTER JOIN public.opr_m_assignment a ON CAST(s.sjh_assid AS VARCHAR) = CAST(a.ass_id AS VARCHAR)
		LEFT OUTER JOIN public.opr_t_esuratjalanrealisasi r ON s.sjh_id = r.sjr_sjhid
		WHERE 1=1 %s
		ORDER BY s.sjh_tanggal DESC, s.sjh_id DESC
	`, filterClause)

	type RawResult struct {
		SJHID             string  `gorm:"column:sjh_id" json:"sjh_id"`
		SJHTanggal        string  `gorm:"column:sjh_tanggal" json:"sjh_tanggal"`
		SJHTanggalKembali string  `gorm:"column:sjh_tanggalkembali" json:"sjh_tanggalkembali"`
		SJHKendID         string  `gorm:"column:sjh_kendid" json:"sjh_kendid"`
		SJHSopir1Nip      string  `gorm:"column:sjh_sopir1_nip" json:"sjh_sopir1_nip"`
		SJHSopir2Nip      string  `gorm:"column:sjh_sopir2_nip" json:"sjh_sopir2_nip"`
		NmSopir1          string  `gorm:"column:nmsopir1" json:"nmsopir1"`
		NmSopir2          string  `gorm:"column:nmsopir2" json:"nmsopir2"`
		SJHAssID          string  `gorm:"column:sjh_assid" json:"sjh_assid"`
		AssNama           string  `gorm:"column:ass_nama" json:"ass_nama"`
		CbgMulai          string  `gorm:"column:cbgmulai" json:"cbgmulai"`
		CbgSelesai        string  `gorm:"column:cbgselesai" json:"cbgselesai"`
		SJHKeterangan     string  `gorm:"column:sjh_keterangan" json:"sjh_keterangan"`
		SJHCompleteYN     string  `gorm:"column:sjh_completeyn" json:"sjh_completeyn"`
		SJHApprove        string  `gorm:"column:sjh_approve" json:"sjh_approve"`
		SJHAktifYN        string  `gorm:"column:sjh_aktifyn" json:"sjh_aktifyn"`
		NominalUM         float64 `gorm:"column:nominal_um" json:"nominal_um"`
	}

	var rawList []RawResult
	if err := database.Raw(queryStr).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query Surat Tugas: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   rawList,
		"count":  len(rawList),
	})
}

// 💾 2. CREATE SURAT TUGAS / SURAT JALAN
// 💾 2. CREATE SURAT TUGAS / SURAT JALAN
func CreateSuratTugas(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.SuratTugasReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input data tidak valid: " + err.Error()})
		return
	}

	nowTime := time.Now()
	nowStr := nowTime.Format("2006-01-02 15:04:05")

	if req.SJHTanggal == "" {
		req.SJHTanggal = nowStr
	}

	// 🌟 AUTO-GENERATE NOMOR SURAT TUGAS JIKA KOSONG DARI FORM TAMBAH
	if strings.TrimSpace(req.SJHID) == "" {
		cabangCode := strings.TrimSpace(req.SJHStartAgenID)
		if len(cabangCode) >= 3 {
			cabangCode = cabangCode[:3]
		} else {
			cabangCode = "001"
		}

		datePrefix := nowTime.Format("20060102") // YYYYMMDD
		prefix := fmt.Sprintf("%sJC%s", cabangCode, datePrefix)

		var lastID string
		database.Raw(`
            SELECT sjh_id 
            FROM public.opr_t_esuratjalan 
            WHERE sjh_id LIKE ? 
            ORDER BY sjh_id DESC 
            LIMIT 1
        `, prefix+"%").Scan(&lastID)

		urut := 1
		if len(lastID) >= len(prefix)+4 {
			suffix := lastID[len(prefix):]
			fmt.Sscanf(suffix, "%d", &urut)
			urut++
		}
		req.SJHID = fmt.Sprintf("%s%04d", prefix, urut)
	}

	insertHeader := `
        INSERT INTO public.opr_t_esuratjalan (
            sjh_id, sjh_tanggal, sjh_tanggalkembali, sjh_kendid, sjh_sopir1_nip, sjh_sopir2_nip,
            sjh_assid, sjh_trayekid, sjh_startagenid, sjh_endagenid, sjh_keterangan,
            sjh_completeyn, sjh_aktifyn, sjh_updateid, sjh_updatetime
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'N', 'Y', ?, ?)
    `

	if err := database.Exec(
		insertHeader,
		req.SJHID, req.SJHTanggal, req.SJHTanggalKembali, req.SJHKendID, req.SJHSopir1Nip, req.SJHSopir2Nip,
		req.SJHAssID, req.SJHTrayekID, req.SJHStartAgenID, req.SJHEndAgenID, req.SJHKeterangan,
		fmt.Sprintf("%v", username), nowStr,
	).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan header Surat Tugas: " + err.Error()})
		return
	}

	if req.NominalUM > 0 {
		insertUM := `
            INSERT INTO public.opr_t_esuratjalanrealisasi (sjr_sjhid, sjr_ket, sjr_nominal, sjr_nominalum)
            VALUES (?, 'UANG MUKA OPERASIONAL', ?, ?)
        `
		database.Exec(insertUM, req.SJHID, req.NominalUM, req.NominalUM)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Surat Tugas nomor %s berhasil direkam!", req.SJHID),
		"sjh_id":  req.SJHID,
	})
}

// ✏️ 3. UPDATE SURAT TUGAS
func UpdateSuratTugas(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.SuratTugasReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	updateHeader := `
		UPDATE public.opr_t_esuratjalan 
		SET sjh_kendid = ?, 
		    sjh_sopir1_nip = ?, 
		    sjh_sopir2_nip = ?, 
		    sjh_assid = ?, 
		    sjh_startagenid = ?, 
		    sjh_endagenid = ?, 
		    sjh_keterangan = ?, 
		    sjh_updateid = ?, 
		    sjh_updatetime = ?
		WHERE sjh_id = ?
	`

	if err := database.Exec(
		updateHeader,
		req.SJHKendID, req.SJHSopir1Nip, req.SJHSopir2Nip, req.SJHAssID,
		req.SJHStartAgenID, req.SJHEndAgenID, req.SJHKeterangan,
		fmt.Sprintf("%v", username), now, req.SJHID,
	).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui Surat Tugas: " + err.Error()})
		return
	}

	database.Exec(`DELETE FROM public.opr_t_esuratjalanrealisasi WHERE sjr_sjhid = ?`, req.SJHID)
	if req.NominalUM > 0 {
		insertUM := `
			INSERT INTO public.opr_t_esuratjalanrealisasi (sjr_sjhid, sjr_ket, sjr_nominal, sjr_nominalum)
			VALUES (?, 'UANG MUKA OPERASIONAL', ?, ?)
		`
		database.Exec(insertUM, req.SJHID, req.NominalUM, req.NominalUM)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Surat Tugas berhasil diperbarui!",
	})
}

// 🗑️ 4. DELETE SURAT TUGAS
func DeleteSuratTugas(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	stID := c.Query("sjh_id")
	if stID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter sjh_id wajib diisi"})
		return
	}

	database.Exec(`DELETE FROM public.opr_t_esuratjalandetil WHERE sjd_sjhid = ?`, stID)
	database.Exec(`DELETE FROM public.opr_t_esuratjalanrealisasi WHERE sjr_sjhid = ?`, stID)
	if err := database.Exec(`DELETE FROM public.opr_t_esuratjalan WHERE sjh_id = ?`, stID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus Surat Tugas: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Surat Tugas berhasil dihapus!",
	})
}

// 🔍 5. GET SEARCH UANG MUKA (TRAYEK + RINCIAN BIAYA)
// 🔍 5. GET SEARCH UANG MUKA (TRAYEK + RINCIAN BIAYA)
func GetUangMukaList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	search := strings.TrimSpace(c.Query("q"))

	// 1. Ambil Header Trayek Uang Muka
	queryHeader := `
		SELECT 
			RTRIM(LTRIM(trh_id)) AS trh_id, 
			RTRIM(LTRIM(trh_name)) AS trh_name 
		FROM public.opr_m_trayekh 
		WHERE trh_aktifyn = 'Y' 
		  AND (UPPER(trh_id) LIKE UPPER(?) OR UPPER(trh_name) LIKE UPPER(?))
		ORDER BY trh_name ASC
		LIMIT 25
	`

	type UMRincian struct {
		NamaBiaya string  `json:"nama_biaya"`
		Nominal   float64 `json:"nominal"`
	}

	type UMHeader struct {
		KodeUangMuka string      `json:"kode_uang_muka"`
		Nama         string      `json:"nama"`
		Total        float64     `json:"total"`
		Details      []UMRincian `json:"details"`
	}

	type HeaderRow struct {
		TRHID   string `gorm:"column:trh_id" json:"trh_id"`
		TRHName string `gorm:"column:trh_name" json:"trh_name"`
	}

	var headers []HeaderRow
	likeParam := "%" + search + "%"
	if err := database.Raw(queryHeader, likeParam, likeParam).Scan(&headers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query uang muka: " + err.Error()})
		return
	}

	// 2. Query Detail Rincian Biaya per Trayek (Langsung ambil dari opr_m_trayekbplk tanpa join tabel lain)
	queryDetail := `
		SELECT 
			'BIAYA OPERASIONAL' AS nama_biaya,
			COALESCE(tri_nominal, 0) AS nominal
		FROM public.opr_m_trayekbplk
		WHERE RTRIM(LTRIM(tri_hid)) = RTRIM(LTRIM(?)) 
		  AND tri_umyn = 'Y'
		ORDER BY tri_bplksubid ASC
	`

	type DetailRow struct {
		NamaBiaya string  `gorm:"column:nama_biaya"`
		Nominal   float64 `gorm:"column:nominal"`
	}

	var result []UMHeader
	for _, h := range headers {
		if h.TRHID == "" {
			continue
		}

		var details []DetailRow
		database.Raw(queryDetail, h.TRHID).Scan(&details)

		var total float64
		var detailList []UMRincian
		for _, d := range details {
			total += d.Nominal
			detailList = append(detailList, UMRincian{
				NamaBiaya: d.NamaBiaya,
				Nominal:   d.Nominal,
			})
		}

		result = append(result, UMHeader{
			KodeUangMuka: h.TRHID,
			Nama:         h.TRHName,
			Total:        total,
			Details:      detailList,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   result,
	})
}

// 🖨️ 6. GET DETAIL SURAT TUGAS UNTUK CETAK (PRINT)
func GetSuratTugasDetailPrint(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	sjhID := strings.TrimSpace(c.Query("sjh_id"))
	if sjhID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter sjh_id wajib disertakan"})
		return
	}

	// 1. Ambil Header Surat Tugas
	queryHeader := `
		SELECT 
			s.sjh_id,
			TO_CHAR(s.sjh_tanggal, 'DD/MM/YYYY HH24:MI:SS') AS sjh_tanggal,
			TO_CHAR(s.sjh_tanggalkembali, 'DD/MM/YYYY HH24:MI:SS') AS sjh_tanggalkembali,
			COALESCE(s.sjh_kendid, '-') AS sjh_kendid,
			COALESCE(s.sjh_sopir1_nip, '-') AS sjh_sopir1_nip,
			COALESCE(s.sjh_sopir2_nip, '-') AS sjh_sopir2_nip,
			COALESCE(sp1.kry_nama, '-') AS nmsopir1,
			COALESCE(sp2.kry_nama, '-') AS nmsopir2,
			COALESCE(a.ass_nama, '-') AS ass_nama,
			COALESCE(s.sjh_trayekid, '') AS sjh_trayekid,
			COALESCE(trh.trh_name, '') AS nama_trayek,
			COALESCE(cStart.agen_nama, 'BEKASI KOTA') AS cbgmulai,
			TO_CHAR(CURRENT_DATE, 'DD TMMonth YYYY') AS tgl_cetak
		FROM public.opr_t_esuratjalan s
		LEFT JOIN public.hrd_m_karyawan sp1 ON CAST(s.sjh_sopir1_nip AS VARCHAR) = CAST(sp1.kry_nip AS VARCHAR)
		LEFT JOIN public.hrd_m_karyawan sp2 ON CAST(s.sjh_sopir2_nip AS VARCHAR) = CAST(sp2.kry_nip AS VARCHAR)
		LEFT JOIN public.glb_m_agen cStart ON CAST(s.sjh_startagenid AS VARCHAR) = CAST(cStart.agen_id AS VARCHAR)
		LEFT JOIN public.opr_m_assignment a ON CAST(s.sjh_assid AS VARCHAR) = CAST(a.ass_id AS VARCHAR)
		LEFT JOIN public.opr_m_trayekh trh ON CAST(s.sjh_trayekid AS VARCHAR) = CAST(trh.trh_id AS VARCHAR)
		WHERE s.sjh_id = ?
		LIMIT 1
	`

	var headerData map[string]interface{}
	if err := database.Raw(queryHeader, sjhID).Scan(&headerData).Error; err != nil || len(headerData) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data cetak tidak ditemukan"})
		return
	}

	// 2. Ambil Rincian Biaya BPLK jika sjh_trayekid ada
	trayekID := fmt.Sprintf("%v", headerData["sjh_trayekid"])
	type BiayaItem struct {
		Kode      string  `json:"kode"`
		NamaBiaya string  `json:"nama_biaya"`
		Nominal   float64 `json:"nominal"`
	}

	var rincianBiaya []BiayaItem
	var totalBPLK float64 = 0

	if trayekID != "" && trayekID != "<nil>" {
		queryBiaya := `
			SELECT 
				COALESCE(tb.tri_bplksubid, '') AS kode,
				CASE 
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '137' THEN 'BBM LK'
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '138' THEN 'Biaya Operasional BPLK'
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '139' THEN 'Biaya Retribusi LK'
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '140' THEN 'Biaya Tol LK'
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '141' THEN 'Biaya Retribusi LK'
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '142' THEN 'Biaya Penyebrangan LK'
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '144' THEN 'Biaya Bongkar Muat / Parkir'
					ELSE 'BIAYA OPERASIONAL'
				END AS nama_biaya,
				COALESCE(tb.tri_nominal, 0) AS nominal
			FROM public.opr_m_trayekbplk tb
			WHERE RTRIM(LTRIM(tb.tri_hid)) = RTRIM(LTRIM(?))
			  AND tb.tri_umyn = 'Y'
			ORDER BY tb.tri_bplksubid ASC
		`
		database.Raw(queryBiaya, trayekID).Scan(&rincianBiaya)

		for _, b := range rincianBiaya {
			totalBPLK += b.Nominal
		}
	}

	// Masukkan rincian dan total ke response
	headerData["rincian_biaya"] = rincianBiaya
	headerData["total_bplk"] = totalBPLK

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": headerData})
}

func GetAgenList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	query := `
        SELECT 
            RTRIM(LTRIM(agen_id)) AS agen_id, 
            CASE 
                -- Jika nama PUSAT DAKOTA atau kode PST001 / 001, ubah menjadi DLI PUSAT
                WHEN UPPER(RTRIM(LTRIM(agen_nama))) = 'PUSAT DAKOTA' OR RTRIM(LTRIM(agen_id)) IN ('PST001', '001') THEN 'DLI PUSAT'
                ELSE RTRIM(LTRIM(agen_nama))
            END AS agen_nama 
        FROM public.glb_m_agen 
        WHERE agen_aktifyn = 'Y' 
          AND UPPER(agen_nama) NOT LIKE '%XXX%'
        ORDER BY 
            CASE 
                WHEN UPPER(RTRIM(LTRIM(agen_nama))) = 'PUSAT DAKOTA' OR RTRIM(LTRIM(agen_id)) IN ('PST001', '001') THEN 0
                ELSE 1
            END,
            agen_nama ASC
    `

	type AgenRow struct {
		AgenID   string `gorm:"column:agen_id" json:"agen_id"`
		AgenNama string `gorm:"column:agen_nama" json:"agen_nama"`
	}

	var list []AgenRow
	if err := database.Raw(query).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// Format: 001JC2026108439 ([agen3digit]JC[tahun4digit][bulan2digit][urut4digit])
func FormatNoSuratTugasResmi(rawNo string, agenID string, tgl time.Time) string {
	raw := strings.TrimSpace(rawNo)

	// Normalisasi agen 3 digit (contoh PST -> 001)
	cleanAgen := strings.TrimSpace(agenID)
	if cleanAgen == "" || strings.ToUpper(cleanAgen) == "PST" || strings.ToUpper(cleanAgen) == "OTA" {
		cleanAgen = "001"
	}
	if len(cleanAgen) < 3 {
		cleanAgen = fmt.Sprintf("%03s", cleanAgen)
	} else if len(cleanAgen) > 3 {
		cleanAgen = cleanAgen[:3]
	}

	// Ekstrak nomor urut dari raw string (ambil 4 digit numerik terakhir)
	digits := regexp.MustCompile(`\D`).ReplaceAllString(raw, "")
	urut := "0001"
	if len(digits) >= 4 {
		urut = digits[len(digits)-4:]
	}

	if tgl.IsZero() {
		tgl = time.Now()
	}

	return fmt.Sprintf("%sJC%s%s%s", cleanAgen, tgl.Format("2006"), tgl.Format("01"), urut)
}

// 1. GET DETAIL UNTUK TAMPILAN HP
func GetPublicSuratTugasDetailHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = c.Query("pt_id")
		if ptID == "" {
			ptID = "C" // Default DLI Corporate
		}
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	noSuratTugas := strings.TrimSpace(c.Query("no"))
	if noSuratTugas == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor surat tugas wajib diisi"})
		return
	}

	// Ekstrak 4 digit urut untuk fallback pencocokan
	digits := regexp.MustCompile(`\D`).ReplaceAllString(noSuratTugas, "")
	urut := ""
	if len(digits) >= 4 {
		urut = digits[len(digits)-4:]
	}

	query := `
		SELECT 
			s.sjh_id AS no_surat_tugas,
			TO_CHAR(s.sjh_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS tgl_berangkat,
			TO_CHAR(s.sjh_tanggalkembali, 'YYYY-MM-DD HH24:MI:SS') AS tgl_kedatangan,
			COALESCE(s.sjh_kendid, '-') AS no_kendaraan,
			COALESCE(sp1.kry_nama, s.sjh_sopir1_nip, '-') AS pengemudi,
			COALESCE(sp2.kry_nama, s.sjh_sopir2_nip, '-') AS pengemudi_2,
			COALESCE(a.ass_nama, 'CHARTER') AS tugas,
			COALESCE(trh.trh_name, cEnd.agen_nama, '-') AS trayek_nama,
			COALESCE(cStart.agen_nama, 'DLI PUSAT') AS cbg_asal,
			COALESCE(cEnd.agen_nama, '-') AS cbg_tujuan,
			CASE 
				WHEN COALESCE(s.sjh_completeyn, 'N') = 'Y' OR s.sjh_approve = 'PROSES' THEN 'PROSES'
				ELSE 'DRAFT'
			END AS status,
			COALESCE(s.sjh_keterangan, '-') AS keterangan,
			COALESCE(r.sjr_nominalum, r.sjr_nominal, 0) AS nominal_um
		FROM public.opr_t_esuratjalan s
		LEFT JOIN public.hrd_m_karyawan sp1 ON CAST(s.sjh_sopir1_nip AS VARCHAR) = CAST(sp1.kry_nip AS VARCHAR)
		LEFT JOIN public.hrd_m_karyawan sp2 ON CAST(s.sjh_sopir2_nip AS VARCHAR) = CAST(sp2.kry_nip AS VARCHAR)
		LEFT JOIN public.glb_m_agen cStart ON CAST(s.sjh_startagenid AS VARCHAR) = CAST(cStart.agen_id AS VARCHAR)
		LEFT JOIN public.glb_m_agen cEnd ON CAST(s.sjh_endagenid AS VARCHAR) = CAST(cEnd.agen_id AS VARCHAR)
		LEFT JOIN public.opr_m_assignment a ON CAST(s.sjh_assid AS VARCHAR) = CAST(a.ass_id AS VARCHAR)
		LEFT JOIN public.opr_m_trayekh trh ON CAST(s.sjh_trayekid AS VARCHAR) = CAST(trh.trh_id AS VARCHAR)
		LEFT JOIN public.opr_t_esuratjalanrealisasi r ON s.sjh_id = r.sjr_sjhid
		WHERE s.sjh_id = ? OR s.sjh_id LIKE ?
		ORDER BY s.sjh_tanggal DESC
		LIMIT 1
	`

	var detail map[string]interface{}
	err := database.Raw(query, noSuratTugas, "%"+urut).Scan(&detail).Error

	if err != nil || len(detail) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Surat tugas tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   detail,
	})
}

// 2. UPDATE STATUS MENJADI PROSES
func UpdateStatusProsesSuratTugasHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = c.Query("pt_id")
		if ptID == "" {
			ptID = "C"
		}
	}

	// Ambil username login dari context auth JWT
	userLogin, _ := c.Get("username")
	userNameStr := fmt.Sprintf("%v", userLogin)
	if userNameStr == "" || userNameStr == "<nil>" {
		userNameStr = "CHECKER"
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		NoSuratTugas string `json:"no_surat_tugas" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	digits := regexp.MustCompile(`\D`).ReplaceAllString(req.NoSuratTugas, "")
	urut := ""
	if len(digits) >= 4 {
		urut = digits[len(digits)-4:]
	}

	nowStr := time.Now().Format("2006-01-02 15:04:05")

	// Update status dan catat username pengubah (sjh_updateid)
	err := database.Exec(`
        UPDATE public.opr_t_esuratjalan 
        SET sjh_completeyn = 'Y',
            sjh_approve = 'PROSES',
            sjh_updateid = ?,
            sjh_updatetime = ?
        WHERE sjh_id = ? OR sjh_id LIKE ?
    `, userNameStr, nowStr, req.NoSuratTugas, "%"+urut).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memproses surat tugas: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Surat Tugas berhasil diproses oleh %s", userNameStr),
	})
}

// POST /operasional/surat-tugas/verify-scan
func VerifySuratTugasScan(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = "C"
	}
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req struct {
		NoSuratTugas string `json:"no_surat_tugas" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor surat tugas wajib diisi"})
		return
	}

	cleanNo := strings.TrimSpace(req.NoSuratTugas)

	type SuratTugasDetail struct {
		NomorSuratTugas string `gorm:"column:nomor_surat_tugas" json:"nomor_surat_tugas"`
		Tugas           string `gorm:"column:tugas" json:"tugas"`
		NoPolisi        string `gorm:"column:no_polisi" json:"no_polisi"`
		NamaSupir       string `gorm:"column:nama_supir" json:"nama_supir"`
		StatusTugas     string `gorm:"column:status" json:"status_tugas"`
	}
	var data SuratTugasDetail

	// Sesuaikan nama tabel surat tugas Anda di database (misal: opr_t_surattugas)
	err := database.Raw(`
		SELECT 
			COALESCE(nosurattugas, ?) AS nomor_surat_tugas,
			COALESCE(jenistugas, 'LOPER') AS tugas,
			COALESCE(nokendaraan, '') AS no_polisi,
			COALESCE(namasupir, '') AS nama_supir,
			'VALID & TERDAFTAR' AS status
		FROM public.opr_t_surattugas
		WHERE UPPER(TRIM(nosurattugas)) = UPPER(TRIM(?))
		LIMIT 1
	`, cleanNo, cleanNo).Scan(&data).Error

	if err != nil || data.NoPolisi == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Surat tugas %s tidak ditemukan dalam database!", cleanNo),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   data,
	})
}
