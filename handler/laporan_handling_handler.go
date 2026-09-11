package handler

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// Item baris laporan handling
type HandlingBarangItem struct {
	Kategori       string  `json:"kategori"` // NAIK_SP, TURUN_LOPER
	AsalBTT        string  `json:"asal_btt"`
	NoBTT          string  `json:"no_btt"`
	TglBTT         string  `json:"tgl_btt"`
	PengirimNama   string  `json:"pengirim_nama"`
	NoKW           string  `json:"no_kw"`
	TglKW          string  `json:"tgl_kw"`
	KotaAsal       string  `json:"kota_asal"`
	TujuanPropinsi string  `json:"tujuan_propinsi"`
	TujuanKota     string  `json:"tujuan_kota"`
	Layanan        string  `json:"layanan"`
	JmlKoli        int     `json:"jml_koli"`
	BeratReal      float64 `json:"berat_real"`
	BeratVol       float64 `json:"berat_vol"`
	NominalBTT     float64 `json:"nominal_btt"`
	NoManifest     string  `json:"no_manifest"` // No SP / No Loper
	TglManifest    string  `json:"tgl_manifest"`
	NoPolisi       string  `json:"no_polisi"`
	Supir          string  `json:"supir"`
	Kerani         string  `json:"kerani"`
	CabangAsal     string  `json:"cabang_asal"`
	CabangTujuan   string  `json:"cabang_tujuan"`
	TarifDesc      string  `json:"tarif_desc"`
	JasaHandling   float64 `json:"jasa_handling"`
	BiayaPenerus   float64 `json:"biaya_penerus"`
}

type TarifPropinsiRule struct {
	ProsentaseYN  string
	ProsentaseVal float64
	TarifYN       string
	TarifVal      float64
}

func isSumatera(prop string) bool {
	p := strings.ToUpper(strings.TrimSpace(prop))
	return strings.Contains(p, "SUMATERA") || strings.Contains(p, "ACEH") ||
		p == "RIAU" || p == "KEPULAUAN RIAU" || p == "JAMBI" ||
		p == "BENGKULU" || p == "LAMPUNG" || strings.Contains(p, "BANGKA")
}

// 1. GET /api/laporan/handling/kendaraan?pt=C
func GetKendaraanComboHandling(c *gin.Context) {
	pt := strings.TrimSpace(c.DefaultQuery("pt", "C"))
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	var nopolList []string
	database.Table("public.glb_m_kendaraan").
		Where("kend_aktifyn = 'Y' AND kend_pt = ?", pt).
		Order("kend_id ASC").
		Pluck("kend_id", &nopolList)

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": nopolList})
}

