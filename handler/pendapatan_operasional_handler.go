package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// GetLaporanPendapatanOperasional Handler Utama Laporan Pendapatan Operasional
func GetLaporanPendapatanOperasional(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database gagal"})
		return
	}

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	transaksi := strings.TrimSpace(c.Query("transaksi")) // "0": Semua, "1": Loper, "2": Jemput, "3": SP
	ckNoMobil := strings.TrimSpace(c.Query("cknomobil"))
	noMobil := strings.TrimSpace(c.Query("nomobil"))

	if tgla == "" {
		tgla = "2017-01-01"
	}
	if tgle == "" {
		tgle = "2026-12-31"
	}

	filterMobilLoper := ""
	filterMobilJemput := ""
	filterMobilSP := ""

	if (ckNoMobil == "true" || ckNoMobil == "on") && noMobil != "" {
		filterMobilLoper = fmt.Sprintf(` AND UPPER(l.loper_nomobil) LIKE UPPER('%%%s%%') `, noMobil)
		filterMobilJemput = fmt.Sprintf(` AND UPPER(oj.order_nopol) LIKE UPPER('%%%s%%') `, noMobil)
		filterMobilSP = fmt.Sprintf(` AND UPPER(sp.spt_nomobil) LIKE UPPER('%%%s%%') `, noMobil)
	}

	var responseData models.LaporanPendapatanOperasionalResponse
	responseData.ListLoper = []models.PendapatanLoperDTO{}
	responseData.ListSP = []models.PendapatanSPDTO{}
	responseData.ListJemput = []models.PendapatanJemputDTO{}

	// =========================================================================
	// 1. QUERY SURAT PENGANTAR (SP) -> Presisi Sesuai ASP Lawas
	// =========================================================================
	if transaksi == "0" || transaksi == "3" {
		querySP := fmt.Sprintf(`
			SELECT 
				COALESCE(sp.spt_nomobil, '-') AS no_mobil,
				TO_CHAR(sp.spt_tanggal, 'YYYY-MM-DD') AS tanggal,
				COALESCE(sp.spt_eid, '-') AS no_sp,
				COALESCE(a.agen_nama, e.bttt_tujuankota, '-') AS tujuan,
				COALESCE(spd.sptd_bttid, '-') AS no_btt,
				(COALESCE(e.bttt_harga, 0) + COALESCE(e.bttt_biayapenerus, 0) + COALESCE(pck.pck_biaya, 0) + COALESCE(asr.totalbiaya, 0)) AS biaya_kirim,
				COALESCE(jd.tjurd_tjurhno, '-') AS no_jurnal,
				COALESCE(jd.tjurd_debet, 0) AS debet
			FROM public.opr_t_esp_terima sp
			INNER JOIN public.opr_t_esp_terimadetil spd ON sp.spt_eid = spd.sptd_esptid
			INNER JOIN public.mkt_t_econote e ON TRIM(UPPER(spd.sptd_bttid)) = TRIM(UPPER(e.bttt_id))
			LEFT JOIN public.pck_t_packing pck ON e.bttt_packingid = pck.pck_id
			LEFT JOIN public.mkt_t_asuransi asr ON e.bttt_id = asr.bttt_id
			LEFT JOIN public.glb_m_agen a ON CAST(sp.spt_tujuanagenid AS VARCHAR) = CAST(a.agen_id AS VARCHAR)
			LEFT JOIN public.gl_t_jurnald jd ON TRIM(UPPER(jd.tjurd_keterangan)) = TRIM(UPPER(sp.spt_eid))
			WHERE CAST(sp.spt_tanggal AS DATE) BETWEEN '%s' AND '%s'
			  %s
			ORDER BY sp.spt_tanggal DESC
			LIMIT 500
		`, tgla, tgle, filterMobilSP)

		type RawSP struct {
			NoMobil    string  `gorm:"column:no_mobil"`
			Tanggal    string  `gorm:"column:tanggal"`
			NoSP       string  `gorm:"column:no_sp"`
			Tujuan     string  `gorm:"column:tujuan"`
			NoBTT      string  `gorm:"column:no_btt"`
			BiayaKirim float64 `gorm:"column:biaya_kirim"`
			NoJurnal   string  `gorm:"column:no_jurnal"`
			Debet      float64 `gorm:"column:debet"`
		}

		var rawSPs []RawSP
		_ = database.Raw(querySP).Scan(&rawSPs).Error

		spMap := make(map[string]*models.PendapatanSPDTO)
		for _, raw := range rawSPs {
			if item, exists := spMap[raw.NoSP]; exists {
				item.ListBTT = append(item.ListBTT, models.DetailBTTItem{
					NoBTT:      raw.NoBTT,
					Tujuan:     raw.Tujuan,
					BiayaKirim: raw.BiayaKirim,
				})
				item.TotalBTT += raw.BiayaKirim
			} else {
				spMap[raw.NoSP] = &models.PendapatanSPDTO{
					NoMobil: raw.NoMobil,
					Tanggal: raw.Tanggal,
					NoSP:    raw.NoSP,
					Tujuan:  raw.Tujuan,
					ListBTT: []models.DetailBTTItem{
						{
							NoBTT:      raw.NoBTT,
							Tujuan:     raw.Tujuan,
							BiayaKirim: raw.BiayaKirim,
						},
					},
					TotalBTT: raw.BiayaKirim,
					ListJurnal: []models.DetailJurnalItem{
						{
							NoJurnal: raw.NoJurnal,
							Debet:    raw.Debet,
						},
					},
					TotalJurnal: raw.Debet,
				}
			}
		}

		for _, item := range spMap {
			responseData.ListSP = append(responseData.ListSP, *item)
		}
	}

	// =========================================================================
	// 2. QUERY PENGANTARAN LOPER
	// =========================================================================
	if transaksi == "0" || transaksi == "1" {
		queryLoper := fmt.Sprintf(`
			SELECT 
				COALESCE(l.loper_nomobil, '-') AS no_mobil,
				TO_CHAR(l.loper_tanggal, 'YYYY-MM-DD') AS tanggal,
				COALESCE(l.loper_eid, '-') AS no_loper,
				COALESCE(k.kry_nama, '-') AS nama_sopir,
				COALESCE(ld.loperd_bttid, '-') AS no_btt,
				COALESCE(e.bttt_tujuankota, '-') AS tujuan,
				(COALESCE(e.bttt_harga, 0) + COALESCE(e.bttt_biayapenerus, 0) + COALESCE(pck.pck_biaya, 0) + COALESCE(asr.totalbiaya, 0)) AS biaya_kirim
			FROM public.opr_t_eloper l
			INNER JOIN public.opr_t_eloperdetail ld ON l.loper_eid = ld.loperd_eloperid
			INNER JOIN public.mkt_t_econote e ON TRIM(UPPER(ld.loperd_bttid)) = TRIM(UPPER(e.bttt_id))
			LEFT JOIN public.pck_t_packing pck ON e.bttt_packingid = pck.pck_id
			LEFT JOIN public.mkt_t_asuransi asr ON e.bttt_id = asr.bttt_id
			LEFT JOIN public.hrd_m_karyawan k ON l.loper_nipsopir = k.kry_nip
			WHERE CAST(l.loper_tanggal AS DATE) BETWEEN '%s' AND '%s'
			  %s
			ORDER BY l.loper_tanggal DESC
			LIMIT 500
		`, tgla, tgle, filterMobilLoper)

		type RawLoper struct {
			NoMobil    string  `gorm:"column:no_mobil"`
			Tanggal    string  `gorm:"column:tanggal"`
			NoLoper    string  `gorm:"column:no_loper"`
			NamaSopir  string  `gorm:"column:nama_sopir"`
			NoBTT      string  `gorm:"column:no_btt"`
			Tujuan     string  `gorm:"column:tujuan"`
			BiayaKirim float64 `gorm:"column:biaya_kirim"`
		}

		var rawLopers []RawLoper
		_ = database.Raw(queryLoper).Scan(&rawLopers).Error

		loperMap := make(map[string]*models.PendapatanLoperDTO)
		for _, raw := range rawLopers {
			if item, exists := loperMap[raw.NoLoper]; exists {
				item.ListBTT = append(item.ListBTT, models.DetailBTTItem{
					NoBTT:      raw.NoBTT,
					Tujuan:     raw.Tujuan,
					BiayaKirim: raw.BiayaKirim,
				})
				item.TotalBTT += raw.BiayaKirim
			} else {
				loperMap[raw.NoLoper] = &models.PendapatanLoperDTO{
					NoMobil:   raw.NoMobil,
					Tanggal:   raw.Tanggal,
					NoLoper:   raw.NoLoper,
					NamaSopir: raw.NamaSopir,
					ListBTT: []models.DetailBTTItem{
						{
							NoBTT:      raw.NoBTT,
							Tujuan:     raw.Tujuan,
							BiayaKirim: raw.BiayaKirim,
						},
					},
					TotalBTT:   raw.BiayaKirim,
					ListJurnal: []models.DetailJurnalItem{},
				}
			}
		}

		for _, item := range loperMap {
			responseData.ListLoper = append(responseData.ListLoper, *item)
		}
	}

	// =========================================================================
	// 3. QUERY ORDER JEMPUT
	// =========================================================================
	if transaksi == "0" || transaksi == "2" {
		queryJemput := fmt.Sprintf(`
			SELECT 
				COALESCE(oj.order_nopol, '-') AS no_mobil,
				TO_CHAR(oj.order_date, 'YYYY-MM-DD') AS tanggal,
				COALESCE(c.cust_name, '-') AS customer,
				COALESCE(oj.order_id, '-') AS no_jemput,
				COALESCE(oj.order_koli, 0) AS jml_koli,
				COALESCE(oj.order_berat, 0) AS berat,
				COALESCE(oj.order_volume, 0) AS volume
			FROM public.mkt_t_orderjemput oj
			LEFT JOIN public.mkt_m_customer c ON oj.order_custid = c.cust_id
			WHERE CAST(oj.order_date AS DATE) BETWEEN '%s' AND '%s'
			  %s
			ORDER BY oj.order_date DESC
			LIMIT 500
		`, tgla, tgle, filterMobilJemput)

		type RawJemput struct {
			NoMobil  string  `gorm:"column:no_mobil"`
			Tanggal  string  `gorm:"column:tanggal"`
			Customer string  `gorm:"column:customer"`
			NoJemput string  `gorm:"column:no_jemput"`
			JmlKoli  float64 `gorm:"column:jml_koli"`
			Berat    float64 `gorm:"column:berat"`
			Volume   float64 `gorm:"column:volume"`
		}

		var rawJemputs []RawJemput
		_ = database.Raw(queryJemput).Scan(&rawJemputs).Error

		for _, oj := range rawJemputs {
			responseData.ListJemput = append(responseData.ListJemput, models.PendapatanJemputDTO{
				NoMobil:     oj.NoMobil,
				Tanggal:     oj.Tanggal,
				Customer:    oj.Customer,
				NoJemput:    oj.NoJemput,
				JmlKoli:     oj.JmlKoli,
				Berat:       oj.Berat,
				Volume:      oj.Volume,
				NoJurnal:    "-",
				TotalJurnal: 0,
				ListJurnal:  []models.DetailJurnalItem{},
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   responseData,
	})
}
