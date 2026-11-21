package handler

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func strPtr(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func resolveMasterKendaraanDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		return db.DB
	}
	return database
}

// 1. NOTIFIKASI EXPIRED SERVICE
func GetExpiredServiceKendaraan(c *gin.Context) {
	database := resolveMasterKendaraanDB(c)
	var expiredVehicles []models.GlbMKendaraan

	err := database.Table("public.glb_m_kendaraan").
		Where("kend_serviceyn = 'Y' AND kend_serviceenddate < CURRENT_DATE").
		Order("kend_serviceenddate ASC").
		Find(&expiredVehicles).Error

	if err != nil {
		log.Println("⚠️ Kolom service belum cocok, bypass array kosong:", err.Error())
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"count":  0,
			"data":   []models.GlbMKendaraan{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"count":  len(expiredVehicles),
		"data":   expiredVehicles,
	})
}

// 2. GET LIST MASTER KENDARAAN
func GetMasterKendaraanList(c *gin.Context) {
	database := resolveMasterKendaraanDB(c)
	var vehicles []models.GlbMKendaraan

	nopol := strings.TrimSpace(c.Query("nopol"))
	aktif := strings.TrimSpace(c.Query("aktif"))

	query := database.Table("public.glb_m_kendaraan")
	if nopol != "" {
		query = query.Where("UPPER(kend_id) LIKE UPPER(?)", "%"+nopol+"%")
	}
	if aktif != "" {
		query = query.Where("kend_aktifyn = ?", aktif)
	} else {
		query = query.Where("kend_aktifyn = 'Y'")
	}

	if err := query.Order("kend_id ASC").Find(&vehicles).Error; err != nil {
		log.Println("❌ ERROR GetMasterKendaraanList DETAIL:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menarik data master kendaraan", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   vehicles,
	})
}

