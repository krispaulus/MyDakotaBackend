package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
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
			COALESCE(bu.bplku_nama, b.bplk_nama, 
				CASE 
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '137' THEN 'BBM LK'
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '138' THEN 'Biaya Operasional BPLK'
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '140' THEN 'Biaya Tol LK'
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '141' THEN 'Biaya Retribusi LK'
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '142' THEN 'Biaya Penyebrangan LK'
					WHEN RIGHT(RTRIM(LTRIM(tb.tri_bplksubid)), 3) = '144' THEN 'Biaya Bongkar Muat / Parkir'
					ELSE 'BIAYA OPERASIONAL'
				END
			) AS nama_biaya,
			COALESCE(tb.tri_nominal, 0) AS nominal
		FROM public.opr_m_trayekbplk tb
		LEFT JOIN public.opr_m_bplk_urut bu ON RTRIM(LTRIM(bu.bplku_id)) = RTRIM(LTRIM(tb.tri_bplksubid))
		LEFT JOIN public.opr_m_bplk b ON RTRIM(LTRIM(b.bplk_id)) = RTRIM(LTRIM(tb.tri_bplksubid))
		WHERE RTRIM(LTRIM(tb.tri_hid)) = RTRIM(LTRIM(?)) 
		  AND tb.tri_umyn = 'Y'
		ORDER BY tb.tri_bplksubid ASC
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

func FormatNoSuratTugasResmi(rawNo string, agenID string, tgl time.Time) string {
	raw := strings.TrimSpace(rawNo)

	// 1. Normalisasi Cabang (PST / OTA / kosong -> 001)
	cleanAgen := strings.TrimSpace(agenID)
	if cleanAgen == "" || strings.ToUpper(cleanAgen) == "PST" || strings.ToUpper(cleanAgen) == "OTA" {
		cleanAgen = "001"
	}
	if len(cleanAgen) < 3 {
		cleanAgen = fmt.Sprintf("%03s", cleanAgen)
	} else if len(cleanAgen) > 3 {
		cleanAgen = cleanAgen[:3]
	}

	// 2. Ambil 4 digit urut terakhir
	digits := regexp.MustCompile(`\D`).ReplaceAllString(raw, "")
	urut := "0001"
	if len(digits) >= 4 {
		urut = digits[len(digits)-4:]
	}

	if tgl.IsZero() {
		tgl = time.Now()
	}

	// 3. Tanpa tanggal hari (hanya Tahun & Bulan) -> 001JC2026119422
	return fmt.Sprintf("%sJC%s%s%s", cleanAgen, tgl.Format("2006"), tgl.Format("01"), urut)
}

func GetPublicSuratTugasDetailHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = c.Query("pt_id")
		if ptID == "" {
			ptID = "C"
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

	// Ekstrak 4 digit nomor urut terakhir (misal: 9422)
	digits := regexp.MustCompile(`\D`).ReplaceAllString(noSuratTugas, "")
	urut := ""
	if len(digits) >= 4 {
		urut = digits[len(digits)-4:]
	}

	query := `
		SELECT 
			s.sjh_id AS raw_sjh_id,
			s.sjh_tanggal,
			TO_CHAR(s.sjh_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS tgl_berangkat,
			TO_CHAR(s.sjh_tanggalkembali, 'YYYY-MM-DD HH24:MI:SS') AS tgl_kedatangan,
			COALESCE(s.sjh_kendid, '-') AS no_kendaraan,
			COALESCE(sp1.kry_nama, s.sjh_sopir1_nip, '-') AS pengemudi,
			COALESCE(sp1.kry_nip, '-') AS nip_pengemudi,
			COALESCE(sp2.kry_nama, s.sjh_sopir2_nip, '-') AS pengemudi_2,
			COALESCE(a.ass_nama, 'LOPER') AS tugas,
			COALESCE(trh.trh_name, cEnd.agen_nama, 'LOPER DALAM KOTA') AS trayek_nama,
			COALESCE(cStart.agen_nama, 'DLI PUSAT') AS cbg_asal,
			COALESCE(s.sjh_startagenid, '001') AS start_agen_id,
			COALESCE(cEnd.agen_nama, '-') AS cbg_tujuan,
			CASE 
				WHEN COALESCE(s.sjh_completeyn, 'N') = 'Y' OR s.sjh_approve = 'PROSES' THEN 'PROSES'
				ELSE 'DRAFT'
			END AS status,
			COALESCE(s.sjh_keterangan, 'LOPER DALAM KOTA') AS keterangan,
			COALESCE(r.sjr_nominalum, r.sjr_nominal, 0) AS nominal_um
		FROM public.opr_t_esuratjalan s
		LEFT JOIN public.hrd_m_karyawan sp1 ON CAST(s.sjh_sopir1_nip AS VARCHAR) = CAST(sp1.kry_nip AS VARCHAR)
		LEFT JOIN public.hrd_m_karyawan sp2 ON CAST(s.sjh_sopir2_nip AS VARCHAR) = CAST(sp2.kry_nip AS VARCHAR)
		LEFT JOIN public.glb_m_agen cStart ON CAST(s.sjh_startagenid AS VARCHAR) = CAST(cStart.agen_id AS VARCHAR)
		LEFT JOIN public.glb_m_agen cEnd ON CAST(s.sjh_endagenid AS VARCHAR) = CAST(cEnd.agen_id AS VARCHAR)
		LEFT JOIN public.opr_m_assignment a ON CAST(s.sjh_assid AS VARCHAR) = CAST(a.ass_id AS VARCHAR)
		LEFT JOIN public.opr_m_trayekh trh ON CAST(s.sjh_trayekid AS VARCHAR) = CAST(trh.trh_id AS VARCHAR)
		LEFT JOIN public.opr_t_esuratjalanrealisasi r ON s.sjh_id = r.sjr_sjhid
		WHERE s.sjh_id = ? 
		   OR s.sjh_id LIKE ? 
		   OR s.sjh_id LIKE ?
		ORDER BY s.sjh_tanggal DESC
		LIMIT 1
	`

	var detail struct {
		RawSJHID     string    `gorm:"column:raw_sjh_id"`
		SJHTanggal   time.Time `gorm:"column:sjh_tanggal"`
		TglBerangkat string    `gorm:"column:tgl_berangkat"`
		TglKembali   string    `gorm:"column:tgl_kedatangan"`
		NoKendaraan  string    `gorm:"column:no_kendaraan"`
		Pengemudi    string    `gorm:"column:pengemudi"`
		NipPengemudi string    `gorm:"column:nip_pengemudi"`
		Pengemudi2   string    `gorm:"column:pengemudi_2"`
		Tugas        string    `gorm:"column:tugas"`
		TrayekNama   string    `gorm:"column:trayek_nama"`
		CbgAsal      string    `gorm:"column:cbg_asal"`
		StartAgenID  string    `gorm:"column:start_agen_id"`
		CbgTujuan    string    `gorm:"column:cbg_tujuan"`
		Status       string    `gorm:"column:status"`
		Keterangan   string    `gorm:"column:keterangan"`
		NominalUM    float64   `gorm:"column:nominal_um"`
	}

	err := database.Raw(query, noSuratTugas, "%"+noSuratTugas+"%", "%"+urut).Scan(&detail).Error
	if err != nil || detail.RawSJHID == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Surat tugas tidak ditemukan"})
		return
	}

	// 🌟 Ubah nomor menjadi standar resmi: 001JC2026119422
	officialNo := FormatNoSuratTugasResmi(detail.RawSJHID, detail.StartAgenID, detail.SJHTanggal)

	// Ambil rincian BTT
	type BTTItem struct {
		NoBTT      string `gorm:"column:nobtt" json:"no_btt"`
		Tujuan     string `gorm:"column:tujuan" json:"tujuan"`
		Colly      int    `gorm:"column:colly" json:"colly"`
		Discan     int    `gorm:"column:discan" json:"discan"`
		Kurang     int    `gorm:"column:kurang" json:"kurang"`
		Keterangan string `gorm:"column:keterangan" json:"keterangan"`
	}

	var listBTT []BTTItem
	queryBTT := `
		SELECT 
			TRIM(COALESCE(d.sjd_bttid, d.sjd_nobtt, '-')) AS nobtt,
			TRIM(COALESCE(a.agen_nama, d.sjd_tujuan, '-')) AS tujuan,
			COALESCE(d.sjd_koli, d.sjd_colly, 0) AS colly,
			COALESCE(d.sjd_koli_scan, d.sjd_colly_scan, COALESCE(d.sjd_koli, d.sjd_colly, 0)) AS discan,
			GREATEST(0, COALESCE(d.sjd_koli, d.sjd_colly, 0) - COALESCE(d.sjd_koli_scan, d.sjd_colly_scan, COALESCE(d.sjd_koli, d.sjd_colly, 0))) AS kurang,
			'SESUAI' AS keterangan
		FROM public.opr_t_esuratjalandetil d
		LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(d.sjd_agenid::varchar)
		WHERE TRIM(d.sjd_sjhid) = TRIM(?) 
		   OR TRIM(d.sjd_sjhid) LIKE ?
		ORDER BY d.sjd_bttid ASC
	`
	database.Raw(queryBTT, detail.RawSJHID, "%"+urut).Scan(&listBTT)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"raw_sjh_id":     detail.RawSJHID,
			"no_surat_tugas": officialNo,
			"tgl_berangkat":  detail.TglBerangkat,
			"tgl_kedatangan": detail.TglKembali,
			"no_kendaraan":   detail.NoKendaraan,
			"pengemudi":      detail.Pengemudi,
			"nip_pengemudi":  detail.NipPengemudi,
			"pengemudi_2":    detail.Pengemudi2,
			"tugas":          detail.Tugas,
			"trayek_nama":    detail.TrayekNama,
			"cbg_asal":       detail.CbgAsal,
			"cbg_tujuan":     detail.CbgTujuan,
			"status":         detail.Status, // 'DRAFT' atau 'PROSES'
			"keterangan":     detail.Keterangan,
			"nominal_um":     detail.NominalUM,
			"btt_list":       listBTT,
			"total_btt":      len(listBTT),
		},
	})
}

func UpdateStatusProsesSuratTugasHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = "C"
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

	userLogin, _ := c.Get("username")
	userNameStr := fmt.Sprintf("%v", userLogin)
	if userNameStr == "" || userNameStr == "<nil>" {
		userNameStr = "CHECKER"
	}

	nowStr := time.Now().Format("2006-01-02 15:04:05")

	err := database.Exec(`
        UPDATE public.opr_t_esuratjalan 
        SET sjh_completeyn = 'Y',
            sjh_approve = 'PROSES',
            sjh_updateid = ?,
            sjh_updatetime = ?
        WHERE sjh_id = ? OR sjh_id LIKE ?
    `, userNameStr, nowStr, req.NoSuratTugas, "%"+urut).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update status: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Surat Tugas berhasil diproses!"})
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

func ExportMasterUangMukaCSVHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	search := strings.TrimSpace(c.Query("q"))

	whereSQL := `WHERE EXISTS (
		SELECT 1 FROM public.opr_m_trayekbplk tb 
		WHERE TRIM(tb.tri_hid) = TRIM(th.trh_id) 
		  AND COALESCE(TRIM(tb.tri_bplkid), '') <> ''
	)`
	var args []interface{}
	if search != "" {
		whereSQL += " AND (th.trh_id ILIKE ? OR th.trh_name ILIKE ?)"
		args = append(args, "%"+search+"%", "%"+search+"%")
	}

	// Query rincian lengkap untuk di-export (Trayek Header + Komponen Biaya)
	query := fmt.Sprintf(`
		SELECT 
			TRIM(th.trh_id) AS kode_trayek,
			TRIM(th.trh_name) AS nama_trayek,
			COALESCE(TRIM(th.trh_jnskend::varchar), '') AS kode_kendaraan,
			COALESCE(TRIM(jk.jnskend_nama), '-') AS nama_kendaraan,
			COALESCE(TRIM(th.trh_aktifyn), 'Y') AS status_aktif,
			COALESCE(TRIM(tb.tri_bplksubid), '') AS kode_biaya,
			COALESCE(TRIM(bu.bplku_nama), 'BIAYA OPERASIONAL') AS nama_biaya,
			COALESCE(tb.tri_nominal, 0) AS nominal,
			COALESCE(TRIM(tb.tri_umyn), 'Y') AS uang_muka_yn
		FROM public.opr_m_trayekh th
		LEFT JOIN public.glb_m_jnskend jk ON TRIM(jk.jnskend_id::varchar) = TRIM(th.trh_jnskend::varchar)
		LEFT JOIN public.opr_m_trayekbplk tb ON TRIM(tb.tri_hid) = TRIM(th.trh_id)
		LEFT JOIN public.opr_m_bplk_urut bu ON TRIM(bu.bplku_id) = TRIM(tb.tri_bplksubid)
		%s
		ORDER BY th.trh_name ASC, tb.tri_bplksubid ASC
	`, whereSQL)

	type ExportRow struct {
		KodeTrayek    string  `gorm:"column:kode_trayek"`
		NamaTrayek    string  `gorm:"column:nama_trayek"`
		KodeKendaraan string  `gorm:"column:kode_kendaraan"`
		NamaKendaraan string  `gorm:"column:nama_kendaraan"`
		StatusAktif   string  `gorm:"column:status_aktif"`
		KodeBiaya     string  `gorm:"column:kode_biaya"`
		NamaBiaya     string  `gorm:"column:nama_biaya"`
		Nominal       float64 `gorm:"column:nominal"`
		UangMukaYN    string  `gorm:"column:uang_muka_yn"`
	}

	var rows []ExportRow
	if err := database.Raw(query, args...).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal export master: " + err.Error()})
		return
	}

	// Buat Header CSV Response
	fileName := fmt.Sprintf("Master_Uang_Muka_BPLK_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Header("Content-Type", "text/csv; charset=utf-8")

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Tulis Baris Header Kolom
	writer.Write([]string{
		"KODE_TRAYEK",
		"NAMA_TRAYEK",
		"KODE_KENDARAAN",
		"NAMA_KENDARAAN",
		"STATUS_AKTIF",
		"KODE_BIAYA",
		"NAMA_BIAYA",
		"NOMINAL",
		"UANG_MUKA_YN",
	})

	for _, r := range rows {
		writer.Write([]string{
			r.KodeTrayek,
			r.NamaTrayek,
			r.KodeKendaraan,
			r.NamaKendaraan,
			r.StatusAktif,
			r.KodeBiaya,
			r.NamaBiaya,
			fmt.Sprintf("%.0f", r.Nominal),
			r.UangMukaYN,
		})
	}
}

