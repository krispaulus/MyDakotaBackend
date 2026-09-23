package handler

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// Response struct sesuai tabel ASP mkt_t_econote_bayar
type EconoteBayarItem struct {
	PPKeID     string  `json:"ppk_eid"`
	PPKTanggal string  `json:"ppk_tanggal"`
	AgenNama   string  `json:"agen_nama"`
	NoBTT      string  `json:"no_btt"`
	Pelanggan  string  `json:"pelanggan"`
	BiayaKirim float64 `json:"biaya_kirim"`
	Dibayar    float64 `json:"dibayar"`
	Aktif      string  `json:"aktif"`
}

// 1. GET LIST PEMBAYARAN KASIR DENGAN FILTER & PAGINATION
func GetEconoteBayarList(c *gin.Context) {
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
	noBTT := strings.TrimSpace(c.Query("no_btt"))
	agenID := strings.TrimSpace(c.Query("agen_id"))

	// Ambil agen_id dari auth context bila tidak dikirim dari query
	if agenID == "" {
		if authAgen, exists := c.Get("agen_id"); exists && authAgen != nil {
			agenID = fmt.Sprintf("%v", authAgen)
		}
	}

	baseQuery := `
		FROM public.mkt_t_econote_bayar b
		LEFT JOIN public.glb_m_agen ag ON b.ppk_agenid = ag.agen_id
		LEFT JOIN public.mkt_t_econote cn ON b.ppk_bttid = cn.bttt_id
		WHERE 1=1
	`

	var conditions []string
	var args []interface{}

	// Filter batas agen jika bukan holding / PUSAT
	if agenID != "" && agenID != "ALL" && !strings.Contains(strings.ToUpper(agenID), "PUSAT") {
		conditions = append(conditions, "b.ppk_agenid = ?")
		args = append(args, agenID)
	}

	if startDate != "" && endDate != "" {
		conditions = append(conditions, "b.ppk_tanggal BETWEEN ? AND ?")
		args = append(args, startDate+" 00:00:00", endDate+" 23:59:59")
	}

	if noBTT != "" {
		conditions = append(conditions, "b.ppk_bttid ILIKE ?")
		args = append(args, "%"+noBTT+"%")
	}

	if len(conditions) > 0 {
		baseQuery += " AND " + strings.Join(conditions, " AND ")
	}

	// Hitung Total Data
	var totalRecords int64
	countSQL := "SELECT COUNT(*) " + baseQuery
	if err := database.Raw(countSQL, args...).Scan(&totalRecords).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghitung data kasir: " + err.Error()})
		return
	}

	// Ambil Data Baris
	selectSQL := `
		SELECT 
			COALESCE(b.ppk_eid, '') AS ppk_eid,
			COALESCE(TO_CHAR(b.ppk_tanggal, 'YYYY-MM-DD'), '') AS ppk_tanggal,
			COALESCE(ag.agen_nama, '-') AS agen_nama,
			COALESCE(b.ppk_bttid, '') AS no_btt,
			COALESCE(cn.bttt_asalname, '-') AS pelanggan,
			(
				COALESCE(NULLIF(cn.bttt_harga::text, '')::numeric, 0) + 
				COALESCE(NULLIF(cn.bttt_biayapenerus::text, '')::numeric, 0) + 
				COALESCE(NULLIF(cn.bttt_packingid::text, '')::numeric, 0) - 
				((COALESCE(NULLIF(cn.bttt_disc::text, '')::numeric, 0) / 100.0) * COALESCE(NULLIF(cn.bttt_harga::text, '')::numeric, 0))
			) AS biaya_kirim,
			COALESCE(b.ppk_bayar, 0) AS dibayar,
			CASE WHEN b.ppk_aktifyn = 'Y' THEN 'Ya' ELSE 'Tidak' END AS aktif
	` + baseQuery + " ORDER BY b.ppk_eid DESC, b.ppk_tanggal DESC LIMIT ? OFFSET ?"

	queryArgs := append(args, limit, offset)
	var list []EconoteBayarItem

	if err := database.Raw(selectSQL, queryArgs...).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengambil data kasir: " + err.Error()})
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

// 2. NONAKTIFKAN TRANSAKSI BAYAR (PPK_AktifYN = 'T')
func DeactivateEconoteBayar(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	ppkEID := c.Param("eid")
	if ppkEID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Kode pembayaran wajib diisi"})
		return
	}

	err := database.Exec("UPDATE public.mkt_t_econote_bayar SET ppk_aktifyn = 'T' WHERE ppk_eid = ?", ppkEID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menonaktifkan pembayaran: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Pembayaran %s berhasil dinonaktifkan", ppkEID)})
}

// Payload input pembayaran kasir
type CreateEconoteBayarReq struct {
	NoBTT   string  `json:"no_btt" binding:"required"`
	Tanggal string  `json:"tanggal" binding:"required"`
	Bayar   float64 `json:"bayar" binding:"required"`
}