// 3. GET LIST JENIS / MODEL KENDARAAN DARI DATABASE (public.glb_m_jnskend)
func GetJenisKendaraanList(c *gin.Context) {
	database := resolveMasterKendaraanDB(c)

	type JenisRow struct {
		ID   string `gorm:"column:jnskend_id" json:"jnskend_id"`
		Nama string `gorm:"column:jnskend_nama" json:"jnskend_nama"`
	}

	var list []JenisRow
	err := database.Raw(`
		SELECT 
			TRIM(jnskend_id) AS jnskend_id,
			TRIM(jnskend_nama) AS jnskend_nama
		FROM public.glb_m_jnskend
		WHERE COALESCE(TRIM(jnskend_aktifyn), 'Y') = 'Y'
		ORDER BY jnskend_nama ASC
	`).Scan(&list).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data jenis kendaraan: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// Struct Payload Lengkap Request Frontend
type KendaraanInputReq struct {
	KendID              string `json:"kend_id"`
	KendPemilik         string `json:"kend_pemilik"`
	KendPT              string `json:"kend_pt"`
	KendLeasing         string `json:"kend_leasing"`
	KendAlamat          string `json:"kend_alamat"`
	KendLokasiID        string `json:"kend_lokasiid"`
	KendMerk            string `json:"kend_merk"`
	KendType            string `json:"kend_type"`
	KendJenis           string `json:"kend_jenis"`
	KendThnBuat         string `json:"kend_thnbuat"`
	KendThnRakit        string `json:"kend_thnrakit"`
	KendIsiSilinder     string `json:"kend_isisilinder"`
	KendWarna           string `json:"kend_warna"`
	KendNIK             string `json:"kend_nik"`
	KendNoMesin         string `json:"kend_nomesin"`
	KendIdentID         string `json:"kend_identid"`
	KendWarnaTNKB       string `json:"kend_warnatnkb"`
	KendBahanBakar      string `json:"kend_bahanbakar"`
	KendJarak           string `json:"kend_jarak"`
	KendWarnaPlat       string `json:"kend_warnaplat"`
	KendBerlakuSTNK     string `json:"kend_berlakustnk"`
	KendBerlakuPajak    string `json:"kend_berlakupajak"`
	KendBerlakuKIR      string `json:"kend_berlakukir"`
	KendSerahTerimaTime string `json:"kend_serahterimatime"`
	KendSerahTerimaID   string `json:"kend_serahterimaid"`
	KendSopir1          string `json:"kend_sopir1"`
	KendSopir2          string `json:"kend_sopir2"`
	KendAktifYN         string `json:"kend_aktifyn"`
	KendGpsImei         string `json:"kend_gps_imei"`
}

// 4. CREATE KENDARAAN BARU
func CreateKendaraanBaru(c *gin.Context) {
	database := resolveMasterKendaraanDB(c)

	var req KendaraanInputReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	req.KendID = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(req.KendID), " ", ""))
	if req.KendID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "No. Polisi wajib diisi!"})
		return
	}

	var count int64
	database.Table("public.glb_m_kendaraan").Where("TRIM(kend_id) = ?", req.KendID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "No. Polisi " + req.KendID + " sudah terdaftar!"})
		return
	}

	if req.KendAktifYN == "" {
		req.KendAktifYN = "Y"
	}

	userLogin, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", userLogin)
	if userStr == "" || userStr == "<nil>" {
		userStr = "ADMIN"
	}

	insertSQL := `
		INSERT INTO public.glb_m_kendaraan (
			kend_id, kend_pemilik, kend_pt, kend_leasing, kend_alamat, kend_lokasiid,
			kend_merk, kend_type, kend_jenis, kend_thnbuat, kend_thnrakit,
			kend_isisilinder, kend_warna, kend_nik, kend_nomesin, kend_identid,
			kend_warnatnkb, kend_bahanbakar, kend_jarak, kend_warnaplat,
			kend_berlakustnk, kend_berlakupajak, kend_berlakukir,
			kend_serahterimatime, kend_serahterimaid, kend_sopir1, kend_sopir2,
			kend_aktifyn, kend_updateid, kend_updatetime
		) VALUES (
			?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
			NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
			NULLIF(REGEXP_REPLACE(?, '[^0-9]', '', 'g'), '')::integer,
			NULLIF(REGEXP_REPLACE(?, '[^0-9]', '', 'g'), '')::integer,
			NULLIF(REGEXP_REPLACE(?, '[^0-9]', '', 'g'), '')::integer,
			NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
			NULLIF(?, ''), NULLIF(?, ''),
			NULLIF(REGEXP_REPLACE(?, '[^0-9.]', '', 'g'), '')::numeric,
			NULLIF(?, ''),
			NULLIF(?, '')::date, NULLIF(?, '')::date, NULLIF(?, '')::date,
			NULLIF(?, '')::timestamp, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
			?, ?, CURRENT_TIMESTAMP
		)
	`

	err := database.Exec(insertSQL,
		req.KendID, req.KendPemilik, req.KendPT, req.KendLeasing, req.KendAlamat, req.KendLokasiID,
		req.KendMerk, req.KendType, req.KendJenis, req.KendThnBuat, req.KendThnRakit,
		req.KendIsiSilinder, req.KendWarna, req.KendNIK, req.KendNoMesin, req.KendIdentID,
		req.KendWarnaTNKB, req.KendBahanBakar, req.KendJarak, req.KendWarnaPlat,
		req.KendBerlakuSTNK, req.KendBerlakuPajak, req.KendBerlakuKIR,
		req.KendSerahTerimaTime, req.KendSerahTerimaID, req.KendSopir1, req.KendSopir2,
		req.KendAktifYN, userStr,
	).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan kendaraan: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success", "message": "Data kendaraan berhasil disimpan"})
}

