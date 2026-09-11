package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// Model Tipe 1 & 4: Detail Resi Penjualan
type PenjualanDetailItem struct {
	BTTTID       string  `gorm:"column:bttt_id" json:"bttt_id"`
	AsalCustID   string  `gorm:"column:bttt_asalcustid" json:"bttt_asalcustid"`
	CustName     string  `gorm:"column:cust_name" json:"cust_name"`
	TujuanNama   string  `gorm:"column:bttt_tujuannama" json:"bttt_tujuannama"`
	JnBayar      string  `gorm:"column:jnbayar" json:"jnbayar"`
	TujuanKota   string  `gorm:"column:bttt_tujuankota" json:"bttt_tujuankota"`
	BTTTTanggal  string  `gorm:"column:bttt_tanggal" json:"bttt_tanggal"`
	UpdateID     string  `gorm:"column:bttt_updateid" json:"bttt_updateid"`
	UpdateTime   string  `gorm:"column:bttt_updatetime" json:"bttt_updatetime"`
	Berat        float64 `gorm:"column:berat" json:"berat"`
	Volume       float64 `gorm:"column:volume" json:"volume"`
	Colly        int     `gorm:"column:colly" json:"colly"`
	BiayaKirim   float64 `gorm:"column:biaya_kirim" json:"biaya_kirim"`
	BiayaPenerus float64 `gorm:"column:biaya_penerus" json:"biaya_penerus"`
	Packing      float64 `gorm:"column:packing" json:"packing"`
	Total        float64 `gorm:"column:total" json:"total"`
}

// Model Tipe 2 & 3: Rekapitulasi Omset Cabang / Loket
type PenjualanOmsetItem struct {
	AgenID        string  `gorm:"column:agen_id" json:"agen_id"`
	AgenNama      string  `gorm:"column:agen_nama" json:"agen_nama"`
	HargaTunai    float64 `gorm:"column:harga_tunai" json:"harga_tunai"`
	PenerusTunai  float64 `gorm:"column:penerus_tunai" json:"penerus_tunai"`
	HargaKredit   float64 `gorm:"column:harga_kredit" json:"harga_kredit"`
	PenerusKredit float64 `gorm:"column:penerus_kredit" json:"penerus_kredit"`
	HargaTagih    float64 `gorm:"column:harga_tagih" json:"harga_tagih"`
	PenerusTagih  float64 `gorm:"column:penerus_tagih" json:"penerus_tagih"`
	Packing       float64 `gorm:"column:packing" json:"packing"`
	TotalOmset    float64 `gorm:"column:total_omset" json:"total_omset"`
	Berat         float64 `gorm:"column:berat" json:"berat"`
	JumlahBTT     int     `gorm:"column:jumlah_btt" json:"jumlah_btt"`
}

