package handler

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// Pastikan struct TerimaReturItem memiliki Customer dan KotaTujuan:
type TerimaReturItem struct {
	NoBTT         string  `json:"no_btt"`
	TglKembali    string  `json:"tgl_kembali"`
	TglBTT        string  `json:"tgl_btt"`
	Customer      string  `json:"customer"`
	KotaTujuan    string  `json:"kota_tujuan"`
	KotaAsalRetur string  `json:"kota_asal_retur"`
	PembayaranID  int     `json:"pembayaran_id"`
	Pembayaran    string  `json:"pembayaran"`
	Harga         float64 `json:"harga"`
	BiayaPenerus  float64 `json:"biaya_penerus"`
	PackingID     float64 `json:"packing_id"`
	Disc          float64 `json:"disc"`
	TotalHarga    float64 `json:"total_harga"`
	NoSuratJalan  string  `json:"no_surat_jalan"`
	Aktif         string  `json:"aktif"`
}

// 1. GET LIST DENGAN FILTER LENGKAP & PAGINATION
func GetTerimaReturList(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database DLI belum terhubung"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "15"))
	offset := (page - 1) * limit

	// Parameter Filter
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	pembayaran := strings.TrimSpace(c.Query("pembayaran"))
	noBTT := strings.TrimSpace(c.Query("no_btt"))
	kota := strings.TrimSpace(c.Query("kota"))
	customer := strings.TrimSpace(c.Query("customer"))
	agenID := strings.TrimSpace(c.Query("agen_id"))

	// Base SQL Query menggabungkan 6 tabel legacy
	baseQuery := `
		FROM public.opr_t_ereturbttdetil rbd
		LEFT JOIN public.mkt_t_eterimaretur tr ON rbd.rbd_bttid = tr.trt_bttid
		LEFT JOIN public.opr_t_ereturbtt rb ON rbd.rbd_rbeid = rb.rb_eid
		LEFT JOIN public.glb_m_agen ag ON rb.rb_agenid = ag.agen_id
		LEFT JOIN public.mkt_t_econote cn ON tr.trt_bttid = cn.bttt_id
		LEFT JOIN public.mkt_m_customer cu ON cn.bttt_asalcustid = cu.cust_id
		WHERE 1=1
	`

	var conditions []string
	var args []interface{}

	if agenID != "" && agenID != "ALL" && !strings.Contains(strings.ToUpper(agenID), "PUSAT") {
		conditions = append(conditions, "tr.trt_agenid = ?")
		args = append(args, agenID)
	}

	if startDate != "" && endDate != "" {
		conditions = append(conditions, "tr.trt_tglkembali BETWEEN ? AND ?")
		args = append(args, startDate+" 00:00:00", endDate+" 23:59:59")
	}

	if pembayaran != "" {
		conditions = append(conditions, "TRIM(cn.bttt_pembayaran::text) = ?")
		args = append(args, pembayaran)
	}

	if noBTT != "" {
		conditions = append(conditions, "tr.trt_bttid ILIKE ?")
		args = append(args, "%"+noBTT+"%")
	}

	if kota != "" {
		conditions = append(conditions, "ag.agen_nama ILIKE ?")
		args = append(args, "%"+kota+"%")
	}

	if customer != "" {
		conditions = append(conditions, "cu.cust_name ILIKE ?")
		args = append(args, "%"+customer+"%")
	}

	if len(conditions) > 0 {
		baseQuery += " AND " + strings.Join(conditions, " AND ")
	}

	// Hitung Total Data
	var totalRecords int64
	countSQL := "SELECT COUNT(*) " + baseQuery
	if err := database.Raw(countSQL, args...).Scan(&totalRecords).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghitung data retur: " + err.Error()})
		return
	}

	// Ambil Data Baris dengan explicit casting PostgreSQL
	selectSQL := `
		SELECT 
			COALESCE(tr.trt_bttid, '') AS no_btt,
			COALESCE(TO_CHAR(tr.trt_tglkembali, 'YYYY-MM-DD'), '') AS tgl_kembali,
			COALESCE(TO_CHAR(cn.bttt_tanggal, 'YYYY-MM-DD'), '') AS tgl_btt,
			COALESCE(ag.agen_nama, '-') AS kota_asal_retur,
			COALESCE(NULLIF(cn.bttt_pembayaran::text, '')::integer, 0) AS pembayaran_id,
			CASE 
				WHEN TRIM(cn.bttt_pembayaran::text) = '1' THEN 'TUNAI'
				WHEN TRIM(cn.bttt_pembayaran::text) = '2' THEN 'KREDIT'
				ELSE 'TAGIH'
			END AS pembayaran,
			COALESCE(NULLIF(cn.bttt_harga::text, '')::numeric, 0) AS harga,
			COALESCE(NULLIF(cn.bttt_biayapenerus::text, '')::numeric, 0) AS biaya_penerus,
			COALESCE(NULLIF(cn.bttt_packingid::text, '')::numeric, 0) AS packing_id,
			COALESCE(NULLIF(cn.bttt_disc::text, '')::numeric, 0) AS disc,
			(
				COALESCE(NULLIF(cn.bttt_harga::text, '')::numeric, 0) + 
				COALESCE(NULLIF(cn.bttt_biayapenerus::text, '')::numeric, 0) + 
				COALESCE(NULLIF(cn.bttt_packingid::text, '')::numeric, 0) - 
				((COALESCE(NULLIF(cn.bttt_disc::text, '')::numeric, 0) / 100.0) * COALESCE(NULLIF(cn.bttt_harga::text, '')::numeric, 0))
			) AS total_harga,
			COALESCE(cn.bttt_nosuratjalan, '-') AS no_surat_jalan,
			CASE WHEN tr.trt_aktifyn = 'Y' THEN 'Ya' ELSE 'Tidak' END AS aktif
	` + baseQuery + " ORDER BY tr.trt_tglkembali DESC LIMIT ? OFFSET ?"

	queryArgs := append(args, limit, offset)
	var list []TerimaReturItem

	if err := database.Raw(selectSQL, queryArgs...).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengambil data retur: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          list,
		"total_records": totalRecords,
		"page":          page,
		"limit":         limit,
		"total_pages":   int(math.Ceil(float64(totalRecords) / float64(limit))),
	})
}

