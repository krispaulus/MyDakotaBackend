package handler

import (
	"fmt"
	"math"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// TariffInfo Penampung cache tarif handling
type TariffInfo struct {
	ProsentaseYN  string
	ProsentaseVal float64
	TarifYN       string
	TarifVal      float64
}

// GetLaporanBarangTurunDetail Handler untuk Tipe 1 (Detail Laporan Barang Turun)
func GetLaporanBarangTurunDetail(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	transit := strings.ToUpper(strings.TrimSpace(c.Query("transit")))
	namaCabang := strings.TrimSpace(c.Query("cabang"))

	if tgla == "" || tgle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Periode Tanggal Wajib Diisi"})
		return
	}

	// 1. Ambil Info Cabang & Filter Clause
	var agenInfo struct {
		AgenID   int    `gorm:"column:agen_id"`
		AgenNama string `gorm:"column:agen_nama"`
	}

	filterCabangClause := ""
	if namaCabang != "" && strings.ToUpper(namaCabang) != "SEMUA" && !strings.Contains(strings.ToUpper(namaCabang), "SEMUA AGEN") {
		cleanCabang := strings.TrimSpace(strings.ReplaceAll(namaCabang, "(HOLDING)", ""))
		database.Raw(`
			SELECT agen_id, agen_nama 
			FROM public.glb_m_agen 
			WHERE UPPER(agen_nama) = UPPER(?) 
			   OR UPPER(agen_nama) LIKE UPPER(?) 
			LIMIT 1
		`, namaCabang, "%"+cleanCabang+"%").Scan(&agenInfo)

		if agenInfo.AgenID > 0 {
			filterCabangClause = fmt.Sprintf(` AND t.sp_agenid = '%d' `, agenInfo.AgenID)
		}
	}

	// 2. Load Mapping Kota -> Propinsi
	rowsKP, _ := database.Raw(`
		SELECT DISTINCT UPPER(TRIM(kotakabupaten)) as kota, UPPER(TRIM(propinsi)) as propinsi 
		FROM public.glb_m_ekodepos 
		WHERE kotakabupaten IS NOT NULL AND propinsi IS NOT NULL
	`).Rows()
	dictKotaPropinsi := make(map[string]string)
	if rowsKP != nil {
		defer rowsKP.Close()
		for rowsKP.Next() {
			var k, p string
			rowsKP.Scan(&k, &p)
			if k != "" {
				dictKotaPropinsi[k] = p
			}
		}
	}

	// 3. Load Tarif Handling By Propinsi
	rowsTarif, _ := database.Raw(`
		SELECT 
			UPPER(TRIM(propinsi)) as prop, 
			COALESCE(prosentaseyn, 'N'), 
			COALESCE(prosentaseval, 0), 
			COALESCE(tarifyn, 'N'), 
			COALESCE(tarifval, 0) 
		FROM public.opr_m_tarifhandlingbypropinsi
	`).Rows()
	dictTarif := make(map[string]TariffInfo)
	if rowsTarif != nil {
		defer rowsTarif.Close()
		for rowsTarif.Next() {
			var prop, pYN, tYN string
			var pVal, tVal float64
			rowsTarif.Scan(&prop, &pYN, &pVal, &tYN, &tVal)
			dictTarif[prop] = TariffInfo{ProsentaseYN: pYN, ProsentaseVal: pVal, TarifYN: tYN, TarifVal: tVal}
		}
	}

	// 4. Build Filter Transit
	filterTransit := ""
	if transit == "YA" {
		filterTransit = ` AND ter.spt_transityn = 'Y' `
	} else if transit == "TIDAK" {
		filterTransit = ` AND ter.spt_transityn = 'N' `
	}

	// 5. Query Raw Data Rincian BTT (Full Dynamic Query & Presisi Join)
	queryStr := fmt.Sprintf(`
		SELECT 
			t.sp_eid AS "SP_eID",
			p.bttt_tujuanagenid AS "BTTt_TujuanAgenID",
			ter.spt_tanggal AS "SPT_Tanggal",
			ter.spt_asalagenid AS "SPT_AsalAgenID",
			a.agen_nama AS "Agen_Nama",
			t.sp_bttid AS "SP_BTTID",
			p.bttt_tanggal AS "BTTT_Tanggal",
			c.cust_name AS "Cust_Name",
			p.bttt_tujuannama AS "BTTT_TujuanNama",
			COALESCE(p.bttt_beratvol, 0) AS "BTTT_Beratvol",
			COALESCE(p.bttt_jmlunit, 0) AS "BTTT_JmlUnit",
			p.bttt_tujuankota AS "BTTT_TujuanKota",
			p.bttt_servid AS "BTTT_ServID",
			COALESCE(p.bttt_berat, 0) AS "BTTT_Berat",
			p.bttt_pembayaran AS "BTTT_Pembayaran",
			COALESCE(p.bttt_harga, 0) AS "BTTT_Harga",
			COALESCE(p.bttt_biayapenerus, 0) AS "BTTT_BiayaPenerus",
			ter.spt_transityn AS "SPT_TransitYN",
			p.bttt_service AS "BTTT_Service"
		FROM public.opr_t_esp_turun t
		LEFT JOIN public.mkt_t_econote p ON TRIM(UPPER(t.sp_bttid)) = TRIM(UPPER(p.bttt_id))
		LEFT JOIN public.mkt_m_customer c ON p.bttt_asalcustid = c.cust_id
		LEFT JOIN public.opr_t_esp_terima ter ON t.sp_eid = ter.spt_eid
		LEFT JOIN public.glb_m_agen a ON ter.spt_asalagenid = a.agen_id
		WHERE CAST(t.sp_tanggal AS DATE) BETWEEN ? AND ? %s %s
		ORDER BY t.sp_eid, t.sp_bttid
	`, filterTransit, filterCabangClause)

	type RawDetail struct {
		SPeID            string  `gorm:"column:SP_eID"`
		BTTtTujuanAgenID string  `gorm:"column:BTTt_TujuanAgenID"`
		SPTTanggal       string  `gorm:"column:SPT_Tanggal"`
		SPTAsalAgenID    string  `gorm:"column:SPT_AsalAgenID"`
		AgenNama         string  `gorm:"column:Agen_Nama"`
		SPBTTID          string  `gorm:"column:SP_BTTID"`
		BTTTTanggal      string  `gorm:"column:BTTT_Tanggal"`
		CustName         string  `gorm:"column:Cust_Name"`
		BTTTTujuanNama   string  `gorm:"column:BTTT_TujuanNama"`
		BTTTBeratVol     float64 `gorm:"column:BTTT_Beratvol"`
		BTTTJmlUnit      float64 `gorm:"column:BTTT_JmlUnit"`
		BTTTTujuanKota   string  `gorm:"column:BTTT_TujuanKota"`
		BTTTServID       int     `gorm:"column:BTTT_ServID"`
		BTTTBerat        float64 `gorm:"column:BTTT_Berat"`
		BTTTPembayaran   int     `gorm:"column:BTTT_Pembayaran"`
		BTTTHarga        float64 `gorm:"column:BTTT_Harga"`
		BTTTBiayaPenerus float64 `gorm:"column:BTTT_BiayaPenerus"`
		SPTTransitYN     string  `gorm:"column:SPT_TransitYN"`
		BTTTService      string  `gorm:"column:BTTT_Service"`
	}

	var rawList []RawDetail
	if err := database.Raw(queryStr, tgla, tgle).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query detail barang turun: " + err.Error()})
		return
	}

	// 6. Loop Process Kalkulasi
	var resultList []models.BarangTurunDetailDTO
	for _, r := range rawList {
		viaText := "DARAT"
		if r.BTTTServID == 2 {
			viaText = "LAUT"
		} else if r.BTTTServID == 3 {
			viaText = "UDARA"
		}

		maxBerat := math.Max(r.BTTTBerat, r.BTTTBeratVol)
		var biayaKirim float64 = 0
		var kreditTunai float64 = 0
		var tagih float64 = 0

		if r.BTTTPembayaran != 3 {
			biayaKirim = r.BTTTHarga
			kreditTunai = r.BTTTHarga
		} else {
			tagih = r.BTTTHarga
		}

		// Calc Jasa Handling
		jasaHandling := 0.0
		tarifDesc := "-"
		tujuanKota := strings.ToUpper(strings.TrimSpace(r.BTTTTujuanKota))
		tujuanPropinsi := dictKotaPropinsi[tujuanKota]

		if tujuanPropinsi != "" {
			if tInfo, exists := dictTarif[tujuanPropinsi]; exists {
				descParts := []string{}
				if tInfo.ProsentaseYN == "Y" {
					jasaHandling += (tInfo.ProsentaseVal / 100.0) * biayaKirim
					descParts = append(descParts, fmt.Sprintf("%.0f%%", tInfo.ProsentaseVal))
				}
				if tInfo.TarifYN == "Y" {
					jasaHandling += tInfo.TarifVal * maxBerat
					descParts = append(descParts, fmt.Sprintf("Rp %.0f/kg", tInfo.TarifVal))
				}
				if len(descParts) > 0 {
					tarifDesc = strings.Join(descParts, " + ")
				}
			}
		}

		// Cek Digit ke-10 No BTT jika 'C' maka Handling = 0
		trimmedBTT := strings.TrimSpace(r.SPBTTID)
		if len(trimmedBTT) >= 10 && strings.ToUpper(string(trimmedBTT[9])) == "C" {
			jasaHandling = 0
			tarifDesc = "-"
		}

		transitYN := "Tidak"
		if fmt.Sprintf("%d", agenInfo.AgenID) != strings.TrimSpace(r.BTTtTujuanAgenID) {
			transitYN = "Ya"
		}

		custName := r.CustName
		if custName == "UMUM" {
			custName = ""
		}

		dto := models.BarangTurunDetailDTO{
			SPeID:          r.SPeID,
			SPTTanggal:     r.SPTTanggal,
			AgenNama:       r.AgenNama,
			TujuanPropinsi: tujuanPropinsi,
			BTTTTujuanKota: r.BTTTTujuanKota,
			ViaText:        viaText,
			SPBTTID:        r.SPBTTID,
			BTTTTanggal:    r.BTTTTanggal,
			CustName:       custName,
			BTTTTujuanNama: r.BTTTTujuanNama,
			BTTTJmlUnit:    r.BTTTJmlUnit,
			BTTTBerat:      r.BTTTBerat,
			BTTTBeratVol:   r.BTTTBeratVol,
			KreditTunai:    kreditTunai,
			Tagih:          tagih,
			BiayaPenerus:   r.BTTTBiayaPenerus,
			TransitYN:      transitYN,
			BTTTService:    r.BTTTService,
			TarifDesc:      tarifDesc,
			JasaHandling:   jasaHandling,
		}
		resultList = append(resultList, dto)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resultList,
		"count":  len(resultList),
	})
}

