package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// Model Header Laporan Penjualan Harian
type PenjualanHarianHeaderItem struct {
	BTTHID        string  `json:"btth_id"`
	BTTH居Tanggal  string  `json:"btth_tanggal"`
	AgenNama      string  `json:"agen_nama"`
	JnBayar       string  `json:"jnbayar"`
	PembayaranID  string  `json:"pembayaran_id"`
	BTTHCBID      string  `json:"btth_cbid"`
	BTTHPostingYN string  `json:"btth_postingyn"`
	BTTHTjurhNo   string  `json:"btth_tjurhno"`
	BTTHNoKW      string  `json:"btth_nokw"`
	BTTHActiveYN  string  `json:"btth_activeyn"`
	TotalBTT      int     `json:"total_btt"`
	TotalColly    int     `json:"total_colly"`
	TotalBerat    float64 `json:"total_berat"`
	TotalNominal  float64 `json:"total_nominal"`
}

// Model Detail Resi BTT per Laporan
type PenjualanHarianDetailItem struct {
	BTTDBTTHID   string  `json:"bttd_btthid"`
	BTTDBTTID    string  `json:"bttd_bttid"`
	BTTTTanggal  string  `json:"bttt_tanggal"`
	Pengirim     string  `json:"pengirim"`
	Penerima     string  `json:"penerima"`
	BTTTJmlUnit  int     `json:"bttt_jmlunit"`
	BTTTBerat    float64 `json:"bttt_berat"`
	BiayaKirim   float64 `json:"biaya_kirim"`
	BiayaPenerus float64 `json:"biaya_penerus"`
	TotalBiaya   float64 `json:"total_biaya"`
}