// 2. NONAKTIFKAN / HAPUS RETUR (TRt_AktifYN = 'T')
func DeactivateTerimaRetur(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	noBTT := c.Param("btt_id")
	if noBTT == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor BTT wajib diisi"})
		return
	}

	err := database.Exec("UPDATE public.mkt_t_eterimaretur SET trt_aktifyn = 'T' WHERE trt_bttid = ?", noBTT).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menonaktifkan retur: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("BTT Retur %s berhasil dinonaktifkan", noBTT)})
}

// Struct payload input simpan retur
type CreateTerimaReturReq struct {
	NoBTT      string `json:"no_btt" binding:"required"`
	TglKembali string `json:"tgl_kembali" binding:"required"`
}

// 3. GET INFO BTT / CEK VALIDASI SEBELUM TERIMA RETUR
func GetInfoBTTForRetur(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	bttID := strings.TrimSpace(c.Param("btt_id"))
	if bttID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor BTT wajib diisi"})
		return
	}

	type BTTInfo struct {
		NoBTT        string  `json:"no_btt"`
		TglBTT       string  `json:"tgl_btt"`
		Pengirim     string  `json:"pengirim"`
		Tujuan       string  `json:"tujuan"`
		TotalHarga   float64 `json:"total_harga"`
		NoSuratJalan string  `json:"no_surat_jalan"`
		StatusRetur  string  `json:"status_retur"`
	}

	var info BTTInfo
	query := `
		SELECT 
			cn.bttt_id AS no_btt,
			COALESCE(TO_CHAR(cn.bttt_tanggal, 'YYYY-MM-DD'), '') AS tgl_btt,
			COALESCE(cu.cust_name, '-') AS pengirim,
			COALESCE(cn.bttt_tujuankota, '-') AS tujuan,
			(COALESCE(cn.bttt_harga::numeric, 0) + COALESCE(cn.bttt_biayapenerus::numeric, 0) + COALESCE(cn.bttt_packingid::numeric, 0) - ((COALESCE(cn.bttt_disc::numeric, 0)/100.0) * COALESCE(cn.bttt_harga::numeric, 0))) AS total_harga,
			COALESCE(cn.bttt_nosuratjalan, '-') AS no_surat_jalan,
			COALESCE(tr.trt_aktifyn, 'N') AS status_retur
		FROM public.mkt_t_econote cn
		LEFT JOIN public.mkt_m_customer cu ON cn.bttt_asalcustid = cu.cust_id
		LEFT JOIN public.mkt_t_eterimaretur tr ON cn.bttt_id = tr.trt_bttid AND tr.trt_aktifyn = 'Y'
		WHERE cn.bttt_id = ?
		LIMIT 1
	`

	if err := database.Raw(query, bttID).Scan(&info).Error; err != nil || info.NoBTT == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Nomor BTT tidak ditemukan dalam sistem Conote"})
		return
	}

	if info.StatusRetur == "Y" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "BTT ini sudah pernah diterima sebagai retur!"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": info})
}