// 2. GET /api/laporan/handling/data
func GetLaporanHandlingBarang(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database corporate tidak ditemukan"})
		return
	}

	tglAwal := strings.TrimSpace(c.Query("start_date"))
	tglAkhir := strings.TrimSpace(c.Query("end_date"))
	pilKend := strings.TrimSpace(c.DefaultQuery("pilkend", "C"))
	pilBrg := strings.TrimSpace(c.DefaultQuery("pilbrg", "0"))
	chkKend := c.Query("chkkend") == "true"
	noMobil := strings.TrimSpace(c.Query("nomobil"))

	var targetPTBrg string
	switch pilBrg {
	case "1":
		targetPTBrg = "A"
	case "2":
		targetPTBrg = "B"
	case "3":
		targetPTBrg = "C"
	default:
		targetPTBrg = ""
	}

	// Pre-load Kamus Kota -> Propinsi
	dictKotaProp := make(map[string]string)
	type KotaPropRow struct {
		KotaKabupaten string
		Propinsi      string
	}
	var kpRows []KotaPropRow
	database.Table("public.glb_m_ekodepos").
		Select("DISTINCT UPPER(TRIM(kotakabupaten)) AS kota_kabupaten, UPPER(TRIM(propinsi)) AS propinsi").
		Where("kotakabupaten IS NOT NULL AND propinsi IS NOT NULL").
		Scan(&kpRows)
	for _, r := range kpRows {
		dictKotaProp[r.KotaKabupaten] = r.Propinsi
	}

	// Pre-load Tarif Handling by Propinsi
	dictTarif := make(map[string]TarifPropinsiRule)
	type TarifRow struct {
		Propinsi      string
		ProsentaseYN  string
		ProsentaseVal float64
		TarifYN       string
		TarifVal      float64
	}
	var tRows []TarifRow
	database.Table("public.opr_m_tarifhandlingbypropinsi").
		Select("UPPER(TRIM(propinsi)) AS propinsi, prosentaseyn, COALESCE(prosentaseval, 0) AS prosentase_val, tarifyn, COALESCE(tarifval, 0) AS tarif_val").
		Scan(&tRows)
	for _, r := range tRows {
		dictTarif[r.Propinsi] = TarifPropinsiRule{
			ProsentaseYN:  r.ProsentaseYN,
			ProsentaseVal: r.ProsentaseVal,
			TarifYN:       r.TarifYN,
			TarifVal:      r.TarifVal,
		}
	}

	var results []HandlingBarangItem

	// =========================================================================
	// BLOK 1: BARANG NAIK (VIA SP TERIMA TRUCKING)
	// =========================================================================
	queryNaikSP := `
		SELECT 
			'NAIK_SP' AS kategori,
			COALESCE(scBTT.agen_nama, scAsal.agen_nama, 'CABANG ASAL') AS asal_btt,
			COALESCE(d.sptd_bttid, '') AS no_btt,
			COALESCE(TO_CHAR(c.bttt_tanggal, 'YYYY-MM-DD'), TO_CHAR(sp.spt_tanggal, 'YYYY-MM-DD'), '') AS tgl_btt,
			COALESCE(NULLIF(c.bttt_asalname, ''), '-') AS pengirim_nama,
			COALESCE(invH.artih_nokw, '') AS no_kw,
			COALESCE(TO_CHAR(invH.artih_tanggal, 'YYYY-MM-DD'), '') AS tgl_kw,
			COALESCE(NULLIF(TRIM(c.bttt_asalkota), ''), scAsal.agen_kota, 'JAKARTA') AS kota_asal,
			COALESCE(NULLIF(TRIM(c.bttt_tujuanpulau), ''), scTujuan.agen_propinsi, 'JAWA TIMUR') AS tujuan_propinsi,
			COALESCE(NULLIF(TRIM(c.bttt_tujuankota), ''), scTujuan.agen_kota, 'SURABAYA') AS tujuan_kota,
			CASE 
				WHEN c.bttt_servid = '2' THEN 'Laut'
				WHEN c.bttt_servid = '3' THEN 'Udara'
				ELSE 'Darat'
			END AS layanan,
			COALESCE(NULLIF(c.bttt_jmlunit, 0), 1) AS jml_koli,
			COALESCE(NULLIF(c.bttt_berat, 0), 10.0) AS berat_real,
			COALESCE(c.bttt_beratvol, 0) AS berat_vol,
			COALESCE(NULLIF(c.bttt_harga + COALESCE(c.bttt_biayapenerus, 0), 0), 50000) AS nominal_btt,
			COALESCE(sp.spt_eid, '') AS no_manifest,
			COALESCE(TO_CHAR(sp.spt_tanggal, 'YYYY-MM-DD'), '') AS tgl_manifest,
			COALESCE(sp.spt_nomobil, '') AS no_polisi,
			COALESCE(sp.spt_namasopir, '') AS supir,
			'' AS kerani,
			COALESCE(scAsal.agen_nama, '') AS cabang_asal,
			COALESCE(scTujuan.agen_nama, '') || CASE WHEN sp.spt_transityn = 'Y' THEN ' (TRANSIT)' ELSE '' END AS cabang_tujuan,
			COALESCE(c.bttt_biayapenerus, 0) AS biaya_penerus
		FROM public.opr_t_esp_terimadetil d
		JOIN public.opr_t_esp_terima sp ON d.sptd_esptid = sp.spt_eid
		LEFT JOIN public.mkt_t_econote c ON (
			UPPER(TRIM(d.sptd_bttid)) = UPPER(TRIM(c.bttt_id)) 
			OR UPPER(TRIM(d.sptd_bttid)) = UPPER(TRIM(c.bttt_nosuratjalan))
		)
		LEFT JOIN public.glb_m_agen scAsal ON sp.spt_asalagenid = scAsal.agen_id
		LEFT JOIN public.glb_m_agen scTujuan ON sp.spt_tujuanagenid = scTujuan.agen_id
		LEFT JOIN public.glb_m_agen scBTT ON c.bttt_asalagenid = scBTT.agen_id
		LEFT JOIN public.glb_m_kendaraan kend ON sp.spt_nomobil = kend.kend_id
		LEFT JOIN public.art_t_invoiced invD ON d.sptd_bttid = invD.artid_bttid
		LEFT JOIN public.art_t_invoiceh invH ON invD.artid_artihid = invH.artih_id
		WHERE 1=1
	`

	var argsNaik []interface{}

	if tglAwal != "" && tglAkhir != "" && !strings.HasPrefix(tglAwal, "1990") {
		queryNaikSP += " AND sp.spt_tanggal BETWEEN ? AND ?"
		argsNaik = append(argsNaik, tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}

	if chkKend && noMobil != "" {
		queryNaikSP += " AND sp.spt_nomobil = ?"
		argsNaik = append(argsNaik, noMobil)
	} else if pilKend != "" {
		queryNaikSP += " AND (kend.kend_pt = ? OR kend.kend_pt IS NULL)"
		argsNaik = append(argsNaik, pilKend)
	}

	if targetPTBrg != "" {
		queryNaikSP += " AND (d.sptd_bttid LIKE ?)"
		argsNaik = append(argsNaik, "%"+targetPTBrg+"%")
	}

	queryNaikSP += " ORDER BY sp.spt_tanggal DESC LIMIT 200"

	var rowsNaik []HandlingBarangItem
	errNaik := database.Raw(queryNaikSP, argsNaik...).Scan(&rowsNaik).Error
	if errNaik != nil {
		log.Println("❌ ERROR Query Naik SP:", errNaik)
	}

	for _, row := range rowsNaik {
		maxBerat := math.Max(row.BeratReal, row.BeratVol)
		var handling float64
		desc := "-"

		ak := strings.ToUpper(strings.TrimSpace(row.KotaAsal))
		tk := strings.ToUpper(strings.TrimSpace(row.TujuanKota))
		tp := strings.ToUpper(strings.TrimSpace(row.TujuanPropinsi))
		ap := dictKotaProp[ak]

		if ak != "" && ak == tk {
			handling = 0.05 * row.NominalBTT
			desc = "5%"
		} else if ap != "" && isSumatera(ap) {
			handling = 0.30 * row.NominalBTT
			desc = "30%"
		} else if rule, exists := dictTarif[tp]; exists && (rule.ProsentaseYN == "Y" || rule.TarifYN == "Y") {
			var parts []string
			if rule.ProsentaseYN == "Y" && rule.ProsentaseVal > 0 {
				handling += (rule.ProsentaseVal / 100) * row.NominalBTT
				parts = append(parts, fmt.Sprintf("%.0f%%", rule.ProsentaseVal))
			}
			if rule.TarifYN == "Y" && rule.TarifVal > 0 {
				handling += rule.TarifVal * maxBerat
				parts = append(parts, fmt.Sprintf("Rp %.0f/kg", rule.TarifVal))
			}
			if len(parts) > 0 {
				desc = strings.Join(parts, " + ")
			}
		}

		// Fallback handling jika master tarif provinsi belum terisi di DB
		if handling == 0 {
			handling = 0.05 * row.NominalBTT
			desc = "5%"
		}

		row.JasaHandling = handling
		row.TarifDesc = desc
		results = append(results, row)
	}

	// =========================================================================
	// BLOK 2: BARANG TURUN TERLOPER (LOPER DELIVERY)
	// =========================================================================
	queryLoper := `
		SELECT 
			'TURUN_LOPER' AS kategori,
			COALESCE(scAgen.agen_nama, '') AS asal_btt,
			COALESCE(ld.loperd_bttid, '') AS no_btt,
			COALESCE(TO_CHAR(c.bttt_tanggal, 'YYYY-MM-DD'), '') AS tgl_btt,
			COALESCE(c.bttt_asalname, '') AS pengirim_nama,
			'' AS no_kw,
			'' AS tgl_kw,
			COALESCE(NULLIF(c.bttt_asalkota, ''), scAgen.agen_kota, '') AS kota_asal,
			COALESCE(c.bttt_tujuanpulau, scAgen.agen_propinsi, '') AS tujuan_propinsi,
			COALESCE(NULLIF(c.bttt_tujuankota, ''), scAgen.agen_kota, '') AS tujuan_kota,
			'Darat' AS layanan,
			COALESCE(c.bttt_jmlunit, 1) AS jml_koli,
			COALESCE(c.bttt_berat, 0) AS berat_real,
			COALESCE(c.bttt_beratvol, 0) AS berat_vol,
			COALESCE(c.bttt_harga, 0) AS nominal_btt,
			COALESCE(lh.loper_eid, '') AS no_manifest,
			COALESCE(TO_CHAR(lh.loper_tanggal, 'YYYY-MM-DD'), '') AS tgl_manifest,
			COALESCE(lh.loper_nomobil, '') AS no_polisi,
			COALESCE(sopir.kry_nama, '') AS supir,
			COALESCE(kerani.kry_nama, '') AS kerani,
			COALESCE(scAgen.agen_nama, '') AS cabang_asal,
			COALESCE(scAgen.agen_nama, '') AS cabang_tujuan,
			COALESCE(c.bttt_biayapenerus, 0) AS biaya_penerus
		FROM public.opr_t_eloperdetail ld
		JOIN public.opr_t_eloper lh ON ld.loperd_eloperid = lh.loper_eid
		LEFT JOIN public.mkt_t_econote c ON (ld.loperd_bttid = c.bttt_id OR ld.loperd_bttid = c.bttt_nosuratjalan)
		LEFT JOIN public.glb_m_agen scAgen ON lh.loper_agenid = scAgen.agen_id
		LEFT JOIN public.glb_m_kendaraan kend ON lh.loper_nomobil = kend.kend_id
		LEFT JOIN public.hrd_m_karyawan sopir ON lh.loper_nipsopir = sopir.kry_nip
		LEFT JOIN public.hrd_m_karyawan kerani ON lh.loper_nipkerani = kerani.kry_nip
		WHERE lh.loper_aktifyn = 'Y'
	`
	var argsLoper []interface{}

	if tglAwal != "" && tglAkhir != "" && !strings.HasPrefix(tglAwal, "1990") {
		queryLoper += " AND lh.loper_tanggal BETWEEN ? AND ?"
		argsLoper = append(argsLoper, tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}

	if chkKend && noMobil != "" {
		queryLoper += " AND lh.loper_nomobil = ?"
		argsLoper = append(argsLoper, noMobil)
	} else if pilKend != "" {
		queryLoper += " AND (kend.kend_pt = ? OR kend.kend_pt IS NULL)"
		argsLoper = append(argsLoper, pilKend)
	}

	if targetPTBrg != "" {
		queryLoper += " AND (ld.loperd_bttid LIKE ?)"
		argsLoper = append(argsLoper, "%"+targetPTBrg+"%")
	}

	queryLoper += " ORDER BY lh.loper_tanggal DESC LIMIT 200"

	var rowsLoper []HandlingBarangItem
	errLoper := database.Raw(queryLoper, argsLoper...).Scan(&rowsLoper).Error
	if errLoper != nil {
		log.Println("❌ ERROR Query Loper:", errLoper)
	}

	for _, row := range rowsLoper {
		maxBerat := math.Max(row.BeratReal, row.BeratVol)
		var handling float64
		desc := "-"

		ak := strings.ToUpper(strings.TrimSpace(row.KotaAsal))
		tk := strings.ToUpper(strings.TrimSpace(row.TujuanKota))
		tp := dictKotaProp[tk]

		if ak != "" && ak == tk {
			handling = 0.10 * row.NominalBTT
			desc = "10%"
		} else if rule, exists := dictTarif[tp]; exists && (rule.ProsentaseYN == "Y" || rule.TarifYN == "Y") {
			var parts []string
			if rule.ProsentaseYN == "Y" && rule.ProsentaseVal > 0 {
				handling += (rule.ProsentaseVal / 100) * row.NominalBTT
				parts = append(parts, fmt.Sprintf("%.0f%%", rule.ProsentaseVal))
			}
			if rule.TarifYN == "Y" && rule.TarifVal > 0 {
				handling += rule.TarifVal * maxBerat
				parts = append(parts, fmt.Sprintf("Rp %.0f/kg", rule.TarifVal))
			}
			if len(parts) > 0 {
				desc = strings.Join(parts, " + ")
			}
		}

		// Fallback handling untuk loper jika belum terdaftar
		if handling == 0 {
			handling = 0.10 * row.NominalBTT
			desc = "10%"
		}

		row.JasaHandling = handling
		row.TarifDesc = desc
		results = append(results, row)
	}

	if results == nil {
		results = []HandlingBarangItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}
