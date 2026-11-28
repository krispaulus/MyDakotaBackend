package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type KondisiBTTItem struct {
	TipeDokumen     string     `json:"tipe_dokumen" gorm:"column:tipe_dokumen"`
	NoDokumen       string     `json:"no_dokumen" gorm:"column:no_dokumen"`
	TglDokumen      time.Time  `json:"tgl_dokumen" gorm:"column:tgl_dokumen"`
	CustID          string     `json:"cust_id" gorm:"column:cust_id"`
	CustName        string     `json:"cust_name" gorm:"column:cust_name"`
	Layanan         string     `json:"layanan" gorm:"column:layanan"`
	MetodeBayar     string     `json:"metode_bayar" gorm:"column:metode_bayar"`
	PackingID       string     `json:"packing_id" gorm:"column:packing_id"`
	ByPacking       float64    `json:"by_packing" gorm:"column:by_packing"`
	NoKwitansi      string     `json:"no_kwitansi" gorm:"column:no_kwitansi"`
	TglKwitansi     *time.Time `json:"tgl_kwitansi" gorm:"column:tgl_kwitansi"`
	NoTT            string     `json:"no_tt" gorm:"column:no_tt"`
	TglTT           *time.Time `json:"tgl_tt" gorm:"column:tgl_tt"`
	NoPembayaran    string     `json:"no_pembayaran" gorm:"column:no_pembayaran"`
	TglBayar        *time.Time `json:"tgl_bayar" gorm:"column:tgl_bayar"`
	ByKirim         float64    `json:"by_kirim" gorm:"column:by_kirim"`
	UangMuka        float64    `json:"uang_muka" gorm:"column:uang_muka"`
	TerbayarNominal float64    `json:"terbayar_nominal" gorm:"column:terbayar_nominal"`
	Dikembalikan    float64    `json:"dikembalikan" gorm:"column:dikembalikan"`
	TotalBiaya      float64    `json:"total_biaya" gorm:"column:total_biaya"`
	Koli            float64    `json:"koli" gorm:"column:koli"`
	Berat           float64    `json:"berat" gorm:"column:berat"`
	KotaTujuan      string     `json:"kota_tujuan" gorm:"column:kota_tujuan"`
	Penerima        string     `json:"penerima" gorm:"column:penerima"`
	TglBTTKembali   *time.Time `json:"tgl_btt_kembali" gorm:"column:tgl_btt_kembali"`
	NoSPB           string     `json:"no_spb" gorm:"column:no_spb"`
	UmurBTT         int        `json:"umur_btt" gorm:"column:umur_btt"`
	BTTDBS          string     `json:"btt_dbs" gorm:"column:btt_dbs"`
	PIC             string     `json:"pic" gorm:"column:pic"`
	CabangBerwenang string     `json:"cabang_berwenang" gorm:"column:cabang_berwenang"`
	BTTKembaliYN    string     `json:"btt_kembali_yn" gorm:"column:btt_kembali_yn"`
	SudahInvoiceYN  string     `json:"sudah_invoice_yn" gorm:"column:sudah_invoice_yn"`
	NoInvoice       string     `json:"no_invoice" gorm:"column:no_invoice"`
	TerbayarYN      string     `json:"terbayar_yn" gorm:"column:terbayar_yn"`
	AgenAsalID      string     `json:"agen_asal_id" gorm:"column:agen_asal_id"`
	AgenAsalNama    string     `json:"agen_asal_nama" gorm:"column:agen_asal_nama"`
}

func tableExists(db *gorm.DB, tableName string) bool {
	var reg *string
	_ = db.Raw("SELECT to_regclass(?)", tableName).Scan(&reg).Error
	return reg != nil && *reg != ""
}

// 1. GET /api/piutang/kondisi-btt/options
func GetKondisiBTTOptionsHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var picList []string
	_ = database.Table("public.mkt_m_customer").
		Where("cust_pic IS NOT NULL AND TRIM(cust_pic) <> ''").
		Distinct("cust_pic").
		Order("cust_pic ASC").
		Pluck("cust_pic", &picList)

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"pic_list": picList,
	})
}

