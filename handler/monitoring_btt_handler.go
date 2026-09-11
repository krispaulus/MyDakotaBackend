package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

type MonitoringBttItem struct {
	BTTTID         string  `gorm:"column:bttt_id" json:"bttt_id"`
	TanggalBTT     string  `gorm:"column:bttt_tanggal" json:"bttt_tanggal"`
	Pengirim       string  `gorm:"column:bttt_asalname" json:"bttt_asalname"`
	CustomerName   string  `gorm:"column:cust_name" json:"cust_name"`
	NoSuratJalan   string  `gorm:"column:bttt_nosuratjalan" json:"bttt_nosuratjalan"`
	NamaBarang     string  `gorm:"column:bttt_namabarang" json:"bttt_namabarang"`
	Colly          int     `gorm:"column:bttt_jmlunit" json:"bttt_jmlunit"`
	Berat          float64 `gorm:"column:bttt_berat" json:"bttt_berat"`
	Ukuran         float64 `gorm:"column:bttt_ukuran" json:"bttt_ukuran"`
	TujuanKota     string  `gorm:"column:bttt_tujuankota" json:"bttt_tujuankota"`
	Penerima       string  `gorm:"column:bttt_tujuannama" json:"bttt_tujuannama"`
	TagihTujuanCOD float64 `gorm:"column:bttt_tagihtujuan" json:"bttt_tagihtujuan"`
	TglHistory     string  `gorm:"column:hist_tanggal" json:"hist_tanggal"`
	Keterangan     string  `gorm:"column:keterangan" json:"keterangan"`
	PosisiBarang   string  `gorm:"column:posisi_barang" json:"posisi_barang"`
	StatusTracking string  `gorm:"column:status_tracking" json:"status_tracking"`
	StatusKategori string  `gorm:"column:status_kategori" json:"status_kategori"`
}

type MonitoringBttSummary struct {
	TotalBarang  int     `json:"total_barang"`
	Diterima     int     `json:"diterima"`
	Proses       int     `json:"proses"`
	Gagal        int     `json:"gagal"`
	BlmBerangkat int     `json:"blm_berangkat"`
	TotalCOD     float64 `json:"total_cod"`
}