// 3. GET DETAIL BTT SEBELUM BAYAR KASIR
func GetBTTForBayar(c *gin.Context) {
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

	type BTTBayarInfo struct {
		NoBTT        string  `json:"no_btt"`
		TglBTT       string  `json:"tgl_btt"`
		Pengirim     string  `json:"pengirim"`
		Penerima     string  `json:"penerima"`
		KotaTujuan   string  `json:"kota_tujuan"`
		TotalBiaya   float64 `json:"total_biaya"`
		SudahDibayar float64 `json:"sudah_dibayar"`
		SisaBayar    float64 `json:"sisa_bayar"`
		BayarYN      string  `json:"bayar_yn"`
	}

	var info BTTBayarInfo
	query := `
		SELECT 
			cn.bttt_id AS no_btt,
			COALESCE(TO_CHAR(cn.bttt_tanggal, 'YYYY-MM-DD'), '') AS tgl_btt,
			COALESCE(cn.bttt_asalname, '-') AS pengirim,
			COALESCE(cn.bttt_tujuanname, '-') AS penerima,
			COALESCE(cn.bttt_tujuankota, '-') AS kota_tujuan,
			(
				COALESCE(NULLIF(cn.bttt_harga::text, '')::numeric, 0) + 
				COALESCE(NULLIF(cn.bttt_biayapenerus::text, '')::numeric, 0) + 
				COALESCE(NULLIF(cn.bttt_packingid::text, '')::numeric, 0) - 
				((COALESCE(NULLIF(cn.bttt_disc::text, '')::numeric, 0) / 100.0) * COALESCE(NULLIF(cn.bttt_harga::text, '')::numeric, 0))
			) AS total_biaya,
			COALESCE((
				SELECT SUM(b.ppk_bayar) 
				FROM public.mkt_t_econote_bayar b 
				WHERE b.ppk_bttid = cn.bttt_id AND b.ppk_aktifyn = 'Y'
			), 0) AS sudah_dibayar,
			COALESCE(cn.bttt_bayaryn, 'T') AS bayar_yn
		FROM public.mkt_t_econote cn
		WHERE cn.bttt_id = ?
		LIMIT 1
	`

	if err := database.Raw(query, bttID).Scan(&info).Error; err != nil || info.NoBTT == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Nomor BTT tidak ditemukan dalam sistem Conote"})
		return
	}

	info.SisaBayar = info.TotalBiaya - info.SudahDibayar
	if info.SisaBayar < 0 {
		info.SisaBayar = 0
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": info})
}

// 4. PROSES SIMPAN PEMBAYARAN KASIR (GENERATE PPK_EID + INSERT + UPDATE ECONOTE)
func CreateEconoteBayar(c *gin.Context) {
	database := db.DLIDB
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req CreateEconoteBayarReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid"})
		return
	}

	if req.Bayar <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Jumlah pembayaran harus lebih dari 0"})
		return
	}

	agenID := fmt.Sprintf("%v", c.MustGet("agen_id"))
	if agenID == "" || agenID == "<nil>" {
		agenID = "001"
	}
	kdcbg := fmt.Sprintf("%03s", strings.TrimSpace(agenID))
	if len(kdcbg) > 3 {
		kdcbg = kdcbg[:3]
	}

	username := fmt.Sprintf("%v", c.MustGet("username"))
	if username == "" || username == "<nil>" {
		username = "superdli"
	}

	// Buat Format Kepala (kdcbg + YYYYMM)
	tglStr := strings.ReplaceAll(req.Tanggal, "-", "")
	if len(tglStr) < 6 {
		tglStr = "20260901"
	}
	kepala := kdcbg + tglStr[:6]

	tx := database.Begin()

	// Hitung counter terakhir 6 digit
	var lastEko struct {
		Eko string
	}
	queryCounter := `
		SELECT RIGHT(ppk_eid, 6) AS eko 
		FROM public.mkt_t_econote_bayar 
		WHERE LEFT(ppk_eid, 9) = ? 
		ORDER BY RIGHT(ppk_eid, 6) DESC 
		LIMIT 1
	`
	tx.Raw(queryCounter, kepala).Scan(&lastEko)

	counter := 1
	if lastEko.Eko != "" {
		if val, err := strconv.Atoi(lastEko.Eko); err == nil {
			counter = val + 1
		}
	}
	ppkEID := fmt.Sprintf("%s%06d", kepala, counter)

	// 1. Insert ke mkt_t_econote_bayar
	insertSQL := `
		INSERT INTO public.mkt_t_econote_bayar 
			(ppk_eid, ppk_agenid, ppk_tanggal, ppk_bttid, ppk_bayar, ppk_updateid, ppk_updatetime, ppk_aktifyn)
		VALUES 
			(?, ?, ?::timestamp, ?, ?, ?, CURRENT_TIMESTAMP, 'Y')
	`
	if err := tx.Exec(insertSQL, ppkEID, agenID, req.Tanggal+" 00:00:00", req.NoBTT, req.Bayar, username).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mencatat pembayaran kasir: " + err.Error()})
		return
	}

	// 2. Update status pelunasan di mkt_t_econote
	if err := tx.Exec("UPDATE public.mkt_t_econote SET bttt_bayaryn = 'Y' WHERE bttt_id = ?", req.NoBTT).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update status Conote: " + err.Error()})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Pembayaran BTT %s berhasil disimpan dengan kode %s", req.NoBTT, ppkEID),
		"ppk_eid": ppkEID,
	})
}