// GetLaporanBarangTurunRekap Handler untuk Tipe 2 (Rekap Laporan Barang Turun Per Cabang)
func GetLaporanBarangTurunRekap(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	transit := strings.ToUpper(strings.TrimSpace(c.Query("transit")))

	filterTransit := ""
	if transit == "YA" {
		filterTransit = ` AND ter.spt_transityn = 'Y' `
	} else if transit == "TIDAK" {
		filterTransit = ` AND ter.spt_transityn = 'N' `
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			a.agen_nama AS agen_nama,
			COUNT(DISTINCT t.sp_bttid) AS jml_btt,
			SUM(COALESCE(p.bttt_jmlunit, 0)) AS colly,
			SUM(COALESCE(p.bttt_berat, 0)) AS berat,
			SUM(COALESCE(p.bttt_beratvol, 0)) AS volume,
			SUM(CASE WHEN p.bttt_pembayaran <> 3 THEN COALESCE(p.bttt_harga, 0) ELSE 0 END) AS kredit_tunai,
			SUM(CASE WHEN p.bttt_pembayaran = 3 THEN COALESCE(p.bttt_harga, 0) ELSE 0 END) AS tagih,
			SUM(COALESCE(p.bttt_biayapenerus, 0)) AS biaya_penerus,
			0 AS jasa_handling
		FROM public.glb_m_param param
		LEFT JOIN public.glb_m_agen a ON param.set_cabid = a.agen_id
		LEFT JOIN public.opr_t_esp_turun t ON param.set_cabid = t.sp_agenid
		LEFT JOIN public.mkt_t_econote p ON TRIM(UPPER(t.sp_bttid)) = TRIM(UPPER(p.bttt_id))
		LEFT JOIN public.opr_t_esp_terima ter ON t.sp_eid = ter.spt_eid
		WHERE CAST(t.sp_tanggal AS DATE) BETWEEN ? AND ? %s
		GROUP BY a.agen_nama, param.set_cabid
		ORDER BY param.set_cabid
	`, filterTransit)

	var rekapList []models.BarangTurunRekapDTO
	if err := database.Raw(queryStr, tgla, tgle).Scan(&rekapList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query rekap barang turun: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   rekapList,
		"count":  len(rekapList),
	})
}
