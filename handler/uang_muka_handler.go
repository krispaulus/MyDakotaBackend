package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Model untuk Response List Grid Master Uang Muka Biaya Operasional (BPLK/BPDK)
type UangMukaListModel struct {
	TrhID        string  `json:"trh_id" gorm:"column:trh_id"`
	TrhName      string  `json:"trh_name" gorm:"column:trh_name"`
	JnsKendNama  string  `json:"jnskend_nama" gorm:"column:jnskend_nama"`
	JUtama       int     `json:"jutama" gorm:"column:jutama"`
	JCadangan    int     `json:"jcadangan" gorm:"column:jcadangan"`
	JBPLK        float64 `json:"jbplk" gorm:"column:jbplk"`
	TrhAktifYN   string  `json:"trh_aktifyn" gorm:"column:trh_aktifyn"`
	RuteUtama    string  `json:"rute_utama" gorm:"column:rute_utama"`
	RuteCadangan string  `json:"rute_cadangan" gorm:"column:rute_cadangan"`
}

// Model Item Rincian Komponen Biaya BPLK
type UangMukaBiayaItem struct {
	Kode      string  `json:"kode" gorm:"column:kode"`
	NamaBiaya string  `json:"nama_biaya" gorm:"column:nama_biaya"`
	Nominal   float64 `json:"nominal" gorm:"column:nominal"`
	UMYN      string  `json:"um_yn" gorm:"column:tri_umyn"`
}