// =========================================================================
// 5. POST /api/master/uang-muka/import-csv (IMPORT & AUTO-UPSERT FILE CSV)
// =========================================================================
func ImportMasterUangMukaCSVHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV wajib diunggah"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membuka file: " + err.Error()})
		return
	}
	defer file.Close()

	// Deteksi Delimiter (koma atau titik koma)
	buf := make([]byte, 1024)
	n, _ := file.Read(buf)
	file.Seek(0, io.SeekStart)

	firstLine := string(buf[:n])
	reader := csv.NewReader(file)
	if strings.Count(firstLine, ";") > strings.Count(firstLine, ",") {
		reader.Comma = ';'
	} else {
		reader.Comma = ','
	}
	reader.LazyQuotes = true

	totalSuccessHeader := 0
	totalSuccessBiaya := 0
	userID := "IMPORT-CSV"

	tx := database.Begin()
	rowIndex := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		rowIndex++
		if rowIndex == 1 {
			continue // Lewati Header baris 1
		}

		if len(record) < 3 {
			continue
		}

		kodeTrayek := strings.TrimSpace(record[0])
		namaTrayek := strings.TrimSpace(record[1])
		if kodeTrayek == "" || namaTrayek == "" {
			continue
		}

		kodeKendaraan := ""
		if len(record) > 2 {
			kodeKendaraan = strings.TrimSpace(record[2])
		}

		namaKendaraan := ""
		if len(record) > 3 {
			namaKendaraan = strings.TrimSpace(record[3])
		}

		// 🛡️ 1. PEMBERSIHAN NAMA KENDARAAN (jika ada angka menempel di depan, misal "1 Colt Diesel Double")
		if strings.HasPrefix(namaKendaraan, "1 ") {
			if kodeKendaraan == "" {
				kodeKendaraan = "001"
			}
			namaKendaraan = strings.TrimPrefix(namaKendaraan, "1 ")
		}

		// 🛡️ 2. JIKA KODE KENDARAAN KOSONG, CARI BERDASARKAN NAMA KENDARAAN DARI DATABASE
		if kodeKendaraan == "" && namaKendaraan != "" {
			var foundKendID string
			database.Raw(`
				SELECT TRIM(jnskend_id) 
				FROM public.glb_m_jnskend 
				WHERE UPPER(TRIM(jnskend_nama)) = UPPER(TRIM(?))
				   OR UPPER(TRIM(jnskend_nama)) LIKE UPPER(TRIM(?))
				LIMIT 1
			`, namaKendaraan, "%"+namaKendaraan+"%").Scan(&foundKendID)

			if foundKendID != "" {
				kodeKendaraan = foundKendID
			}
		}

		// 🛡️ 3. STANDARISASI FORMAT KODE KENDARAAN (misal "1" menjadi "001")
		if len(kodeKendaraan) > 0 && len(kodeKendaraan) < 3 {
			kodeKendaraan = fmt.Sprintf("%03s", kodeKendaraan)
		}

		statusAktif := "Y"
		if len(record) > 4 && strings.ToUpper(strings.TrimSpace(record[4])) == "N" {
			statusAktif = "N"
		}

		// 1. Upsert Header Trayek ke opr_m_trayekh
		upsertHeaderQuery := `
			INSERT INTO public.opr_m_trayekh (trh_id, trh_name, trh_jnskend, trh_aktifyn, trh_updateid, trh_updatetime)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT (trh_id) 
			DO UPDATE SET 
				trh_name = EXCLUDED.trh_name,
				trh_jnskend = CASE WHEN EXCLUDED.trh_jnskend <> '' THEN EXCLUDED.trh_jnskend ELSE opr_m_trayekh.trh_jnskend END,
				trh_aktifyn = EXCLUDED.trh_aktifyn,
				trh_updateid = EXCLUDED.trh_updateid,
				trh_updatetime = CURRENT_TIMESTAMP
		`
		if err := tx.Exec(upsertHeaderQuery, kodeTrayek, namaTrayek, kodeKendaraan, statusAktif, userID).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": fmt.Sprintf("Baris %d: Gagal simpan trayek %s: %v", rowIndex, kodeTrayek, err)})
			return
		}
		totalSuccessHeader++

		// 2. Jika baris memiliki komponen biaya (Kolom 5 & 7)
		if len(record) >= 8 {
			kodeBiaya := strings.TrimSpace(record[5])

			// 🛡️ Proteksi Auto-Padding Leading Zero dari Excel (Standar Kode Biaya Dakota 12 Karakter)
			if len(kodeBiaya) > 0 && len(kodeBiaya) < 12 {
				// Ubah angka Excel seperti 8400000137 kembali utuh menjadi 084000000137
				kodeBiaya = fmt.Sprintf("%012s", kodeBiaya)
			}

			cleanNominal := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(record[7]), ".", ""), ",", ".")
			nominal, _ := strconv.ParseFloat(cleanNominal, 64)

			umYN := "Y"
			if len(record) > 8 && (strings.ToUpper(strings.TrimSpace(record[8])) == "N" || strings.ToUpper(strings.TrimSpace(record[8])) == "TIDAK") {
				umYN = "N"
			}

			if kodeBiaya != "" && nominal > 0 {
				upsertBiayaQuery := `
					INSERT INTO public.opr_m_trayekbplk (tri_hid, tri_bplkid, tri_bplksubid, tri_nominal, tri_umyn, tri_updateid, tri_updatetime)
					VALUES (?, '084000000029', ?, ?, ?, ?, CURRENT_TIMESTAMP)
					ON CONFLICT (tri_hid, tri_bplksubid)
					DO UPDATE SET 
						tri_nominal = EXCLUDED.tri_nominal,
						tri_umyn = EXCLUDED.tri_umyn,
						tri_updateid = EXCLUDED.tri_updateid,
						tri_updatetime = CURRENT_TIMESTAMP
				`
				if err := tx.Exec(upsertBiayaQuery, kodeTrayek, kodeBiaya, nominal, umYN, userID).Error; err != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": fmt.Sprintf("Baris %d: Gagal simpan biaya %s: %v", rowIndex, kodeBiaya, err)})
					return
				}
				totalSuccessBiaya++
			}
		}
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"message":      fmt.Sprintf("Import selesai! %d data trayek dan %d rincian biaya berhasil disimpan/diperbarui.", totalSuccessHeader, totalSuccessBiaya),
		"total_header": totalSuccessHeader,
		"total_biaya":  totalSuccessBiaya,
	})
}

