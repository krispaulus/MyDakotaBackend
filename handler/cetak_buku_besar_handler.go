package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// SearchCoaHandler Auto-complete Pencarian Kode/Nama Akun COA
func SearchCoaHandler(c *gin.Context) {
	keyword := c.Query("caname")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.GetDB()
	}

	query := `
		SELECT ca_id, ca_name 
		FROM public.gl_m_chartaccount 
		WHERE COALESCE(ca_name, '') != '' 
		  AND (ca_name ILIKE $1 OR ca_id ILIKE $1)
		  AND COALESCE(ca_aktifyn, 'Y') = 'Y'
		ORDER BY ca_id ASC LIMIT 15
	`

	var results []map[string]interface{}
	_ = database.Raw(query, "%"+keyword+"%").Scan(&results)

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// GetBukuBesarReportHandler Menghitung & Menyajikan Data Laporan Buku Besar
func GetBukuBesarReportHandler(c *gin.Context) {
	tanggalStart := c.Query("tanggalStart")
	tanggalEnd := c.Query("tanggalEnd")
	akuna := c.Query("akuna")
	akune := c.Query("akune")
	cabangID := c.Query("cabang")
	piltrans := c.Query("piltrans") // "S" = Semua, "A" = Ada Transaksi Saja

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.GetDB()
	}

	// 1. Fetch Daftar Akun COA
	queryCoa := `
		SELECT 
			ca_id, 
			ca_name, 
			COALESCE(ca_type, 'DEBET') AS ca_type, 
			COALESCE(ca_jenis, 'D') AS ca_jenis
		FROM public.gl_m_chartaccount
		WHERE COALESCE(ca_aktifyn, 'Y') = 'Y'
	`
	if akuna != "" && akune != "" {
		queryCoa += fmt.Sprintf(" AND ca_id BETWEEN '%s' AND '%s'", akuna, akune)
	} else if akuna != "" {
		queryCoa += fmt.Sprintf(" AND ca_id ILIKE '%%%s%%'", akuna)
	}
	queryCoa += " ORDER BY ca_id ASC LIMIT 100"

	var coaList []map[string]interface{}
	if err := database.Raw(queryCoa).Scan(&coaList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	var reportData []map[string]interface{}

	// 2. Loop Setiap COA untuk kalkulasi Saldo Awal + Mutasi
	for _, coa := range coaList {
		caID := fmt.Sprintf("%v", coa["ca_id"])
		caName := fmt.Sprintf("%v", coa["ca_name"])
		caJenis := fmt.Sprintf("%v", coa["ca_jenis"])

		queryDetail := `
			SELECT 
				COALESCE(d.tjurd_tjurhno, h.tjurh_no, '-') AS no_jurnal,
				CASE 
					WHEN h.tjurh_tanggal IS NULL THEN '-'
					ELSE TO_CHAR(h.tjurh_tanggal::timestamp, 'MM/DD/YYYY')
				END AS tanggal,
				COALESCE(d.tjurd_keterangan, h.tjurh_keterangan, '-') AS keterangan,
				COALESCE(d.tjurd_debet, 0) AS debet,
				COALESCE(d.tjurd_kredit, 0) AS kredit
			FROM public.gl_t_jurnald d
			LEFT JOIN public.gl_t_jurnalh h ON d.tjurd_tjurhno = h.tjurh_no
			WHERE d.tjurd_acccode = $1
		`

		if tanggalStart != "" && tanggalEnd != "" {
			queryDetail += fmt.Sprintf(" AND (h.tjurh_tanggal IS NULL OR h.tjurh_tanggal BETWEEN '%s 00:00:00' AND '%s 23:59:59')", tanggalStart, tanggalEnd)
		}

		if cabangID != "" && cabangID != "1" && cabangID != "PUSAT" && cabangID != "HOLDING" {
			queryDetail += fmt.Sprintf(" AND CAST(d.tjurd_agenid AS VARCHAR) = '%s'", cabangID)
		}

		queryDetail += " ORDER BY h.tjurh_tanggal DESC, h.tjurh_no ASC"

		var details []map[string]interface{}
		_ = database.Raw(queryDetail, caID).Scan(&details)

		if piltrans == "A" && len(details) == 0 {
			continue
		}

		var totalDebet, totalKredit float64
		for _, det := range details {
			if dVal, ok := det["debet"].(float64); ok {
				totalDebet += dVal
			}
			if kVal, ok := det["kredit"].(float64); ok {
				totalKredit += kVal
			}
		}

		var saldoAkhir float64
		if caJenis == "D" || caJenis == "DEBET" {
			saldoAkhir = totalDebet - totalKredit
		} else {
			saldoAkhir = totalKredit - totalDebet
		}

		reportData = append(reportData, map[string]interface{}{
			"ca_id":        caID,
			"ca_name":      caName,
			"ca_jenis":     caJenis,
			"saldo_awal":   0,
			"total_debet":  totalDebet,
			"total_kredit": totalKredit,
			"saldo_akhir":  saldoAkhir,
			"details":      details,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   reportData,
	})
}

// ExportBukuBesarXlsHandler Menghasilkan File XLS Murni Persis Format App Lawas
func ExportBukuBesarXlsHandler(c *gin.Context) {
	tanggalStart := c.Query("tanggalStart")
	tanggalEnd := c.Query("tanggalEnd")
	akuna := c.Query("akuna")
	akune := c.Query("akune")
	cabangID := c.Query("cabang")
	piltrans := c.Query("piltrans")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.GetDB()
	}

	ptNama := "DAKOTA LOGISTIK INDONESIA"
	if fmt.Sprintf("%v", ptID) == "A" {
		ptNama = "DAKOTA BUANA SEMESTA"
	} else if fmt.Sprintf("%v", ptID) == "B" {
		ptNama = "DAKOTA LINTAS BUANA"
	}

	cabangNama := "KONSOLIDASI"
	if cabangID != "" && cabangID != "1" && cabangID != "PUSAT" {
		_ = database.Table("public.glb_m_agen").
			Select("agen_nama").
			Where("CAST(agen_id AS VARCHAR) = ?", cabangID).
			Scan(&cabangNama)
	}

	queryCoa := `
		SELECT ca_id, ca_name, COALESCE(ca_jenis, 'D') AS ca_jenis
		FROM public.gl_m_chartaccount
		WHERE COALESCE(ca_aktifyn, 'Y') = 'Y'
	`
	if akuna != "" && akune != "" {
		queryCoa += fmt.Sprintf(" AND ca_id BETWEEN '%s' AND '%s'", akuna, akune)
	} else if akuna != "" {
		queryCoa += fmt.Sprintf(" AND ca_id ILIKE '%%%s%%'", akuna)
	}
	queryCoa += " ORDER BY ca_id ASC"

	var coaList []map[string]interface{}
	_ = database.Raw(queryCoa).Scan(&coaList)

	nowStr := time.Now().Format("02/01/2006 15:04:05")
	var html strings.Builder

	html.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8">`)
	html.WriteString(`<style>
		td { font-family: Calibri, Arial, sans-serif; font-size: 11px; }
		.corporate-header { font-size: 14pt; font-weight: bold; font-family: Calibri, Arial, sans-serif; text-transform: uppercase; }
		.title { font-size: 13pt; font-weight: bold; text-align: center; }
		.sub-title { font-size: 10pt; font-weight: bold; text-align: center; }
		.bg-grey { background-color: #CFCFCF; font-weight: bold; text-align: center; }
		.bg-light { background-color: #EBEBEB; font-weight: bold; }
		.text-right { text-align: right; }
		.font-bold { font-weight: bold; }
	</style></head><body>`)

	// 🌟 NAMA CORPORATE DIBUAT FONT 14PT + BOLD
	html.WriteString(fmt.Sprintf(`<div class="corporate-header">%s</div>`, ptNama))
	html.WriteString(`<div>Jl. Wibawa Mukti II No. 8 Jatiasih, Bekasi</div><div>BEKASI KOTA</div><br>`)

	html.WriteString(`<table width="100%" border="0">`)
	html.WriteString(`<tr><td colspan="6" class="title">BUKU BESAR</td></tr>`)
	html.WriteString(fmt.Sprintf(`<tr><td colspan="6" class="sub-title">%s</td></tr>`, strings.ToUpper(cabangNama)))
	html.WriteString(fmt.Sprintf(`<tr><td colspan="6" class="sub-title">PERIODE %s - %s</td></tr>`, tanggalStart, tanggalEnd))
	html.WriteString(fmt.Sprintf(`<tr><td colspan="6">Tanggal Cetak : %s</td></tr>`, nowStr))
	html.WriteString(`</table><br>`)

	html.WriteString(`<table width="100%" border="1" cellspacing="0" cellpadding="3">`)
	html.WriteString(`<tr class="bg-grey">
		<td>Tanggal</td>
		<td>No. Jurnal</td>
		<td colspan="2">Keterangan</td>
		<td>Debet</td>
		<td>Kredit</td>
		<td>Saldo</td>
	</tr>`)

	for _, coa := range coaList {
		caID := fmt.Sprintf("%v", coa["ca_id"])
		caName := fmt.Sprintf("%v", coa["ca_name"])
		caJenis := fmt.Sprintf("%v", coa["ca_jenis"])

		queryDetail := `
			SELECT 
				COALESCE(d.tjurd_tjurhno, h.tjurh_no, '-') AS no_jurnal,
				CASE 
					WHEN h.tjurh_tanggal IS NULL THEN '-'
					ELSE TO_CHAR(h.tjurh_tanggal::timestamp, 'MM/DD/YYYY')
				END AS tanggal,
				COALESCE(d.tjurd_keterangan, h.tjurh_keterangan, '-') AS keterangan,
				COALESCE(d.tjurd_debet, 0) AS debet,
				COALESCE(d.tjurd_kredit, 0) AS kredit
			FROM public.gl_t_jurnald d
			LEFT JOIN public.gl_t_jurnalh h ON d.tjurd_tjurhno = h.tjurh_no
			WHERE d.tjurd_acccode = $1
		`
		if tanggalStart != "" && tanggalEnd != "" {
			queryDetail += fmt.Sprintf(" AND (h.tjurh_tanggal IS NULL OR h.tjurh_tanggal BETWEEN '%s 00:00:00' AND '%s 23:59:59')", tanggalStart, tanggalEnd)
		}
		if cabangID != "" && cabangID != "1" && cabangID != "PUSAT" {
			queryDetail += fmt.Sprintf(" AND CAST(d.tjurd_agenid AS VARCHAR) = '%s'", cabangID)
		}
		queryDetail += " ORDER BY h.tjurh_tanggal ASC, h.tjurh_no ASC"

		var details []map[string]interface{}
		_ = database.Raw(queryDetail, caID).Scan(&details)

		if piltrans == "A" && len(details) == 0 {
			continue
		}

		html.WriteString(fmt.Sprintf(`<tr>
			<td class="font-bold">%s</td>
			<td class="font-bold" colspan="2">%s</td>
			<td class="text-right font-bold">Saldo Awal :</td>
			<td class="text-right font-bold">0</td>
			<td class="text-right font-bold">0</td>
			<td></td>
		</tr>`, caID, caName))

		var totalDebet, totalKredit float64
		for _, det := range details {
			dVal, _ := det["debet"].(float64)
			kVal, _ := det["kredit"].(float64)
			totalDebet += dVal
			totalKredit += kVal

			html.WriteString(fmt.Sprintf(`<tr>
				<td>%s</td>
				<td>%s</td>
				<td colspan="2">%s</td>
				<td class="text-right">%.0f</td>
				<td class="text-right">%.0f</td>
				<td></td>
			</tr>`, det["tanggal"], det["no_jurnal"], det["keterangan"], dVal, kVal))
		}

		var saldoAkhir float64
		if caJenis == "D" || caJenis == "DEBET" {
			saldoAkhir = totalDebet - totalKredit
		} else {
			saldoAkhir = totalKredit - totalDebet
		}

		html.WriteString(fmt.Sprintf(`<tr class="bg-light">
			<td colspan="4" class="text-right font-bold">Saldo Akhir :</td>
			<td class="text-right font-bold">%.0f</td>
			<td class="text-right font-bold">%.0f</td>
			<td class="text-right font-bold">%.0f</td>
		</tr>`, totalDebet, totalKredit, saldoAkhir))
	}

	html.WriteString(`</table></body></html>`)

	filename := fmt.Sprintf("BUKU_BESAR_%s.xls", time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "application/vnd.ms-excel")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.String(http.StatusOK, html.String())
}