// =========================================================================
// 1. GET /api/master/uang-muka (READ LIST DATA MASTER UANG MUKA)
// =========================================================================
func GetMasterUangMukaListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	search := strings.TrimSpace(c.Query("q"))
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "100")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 100
	}
	offset := (page - 1) * limit

	var whereConds []string
	var args []interface{}

	// Filter pencarian berdasarkan nama trayek atau kode
	if search != "" {
		whereConds = append(whereConds, "(th.trh_id ILIKE ? OR th.trh_name ILIKE ?)")
		args = append(args, "%"+search+"%", "%"+search+"%")
	}

	// 🌟 Mengadopsi file lawas: Hanya tampilkan trayek yang memiliki relasi biaya BPLK aktif (dengan TRIM agar kebal spasi SQL Server)
	whereConds = append(whereConds, `EXISTS (
		SELECT 1 FROM public.opr_m_trayekbplk tb 
		WHERE TRIM(tb.tri_hid) = TRIM(th.trh_id) 
		  AND COALESCE(TRIM(tb.tri_bplkid), '') <> ''
	)`)

	whereSQL := ""
	if len(whereConds) > 0 {
		whereSQL = "WHERE " + strings.Join(whereConds, " AND ")
	}

	// 🌟 Query List dengan pembersihan spasi TRIM() di semua tabel relasi
	query := fmt.Sprintf(`
		SELECT 
			TRIM(th.trh_id) AS trh_id, 
			TRIM(th.trh_name) AS trh_name, 
			COALESCE(TRIM(jk.jnskend_nama), '-') AS jnskend_nama,
			0 AS jutama,
			0 AS jcadangan,
			CASE 
				WHEN TRIM(COALESCE(th.trh_aktifyn, 'Y')) = 'N' THEN 'N'
				ELSE 'Y'
			END AS trh_aktifyn,
			COALESCE((
				SELECT SUM(b.tri_nominal) 
				FROM public.opr_m_trayekbplk b 
				WHERE TRIM(b.tri_hid) = TRIM(th.trh_id)
			), 0) AS jbplk,
			COALESCE((
				SELECT STRING_AGG(TRIM(a.agen_nama), ' -- ' ORDER BY td.trd_status, td.trd_urut)
				FROM public.opr_m_trayekd td
				JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(td.trd_agenid::varchar)
				WHERE TRIM(td.trd_hid) = TRIM(th.trh_id) AND td.trd_jenis = 'U'
			), '-') AS rute_utama,
			COALESCE((
				SELECT STRING_AGG(TRIM(a.agen_nama), ' ; ' ORDER BY td.trd_status, td.trd_urut)
				FROM public.opr_m_trayekd td
				JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(td.trd_agenid::varchar)
				WHERE TRIM(td.trd_hid) = TRIM(th.trh_id) AND td.trd_jenis = 'C'
			), '-') AS rute_cadangan
		FROM public.opr_m_trayekh th
		LEFT JOIN public.glb_m_jnskend jk ON TRIM(jk.jnskend_id::varchar) = TRIM(th.trh_jnskend::varchar)
		%s
		ORDER BY th.trh_name ASC
		LIMIT ? OFFSET ?
	`, whereSQL)

	queryArgs := append(args, limit, offset)

	var list []UangMukaListModel
	if err := database.Raw(query, queryArgs...).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal load master uang muka: " + err.Error()})
		return
	}

	var totalRecords int64
	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM public.opr_m_trayekh th %s`, whereSQL)
	database.Raw(countQuery, args...).Scan(&totalRecords)

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          list,
		"total_records": totalRecords,
	})
}

// =========================================================================
// 2. GET /api/master/uang-muka/detail/:id (RINCIAN BIAYA BPLK BERDASARKAN ID)
// =========================================================================
func GetMasterUangMukaDetailHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	trhID := strings.TrimSpace(c.Param("id"))

	var header struct {
		TrhID       string `json:"trh_id"`
		TrhName     string `json:"trh_name"`
		JnsKendNama string `json:"jnskend_nama"`
		TrhAktifYN  string `json:"trh_aktifyn"`
	}

	err := database.Table("public.opr_m_trayekh th").
		Select("th.trh_id, th.trh_name, COALESCE(jk.jnskend_nama, '-') as jnskend_nama, th.trh_aktifyn").
		Joins("LEFT JOIN public.glb_m_jnskend jk ON RTRIM(LTRIM(jk.jnskend_id::varchar)) = RTRIM(LTRIM(th.trh_jnskend::varchar))").
		Where("RTRIM(LTRIM(th.trh_id)) = RTRIM(LTRIM(?))", trhID).
		Scan(&header).Error

	if err != nil || header.TrhID == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data Uang Muka Trayek tidak ditemukan"})
		return
	}

	// Query rincian komponen biaya BPLK dengan pencocokan nama dari master
	queryBiaya := `
		SELECT 
			COALESCE(tb.tri_bplksubid, '') AS kode,
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
			COALESCE(tb.tri_nominal, 0) AS nominal,
			COALESCE(tb.tri_umyn, 'Y') AS tri_umyn
		FROM public.opr_m_trayekbplk tb
		LEFT JOIN public.opr_m_bplk_urut bu ON RTRIM(LTRIM(bu.bplku_id)) = RTRIM(LTRIM(tb.tri_bplksubid))
		LEFT JOIN public.opr_m_bplk b ON RTRIM(LTRIM(b.bplk_id)) = RTRIM(LTRIM(tb.tri_bplksubid))
		WHERE RTRIM(LTRIM(tb.tri_hid)) = RTRIM(LTRIM(?))
		ORDER BY tb.tri_bplksubid ASC
	`
	var rincianBiaya []UangMukaBiayaItem
	database.Raw(queryBiaya, trhID).Scan(&rincianBiaya)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"header": header,
		"biaya":  rincianBiaya,
	})
}

// =========================================================================
// 3. POST /api/master/uang-muka/toggle-status (UBAH STATUS AKTIF / NON-AKTIF)
// =========================================================================
func ToggleUangMukaStatusHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		TrhID      string `json:"trh_id" binding:"required"`
		TrhAktifYN string `json:"trh_aktifyn" binding:"required"` // 'Y' atau 'N'
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	err := database.Table("public.opr_m_trayekh").
		Where("RTRIM(LTRIM(trh_id)) = RTRIM(LTRIM(?))", req.TrhID).
		Update("trh_aktifyn", req.TrhAktifYN).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengubah status: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Status Uang Muka %s berhasil diubah menjadi %s", req.TrhID, req.TrhAktifYN),
	})
}

// GET /api/master/jenis-kendaraan
func GetMasterJenisKendaraanHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	type JnsKendModel struct {
		JnsKendID   string `json:"jnskend_id" gorm:"column:jnskend_id"`
		JnsKendNama string `json:"jnskend_nama" gorm:"column:jnskend_nama"`
	}

	var list []JnsKendModel
	err := database.Raw(`
		SELECT 
			TRIM(jnskend_id) AS jnskend_id, 
			TRIM(jnskend_nama) AS jnskend_nama 
		FROM public.glb_m_jnskend 
		ORDER BY jnskend_nama ASC
	`).Scan(&list).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// Model Payload Simpan Biaya Baru
type SaveBiayaItemRequest struct {
	TrhID string `json:"trh_id" binding:"required"`
	Items []struct {
		KodeBiaya string  `json:"kode_biaya" binding:"required"` // bplku_id (contoh: 084000000137)
		Nominal   float64 `json:"nominal"`
		UMYN      string  `json:"um_yn"` // 'Ya' / 'Tidak' atau 'Y' / 'N'
	} `json:"items" binding:"required"`
}

// 1. POST /api/master/uang-muka/biaya-save (TAMBAH / UPDATE BANYAK KOMPONEN BIAYA SEKALIGUS)
func SaveMasterUangMukaBiayaHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req SaveBiayaItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	trhID := strings.TrimSpace(req.TrhID)
	userID := "SYSTEM" // sesuaikan dengan claims JWT login bila ada

	// Simpan atau replace per item
	tx := database.Begin()
	for _, itm := range req.Items {
		kode := strings.TrimSpace(itm.KodeBiaya)
		if kode == "" {
			continue
		}

		umFlag := "Y"
		if strings.ToUpper(itm.UMYN) == "TIDAK" || strings.ToUpper(itm.UMYN) == "N" {
			umFlag = "N"
		}

		// Insert or Update (Upsert)
		upsertQuery := `
			INSERT INTO public.opr_m_trayekbplk (tri_hid, tri_bplkid, tri_bplksubid, tri_nominal, tri_umyn, tri_updateid, tri_updatetime)
			VALUES (?, '084000000029', ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT (tri_hid, tri_bplksubid) 
			DO UPDATE SET 
				tri_nominal = EXCLUDED.tri_nominal,
				tri_umyn = EXCLUDED.tri_umyn,
				tri_updateid = EXCLUDED.tri_updateid,
				tri_updatetime = CURRENT_TIMESTAMP
		`
		if err := tx.Exec(upsertQuery, trhID, kode, itm.Nominal, umFlag, userID).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan rincian biaya: " + err.Error()})
			return
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Komponen biaya operasional berhasil disimpan"})
}

// 2. DELETE /api/master/uang-muka/biaya-delete (HAPUS SATU BARIS KOMPONEN BIAYA)
func DeleteMasterUangMukaBiayaHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	trhID := strings.TrimSpace(c.Query("trh_id"))
	kodeBiaya := strings.TrimSpace(c.Query("kode"))

	if trhID == "" || kodeBiaya == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter trh_id dan kode biaya wajib diisi"})
		return
	}

	err := database.Exec(`
		DELETE FROM public.opr_m_trayekbplk 
		WHERE TRIM(tri_hid) = TRIM(?) AND TRIM(tri_bplksubid) = TRIM(?)
	`, trhID, kodeBiaya).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus komponen biaya: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Komponen biaya berhasil dihapus"})
}

// GET /api/master/uang-muka/kamus-biaya
func GetMasterKamusBiayaHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	type KamusBiayaItem struct {
		BplkuID   string `json:"bplku_id" gorm:"column:bplku_id"`
		BplkuNama string `json:"bplku_nama" gorm:"column:bplku_nama"`
	}

	var list []KamusBiayaItem
	err := database.Raw(`
		SELECT 
			TRIM(bplku_id) AS bplku_id, 
			TRIM(bplku_nama) AS bplku_nama 
		FROM public.opr_m_bplk_urut 
		WHERE COALESCE(bplku_aktifyn, 'Y') = 'Y'
		ORDER BY bplku_nama ASC
	`).Scan(&list).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membaca kamus biaya: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}