// GET /api/laporan/penjualan-harian/data
func GetLaporanPenjualanHarian(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	tglAwal := strings.TrimSpace(c.Query("start_date"))
	tglAkhir := strings.TrimSpace(c.Query("end_date"))
	chkTanggal := c.Query("chktanggal") == "true"
	cabang := strings.TrimSpace(c.Query("cabang"))
	chkCabang := c.Query("chkcabang") == "true"
	pembayaran := strings.TrimSpace(c.Query("pembayaran"))
	chkPembayaran := c.Query("chkpembayaran") == "true"
	posting := strings.TrimSpace(c.Query("posting"))
	chkPosting := c.Query("chkposting") == "true"
	noLap := strings.TrimSpace(c.Query("nolap"))
	chkNoLap := c.Query("chknolap") == "true"

	query := `
		SELECT 
			h.btth_id,
			COALESCE(TO_CHAR(h.btth_tanggal, 'YYYY-MM-DD'), '') AS btth_tanggal,
			COALESCE(
				ag.agen_nama,
				CASE 
					WHEN UPPER(TRIM(SPLIT_PART(h.btth_id, '/', 4))) = 'SB' THEN 'DLI SURABAYA'
					WHEN UPPER(TRIM(SPLIT_PART(h.btth_id, '/', 4))) = 'JKT' THEN 'DLI JAKARTA'
					WHEN UPPER(TRIM(SPLIT_PART(h.btth_id, '/', 4))) = 'BDG' THEN 'DLI BANDUNG'
					WHEN UPPER(TRIM(SPLIT_PART(h.btth_id, '/', 4))) = 'SMG' THEN 'DLI SEMARANG'
					ELSE 'CABANG ' || NULLIF(TRIM(SPLIT_PART(h.btth_id, '/', 4)), '')
				END,
				'CABANG PUSAT'
			) AS agen_nama,
			COALESCE(NULLIF(TRIM(h.btth_pembayaran::text), ''), '1') AS pembayaran_id,
			CASE 
				WHEN TRIM(h.btth_pembayaran::text) = '1' THEN 'Tunai'
				WHEN TRIM(h.btth_pembayaran::text) = '2' THEN 'Kredit'
				WHEN TRIM(h.btth_pembayaran::text) = '3' THEN 'Tagih Naik'
				WHEN TRIM(h.btth_pembayaran::text) = '4' THEN 'Order Jemput'
				WHEN TRIM(h.btth_pembayaran::text) = '5' THEN 'Tagih Turun'
				ELSE 'Tunai'
			END AS jnbayar,
			COALESCE(NULLIF(TRIM(h.btth_cbid), ''), '-') AS btth_cbid,
			COALESCE(NULLIF(TRIM(h.btth_postingyn), ''), 'N') AS btth_postingyn,
			COALESCE(NULLIF(TRIM(h.btth_tjurhno), ''), '-') AS btth_tjurhno,
			COALESCE(NULLIF(TRIM(h.btth_nokw), ''), '-') AS btth_nokw,
			COALESCE(NULLIF(TRIM(h.btth_activeyn), ''), 'Y') AS btth_activeyn,
			COALESCE(sub.jml_btt, 0) AS total_btt
		FROM public.art_t_penjualanbtth h
		LEFT JOIN (
			SELECT bttd_btthid, COUNT(*) AS jml_btt
			FROM public.art_t_penjualanbttd
			GROUP BY bttd_btthid
		) sub ON TRIM(h.btth_id) = TRIM(sub.bttd_btthid)
		LEFT JOIN public.glb_m_agen ag ON TRIM(h.btth_agenid::text) = TRIM(ag.agen_id::text)
		WHERE 1=1
	`

	var args []interface{}

	if chkTanggal && tglAwal != "" && tglAkhir != "" {
		query += " AND h.btth_tanggal BETWEEN ? AND ?"
		args = append(args, tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}
	if chkCabang && cabang != "" {
		query += " AND (ag_prefix.agen_nama = ? OR ag_id.agen_nama = ? OR ag_num.agen_nama = ?)"
		args = append(args, cabang, cabang, cabang)
	}
	if chkPembayaran && pembayaran != "" {
		query += " AND TRIM(h.btth_pembayaran::text) = ?"
		args = append(args, pembayaran)
	}
	if chkPosting && posting != "" {
		query += " AND TRIM(h.btth_postingyn) = ?"
		args = append(args, posting)
	}
	if chkNoLap && noLap != "" {
		query += " AND h.btth_id ILIKE ?"
		args = append(args, "%"+noLap+"%")
	}

	query += " ORDER BY h.btth_tanggal DESC, h.btth_id DESC LIMIT 300"

	var results []PenjualanHarianHeaderItem
	database.Raw(query, args...).Scan(&results)

	if results == nil {
		results = []PenjualanHarianHeaderItem{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 2. GET /api/laporan/penjualan-harian/detail/:id (Rincian BTT Laporan)
func GetDetailLaporanPenjualanHarian(c *gin.Context) {
	btthID := strings.TrimSpace(c.Param("id"))
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	query := `
		SELECT 
			d.bttd_btthid,
			d.bttd_bttid,
			COALESCE(TO_CHAR(c.bttt_tanggal, 'YYYY-MM-DD'), '-') AS bttt_tanggal,
			COALESCE(NULLIF(c.bttt_asalname, ''), '-') AS pengirim,
			COALESCE(NULLIF(c.bttt_tujuannama, ''), '-') AS penerima,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_jmlunit::text, '[^0-9]', '', 'g'), '')::integer, 1) AS bttt_jmlunit,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_berat::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS bttt_berat,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_harga::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS biaya_kirim,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_biayapenerus::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS biaya_penerus,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_harga::text, '[^0-9.]', '', 'g'), '')::numeric, 0) + 
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_biayapenerus::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS total_biaya
		FROM public.art_t_penjualanbttd d
		LEFT JOIN public.mkt_t_econote c ON d.bttd_bttid = c.bttt_id
		WHERE d.bttd_btthid = ?
		ORDER BY d.bttd_bttid ASC
	`

	var details []PenjualanHarianDetailItem
	database.Raw(query, btthID).Scan(&details)

	if details == nil {
		details = []PenjualanHarianDetailItem{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": details})
}

// 3. GET /api/laporan/penjualan-harian/btt-tersedia (Cari Resi yang Belum Masuk Laporan)
func GetBTTTersediaUntukLaporan(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	agenID := strings.TrimSpace(c.Query("agen_id"))
	pembayaran := strings.TrimSpace(c.DefaultQuery("pembayaran", "1"))

	query := `
		SELECT 
			c.bttt_id,
			COALESCE(TO_CHAR(c.bttt_tanggal, 'YYYY-MM-DD'), '-') AS bttt_tanggal,
			COALESCE(NULLIF(c.bttt_asalname, ''), '-') AS pengirim,
			COALESCE(NULLIF(c.bttt_tujuannama, ''), '-') AS penerima,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_jmlunit::text, '[^0-9]', '', 'g'), '')::integer, 1) AS bttt_jmlunit,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_berat::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS bttt_berat,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_harga::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS biaya_kirim,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_biayapenerus::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS biaya_penerus,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_harga::text, '[^0-9.]', '', 'g'), '')::numeric, 0) + 
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_biayapenerus::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS total_biaya
		FROM public.mkt_t_econote c
		LEFT JOIN public.art_t_penjualanbttd d ON c.bttt_id = d.bttd_bttid
		WHERE d.bttd_bttid IS NULL
	`
	var args []interface{}
	if pembayaran != "" {
		query += " AND c.bttt_pembayaran = ?"
		args = append(args, pembayaran)
	}
	if agenID != "" {
		query += " AND c.bttt_asalagenid = ?"
		args = append(args, agenID)
	}
	query += " ORDER BY c.bttt_tanggal DESC LIMIT 100"

	var results []PenjualanHarianDetailItem
	database.Raw(query, args...).Scan(&results)

	if results == nil {
		results = []PenjualanHarianDetailItem{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 4. POST /api/laporan/penjualan-harian/create (Buat Laporan Baru + Closing Resi)
type CreatePenjualanHarianPayload struct {
	Tanggal    string   `json:"tanggal"`
	AgenID     string   `json:"agen_id"`
	Pembayaran string   `json:"pembayaran"`
	NoBTTList  []string `json:"nobtt_list"`
}

func CreateLaporanPenjualanHarian(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	var req CreatePenjualanHarianPayload
	if err := c.ShouldBindJSON(&req); err != nil || len(req.NoBTTList) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Data tidak valid atau daftar resi kosong"})
		return
	}

	tx := database.Begin()
	now := time.Now()
	// Format ID Laporan: LPH/TAHUNBULAN/RANDOM/SEQ
	noLap := fmt.Sprintf("LPH%s%04d", now.Format("060102"), now.Nanosecond()%10000)

	// Simpan Header
	sqlHeader := `
		INSERT INTO public.art_t_penjualanbtth 
		(btth_id, btth_tanggal, btth_agenid, btth_pembayaran, btth_postingyn, btth_activeyn, btth_cbid, btth_tjurhno, btth_nokw, btth_updatetime)
		VALUES (?, ?, ?, ?, 'N', 'Y', '-', '-', '-', CURRENT_TIMESTAMP)
	`
	tglLaporan := req.Tanggal
	if tglLaporan == "" {
		tglLaporan = now.Format("2006-01-02")
	}

	if err := tx.Exec(sqlHeader, noLap, tglLaporan, req.AgenID, req.Pembayaran).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan header: " + err.Error()})
		return
	}

	// Simpan Detail Resi
	for _, bttID := range req.NoBTTList {
		sqlDetail := `INSERT INTO public.art_t_penjualanbttd (bttd_btthid, bttd_bttid) VALUES (?, ?)`
		if err := tx.Exec(sqlDetail, noLap, bttID).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan rincian resi: " + err.Error()})
			return
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Laporan Penjualan berhasil dibuat", "nolap": noLap})
}

// 5. POST /api/laporan/penjualan-harian/toggle-posting (Posting / Unposting)
type TogglePostingPayload struct {
	BTTHID string `json:"btth_id"`
	Action string `json:"action"` // POSTING atau UNPOSTING
}

func TogglePostingPenjualanHarian(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	var req TogglePostingPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload tidak valid"})
		return
	}

	newStatus := "N"
	if strings.ToUpper(req.Action) == "POSTING" {
		newStatus = "Y"
	}

	err := database.Exec("UPDATE public.art_t_penjualanbtth SET btth_postingyn = ?, btth_updatetime = CURRENT_TIMESTAMP WHERE btth_id = ?", newStatus, req.BTTHID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengubah status posting: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Laporan berhasil di-%s", req.Action), "new_status": newStatus})
}