// 2. GET /api/piutang/kondisi-btt (Fast Hash Join Version)
func GetKondisiBTTListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	cabangAsal := strings.TrimSpace(c.Query("cabang_asal"))
	cabangBerwenang := strings.TrimSpace(c.Query("cabang_berwenang"))
	pic := strings.TrimSpace(c.Query("pic"))
	customer := strings.TrimSpace(c.Query("customer"))
	bttKembali := strings.TrimSpace(c.Query("btt_kembali"))
	sudahInvoice := strings.TrimSpace(c.Query("sudah_invoice"))
	bayar := strings.TrimSpace(c.Query("bayar"))
	pembayaran := strings.TrimSpace(c.Query("pembayaran"))
	tglPelunasan := strings.TrimSpace(c.Query("tgl_pelunasan"))
	jenisNaikTurun := strings.ToUpper(strings.TrimSpace(c.Query("jenis")))
	showBTT := c.Query("show_btt") != "false" && c.Query("show_btt") != "0"
	showOJ := c.Query("show_oj") != "false" && c.Query("show_oj") != "0"
	bypassTanggal := c.Query("bypass_tanggal") == "true" || c.Query("bypass_tanggal") == "1"

	hasLSPB := tableExists(database, "public.opr_t_transportlspb")
	hasESPTurun := tableExists(database, "public.opr_t_esp_turun")
	hasOrderJemput := tableExists(database, "public.mkt_t_orderjemput")

	var allResults []KondisiBTTItem

	if showBTT {
		lspbJoin := `LEFT JOIN (SELECT ''::varchar AS btt_key, 'N'::varchar AS is_kembali, NULL::timestamp AS tgl_kembali, ''::varchar AS no_spb WHERE 1=0) lspb ON lspb.btt_key = TRIM(LOWER(e.bttt_id::varchar))`
		if hasLSPB {
			lspbJoin = `LEFT JOIN (
				SELECT
					TRIM(LOWER(nomorbtt::varchar)) AS btt_key,
					MAX(CASE WHEN COALESCE(grn, 'N') = 'Y' OR tanggal_diterima IS NOT NULL THEN 'Y' ELSE 'N' END) AS is_kembali,
					MAX(tanggal_diterima)::timestamp AS tgl_kembali,
					MAX(COALESCE(NULLIF(TRIM(dn_od_sj), ''), '')) AS no_spb
				FROM public.opr_t_transportlspb
				WHERE nomorbtt IS NOT NULL AND TRIM(nomorbtt::varchar) <> ''
				GROUP BY TRIM(LOWER(nomorbtt::varchar))
			) lspb ON lspb.btt_key = TRIM(LOWER(e.bttt_id::varchar))`
		}

		spbJoin := `LEFT JOIN (SELECT ''::varchar AS btt_key, ''::varchar AS sp_id WHERE 1=0) spb ON spb.btt_key = TRIM(LOWER(e.bttt_id::varchar))`
		if hasESPTurun {
			spbJoin = `LEFT JOIN (
				SELECT
					TRIM(LOWER(COALESCE(to_jsonb(et)->>'sp_bttid', to_jsonb(et)->>'bttt_id', ''))) AS btt_key,
					MAX(COALESCE(
						NULLIF(TRIM(COALESCE(to_jsonb(et)->>'sp_id', '')), ''),
						NULLIF(TRIM(COALESCE(to_jsonb(et)->>'sp_no', '')), ''),
						NULLIF(TRIM(COALESCE(to_jsonb(et)->>'sp_turunid', '')), ''),
						NULLIF(TRIM(COALESCE(to_jsonb(et)->>'sp_spbid', '')), ''),
						''
					)) AS sp_id
				FROM public.opr_t_esp_turun et
				GROUP BY TRIM(LOWER(COALESCE(to_jsonb(et)->>'sp_bttid', to_jsonb(et)->>'bttt_id', '')))
			) spb ON spb.btt_key = TRIM(LOWER(e.bttt_id::varchar))`
		}

		query := database.Table("public.mkt_t_econote e").
			Select(`
				CASE WHEN TRIM(COALESCE(e.bttt_pembayaran::varchar, '')) = '3' THEN 'BTT TURUN' ELSE 'BTT NAIK' END AS tipe_dokumen,
				TRIM(e.bttt_id::varchar) AS no_dokumen,
				e.bttt_tanggal AS tgl_dokumen,
				COALESCE(TRIM(e.bttt_asalcustid::varchar), '') AS cust_id,
				COALESCE(c.cust_name, e.bttt_asalname, '') AS cust_name,
				CASE
					WHEN UPPER(TRIM(COALESCE(s.serv_name, ''))) <> '' THEN UPPER(TRIM(s.serv_name))
					WHEN TRIM(COALESCE(e.bttt_servid::varchar, '')) IN ('1', '01', 'D') THEN 'DARAT'
					WHEN TRIM(COALESCE(e.bttt_servid::varchar, '')) IN ('2', '02', 'L') THEN 'LAUT'
					WHEN TRIM(COALESCE(e.bttt_servid::varchar, '')) IN ('3', '03', 'U') THEN 'UDARA'
					ELSE 'DARAT'
				END AS layanan,
				CASE 
					WHEN TRIM(COALESCE(e.bttt_pembayaran::varchar, '')) = '1' THEN 'TUNAI'
					WHEN TRIM(COALESCE(e.bttt_pembayaran::varchar, '')) = '2' THEN 'KREDIT'
					ELSE 'TAGIH TURUN'
				END AS metode_bayar,
				COALESCE(NULLIF(TRIM(COALESCE(e.bttt_packingid::varchar, '')), ''), COALESCE(p2.pck_id, '')) AS packing_id,
				COALESCE(
					NULLIF(TRIM(COALESCE(to_jsonb(e)->>'bttt_biayapacking', '')), '')::numeric,
					p.pck_biaya::numeric,
					p2.pck_biaya::numeric,
					0
				) AS by_packing,
				COALESCE(inv.artih_nokw, '') AS no_kwitansi,
				inv.artih_tanggal AS tgl_kwitansi,
				'' AS no_tt,
				NULL::timestamp AS tgl_tt,
				COALESCE(rc.no_bayar, '') AS no_pembayaran,
				rc.tgl_bayar AS tgl_bayar,
				(COALESCE(e.bttt_harga::numeric, 0) + COALESCE(e.bttt_biayapenerus::numeric, 0)) AS by_kirim,
				0::numeric AS uang_muka,
				COALESCE(rc.total_bayar, 0) AS terbayar_nominal,
				0::numeric AS dikembalikan,
				(
					COALESCE(e.bttt_harga::numeric, 0) +
					COALESCE(e.bttt_biayapenerus::numeric, 0) +
					COALESCE(NULLIF(TRIM(COALESCE(to_jsonb(e)->>'bttt_biayapacking', '')), '')::numeric, p.pck_biaya::numeric, p2.pck_biaya::numeric, 0)
				) AS total_biaya,
				COALESCE(e.bttt_jmlunit::numeric, 0) AS koli,
				GREATEST(COALESCE(e.bttt_berat::numeric, 0), COALESCE(e.bttt_beratvol::numeric, 0)) AS berat,
				COALESCE(e.bttt_tujuankota, '') AS kota_tujuan,
				COALESCE(e.bttt_tujuannama, '') AS penerima,
				lspb.tgl_kembali AS tgl_btt_kembali,
				COALESCE(NULLIF(spb.sp_id, ''), lspb.no_spb, '') AS no_spb,
				CASE
					WHEN lspb.is_kembali = 'Y' THEN NULL
					ELSE GREATEST(EXTRACT(DAY FROM (NOW() - e.bttt_tanggal))::int, 0)
				END AS umur_btt,
				COALESCE(e.bttt_nosuratjalan, '') AS btt_dbs,
				COALESCE(c.cust_pic, '') AS pic,
				COALESCE(
					NULLIF(TRIM(ag_tujuan.agen_nama), ''),
					NULLIF(TRIM(COALESCE(to_jsonb(e)->>'bttt_tujuanagennama', '')), ''),
					NULLIF(TRIM(e.bttt_tujuankota), ''),
					'DLI PUSAT'
				) AS cabang_berwenang,
				COALESCE(lspb.is_kembali, 'N') AS btt_kembali_yn,
				CASE WHEN inv.artih_id IS NOT NULL THEN 'Y' ELSE 'N' END AS sudah_invoice_yn,
				COALESCE(inv.artih_id, '-') AS no_invoice,
				CASE
					WHEN COALESCE(rc.total_bayar, 0) > 0 OR COALESCE(e.bttt_bayaryn, 'N') = 'Y'
					THEN 'Y' ELSE 'N'
				END AS terbayar_yn,
				LPAD(LEFT(TRIM(e.bttt_id::varchar), 3), 3, '0') AS agen_asal_id,
				COALESCE(a.agen_nama, 'DLI PUSAT') AS agen_asal_nama
			`).
			Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(LOWER(c.cust_id::varchar)) = TRIM(LOWER(e.bttt_asalcustid::varchar))").
			Joins("LEFT JOIN public.glb_m_agen a ON LPAD(TRIM(a.agen_id::varchar), 3, '0') = LPAD(LEFT(TRIM(e.bttt_id::varchar), 3), 3, '0')").
			Joins("LEFT JOIN public.glb_m_agen ag_tujuan ON LPAD(TRIM(ag_tujuan.agen_id::varchar), 3, '0') = LPAD(TRIM(COALESCE(e.bttt_tujuanagenid::varchar, '')), 3, '0')").
			Joins("LEFT JOIN public.mkt_m_service s ON TRIM(s.serv_id::varchar) = TRIM(e.bttt_servid::varchar)").
			Joins("LEFT JOIN public.pck_t_packing p ON TRIM(LOWER(p.pck_id::varchar)) = TRIM(LOWER(COALESCE(e.bttt_packingid::varchar, '')))").
			Joins(`LEFT JOIN (
				SELECT
					TRIM(LOWER(COALESCE(to_jsonb(pk)->>'pck_bttid', ''))) AS btt_key,
					MAX(TRIM(pk.pck_id::varchar)) AS pck_id,
					MAX(COALESCE(pk.pck_biaya::numeric, 0)) AS pck_biaya
				FROM public.pck_t_packing pk
				WHERE COALESCE(to_jsonb(pk)->>'pck_bttid', '') <> ''
				GROUP BY TRIM(LOWER(COALESCE(to_jsonb(pk)->>'pck_bttid', '')))
			) p2 ON p2.btt_key = TRIM(LOWER(e.bttt_id::varchar))`).
			Joins(`LEFT JOIN (
				SELECT
					TRIM(LOWER(d.artid_bttid::varchar)) AS btt_key,
					MAX(h.artih_id) AS artih_id,
					MAX(
						CASE
							WHEN TRIM(COALESCE(h.artih_nokw, '')) <> '' THEN REPLACE(TRIM(h.artih_nokw), '/OTA/', '/001/')
							ELSE CONCAT(RIGHT(TRIM(h.artih_id), 4), '/DLI/', LPAD(LEFT(TRIM(h.artih_id), 3), 3, '0'), '/', TO_CHAR(h.artih_tanggal, 'MM/YY'))
						END
					) AS artih_nokw,
					MAX(h.artih_tanggal) AS artih_tanggal
				FROM public.art_t_invoiced d
				INNER JOIN public.art_t_invoiceh h ON TRIM(LOWER(h.artih_id)) = TRIM(LOWER(d.artid_artihid))
				WHERE COALESCE(h.artih_delete, 'N') <> 'Y'
				GROUP BY TRIM(LOWER(d.artid_bttid::varchar))
			) inv ON inv.btt_key = TRIM(LOWER(e.bttt_id::varchar))`).
			Joins(`LEFT JOIN (
				SELECT
					TRIM(LOWER(rd.trectddd_nobtt::varchar)) AS btt_key,
					MAX(rh.trecth_no) AS no_bayar,
					MAX(rh.trecth_tanggal) AS tgl_bayar,
					SUM(COALESCE(rd.trectddd_bayar, 0)) AS total_bayar
				FROM public.art_t_receiptdddd rd
				INNER JOIN public.art_t_receipth rh ON TRIM(LOWER(rh.trecth_no)) = TRIM(LOWER(rd.trectddd_trecthno))
				GROUP BY TRIM(LOWER(rd.trectddd_nobtt::varchar))
			) rc ON rc.btt_key = TRIM(LOWER(e.bttt_id::varchar))`).
			Joins(lspbJoin).
			Joins(spbJoin).
			Where("COALESCE(e.bttt_aktifyn, 'Y') = 'Y'")

		if !bypassTanggal && startDate != "" && endDate != "" {
			query = query.Where("e.bttt_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
		}
		if cabangAsal != "" && cabangAsal != "ALL" {
			query = query.Where("LPAD(LEFT(TRIM(e.bttt_id::varchar), 3), 3, '0') = LPAD(?, 3, '0')", cabangAsal)
		}
		if cabangBerwenang != "" {
			query = query.Where("(ag_tujuan.agen_nama ILIKE ? OR e.bttt_tujuankota ILIKE ? OR LPAD(TRIM(COALESCE(e.bttt_tujuanagenid::varchar, '')), 3, '0') = LPAD(?, 3, '0'))",
				"%"+cabangBerwenang+"%", "%"+cabangBerwenang+"%", cabangBerwenang)
		}
		if pic != "" {
			query = query.Where("c.cust_pic ILIKE ?", "%"+pic+"%")
		}
		if customer != "" {
			query = query.Where("(c.cust_name ILIKE ? OR e.bttt_asalname ILIKE ? OR e.bttt_asalcustid::varchar ILIKE ?)",
				"%"+customer+"%", "%"+customer+"%", "%"+customer+"%")
		}
		if bttKembali == "Y" {
			query = query.Where("COALESCE(lspb.is_kembali, 'N') = 'Y'")
		} else if bttKembali == "N" {
			query = query.Where("COALESCE(lspb.is_kembali, 'N') <> 'Y'")
		}
		if sudahInvoice == "Y" {
			query = query.Where("inv.artih_id IS NOT NULL")
		} else if sudahInvoice == "N" {
			query = query.Where("inv.artih_id IS NULL")
		}
		if bayar == "Y" {
			query = query.Where("(COALESCE(rc.total_bayar, 0) > 0 OR COALESCE(e.bttt_bayaryn, 'N') = 'Y')")
		} else if bayar == "N" {
			query = query.Where("(COALESCE(rc.total_bayar, 0) = 0 AND COALESCE(e.bttt_bayaryn, 'N') <> 'Y')")
		}
		if tglPelunasan != "" {
			query = query.Where("rc.tgl_bayar <= ?", tglPelunasan+" 23:59:59")
		}
		if pembayaran == "TUNAI" {
			query = query.Where("TRIM(COALESCE(e.bttt_pembayaran::varchar, '')) = '1'")
		} else if pembayaran == "KREDIT" {
			query = query.Where("TRIM(COALESCE(e.bttt_pembayaran::varchar, '')) = '2'")
		} else if pembayaran == "TAGIH TURUN" {
			query = query.Where("TRIM(COALESCE(e.bttt_pembayaran::varchar, '')) NOT IN ('1', '2')")
		}
		if jenisNaikTurun == "N" {
			query = query.Where("TRIM(COALESCE(e.bttt_pembayaran::varchar, '')) <> '3'")
		} else if jenisNaikTurun == "T" {
			query = query.Where("TRIM(COALESCE(e.bttt_pembayaran::varchar, '')) = '3'")
		}

		var bttItems []KondisiBTTItem
		if err := query.Order("e.bttt_tanggal ASC, e.bttt_id ASC").Limit(5000).Scan(&bttItems).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query kondisi BTT: " + err.Error()})
			return
		}
		allResults = append(allResults, bttItems...)
	}

	if showOJ && hasOrderJemput {
		var ojItems []KondisiBTTItem
		ojQuery := database.Table("public.mkt_t_orderjemput o").
			Select(`
				'ORDER JEMPUT' AS tipe_dokumen,
				TRIM(o.order_id::varchar) AS no_dokumen,
				o.order_date AS tgl_dokumen,
				COALESCE(TRIM(o.order_custid::varchar), '') AS cust_id,
				COALESCE(c.cust_name, '') AS cust_name,
				'' AS layanan,
				'KREDIT' AS metode_bayar,
				'' AS packing_id,
				0::numeric AS by_packing,
				'' AS no_kwitansi,
				NULL::timestamp AS tgl_kwitansi,
				'' AS no_tt,
				NULL::timestamp AS tgl_tt,
				'' AS no_pembayaran,
				NULL::timestamp AS tgl_bayar,
				COALESCE(o.order_amount::numeric, 0) AS by_kirim,
				0::numeric AS uang_muka,
				0::numeric AS terbayar_nominal,
				0::numeric AS dikembalikan,
				COALESCE(o.order_amount::numeric, 0) AS total_biaya,
				COALESCE(o.order_koli::numeric, 0) AS koli,
				GREATEST(COALESCE(o.order_berat::numeric, 0), COALESCE(o.order_volume::numeric, 0)) AS berat,
				'' AS kota_tujuan,
				'' AS penerima,
				NULL::timestamp AS tgl_btt_kembali,
				'' AS no_spb,
				0 AS umur_btt,
				'' AS btt_dbs,
				COALESCE(c.cust_pic, '') AS pic,
				'' AS cabang_berwenang,
				'N' AS btt_kembali_yn,
				'N' AS sudah_invoice_yn,
				'-' AS no_invoice,
				'N' AS terbayar_yn,
				LPAD(LEFT(TRIM(o.order_id::varchar), 3), 3, '0') AS agen_asal_id,
				COALESCE(a.agen_nama, 'DLI PUSAT') AS agen_asal_nama
			`).
			Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id::varchar) = TRIM(o.order_custid::varchar)").
			Joins("LEFT JOIN public.glb_m_agen a ON LPAD(TRIM(a.agen_id::varchar), 3, '0') = LPAD(LEFT(TRIM(o.order_id::varchar), 3), 3, '0')").
			Where("COALESCE(o.order_activeyn, 'Y') = 'Y'")

		if !bypassTanggal && startDate != "" && endDate != "" {
			ojQuery = ojQuery.Where("o.order_date BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
		}
		if cabangAsal != "" && cabangAsal != "ALL" {
			ojQuery = ojQuery.Where("LPAD(LEFT(TRIM(o.order_id::varchar), 3), 3, '0') = LPAD(?, 3, '0')", cabangAsal)
		}
		if customer != "" {
			ojQuery = ojQuery.Where("(c.cust_name ILIKE ? OR o.order_custid::varchar ILIKE ?)", "%"+customer+"%", "%"+customer+"%")
		}
		if pic != "" {
			ojQuery = ojQuery.Where("c.cust_pic ILIKE ?", "%"+pic+"%")
		}

		if err := ojQuery.Order("o.order_date DESC, o.order_id DESC").Limit(500).Scan(&ojItems).Error; err == nil {
			allResults = append(allResults, ojItems...)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   allResults,
	})
}
