package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

type BTTCounterItem struct {
	BTTTID           string  `json:"bttt_id"`
	BTTTTanggal      string  `json:"bttt_tanggal"`
	ServisJD         string  `json:"servisjd"`
	AgenNama         string  `json:"agen_nama"`
	CabangInduk      string  `json:"cabang_induk"`
	CustName         string  `json:"cust_name"`
	BTTTAsalName     string  `json:"bttt_asalname"`
	BTTTTujuanNama   string  `json:"bttt_tujuannama"`
	JnBayar          string  `json:"jnbayar"`
	BTTTNamaBarang   string  `json:"bttt_namabarang"`
	BTTTNoSuratJalan string  `json:"bttt_nosuratjalan"`
	BTTTJmlUnit      int     `json:"bttt_jmlunit"`
	BTTTBerat        float64 `json:"bttt_berat"`
	BTTTUkuran       float64 `json:"bttt_ukuran"`
	BTTTTagihTujuan  float64 `json:"bttt_tagihtujuan"`
	AktifJD          string  `json:"aktifjd"`
}

// 1. GET /api/laporan/btt-counter/combo-cabang
func GetCabangComboCounter(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	var cabangList []string
	database.Table("public.glb_m_agen").
		Where("agen_aktifyn = 'Y'").
		Order("agen_nama ASC").
		Pluck("agen_nama", &cabangList)

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": cabangList})
}

// 2. GET /api/laporan/btt-counter/combo-kota
func GetKotaComboCounter(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	var kotaList []string
	database.Table("public.glb_m_ekodepos").
		Select("DISTINCT kotakabupaten").
		Where("kotakabupaten IS NOT NULL AND TRIM(kotakabupaten) != ''").
		Order("kotakabupaten ASC").
		Pluck("kotakabupaten", &kotaList)

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": kotaList})
}