// 4. PROSES SIMPAN PENERIMAAN RETUR (TRANSACTION: HISTORY + TERIMA RETUR + UPDATE ECONOTE)
func CreateTerimaRetur(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req CreateTerimaReturReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload tidak valid"})
		return
	}

	agenID := fmt.Sprintf("%v", c.MustGet("agen_id"))
	if agenID == "" || agenID == "<nil>" {
		agenID = "001"
	}
	// Pastikan 3 digit format cabang
	kdcbg := fmt.Sprintf("%03s", strings.TrimSpace(agenID))
	if len(kdcbg) > 3 {
		kdcbg = kdcbg[:3]
	}

	username := fmt.Sprintf("%v", c.MustGet("username"))
	if username == "" || username == "<nil>" {
		username = "superdli"
	}

	// Buat Kepala Hist (kdcbg + YYYYMM)
	tglStr := strings.ReplaceAll(req.TglKembali, "-", "")
	kplhist := kdcbg + tglStr[:6]

	// Mulai Database Transaction
	tx := database.Begin()

	// 1. Generate Hist_ID
	var lastEko sql.NullString
	queryCounter := `
		SELECT RIGHT(hist_id, 7) AS eko 
		FROM public.mkt_t_ehistory 
		WHERE LEFT(hist_id, 9) = ? 
		ORDER BY RIGHT(hist_id, 7) DESC 
		LIMIT 1
	`
	tx.Raw(queryCounter, kplhist).Scan(&lastEko)

	hitunghist := 1
	if lastEko.Valid && lastEko.String != "" {
		if val, err := strconv.Atoi(lastEko.String); err == nil {
			hitunghist = val + 1
		}
	}
	hID := fmt.Sprintf("%s%07d", kplhist, hitunghist)

	// 2. Insert ke mkt_t_ehistory (Status retur = 16)
	insertHistSQL := `
		INSERT INTO public.mkt_t_ehistory (hist_id, hist_bttid, hist_agenid, hist_staturut, hist_tanggal)
		VALUES (?, ?, ?, 16, ?::timestamp)
	`
	if err := tx.Exec(insertHistSQL, hID, req.NoBTT, agenID, req.TglKembali+" 00:00:00").Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mencatat history: " + err.Error()})
		return
	}

	// 3. Upsert / Insert ke mkt_t_eterimaretur
	insertReturSQL := `
		INSERT INTO public.mkt_t_eterimaretur 
			(trt_agenid, trt_tglkembali, trt_bttid, trt_aktifyn, trt_updateid, trt_updatetime, trt_histid)
		VALUES 
			(?, ?::timestamp, ?, 'Y', ?, CURRENT_TIMESTAMP, ?)
		ON CONFLICT (trt_bttid) DO UPDATE 
		SET trt_tglkembali = EXCLUDED.trt_tglkembali,
		    trt_aktifyn = 'Y',
		    trt_updateid = EXCLUDED.trt_updateid,
		    trt_updatetime = CURRENT_TIMESTAMP,
		    trt_histid = EXCLUDED.trt_histid
	`
	if err := tx.Exec(insertReturSQL, agenID, req.TglKembali+" 00:00:00", req.NoBTT, username, hID).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan penerimaan retur: " + err.Error()})
		return
	}

	// 4. Update mkt_t_econote BTTT_KirimYN = 'Y'
	if err := tx.Exec("UPDATE public.mkt_t_econote SET bttt_kirimyn = 'Y' WHERE bttt_id = ?", req.NoBTT).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update status BTT Conote: " + err.Error()})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("BTT Retur %s berhasil diproses dan dicatat", req.NoBTT),
		"hist_id": hID,
	})
}

// Struct opsi untuk dropdown
type OptionItem struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Handler mengambil opsi dropdown Kota Tujuan & Customer
func GetFilterOptionsTerimaRetur(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var kotaOptions []OptionItem
	var custOptions []OptionItem

	// 1. Ambil Kota Tujuan dari Master Agen / Conote yang ada
	queryKota := `
		SELECT DISTINCT UPPER(TRIM(agen_nama)) AS value, UPPER(TRIM(agen_nama)) AS label
		FROM public.glb_m_agen
		WHERE agen_nama IS NOT NULL AND agen_nama <> ''
		ORDER BY label ASC
	`
	if err := database.Raw(queryKota).Scan(&kotaOptions).Error; err != nil || len(kotaOptions) == 0 {
		// Fallback jika glb_m_agen kosong, ambil dari conote
		database.Raw(`
			SELECT DISTINCT UPPER(TRIM(bttt_tujuankota)) AS value, UPPER(TRIM(bttt_tujuankota)) AS label 
			FROM public.mkt_t_econote 
			WHERE bttt_tujuankota IS NOT NULL AND bttt_tujuankota <> '' 
			ORDER BY label ASC
		`).Scan(&kotaOptions)
	}

	// 2. Ambil Customer / Pengirim dari Master Customer
	queryCust := `
		SELECT cust_id AS value, (cust_id || ' - ' || cust_name) AS label 
		FROM public.mkt_m_customer 
		ORDER BY cust_name ASC
		LIMIT 300
	`
	database.Raw(queryCust).Scan(&custOptions)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"kota":   kotaOptions,
		"cust":   custOptions,
	})
}
