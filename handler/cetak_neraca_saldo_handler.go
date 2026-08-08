package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// GetNeracaSaldoReportHandler Menghitung & Menyajikan Data Laporan Neraca Saldo
func GetNeracaSaldoReportHandler(c *gin.Context) {
	bulanStr := c.Query("bulan")
	tahunStr := c.Query("tahun")
	cabangID := c.Query("cabang")

	if bulanStr == "" {
		bulanStr = fmt.Sprintf("%02d", int(time.Now().Month()))
	}
	if tahunStr == "" {
		tahunStr = fmt.Sprintf("%d", time.Now().Year())
	}

	blInt, _ := strconv.Atoi(bulanStr)

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.GetDB()
	}

	colDebet := fmt.Sprintf("tmsca_saldobln%02dd", blInt)
	colKredit := fmt.Sprintf("tmsca_saldobln%02dk", blInt)

	var sumDebetAwal, sumKreditAwal []string
	sumDebetAwal = append(sumDebetAwal, "COALESCE(m.tmsca_saldoawald, 0)")
	sumKreditAwal = append(sumKreditAwal, "COALESCE(m.tmsca_saldoawalk, 0)")

	for i := 1; i < blInt; i++ {
		sumDebetAwal = append(sumDebetAwal, fmt.Sprintf("COALESCE(m.tmsca_saldobln%02dd, 0)", i))
		sumKreditAwal = append(sumKreditAwal, fmt.Sprintf("COALESCE(m.tmsca_saldobln%02dk, 0)", i))
	}

	formulaDebetAwal := strings.Join(sumDebetAwal, " + ")
	formulaKreditAwal := strings.Join(sumKreditAwal, " + ")

	// Query disesuaikan agar jika KONSOLIDASI (cabang == "1" atau "KONSOLIDASI"), abaikan filter agen
	query := fmt.Sprintf(`
		SELECT 
			c.ca_id,
			c.ca_name,
			COALESCE(c.ca_jenis, 'D') AS ca_jenis,
			COALESCE(SUM(%s), 0) AS raw_awal_debet,
			COALESCE(SUM(%s), 0) AS raw_awal_kredit,
			COALESCE(SUM(COALESCE(m.%s, 0)), 0) AS debet_mutasi,
			COALESCE(SUM(COALESCE(m.%s, 0)), 0) AS kredit_mutasi
		FROM public.gl_m_chartaccount c
		LEFT JOIN public.gl_t_mutasisaldoca m 
			ON c.ca_id = m.tmsca_caid AND m.tmsca_tahun = $1
	`, formulaDebetAwal, formulaKreditAwal, colDebet, colKredit)

	var args []interface{}
	args = append(args, tahunStr)

	// Filter cabang spesifik hanya jika BUKAN Konsolidasi (bukan ID "1")
	if cabangID != "" && cabangID != "1" && cabangID != "KONSOLIDASI" && cabangID != "PUSAT" {
		query += " AND CAST(m.tmsca_agenid AS VARCHAR) = $2"
		args = append(args, cabangID)
	}

	query += " WHERE COALESCE(c.ca_aktifyn, 'Y') = 'Y' GROUP BY c.ca_id, c.ca_name, c.ca_jenis ORDER BY c.ca_id ASC"

	var rawResults []map[string]interface{}
	if err := database.Raw(query, args...).Scan(&rawResults).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	var reportData []map[string]interface{}
	var grandTotalDebet, grandTotalKredit float64

	for _, row := range rawResults {
		caID := fmt.Sprintf("%v", row["ca_id"])
		caName := fmt.Sprintf("%v", row["ca_name"])
		caJenis := fmt.Sprintf("%v", row["ca_jenis"])

		rawAwalD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_debet"]), 64)
		rawAwalK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_kredit"]), 64)
		mutasiD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["debet_mutasi"]), 64)
		mutasiK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["kredit_mutasi"]), 64)

		var saldoDebet, saldoKredit float64

		if caJenis == "D" || caJenis == "DEBET" {
			saldoAwal := rawAwalD - rawAwalK
			saldoDebet = (saldoAwal + mutasiD) - mutasiK
			if saldoDebet < 0 {
				saldoKredit = -saldoDebet
				saldoDebet = 0
			}
		} else {
			saldoAwal := rawAwalK - rawAwalD
			saldoKredit = (saldoAwal + mutasiK) - mutasiD
			if saldoKredit < 0 {
				saldoDebet = -saldoKredit
				saldoKredit = 0
			}
		}

		grandTotalDebet += saldoDebet
		grandTotalKredit += saldoKredit

		reportData = append(reportData, map[string]interface{}{
			"ca_id":         caID,
			"ca_name":       caName,
			"ca_jenis":      caJenis,
			"mutasi_debet":  mutasiD,
			"mutasi_kredit": mutasiK,
			"saldo_debet":   saldoDebet,
			"saldo_kredit":  saldoKredit,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":             "success",
		"data":               reportData,
		"grand_total_debet":  grandTotalDebet,
		"grand_total_kredit": grandTotalKredit,
		"is_balanced":        grandTotalDebet == grandTotalKredit,
	})
}