// 3. GET /api/laporan/btt-counter/data
func GetLaporanBTTCounter(c *gin.Context) {
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
	service := strings.TrimSpace(c.Query("service"))
	chkService := c.Query("chkservice") == "true"
	customer := strings.TrimSpace(c.Query("customer"))
	chkCustomer := c.Query("chkcustomer") == "true"
	tujuan := strings.TrimSpace(c.Query("tujuan"))
	chkTujuan := c.Query("chktujuan") == "true"
	posting := strings.TrimSpace(c.Query("posting"))
	chkPosting := c.Query("chkposting") == "true"
	kirim := strings.TrimSpace(c.Query("kirim"))
	chkKirim := c.Query("chkkirim") == "true"
	nobtt := strings.TrimSpace(c.Query("nobtt"))
	chkNoBTT := c.Query("chknobtt") == "true"
	nosmu := strings.TrimSpace(c.Query("nosmu"))
	chkNoSMU := c.Query("chknosmu") == "true"
	agen := strings.TrimSpace(c.Query("agen"))
	chkAgen := c.Query("chkagen") == "true"
	bayar := strings.TrimSpace(c.Query("bayar"))
	chkBayar := c.Query("chkbayar") == "true"
	pembayaran := strings.TrimSpace(c.Query("pembayaran"))
	chkPembayaran := c.Query("chkpembayaran") == "true"
	nosj := strings.TrimSpace(c.Query("nosj"))
	chkNoSJ := c.Query("chknosj") == "true"

	query := `
		SELECT 
			c.bttt_id,
			COALESCE(TO_CHAR(c.bttt_tanggal, 'YYYY-MM-DD'), '') AS bttt_tanggal,
			CASE 
				WHEN TRIM(c.bttt_servid::text) = '1' THEN 'Darat'
				WHEN TRIM(c.bttt_servid::text) = '2' THEN 'Laut'
				ELSE 'Udara'
			END AS servisjd,
			COALESCE(NULLIF(ag.agen_nama, ''), NULLIF(c.bttt_asalkota, ''), 'CABANG PUSAT') AS agen_nama,
			COALESCE(ag_parent.agen_nama, '') AS cabang_induk,
			COALESCE(cust.cust_name, '') AS cust_name,
			COALESCE(NULLIF(c.bttt_asalname, ''), cust.cust_name, '-') AS bttt_asalname,
			COALESCE(NULLIF(c.bttt_tujuannama, ''), NULLIF(c.bttt_tujuankota, ''), '-') AS bttt_tujuannama,
			CASE 
				WHEN TRIM(c.bttt_pembayaran::text) = '1' THEN 'Tunai'
				WHEN TRIM(c.bttt_pembayaran::text) = '2' THEN 'Kredit'
				ELSE 'Tagih'
			END AS jnbayar,
			COALESCE(NULLIF(c.bttt_namabarang, ''), 'PAKET / GENERAL CARGO') AS bttt_namabarang,
			COALESCE(NULLIF(c.bttt_nosuratjalan, ''), '-') AS bttt_nosuratjalan,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_jmlunit::text, '[^0-9]', '', 'g'), '')::integer, 1) AS bttt_jmlunit,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_berat::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS bttt_berat,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_ukuran::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS bttt_ukuran,
			COALESCE(NULLIF(REGEXP_REPLACE(c.bttt_tagihtujuan::text, '[^0-9.]', '', 'g'), '')::numeric, 0) AS bttt_tagihtujuan,
			CASE 
				WHEN UPPER(TRIM(c.bttt_aktifyn::text)) = 'Y' THEN 'Ya'
				ELSE 'Tidak'
			END AS aktifjd
		FROM public.mkt_t_econote c
		LEFT JOIN public.glb_m_agen ag ON TRIM(c.bttt_asalagenid::text) = TRIM(ag.agen_id::text)
		LEFT JOIN public.glb_m_agen ag_parent ON TRIM(ag.agen_cabangid::text) = TRIM(ag_parent.agen_id::text)
		LEFT JOIN public.mkt_m_customer cust ON TRIM(c.bttt_asalcustid::text) = TRIM(cust.cust_id::text)
		WHERE c.bttt_id IS NOT NULL
	`

	var args []interface{}

	if chkTanggal && tglAwal != "" && tglAkhir != "" {
		query += " AND c.bttt_tanggal BETWEEN ? AND ?"
		args = append(args, tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}

	if chkCabang && cabang != "" {
		query += " AND ag.agen_nama = ?"
		args = append(args, cabang)
	}

	if chkService && service != "" {
		query += " AND c.bttt_servid = ?"
		args = append(args, service)
	}

	if chkCustomer && customer != "" {
		query += " AND (cust.cust_name ILIKE ? OR c.bttt_asalname ILIKE ?)"
		args = append(args, "%"+customer+"%", "%"+customer+"%")
	}

	if chkTujuan && tujuan != "" {
		query += " AND c.bttt_tujuankota ILIKE ?"
		args = append(args, "%"+tujuan+"%")
	}

	if chkPosting && posting != "" {
		query += " AND c.bttt_postingyn = ?"
		args = append(args, posting)
	}

	if chkKirim && kirim != "" {
		query += " AND c.bttt_kirimyn = ?"
		args = append(args, kirim)
	}

	if chkNoBTT && nobtt != "" {
		query += " AND c.bttt_id ILIKE ?"
		args = append(args, "%"+nobtt+"%")
	}

	if chkNoSMU && nosmu != "" {
		query += " AND c.bttt_smuno ILIKE ?"
		args = append(args, "%"+nosmu+"%")
	}

	if chkAgen && agen != "" {
		query += " AND c.bttt_agenyn = ?"
		args = append(args, agen)
	}

	if chkBayar && bayar != "" {
		query += " AND c.bttt_bayaryn = ?"
		args = append(args, bayar)
	}

	if chkPembayaran && pembayaran != "" {
		query += " AND c.bttt_pembayaran = ?"
		args = append(args, pembayaran)
	}

	if chkNoSJ && nosj != "" {
		query += " AND c.bttt_nosuratjalan = ?"
		args = append(args, nosj)
	}

	query += " ORDER BY c.bttt_asalagenid ASC, c.bttt_tanggal DESC, c.bttt_id DESC LIMIT 500"

	var results []BTTCounterItem
	database.Raw(query, args...).Scan(&results)

	if results == nil {
		results = []BTTCounterItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}
