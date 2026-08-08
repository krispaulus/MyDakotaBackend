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

// GetRugiLabaReportHandler Menghitung & Menyajikan Data Laporan Rugi / Laba
func GetRugiLabaReportHandler(c *gin.Context) {
	bulanStr := c.Query("bulan")
	tahunStr := c.Query("tahun")
	cabangID := c.Query("cabang")
	jnslap := c.Query("jnslap") // "H" = Header, "D" = Detail

	if bulanStr == "" {
		bulanStr = fmt.Sprintf("%02d", int(time.Now().Month()))
	}
	if tahunStr == "" {
		tahunStr = fmt.Sprintf("%d", time.Now().Year())
	}
	if jnslap == "" {
		jnslap = "H"
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

	query := fmt.Sprintf(`
		SELECT 
			c.ca_id,
			c.ca_name,
			UPPER(COALESCE(c.ca_jenis, 'D')) AS ca_jenis,
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

	if cabangID != "" && cabangID != "1" && cabangID != "PUSAT" && cabangID != "KONSOLIDASI" {
		query += " AND CAST(m.tmsca_agenid AS VARCHAR) = $2"
		args = append(args, cabangID)
	}

	query += " WHERE COALESCE(c.ca_aktifyn, 'Y') = 'Y' GROUP BY c.ca_id, c.ca_name, c.ca_jenis ORDER BY c.ca_id ASC"

	var rawResults []map[string]interface{}
	if err := database.Raw(query, args...).Scan(&rawResults).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	var pendapatanList []map[string]interface{}
	var bebanList []map[string]interface{}
	var totalPendapatan, totalBeban float64

	for _, row := range rawResults {
		caID := fmt.Sprintf("%v", row["ca_id"])
		caName := fmt.Sprintf("%v", row["ca_name"])
		caJenis := fmt.Sprintf("%v", row["ca_jenis"])

		// Filter Mode Header ('H'): Tampilkan hanya akun header berakhiran '0000'
		if jnslap == "H" && len(caID) >= 10 && !strings.HasSuffix(caID, "0000") && caJenis != "H" {
			continue
		}

		rawAwalD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_debet"]), 64)
		rawAwalK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_kredit"]), 64)
		mutasiD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["debet_mutasi"]), 64)
		mutasiK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["kredit_mutasi"]), 64)

		upperID := strings.ToUpper(caID)
		isParentGroupHeader := caID == "D100000000" || caID == "E100000000" || caID == "4000000000" || caID == "5000000000"

		// 🔹 KELOMPOK PENDAPATAN (PREFIX 'D', 'P', '4') -> NORMAL KREDIT
		if strings.HasPrefix(upperID, "D") || strings.HasPrefix(upperID, "P") || strings.HasPrefix(upperID, "4") {
			saldoAkhir := (rawAwalK - rawAwalD) + mutasiK - mutasiD

			if (jnslap == "H" && !isParentGroupHeader) || (jnslap == "D" && caJenis == "D") {
				totalPendapatan += saldoAkhir
			}

			pendapatanList = append(pendapatanList, map[string]interface{}{
				"ca_id":       caID,
				"ca_name":     caName,
				"saldo_akhir": saldoAkhir,
			})
		} else if strings.HasPrefix(upperID, "E") || strings.HasPrefix(upperID, "5") ||
			strings.HasPrefix(upperID, "6") || strings.HasPrefix(upperID, "7") {
			// 🔸 KELOMPOK BEBAN/BIAYA (PREFIX 'E', '5', '6', '7') -> NORMAL DEBET
			saldoAkhir := (rawAwalD - rawAwalK) + mutasiD - mutasiK

			if (jnslap == "H" && !isParentGroupHeader) || (jnslap == "D" && caJenis == "D") {
				totalBeban += saldoAkhir
			}

			bebanList = append(bebanList, map[string]interface{}{
				"ca_id":       caID,
				"ca_name":     caName,
				"saldo_akhir": saldoAkhir,
			})
		}
	}

	labaRugiBersih := totalPendapatan - totalBeban

	c.JSON(http.StatusOK, gin.H{
		"status":           "success",
		"pendapatan":       pendapatanList,
		"beban":            bebanList,
		"total_pendapatan": totalPendapatan,
		"total_beban":      totalBeban,
		"laba_rugi_bersih": labaRugiBersih,
		"jnslap":           jnslap,
	})
}

// ExportRugiLabaXlsHandler Export Laporan Rugi / Laba ke Excel (.xls)
func ExportRugiLabaXlsHandler(c *gin.Context) {
	bulanStr := c.Query("bulan")
	tahunStr := c.Query("tahun")
	cabangID := c.Query("cabang")
	jnslap := c.Query("jnslap")

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
	if cabangID != "" && cabangID != "1" && cabangID != "PUSAT" && cabangID != "KONSOLIDASI" {
		_ = database.Table("public.glb_m_agen").
			Select("agen_nama").
			Where("CAST(agen_id AS VARCHAR) = ?", cabangID).
			Scan(&cabangNama)
	}

	blInt, _ := strconv.Atoi(bulanStr)
	colDebet := fmt.Sprintf("tmsca_saldobln%02dd", blInt)
	colKredit := fmt.Sprintf("tmsca_saldobln%02dk", blInt)

	var sumDebetAwal, sumKreditAwal []string
	sumDebetAwal = append(sumDebetAwal, "COALESCE(m.tmsca_saldoawald, 0)")
	sumKreditAwal = append(sumKreditAwal, "COALESCE(m.tmsca_saldoawalk, 0)")
	for i := 1; i < blInt; i++ {
		sumDebetAwal = append(sumDebetAwal, fmt.Sprintf("COALESCE(m.tmsca_saldobln%02dd, 0)", i))
		sumKreditAwal = append(sumKreditAwal, fmt.Sprintf("COALESCE(m.tmsca_saldobln%02dk, 0)", i))
	}

	query := fmt.Sprintf(`
		SELECT 
			c.ca_id, c.ca_name, UPPER(COALESCE(c.ca_jenis, 'D')) AS ca_jenis,
			COALESCE(SUM(%s), 0) AS raw_awal_debet,
			COALESCE(SUM(%s), 0) AS raw_awal_kredit,
			COALESCE(SUM(COALESCE(m.%s, 0)), 0) AS debet_mutasi,
			COALESCE(SUM(COALESCE(m.%s, 0)), 0) AS kredit_mutasi
		FROM public.gl_m_chartaccount c
		LEFT JOIN public.gl_t_mutasisaldoca m ON c.ca_id = m.tmsca_caid AND m.tmsca_tahun = $1
	`, strings.Join(sumDebetAwal, " + "), strings.Join(sumKreditAwal, " + "), colDebet, colKredit)

	var args []interface{}
	args = append(args, tahunStr)

	if cabangID != "" && cabangID != "1" && cabangID != "PUSAT" && cabangID != "KONSOLIDASI" {
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
	html.WriteString(`<tr><td colspan="4" class="title">LAPORAN RUGI / LABA (INCOME STATEMENT)</td></tr>`)
	html.WriteString(fmt.Sprintf(`<tr><td colspan="4" class="sub-title">%s</td></tr>`, strings.ToUpper(cabangNama)))
	html.WriteString(fmt.Sprintf(`<tr><td colspan="4" class="sub-title">PERIODE BULAN %s TAHUN %s</td></tr>`, bulanStr, tahunStr))
	html.WriteString(fmt.Sprintf(`<tr><td colspan="4">Tanggal Cetak : %s</td></tr>`, nowStr))
	html.WriteString(`</table><br>`)

	html.WriteString(`<table width="100%" border="1" cellspacing="0" cellpadding="3">`)
	html.WriteString(`<tr class="bg-grey">
		<td width="20%">Kode Akun</td>
		<td width="50%">Nama Akun COA</td>
		<td width="30%" class="text-right">Saldo (Rp)</td>
	</tr>`)

	var totalPendapatan, totalBeban float64

	// PENDAPATAN
	html.WriteString(`<tr class="bg-light"><td colspan="3" class="font-bold">PENDAPATAN (REVENUE)</td></tr>`)
	for _, row := range rawResults {
		caID := fmt.Sprintf("%v", row["ca_id"])
		caName := fmt.Sprintf("%v", row["ca_name"])
		caJenis := fmt.Sprintf("%v", row["ca_jenis"])

		if jnslap == "H" && len(caID) >= 10 && !strings.HasSuffix(caID, "0000") && caJenis != "H" {
			continue
		}

		upperID := strings.ToUpper(caID)
		isParentGroupHeader := caID == "D100000000" || caID == "4000000000"

		if strings.HasPrefix(upperID, "D") || strings.HasPrefix(upperID, "P") || strings.HasPrefix(upperID, "4") {
			rawAwalD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_debet"]), 64)
			rawAwalK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_kredit"]), 64)
			mutasiD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["debet_mutasi"]), 64)
			mutasiK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["kredit_mutasi"]), 64)

			saldoAkhir := (rawAwalK - rawAwalD) + mutasiK - mutasiD

			if (jnslap == "H" && !isParentGroupHeader) || (jnslap == "D" && caJenis == "D") {
				totalPendapatan += saldoAkhir
			}

			html.WriteString(fmt.Sprintf(`<tr>
				<td>%s</td>
				<td>%s</td>
				<td class="text-right">%.0f</td>
			</tr>`, caID, caName, saldoAkhir))
		}
	}
	html.WriteString(fmt.Sprintf(`<tr class="bg-light">
		<td colspan="2" class="text-right font-bold">TOTAL PENDAPATAN :</td>
		<td class="text-right font-bold">%.0f</td>
	</tr>`, totalPendapatan))

	// BEBAN / BIAYA
	html.WriteString(`<tr class="bg-light"><td colspan="3" class="font-bold">BEBAN / BIAYA (EXPENSES)</td></tr>`)
	for _, row := range rawResults {
		caID := fmt.Sprintf("%v", row["ca_id"])
		caName := fmt.Sprintf("%v", row["ca_name"])
		caJenis := fmt.Sprintf("%v", row["ca_jenis"])

		if jnslap == "H" && len(caID) >= 10 && !strings.HasSuffix(caID, "0000") && caJenis != "H" {
			continue
		}

		upperID := strings.ToUpper(caID)
		isParentGroupHeader := caID == "E100000000" || caID == "5000000000"

		if strings.HasPrefix(upperID, "E") || strings.HasPrefix(upperID, "5") ||
			strings.HasPrefix(upperID, "6") || strings.HasPrefix(upperID, "7") {
			rawAwalD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_debet"]), 64)
			rawAwalK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_kredit"]), 64)
			mutasiD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["debet_mutasi"]), 64)
			mutasiK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["kredit_mutasi"]), 64)

			saldoAkhir := (rawAwalD - rawAwalK) + mutasiD - mutasiK

			if (jnslap == "H" && !isParentGroupHeader) || (jnslap == "D" && caJenis == "D") {
				totalBeban += saldoAkhir
			}

			html.WriteString(fmt.Sprintf(`<tr>
				<td>%s</td>
				<td>%s</td>
				<td class="text-right">%.0f</td>
			</tr>`, caID, caName, saldoAkhir))
		}
	}
	html.WriteString(fmt.Sprintf(`<tr class="bg-light">
		<td colspan="2" class="text-right font-bold">TOTAL BEBAN :</td>
		<td class="text-right font-bold">%.0f</td>
	</tr>`, totalBeban))

	// LABA / RUGI BERSIH
	labaRugi := totalPendapatan - totalBeban
	html.WriteString(fmt.Sprintf(`<tr class="bg-grey">
		<td colspan="2" class="text-right font-bold">LABA / RUGI BERSIH :</td>
		<td class="text-right font-bold">%.0f</td>
	</tr>`, labaRugi))

	html.WriteString(`</table></body></html>`)

	filename := fmt.Sprintf("RUGILABA_%s_%s.xls", bulanStr, tahunStr)
	c.Header("Content-Type", "application/vnd.ms-excel")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.String(http.StatusOK, html.String())
}