// ExportNeracaSaldoXlsHandler Export Neraca Saldo ke Format File Excel (.xls)
func ExportNeracaSaldoXlsHandler(c *gin.Context) {
	bulanStr := c.Query("bulan")
	tahunStr := c.Query("tahun")
	cabangID := c.Query("cabang")
	pilcab := c.Query("pilcab")

	if bulanStr == "" {
		bulanStr = fmt.Sprintf("%02d", int(time.Now().Month()))
	}
	if tahunStr == "" {
		tahunStr = fmt.Sprintf("%d", time.Now().Year())
	}

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
	if pilcab == "Y" && cabangID != "" && cabangID != "1" && cabangID != "PUSAT" {
		_ = database.Table("public.glb_m_agen").
			Select("agen_nama").
			Where("CAST(agen_id AS VARCHAR) = ?", cabangID).
			Scan(&cabangNama)
	}

	blInt, _ := strconv.Atoi(bulanStr)
	colDebet := fmt.Sprintf("tmsca_saldobln%02dd", blInt)
	colKredit := fmt.Sprintf("tmsca_saldobln%02dk", blInt)

	var sumDebetAwal, sumKreditAwal []string
	sumDebetAwal = append(sumDebetAwal, "m.tmsca_saldoawald")
	sumKreditAwal = append(sumKreditAwal, "m.tmsca_saldoawalk")
	for i := 1; i < blInt; i++ {
		sumDebetAwal = append(sumDebetAwal, fmt.Sprintf("m.tmsca_saldobln%02dd", i))
		sumKreditAwal = append(sumKreditAwal, fmt.Sprintf("m.tmsca_saldobln%02dk", i))
	}

	query := fmt.Sprintf(`
		SELECT 
			c.ca_id, c.ca_name, COALESCE(c.ca_jenis, 'D') AS ca_jenis,
			COALESCE(SUM(%s), 0) AS raw_awal_debet,
			COALESCE(SUM(%s), 0) AS raw_awal_kredit,
			COALESCE(SUM(m.%s), 0) AS debet_mutasi,
			COALESCE(SUM(m.%s), 0) AS kredit_mutasi
		FROM public.gl_m_chartaccount c
		LEFT JOIN public.gl_t_mutasisaldoca m ON c.ca_id = m.tmsca_caid AND m.tmsca_tahun = $1
	`, strings.Join(sumDebetAwal, " + "), strings.Join(sumKreditAwal, " + "), colDebet, colKredit)

	var args []interface{}
	args = append(args, tahunStr)

	if pilcab == "Y" && cabangID != "" && cabangID != "1" && cabangID != "PUSAT" {
		query += " AND CAST(m.tmsca_agenid AS VARCHAR) = $2"
		args = append(args, cabangID)
	}
	query += " WHERE COALESCE(c.ca_aktifyn, 'Y') = 'Y' GROUP BY c.ca_id, c.ca_name, c.ca_jenis ORDER BY c.ca_id ASC"

	var rawResults []map[string]interface{}
	_ = database.Raw(query, args...).Scan(&rawResults)

	nowStr := time.Now().Format("02/01/2006 15:04:05")
	var html strings.Builder

	html.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8">`)
	html.WriteString(`<style>
		td { font-family: Calibri, Arial, sans-serif; font-size: 11px; }
		.corporate-header { font-size: 14pt; font-weight: bold; text-transform: uppercase; }
		.title { font-size: 13pt; font-weight: bold; text-align: center; }
		.sub-title { font-size: 10pt; font-weight: bold; text-align: center; }
		.bg-grey { background-color: #CFCFCF; font-weight: bold; text-align: center; }
		.bg-light { background-color: #EBEBEB; font-weight: bold; }
		.text-right { text-align: right; }
		.font-bold { font-weight: bold; }
	</style></head><body>`)

	html.WriteString(fmt.Sprintf(`<div class="corporate-header">%s</div>`, ptNama))
	html.WriteString(`<div>Jl. Wibawa Mukti II No. 8 Jatiasih, Bekasi</div><div>BEKASI KOTA</div><br>`)

	html.WriteString(`<table width="100%" border="0">`)
	html.WriteString(`<tr><td colspan="4" class="title">NERACA SALDO</td></tr>`)
	html.WriteString(fmt.Sprintf(`<tr><td colspan="4" class="sub-title">%s</td></tr>`, strings.ToUpper(cabangNama)))
	html.WriteString(fmt.Sprintf(`<tr><td colspan="4" class="sub-title">PERIODE BULAN %s TAHUN %s</td></tr>`, bulanStr, tahunStr))
	html.WriteString(fmt.Sprintf(`<tr><td colspan="4">Tanggal Cetak : %s</td></tr>`, nowStr))
	html.WriteString(`</table><br>`)

	html.WriteString(`<table width="100%" border="1" cellspacing="0" cellpadding="3">`)
	html.WriteString(`<tr class="bg-grey">
		<td>Kode Akun</td>
		<td>Nama Akun</td>
		<td>Debet</td>
		<td>Kredit</td>
	</tr>`)

	var gDebet, gKredit float64

	for _, row := range rawResults {
		caID := fmt.Sprintf("%v", row["ca_id"])
		caName := fmt.Sprintf("%v", row["ca_name"])
		caJenis := fmt.Sprintf("%v", row["ca_jenis"])

		rawAwalD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_debet"]), 64)
		rawAwalK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_kredit"]), 64)
		mutasiD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["debet_mutasi"]), 64)
		mutasiK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["kredit_mutasi"]), 64)

		var saldoDebet, saldoKredit float64

		if caJenis == "D" || caJenis == "DEBET" {
			saldoAwal := rawAwalD - rawAwalK
			saldoDebet = (saldoAwal + mutasiD) - mutasiK
			if saldoDebet < 0 {
				saldoKredit = -saldoDebet
				saldoDebet = 0
			}
		} else {
			saldoAwal := rawAwalK - rawAwalD
			saldoKredit = (saldoAwal + mutasiK) - mutasiD
			if saldoKredit < 0 {
				saldoDebet = -saldoKredit
				saldoKredit = 0
			}
		}

		gDebet += saldoDebet
		gKredit += saldoKredit

		html.WriteString(fmt.Sprintf(`<tr>
			<td class="font-bold">%s</td>
			<td>%s</td>
			<td class="text-right">%.0f</td>
			<td class="text-right">%.0f</td>
		</tr>`, caID, caName, saldoDebet, saldoKredit))
	}

	html.WriteString(fmt.Sprintf(`<tr class="bg-light">
		<td colspan="2" class="text-right font-bold">TOTAL :</td>
		<td class="text-right font-bold">%.0f</td>
		<td class="text-right font-bold">%.0f</td>
	</tr>`, gDebet, gKredit))

	html.WriteString(`</table></body></html>`)

	filename := fmt.Sprintf("NERACA_SALDO_%s_%s.xls", bulanStr, tahunStr)
	c.Header("Content-Type", "application/vnd.ms-excel")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.String(http.StatusOK, html.String())
}