// 5. UPDATE KENDARAAN
func UpdateKendaraanBaru(c *gin.Context) {
	database := resolveMasterKendaraanDB(c)
	kendID := strings.ToUpper(strings.TrimSpace(c.Param("id")))

	var req KendaraanInputReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid"})
		return
	}

	userLogin, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", userLogin)
	if userStr == "" || userStr == "<nil>" {
		userStr = "ADMIN"
	}

	updateSQL := `
		UPDATE public.glb_m_kendaraan SET
			kend_pemilik = NULLIF(?, ''),
			kend_pt = NULLIF(?, ''),
			kend_leasing = NULLIF(?, ''),
			kend_alamat = NULLIF(?, ''),
			kend_lokasiid = NULLIF(?, ''),
			kend_merk = NULLIF(?, ''),
			kend_type = NULLIF(?, ''),
			kend_jenis = NULLIF(?, ''),
			kend_thnbuat = NULLIF(REGEXP_REPLACE(?, '[^0-9]', '', 'g'), '')::integer,
			kend_thnrakit = NULLIF(REGEXP_REPLACE(?, '[^0-9]', '', 'g'), '')::integer,
			kend_isisilinder = NULLIF(REGEXP_REPLACE(?, '[^0-9]', '', 'g'), '')::integer,
			kend_warna = NULLIF(?, ''),
			kend_nik = NULLIF(?, ''),
			kend_nomesin = NULLIF(?, ''),
			kend_identid = NULLIF(?, ''),
			kend_warnatnkb = NULLIF(?, ''),
			kend_bahanbakar = NULLIF(?, ''),
			kend_jarak = NULLIF(REGEXP_REPLACE(?, '[^0-9.]', '', 'g'), '')::numeric,
			kend_warnaplat = NULLIF(?, ''),
			kend_berlakustnk = NULLIF(?, '')::date,
			kend_berlakupajak = NULLIF(?, '')::date,
			kend_berlakukir = NULLIF(?, '')::date,
			kend_serahterimatime = NULLIF(?, '')::timestamp,
			kend_serahterimaid = NULLIF(?, ''),
			kend_sopir1 = NULLIF(?, ''),
			kend_sopir2 = NULLIF(?, ''),
			kend_aktifyn = COALESCE(NULLIF(?, ''), kend_aktifyn),
			kend_updateid = ?,
			kend_updatetime = CURRENT_TIMESTAMP
		WHERE TRIM(kend_id) = ?
	`

	err := database.Exec(updateSQL,
		req.KendPemilik, req.KendPT, req.KendLeasing, req.KendAlamat, req.KendLokasiID,
		req.KendMerk, req.KendType, req.KendJenis, req.KendThnBuat, req.KendThnRakit,
		req.KendIsiSilinder, req.KendWarna, req.KendNIK, req.KendNoMesin, req.KendIdentID,
		req.KendWarnaTNKB, req.KendBahanBakar, req.KendJarak, req.KendWarnaPlat,
		req.KendBerlakuSTNK, req.KendBerlakuPajak, req.KendBerlakuKIR,
		req.KendSerahTerimaTime, req.KendSerahTerimaID, req.KendSopir1, req.KendSopir2,
		req.KendAktifYN, userStr, kendID,
	).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate kendaraan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data kendaraan berhasil diperbarui"})
}

