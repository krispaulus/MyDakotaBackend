package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

type BttOutstandingItem struct {
	BTTTID       string  `gorm:"column:bttt_id" json:"bttt_id"`
	Tanggal      string  `gorm:"column:bttt_tanggal" json:"bttt_tanggal"`
	Colly        int     `gorm:"column:bttt_jmlunit" json:"bttt_jmlunit"`
	Berat        float64 `gorm:"column:bttt_berat" json:"bttt_berat"`
	NamaBarang   string  `gorm:"column:bttt_namabarang" json:"bttt_namabarang"`
	TujuanKota   string  `gorm:"column:bttt_tujuankota" json:"bttt_tujuankota"`
	PenerimaAgen string  `gorm:"column:agen_nama2" json:"agen_nama2"`
	TujuanAlamat string  `gorm:"column:bttt_tujuanalamat" json:"bttt_tujuanalamat"`
	Harga        float64 `gorm:"column:bttt_harga" json:"bttt_harga"`
	AsalName     string  `gorm:"column:bttt_asalname" json:"bttt_asalname"`
	Pembayaran   string  `gorm:"column:pembayaran_label" json:"pembayaran_label"`
	StatusBayar  string  `gorm:"column:status_bayar" json:"status_bayar"`
}

// 1. GET /api/laporan/btt-outstanding/data
func GetBttOutstanding(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	useTanggal := c.Query("use_tanggal") == "true"
	tglAwal := strings.TrimSpace(c.Query("start_date"))
	tglAkhir := strings.TrimSpace(c.Query("end_date"))
	cabang := strings.TrimSpace(c.Query("cabang"))
	tujuan := strings.TrimSpace(c.Query("tujuan"))
	pengirim := strings.TrimSpace(c.Query("pengirim"))
	noBtt := strings.TrimSpace(c.Query("nobtt"))
	pembayaran := strings.TrimSpace(c.Query("pembayaran"))

	query := `
		SELECT 
			ec.bttt_id,
			COALESCE(TO_CHAR(ec.bttt_tanggal, 'YYYY-MM-DD'), '') AS bttt_tanggal,
			COALESCE(ec.bttt_jmlunit, 1) AS bttt_jmlunit,
			COALESCE(ec.bttt_berat, 0) AS bttt_berat,
			COALESCE(NULLIF(TRIM(ec.bttt_namabarang), ''), '-') AS bttt_namabarang,
			COALESCE(NULLIF(TRIM(ec.bttt_tujuankota), ''), '-') AS bttt_tujuankota,
			CASE 
				WHEN ag2.agen_nama IS NOT NULL AND TRIM(ag2.agen_nama) <> '' THEN ag2.agen_nama
				WHEN ec.bttt_tujuanagenid::text NOT IN ('0', '', '-') THEN 'CABANG ' || ec.bttt_tujuanagenid::text
				ELSE COALESCE(NULLIF(TRIM(ec.bttt_tujuankota), ''), '-')
			END AS agen_nama2,
			COALESCE(NULLIF(TRIM(ec.bttt_tujuanalamat), ''), '-') AS bttt_tujuanalamat,
			COALESCE(ec.bttt_harga, 0) AS bttt_harga,
			COALESCE(NULLIF(TRIM(ec.bttt_asalname), ''), '-') AS bttt_asalname,
			CASE 
				WHEN TRIM(ec.bttt_pembayaran::text) = '1' THEN 'TUNAI'
				WHEN TRIM(ec.bttt_pembayaran::text) = '2' THEN 'KREDIT'
				ELSE 'TAGIH'
			END AS pembayaran_label,
			CASE 
				WHEN UPPER(TRIM(ec.bttt_bayaryn::text)) = 'N' THEN 'BELUM'
				ELSE 'SUDAH'
			END AS status_bayar
		FROM public.mkt_t_econote ec
		LEFT JOIN public.glb_m_agen ag ON TRIM(ec.bttt_asalagenid::text) = TRIM(ag.agen_id::text)
		LEFT JOIN public.glb_m_agen ag2 ON TRIM(ec.bttt_tujuanagenid::text) = TRIM(ag2.agen_id::text)
		WHERE COALESCE(NULLIF(TRIM(ec.bttt_aktifyn), ''), 'Y') = 'Y'
		  AND UPPER(TRIM(ec.bttt_kirimyn::text)) = 'Y'
		  AND NOT EXISTS (
			  SELECT 1 FROM public.mkt_t_eterimabtt tb 
			  WHERE TRIM(tb.tb_bttid::text) = TRIM(ec.bttt_id::text)
		  )
	`
	var args []interface{}

	if useTanggal && tglAwal != "" && tglAkhir != "" {
		query += " AND ec.bttt_tanggal BETWEEN ? AND ?"
		args = append(args, tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}
	if cabang != "" && !strings.Contains(strings.ToUpper(cabang), "SEMUA") {
		query += " AND (ag.agen_nama = ? OR ec.bttt_asalagenid::text = ?)"
		args = append(args, cabang, cabang)
	}
	if tujuan != "" {
		query += " AND ec.bttt_tujuankota = ?"
		args = append(args, tujuan)
	}
	if pengirim != "" {
		query += " AND ec.bttt_asalname ILIKE ?"
		args = append(args, "%"+pengirim+"%")
	}
	if noBtt != "" {
		query += " AND ec.bttt_id ILIKE ?"
		args = append(args, "%"+noBtt)
	}
	if pembayaran != "" && pembayaran != "0" {
		query += " AND TRIM(ec.bttt_pembayaran::text) = ?"
		args = append(args, pembayaran)
	}

	query += " ORDER BY ec.bttt_tanggal DESC, ec.bttt_id DESC LIMIT 400"

	var results []BttOutstandingItem
	database.Raw(query, args...).Scan(&results)
	if results == nil {
		results = []BttOutstandingItem{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}