// 1. GET /api/marketing/monitoring-btt/data
func GetMonitoringBtt(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	useTanggal := c.Query("use_tanggal") == "true"
	tglAwal := strings.TrimSpace(c.Query("start_date"))
	tglAkhir := strings.TrimSpace(c.Query("end_date"))
	customer := strings.TrimSpace(c.Query("customer"))
	tujuanKota := strings.TrimSpace(c.Query("tujuan"))
	noBtt := strings.TrimSpace(c.Query("nobtt"))
	noSJ := strings.TrimSpace(c.Query("nosj"))
	statusFilter := strings.TrimSpace(c.Query("status"))

	query := `
		WITH latest_history AS (
			SELECT DISTINCT ON (TRIM(h.hist_bttid::text))
				TRIM(h.hist_bttid::text) AS btt_id,
				COALESCE(TO_CHAR(h.hist_tanggal, 'YYYY-MM-DD HH24:MI'), '') AS hist_tanggal,
				COALESCE(NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer, 0) AS hist_staturut,
				CASE 
					WHEN h.hist_ket IS NOT NULL AND TRIM(h.hist_ket::text) <> '' AND TRIM(h.hist_ket::text) <> '0' 
						THEN TRIM(h.hist_ket::text)
					WHEN NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer = 0 THEN 'BTT Dibuat / Entry Loket'
					WHEN NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer = 1 THEN 'Manifest Keluar'
					WHEN NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer = 2 THEN 'Muat Armada'
					WHEN NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer = 3 THEN 'Berangkat Hub'
					WHEN NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer = 4 THEN 'Sampai Hub Transit'
					WHEN NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer = 5 THEN 'Bongkar Hub'
					WHEN NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer = 6 THEN 'Diterima Cabang Tujuan'
					WHEN NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer = 7 THEN 'Proses Antar Kurir'
					WHEN NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer = 8 THEN 'Gagal Diantar'
					WHEN NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer BETWEEN 10 AND 13 THEN 'Barang Diterima'
					WHEN NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer = 14 THEN 'Proses Pengantaran Ulang'
					ELSE 'Dalam Perjalanan'
				END AS keterangan,
				COALESCE(ag.agen_nama, ag.agen_kota, '-') AS posisi_barang
			FROM public.mkt_t_ehistory h
			LEFT JOIN public.glb_m_agen ag ON TRIM(h.hist_agenid::text) = TRIM(ag.agen_id::text)
			WHERE COALESCE(NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer, 0) < 20
			ORDER BY TRIM(h.hist_bttid::text), h.hist_tanggal DESC, COALESCE(NULLIF(REGEXP_REPLACE(h.hist_staturut::text, '[^0-9]', '', 'g'), '')::integer, 0) DESC
		)
		SELECT 
			ec.bttt_id,
			COALESCE(TO_CHAR(ec.bttt_tanggal, 'YYYY-MM-DD'), '') AS bttt_tanggal,
			COALESCE(NULLIF(TRIM(ec.bttt_asalname::text), ''), '-') AS bttt_asalname,
			COALESCE(NULLIF(TRIM(cust.cust_name::text), ''), '-') AS cust_name,
			COALESCE(NULLIF(TRIM(ec.bttt_nosuratjalan::text), ''), '-') AS bttt_nosuratjalan,
			COALESCE(NULLIF(TRIM(ec.bttt_namabarang::text), ''), '-') AS bttt_namabarang,
			COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_jmlunit::text, '[^0-9]', '', 'g'), '')::integer, 1) AS bttt_jmlunit,
			COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_berat::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS bttt_berat,
			COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_ukuran::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS bttt_ukuran,
			COALESCE(NULLIF(TRIM(ec.bttt_tujuankota::text), ''), '-') AS bttt_tujuankota,
			COALESCE(NULLIF(TRIM(ec.bttt_tujuannama::text), ''), '-') AS bttt_tujuannama,
			COALESCE(NULLIF(REGEXP_REPLACE(ec.bttt_tagihtujuan::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS bttt_tagihtujuan,
			COALESCE(lh.hist_tanggal, '') AS hist_tanggal,
			COALESCE(lh.keterangan, '-') AS keterangan,
			COALESCE(lh.posisi_barang, '-') AS posisi_barang,
			CASE 
				WHEN lh.hist_staturut >= 10 AND lh.hist_staturut <= 13 THEN 'Diterima'
				WHEN lh.hist_staturut = 8 THEN 'Gagal Diantar'
				WHEN lh.hist_staturut = 0 OR lh.hist_staturut IS NULL THEN 'Belum Diberangkatkan'
				ELSE 'Dalam Proses'
			END AS status_tracking,
			CASE 
				WHEN lh.hist_staturut >= 10 AND lh.hist_staturut <= 13 THEN 'diterima'
				WHEN lh.hist_staturut = 8 THEN 'gagal'
				WHEN lh.hist_staturut = 0 OR lh.hist_staturut IS NULL THEN 'blm_berangkat'
				ELSE 'proses'
			END AS status_kategori
		FROM public.mkt_t_econote ec
		LEFT JOIN latest_history lh ON TRIM(ec.bttt_id::text) = lh.btt_id
		LEFT JOIN public.mkt_m_customer cust ON TRIM(ec.bttt_asalcustid::text) = TRIM(cust.cust_id::text)
		WHERE COALESCE(NULLIF(TRIM(ec.bttt_aktifyn::text), ''), 'Y') = 'Y'
	`

	var args []interface{}

	if useTanggal && tglAwal != "" && tglAkhir != "" {
		query += " AND ec.bttt_tanggal BETWEEN ? AND ?"
		args = append(args, tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}
	if customer != "" && !strings.Contains(strings.ToUpper(customer), "SEMUA") {
		query += " AND (cust.cust_name ILIKE ? OR ec.bttt_asalname ILIKE ?)"
		args = append(args, "%"+customer+"%", "%"+customer+"%")
	}
	if tujuanKota != "" && tujuanKota != "Semua Kota Tujuan" {
		query += " AND ec.bttt_tujuankota = ?"
		args = append(args, tujuanKota)
	}
	if noBtt != "" {
		query += " AND ec.bttt_id ILIKE ?"
		args = append(args, "%"+noBtt)
	}
	if noSJ != "" {
		query += " AND ec.bttt_nosuratjalan ILIKE ?"
		args = append(args, "%"+noSJ+"%")
	}

	if statusFilter != "" {
		switch statusFilter {
		case "Diterima":
			query += " AND (lh.hist_staturut >= 10 AND lh.hist_staturut <= 13)"
		case "Gagal Diantar":
			query += " AND lh.hist_staturut = 8"
		case "Belum Diberangkatkan":
			query += " AND (lh.hist_staturut = 0 OR lh.hist_staturut IS NULL)"
		case "Dalam Proses":
			query += " AND (lh.hist_staturut NOT IN (0, 8, 10, 11, 12, 13) AND lh.hist_staturut IS NOT NULL)"
		}
	}

	query += " ORDER BY ec.bttt_tanggal DESC, ec.bttt_id DESC LIMIT 500"

	var results []MonitoringBttItem
	database.Raw(query, args...).Scan(&results)
	if results == nil {
		results = []MonitoringBttItem{}
	}

	summary := MonitoringBttSummary{TotalBarang: len(results)}
	for _, item := range results {
		summary.TotalCOD += item.TagihTujuanCOD
		switch item.StatusKategori {
		case "diterima":
			summary.Diterima++
		case "gagal":
			summary.Gagal++
		case "blm_berangkat":
			summary.BlmBerangkat++
		case "proses":
			summary.Proses++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"data":    results,
		"summary": summary,
	})
}

// 2. GET /api/marketing/monitoring-btt/combo-customer
func GetComboCustomerMonitoring(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	var customers []string
	query := `
		SELECT DISTINCT TRIM(cust_name) 
		FROM public.mkt_m_customer 
		WHERE cust_aktifyn = 'Y' AND cust_name IS NOT NULL AND TRIM(cust_name) <> '' 
		ORDER BY TRIM(cust_name) ASC 
		LIMIT 250
	`
	database.Raw(query).Scan(&customers)
	if customers == nil {
		customers = []string{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": customers})
}