// 6. DELETE KENDARAAN
func DeleteKendaraanBaru(c *gin.Context) {
	database := resolveMasterKendaraanDB(c)
	kendID := strings.ToUpper(strings.TrimSpace(c.Param("id")))

	if err := database.Table("public.glb_m_kendaraan").Where("TRIM(kend_id) = ?", kendID).Delete(&models.GlbMKendaraan{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus kendaraan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data kendaraan berhasil dihapus"})
}

// 7. PENDAFTARAN TRUK MASUK BENGKEL SERVICE
func RegisterTrukService(c *gin.Context) {
	database := resolveMasterKendaraanDB(c)
	kendid := strings.ToUpper(strings.TrimSpace(c.PostForm("kendid")))
	tglselesai := strings.TrimSpace(c.PostForm("tglselesai"))

	if kendid == "" || tglselesai == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Nopol dan Tanggal Estimasi Wajib Diisi!"})
		return
	}

	err := database.Table("public.glb_m_kendaraan").Where("TRIM(kend_id) = ?", kendid).Updates(map[string]interface{}{
		"kend_serviceyn":        "Y",
		"kend_servicestartdate": gorm.Expr("CURRENT_DATE"),
		"kend_serviceenddate":   tglselesai,
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memasukkan armada ke bengkel: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Truk sukses dikunci di dalam bengkel!"})
}

// 8. ALIAS / KOMPATIBILITAS ROUTE LAMA
func CreateKendaraan(c *gin.Context) { CreateKendaraanBaru(c) }
func UpdateKendaraan(c *gin.Context) { UpdateKendaraanBaru(c) }

func GetDaftarKendaraanList(c *gin.Context) {
	database := resolveMasterKendaraanDB(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset := (page - 1) * limit

	nopol := strings.TrimSpace(c.Query("nopol"))
	aktif := strings.TrimSpace(c.Query("aktif"))

	query := database.Table("public.glb_m_kendaraan")
	if nopol != "" {
		query = query.Where("UPPER(kend_id) LIKE UPPER(?)", "%"+nopol+"%")
	}
	if aktif != "" {
		query = query.Where("kend_aktifyn = ?", aktif)
	}

	var totalRecords int64
	query.Count(&totalRecords)

	var listData []models.GlbMKendaraan
	err := query.Order("kend_id ASC").Offset(offset).Limit(limit).Find(&listData).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data kendaraan: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          listData,
		"total_records": totalRecords,
		"page":          page,
		"limit":         limit,
	})
}

// 9. EXPORT DATA MASTER KENDARAAN KE EXCEL (CSV UTF-8 DENGAN BOM AGAR LANGSUNG RAPI DI EXCEL)
func ExportMasterKendaraanExcel(c *gin.Context) {
	database := resolveMasterKendaraanDB(c)

	nopol := strings.TrimSpace(c.Query("nopol"))
	aktif := strings.TrimSpace(c.Query("aktif"))

	whereClause := " WHERE 1=1 "
	var args []interface{}
	if nopol != "" {
		whereClause += " AND UPPER(k.kend_id) LIKE UPPER(?) "
		args = append(args, "%"+nopol+"%")
	}
	if aktif != "" {
		whereClause += " AND k.kend_aktifyn = ? "
		args = append(args, aktif)
	}

	query := fmt.Sprintf(`
		SELECT 
			TRIM(k.kend_id) AS kend_id,
			COALESCE(TRIM(k.kend_pemilik), '-') AS kend_pemilik,
			CASE k.kend_pt WHEN 'A' THEN 'DBS' WHEN 'B' THEN 'DLB' WHEN 'C' THEN 'DLI' ELSE COALESCE(k.kend_pt, '-') END AS pengguna,
			CASE k.kend_leasing WHEN 'A' THEN 'DBS' WHEN 'B' THEN 'DLB' WHEN 'C' THEN 'DLI' ELSE COALESCE(k.kend_leasing, '-') END AS leasing,
			COALESCE(TRIM(a.agen_nama), TRIM(k.kend_lokasiid), '-') AS cabang,
			COALESCE(TRIM(k.kend_merk), '-') AS kend_merk,
			COALESCE(TRIM(k.kend_type), '-') AS kend_type,
			COALESCE(TRIM(j.jnskend_nama), TRIM(k.kend_jenis), '-') AS jenis_mobil,
			COALESCE(CAST(k.kend_thnbuat AS VARCHAR), '-') AS thn_buat,
			COALESCE(CAST(k.kend_thnrakit AS VARCHAR), '-') AS thn_rakit,
			COALESCE(CAST(k.kend_isisilinder AS VARCHAR), '0') AS isi_silinder,
			COALESCE(TRIM(k.kend_warna), '-') AS warna,
			COALESCE(TRIM(k.kend_nik), '-') AS no_rangka,
			COALESCE(TRIM(k.kend_nomesin), '-') AS no_mesin,
			COALESCE(TRIM(k.kend_bahanbakar), '-') AS bahan_bakar,
			COALESCE(TO_CHAR(k.kend_berlakustnk, 'YYYY-MM-DD'), '-') AS berlaku_stnk,
			COALESCE(TO_CHAR(k.kend_berlakupajak, 'YYYY-MM-DD'), '-') AS berlaku_pajak,
			COALESCE(TO_CHAR(k.kend_berlakukir, 'YYYY-MM-DD'), '-') AS berlaku_kir,
			COALESCE(TRIM(p.kry_nama), TRIM(k.kend_serahterimaid), '-') AS penerima,
			COALESCE(TRIM(s1.kry_nama), TRIM(k.kend_sopir1), '-') AS driver_1,
			COALESCE(TRIM(s2.kry_nama), TRIM(k.kend_sopir2), '-') AS driver_2,
			CASE WHEN k.kend_serviceyn = 'Y' THEN 'UNDER SERVICE' ELSE 'READY' END AS status_bengkel,
			CASE WHEN k.kend_aktifyn = 'Y' THEN 'YA' ELSE 'TIDAK' END AS aktif
		FROM public.glb_m_kendaraan k
		LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id) = TRIM(k.kend_lokasiid)
		LEFT JOIN public.glb_m_jnskend j ON TRIM(j.jnskend_id) = TRIM(k.kend_jenis)
		LEFT JOIN public.hrd_m_karyawan p ON TRIM(p.kry_nip) = TRIM(k.kend_serahterimaid)
		LEFT JOIN public.hrd_m_karyawan s1 ON TRIM(s1.kry_nip) = TRIM(k.kend_sopir1)
		LEFT JOIN public.hrd_m_karyawan s2 ON TRIM(s2.kry_nip) = TRIM(k.kend_sopir2)
		%s
		ORDER BY k.kend_id ASC
	`, whereClause)

	type ExportRow struct {
		KendID        string `gorm:"column:kend_id"`
		KendPemilik   string `gorm:"column:kend_pemilik"`
		Pengguna      string `gorm:"column:pengguna"`
		Leasing       string `gorm:"column:leasing"`
		Cabang        string `gorm:"column:cabang"`
		KendMerk      string `gorm:"column:kend_merk"`
		KendType      string `gorm:"column:kend_type"`
		JenisMobil    string `gorm:"column:jenis_mobil"`
		ThnBuat       string `gorm:"column:thn_buat"`
		ThnRakit      string `gorm:"column:thn_rakit"`
		IsiSilinder   string `gorm:"column:isi_silinder"`
		Warna         string `gorm:"column:warna"`
		NoRangka      string `gorm:"column:no_rangka"`
		NoMesin       string `gorm:"column:no_mesin"`
		BahanBakar    string `gorm:"column:bahan_bakar"`
		BerlakuSTNK   string `gorm:"column:berlaku_stnk"`
		BerlakuPajak  string `gorm:"column:berlaku_pajak"`
		BerlakuKIR    string `gorm:"column:berlaku_kir"`
		Penerima      string `gorm:"column:penerima"`
		Driver1       string `gorm:"column:driver_1"`
		Driver2       string `gorm:"column:driver_2"`
		StatusBengkel string `gorm:"column:status_bengkel"`
		Aktif         string `gorm:"column:aktif"`
	}

	var rows []ExportRow
	if err := database.Raw(query, args...).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal export kendaraan: " + err.Error()})
		return
	}

	fileName := fmt.Sprintf("Master_Kendaraan_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Header("Content-Type", "text/csv; charset=utf-8")

	// Tulis UTF-8 BOM agar Excel langsung memisahkan kolom dengan rapi
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(c.Writer)
	writer.Comma = ';'
	defer writer.Flush()

	writer.Write([]string{
		"NO POLISI", "NAMA PEMILIK", "PENGGUNA", "LEASING", "CABANG",
		"MERK", "TIPE", "JENIS/MODEL", "THN BUAT", "THN RAKIT",
		"ISI SILINDER", "WARNA", "NO RANGKA/NIK", "NO MESIN", "BAHAN BAKAR",
		"BERLAKU STNK", "BERLAKU PAJAK", "BERLAKU KIR",
		"PENERIMA", "DRIVER 1", "DRIVER 2", "STATUS BENGKEL", "AKTIF",
	})

	for _, r := range rows {
		writer.Write([]string{
			r.KendID, r.KendPemilik, r.Pengguna, r.Leasing, r.Cabang,
			r.KendMerk, r.KendType, r.JenisMobil, r.ThnBuat, r.ThnRakit,
			r.IsiSilinder, r.Warna, r.NoRangka, r.NoMesin, r.BahanBakar,
			r.BerlakuSTNK, r.BerlakuPajak, r.BerlakuKIR,
			r.Penerima, r.Driver1, r.Driver2, r.StatusBengkel, r.Aktif,
		})
	}
}