// 1. GET /api/laporan/penjualan/data (Tipe 1, 2, 3, 4)
func GetLaporanPenjualan(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	tipe := strings.TrimSpace(c.DefaultQuery("tipe", "1"))
	useTanggal := c.Query("use_tanggal") == "true"
	tglAwal := strings.TrimSpace(c.Query("start_date"))
	tglAkhir := strings.TrimSpace(c.Query("end_date"))
	cabang := strings.TrimSpace(c.Query("cabang"))
	penjualan := strings.TrimSpace(c.DefaultQuery("penjualan", "0"))
	tujuanKota := strings.TrimSpace(c.Query("tujuan_kota"))
	urut := strings.TrimSpace(c.DefaultQuery("urut", "1"))

	// ==========================================
	// JIKA TIPE 2 ATAU 3: REKAPITULASI OMSET
	// ==========================================
	if tipe == "2" || tipe == "3" {
		query := `
			SELECT 
				COALESCE(ag.agen_id::text, ec.bttt_asalagenid::text, '-') AS agen_id,
				COALESCE(ag.agen_nama, 'CABANG ' || ec.bttt_asalagenid::text) AS agen_nama,
				COALESCE(SUM(CASE WHEN TRIM(ec.bttt_pembayaran::text) = '1' THEN NULLIF(REGEXP_REPLACE(ec.bttt_harga::text, '[^0-9.]', '', 'g'), '')::numeric ELSE 0 END), 0) AS harga_tunai,
				COALESCE(SUM(CASE WHEN TRIM(ec.bttt_pembayaran::text) = '1' THEN NULLIF(REGEXP_REPLACE(ec.bttt_biayapenerus::text, '[^0-9.]', '', 'g'), '')::numeric ELSE 0 END), 0) AS penerus_tunai,
				COALESCE(SUM(CASE WHEN TRIM(ec.bttt_pembayaran::text) = '2' THEN NULLIF(REGEXP_REPLACE(ec.bttt_harga::text, '[^0-9.]', '', 'g'), '')::numeric ELSE 0 END), 0) AS harga_kredit,
				COALESCE(SUM(CASE WHEN TRIM(ec.bttt_pembayaran::text) = '2' THEN NULLIF(REGEXP_REPLACE(ec.bttt_biayapenerus::text, '[^0-9.]', '', 'g'), '')::numeric ELSE 0 END), 0) AS penerus_kredit,
				COALESCE(SUM(CASE WHEN TRIM(ec.bttt_pembayaran::text) NOT IN ('1', '2') THEN NULLIF(REGEXP_REPLACE(ec.bttt_harga::text, '[^0-9.]', '', 'g'), '')::numeric ELSE 0 END), 0) AS harga_tagih,
				COALESCE(SUM(CASE WHEN TRIM(ec.bttt_pembayaran::text) NOT IN ('1', '2') THEN NULLIF(REGEXP_REPLACE(ec.bttt_biayapenerus::text, '[^0-9.]', '', 'g'), '')::numeric ELSE 0 END), 0) AS penerus_tagih,
				COALESCE(SUM(NULLIF(REGEXP_REPLACE(pck.pck_biaya::text, '[^0-9.]', '', 'g'), '')::numeric), 0) AS packing,
				COALESCE(SUM(
					COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_harga::text, '[^0-9.]', '', 'g'), '')::numeric, 0) + 
					COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_biayapenerus::text, '[^0-9.]', '', 'g'), '')::numeric, 0) + 
					COALESCE(NULLIF(REGEXP_REPLACE(pck.pck_biaya::text, '[^0-9.]', '', 'g'), '')::numeric, 0)
				), 0) AS total_omset,
				COALESCE(SUM(NULLIF(REGEXP_REPLACE(ec.bttt_berat::text, '[^0-9.]', '', 'g'), '')::numeric), 0) AS berat,
				COUNT(ec.bttt_id) AS jumlah_btt
			FROM public.mkt_t_econote ec
			LEFT JOIN public.glb_m_agen ag ON TRIM(ec.bttt_asalagenid::text) = TRIM(ag.agen_id::text)
			LEFT JOIN public.pck_t_packing pck ON TRIM(ec.bttt_packingid::text) = TRIM(pck.pck_id::text)
			WHERE COALESCE(NULLIF(TRIM(ec.bttt_aktifyn), ''), 'Y') = 'Y'
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
		query += " GROUP BY ag.agen_id, ag.agen_nama, ec.bttt_asalagenid ORDER BY total_omset DESC LIMIT 300"

		var results []PenjualanOmsetItem
		database.Raw(query, args...).Scan(&results)
		if results == nil {
			results = []PenjualanOmsetItem{}
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": results, "tipe": tipe})
		return
	}

	// ==========================================
	// TIPE 1 (DETAIL TRANSAKSI) & TIPE 4 (BATAL)
	// ==========================================
	filterAktif := "COALESCE(NULLIF(TRIM(ec.bttt_aktifyn), ''), 'Y') = 'Y'"
	if tipe == "4" {
		filterAktif = "TRIM(ec.bttt_aktifyn) = 'N'"
	}

	query := `
		SELECT 
			ec.bttt_id,
			COALESCE(ec.bttt_asalcustid::text, '-') AS bttt_asalcustid,
			COALESCE(NULLIF(TRIM(cust.cust_name), ''), NULLIF(TRIM(ec.bttt_asalname), ''), '-') AS cust_name,
			COALESCE(NULLIF(TRIM(ec.bttt_tujuannama), ''), '-') AS bttt_tujuannama,
			CASE 
				WHEN TRIM(ec.bttt_pembayaran::text) = '1' THEN 'Tunai'
				WHEN TRIM(ec.bttt_pembayaran::text) = '2' THEN 'Kredit'
				WHEN TRIM(ec.bttt_pembayaran::text) = '3' THEN 'Tagih Naik'
				WHEN TRIM(ec.bttt_pembayaran::text) = '4' THEN 'Order Jemput'
				WHEN TRIM(ec.bttt_pembayaran::text) = '5' THEN 'Tagih Turun'
				WHEN UPPER(TRIM(ec.bttt_pembayaran::text)) LIKE '%TUNAI%' THEN 'Tunai'
				WHEN UPPER(TRIM(ec.bttt_pembayaran::text)) LIKE '%KREDIT%' THEN 'Kredit'
				ELSE 'Tunai'
			END AS jnbayar,
			COALESCE(NULLIF(TRIM(ec.bttt_tujuankota), ''), '-') AS bttt_tujuankota,
			COALESCE(TO_CHAR(ec.bttt_tanggal, 'YYYY-MM-DD'), '') AS bttt_tanggal,
			COALESCE(ec.bttt_updateid, '-') AS bttt_updateid,
			COALESCE(TO_CHAR(ec.bttt_updatetime, 'YYYY-MM-DD HH24:MI'), '-') AS bttt_updatetime,
			COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_berat::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS berat,
			COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_ukuran::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS volume,
			COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_jmlunit::text, '[^0-9]', '', 'g'), '')::integer, 1) AS colly,
			COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_harga::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS biaya_kirim,
			COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_biayapenerus::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS biaya_penerus,
			COALESCE(NULLIF(REGEXP_REPLACE(pck.pck_biaya::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS packing,
			COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_harga::text, '[^0-9.]', '', 'g'), '')::numeric, 0) + 
			COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_biayapenerus::text, '[^0-9.]', '', 'g'), '')::numeric, 0) + 
			COALESCE(NULLIF(REGEXP_REPLACE(pck.pck_biaya::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS total
		FROM public.mkt_t_econote ec
		LEFT JOIN public.mkt_m_customer cust ON TRIM(ec.bttt_asalcustid::text) = TRIM(cust.cust_id::text)
		LEFT JOIN public.glb_m_agen ag ON TRIM(ec.bttt_asalagenid::text) = TRIM(ag.agen_id::text)
		LEFT JOIN public.pck_t_packing pck ON TRIM(ec.bttt_packingid::text) = TRIM(pck.pck_id::text)
		WHERE ` + filterAktif

	var args []interface{}

	if useTanggal && tglAwal != "" && tglAkhir != "" {
		query += " AND ec.bttt_tanggal BETWEEN ? AND ?"
		args = append(args, tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}
	if cabang != "" && !strings.Contains(strings.ToUpper(cabang), "SEMUA") {
		query += " AND (ag.agen_nama = ? OR ec.bttt_asalagenid::text = ?)"
		args = append(args, cabang, cabang)
	}
	if penjualan != "" && penjualan != "0" {
		query += " AND TRIM(ec.bttt_pembayaran::text) = ?"
		args = append(args, penjualan)
	}
	if tujuanKota != "" && tujuanKota != "Semua Kota Tujuan" {
		query += " AND ec.bttt_tujuankota = ?"
		args = append(args, tujuanKota)
	}

	if urut == "2" {
		query += " ORDER BY biaya_kirim DESC LIMIT 300"
	} else {
		query += " ORDER BY ec.bttt_tanggal DESC, ec.bttt_id DESC LIMIT 300"
	}

	var results []PenjualanDetailItem
	database.Raw(query, args...).Scan(&results)
	if results == nil {
		results = []PenjualanDetailItem{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results, "tipe": tipe})
}

// 2. GET /api/laporan/penjualan/combo-kota (Dropdown Kota Tujuan)
func GetComboKotaPenjualan(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	var kotas []string
	query := `
		SELECT DISTINCT TRIM(bttt_tujuankota) AS kota
		FROM public.mkt_t_econote 
		WHERE bttt_tujuankota IS NOT NULL 
		  AND TRIM(bttt_tujuankota) <> ''
		  AND TRIM(bttt_tujuankota) <> '-'
		ORDER BY kota ASC 
		LIMIT 200
	`
	database.Raw(query).Scan(&kotas)
	if kotas == nil {
		kotas = []string{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": kotas})
}