// POST /api/master/uang-muka/update-header
func UpdateMasterUangMukaHeaderHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		TrhID      string `json:"trh_id" binding:"required"`
		TrhName    string `json:"trh_name" binding:"required"`
		TrhJnsKend string `json:"trh_jnskend" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Data tidak lengkap: " + err.Error()})
		return
	}

	trhID := strings.TrimSpace(req.TrhID)
	trhName := strings.TrimSpace(req.TrhName)

	// 🛡️ Bersihkan BOM / Karakter Kotor tersembunyi (∩╗┐ atau \ufeff)
	cleanKend := strings.TrimSpace(req.TrhJnsKend)
	cleanKend = strings.TrimPrefix(cleanKend, "\ufeff")
	cleanKend = strings.ReplaceAll(cleanKend, "∩╗┐", "")

	// Jika formatnya angka (misal "1"), jadikan 3 digit ("001")
	if len(cleanKend) > 0 && len(cleanKend) < 3 {
		cleanKend = fmt.Sprintf("%03s", cleanKend)
	}

	err := database.Exec(`
		UPDATE public.opr_m_trayekh 
		SET trh_name = ?, 
		    trh_jnskend = ?,
		    trh_updatetime = CURRENT_TIMESTAMP
		WHERE TRIM(trh_id) = TRIM(?)
	`, trhName, cleanKend, trhID).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update master trayek: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Trayek %s berhasil diperbarui!", trhID),
	})
}
