package handler

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// GetPosisiKeuanganReportHandler Menghitung & Menyajikan Data Laporan Posisi Keuangan
func GetPosisiKeuanganReportHandler(c *gin.Context) {
	bulanStr := c.Query("bulanend")
	if bulanStr == "" {
		bulanStr = c.Query("bulan")
	}
	tahunStr := c.Query("tahun")
	cabangID := c.Query("cabang")
	jnslap := c.Query("jnslap")

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

	var aktivaList []map[string]interface{}
	var pasivaList []map[string]interface{}
	var totalAktiva, totalPasiva float64
	var totalPendapatan, totalBeban float64

	for _, row := range rawResults {
		caID := fmt.Sprintf("%v", row["ca_id"])
		caName := fmt.Sprintf("%v", row["ca_name"])
		caJenis := fmt.Sprintf("%v", row["ca_jenis"])

		// Filter Mode Header ('H')
		if jnslap == "H" && len(caID) >= 10 && !strings.HasSuffix(caID, "0000") && caJenis != "H" {
			continue
		}

		rawAwalD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_debet"]), 64)
		rawAwalK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_kredit"]), 64)
		mutasiD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["debet_mutasi"]), 64)
		mutasiK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["kredit_mutasi"]), 64)

		upperID := strings.ToUpper(caID)

		// Hitung Laba/Rugi berjalan dari kelompok Pendapatan & Beban
		if strings.HasPrefix(upperID, "D") || strings.HasPrefix(upperID, "P") || strings.HasPrefix(upperID, "4") {
			if caID != "D100000000" && caID != "4000000000" {
				totalPendapatan += (rawAwalK - rawAwalD) + mutasiK - mutasiD
			}
		} else if strings.HasPrefix(upperID, "E") || strings.HasPrefix(upperID, "5") || strings.HasPrefix(upperID, "6") || strings.HasPrefix(upperID, "7") {
			if caID != "E100000000" && caID != "5000000000" {
				totalBeban += (rawAwalD - rawAwalK) + mutasiD - mutasiK
			}
		}

		// 🔹 KELOMPOK AKTIVA (HITUNG SEMUA AKUN YANG TAMPIL DI TABEL MODE HEADER)
		if strings.HasPrefix(upperID, "A") || strings.HasPrefix(caID, "1") || strings.HasPrefix(caID, "2") {
			saldoAkhir := (rawAwalD - rawAwalK) + mutasiD - mutasiK

			// Di Aktiva, HANYA skip A100000000 atau A200000000 jika ada, A102000000 (Piutang) TETAP DIHITUNG!
			if caID != "A100000000" && caID != "A200000000" {
				totalAktiva += saldoAkhir
			}

			aktivaList = append(aktivaList, map[string]interface{}{
				"ca_id":       caID,
				"ca_name":     caName,
				"saldo_akhir": saldoAkhir,
			})
		} else if strings.HasPrefix(upperID, "B") || strings.HasPrefix(upperID, "C") ||
			strings.HasPrefix(caID, "3") || strings.HasPrefix(caID, "K") {
			// 🔸 KELOMPOK PASIVA
			saldoAkhir := (rawAwalK - rawAwalD) + mutasiK - mutasiD

			// Di Pasiva, ABAIKAN HANYA Induk Header Utama B100000000 Kewajiban & C100000000 Ekuitas agar tidak double!
			isPasivaParentHeader := caID == "B100000000" || caID == "C100000000"
			if !isPasivaParentHeader {
				totalPasiva += saldoAkhir
			}

			pasivaList = append(pasivaList, map[string]interface{}{
				"ca_id":       caID,
				"ca_name":     caName,
				"saldo_akhir": saldoAkhir,
			})
		}
	}

	// 🎯 LABA BERJALAN DITAMBAHKAN KE PASIVA
	labaRugiBerjalan := totalPendapatan - totalBeban
	totalPasiva += labaRugiBerjalan

	pasivaList = append(pasivaList, map[string]interface{}{
		"ca_id":       "C199000000",
		"ca_name":     "LABA (RUGI) TAHUN BERJALAN",
		"saldo_akhir": labaRugiBerjalan,
	})

	diff := math.Abs(totalAktiva - totalPasiva)
	isBalanced := diff < 100.0 // Toleransi desimal pembulatan

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"aktiva":       aktivaList,
		"pasiva":       pasivaList,
		"total_aktiva": totalAktiva,
		"total_pasiva": totalPasiva,
		"is_balanced":  isBalanced,
		"jnslap":       jnslap,
	})
}

// ExportPosisiKeuanganXlsHandler Export Laporan Posisi Keuangan ke Excel (.xls)
func ExportPosisiKeuanganXlsHandler(c *gin.Context) {
	bulanStr := c.Query("bulanend")
	if bulanStr == "" {
		bulanStr = c.Query("bulan")
	}
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
	html.WriteString(`<tr><td colspan="4" class="title">LAPORAN POSISI KEUANGAN</td></tr>`)
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

	var totalAktiva, totalPasiva float64
	var totalPendapatan, totalBeban float64

	// LOOP AKTIVA
	html.WriteString(`<tr class="bg-light"><td colspan="3" class="font-bold">AKTIVA (ASSETS)</td></tr>`)
	for _, row := range rawResults {
		caID := fmt.Sprintf("%v", row["ca_id"])
		caName := fmt.Sprintf("%v", row["ca_name"])
		caJenis := fmt.Sprintf("%v", row["ca_jenis"])

		if jnslap == "H" && len(caID) >= 10 && !stringsHasSuffix(caID, "0000") && caJenis != "H" {
			continue
		}

		upperID := strings.ToUpper(caID)
		rawAwalD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_debet"]), 64)
		rawAwalK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_kredit"]), 64)
		mutasiD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["debet_mutasi"]), 64)
		mutasiK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["kredit_mutasi"]), 64)

		if strings.HasPrefix(upperID, "D") || strings.HasPrefix(upperID, "P") || strings.HasPrefix(upperID, "4") {
			if caID != "D100000000" && caID != "4000000000" {
				totalPendapatan += (rawAwalK - rawAwalD) + mutasiK - mutasiD
			}
		} else if strings.HasPrefix(upperID, "E") || strings.HasPrefix(upperID, "5") || strings.HasPrefix(upperID, "6") || strings.HasPrefix(upperID, "7") {
			if caID != "E100000000" && caID != "5000000000" {
				totalBeban += (rawAwalD - rawAwalK) + mutasiD - mutasiK
			}
		}

		if strings.HasPrefix(upperID, "A") || strings.HasPrefix(caID, "1") || strings.HasPrefix(caID, "2") {
			saldoAkhir := (rawAwalD - rawAwalK) + mutasiD - mutasiK

			if caID != "A100000000" && caID != "A200000000" {
				totalAktiva += saldoAkhir
			}

			html.WriteString(fmt.Sprintf(`<tr>
				<td>%s</td>
				<td>%s</td>
				<td class="text-right">%.0f</td>
			</tr>`, caID, caName, saldoAkhir))
		}
	}
	html.WriteString(fmt.Sprintf(`<tr class="bg-light">
		<td colspan="2" class="text-right font-bold">TOTAL AKTIVA :</td>
		<td class="text-right font-bold">%.0f</td>
	</tr>`, totalAktiva))

	// LOOP PASIVA
	html.WriteString(`<tr class="bg-light"><td colspan="3" class="font-bold">PASIVA (KEWAJIBAN & EKUITAS)</td></tr>`)
	for _, row := range rawResults {
		caID := fmt.Sprintf("%v", row["ca_id"])
		caName := fmt.Sprintf("%v", row["ca_name"])
		caJenis := fmt.Sprintf("%v", row["ca_jenis"])

		if jnslap == "H" && len(caID) >= 10 && !stringsHasSuffix(caID, "0000") && caJenis != "H" {
			continue
		}

		upperID := strings.ToUpper(caID)

		if strings.HasPrefix(upperID, "B") || strings.HasPrefix(upperID, "C") ||
			strings.HasPrefix(caID, "3") || strings.HasPrefix(caID, "K") {
			rawAwalD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_debet"]), 64)
			rawAwalK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["raw_awal_kredit"]), 64)
			mutasiD, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["debet_mutasi"]), 64)
			mutasiK, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["kredit_mutasi"]), 64)

			saldoAkhir := (rawAwalK - rawAwalD) + mutasiK - mutasiD

			isPasivaParentHeader := caID == "B100000000" || caID == "C100000000"
			if !isPasivaParentHeader {
				totalPasiva += saldoAkhir
			}

			html.WriteString(fmt.Sprintf(`<tr>
				<td>%s</td>
				<td>%s</td>
				<td class="text-right">%.0f</td>
			</tr>`, caID, caName, saldoAkhir))
		}
	}

	labaBerjalan := totalPendapatan - totalBeban
	totalPasiva += labaBerjalan

	html.WriteString(fmt.Sprintf(`<tr>
		<td>C199000000</td>
		<td>LABA (RUGI) TAHUN BERJALAN</td>
		<td class="text-right">%.0f</td>
	</tr>`, labaBerjalan))

	html.WriteString(fmt.Sprintf(`<tr class="bg-light">
		<td colspan="2" class="text-right font-bold">TOTAL PASIVA :</td>
		<td class="text-right font-bold">%.0f</td>
	</tr>`, totalPasiva))

	html.WriteString(`</table></body></html>`)

	filename := fmt.Sprintf("POSISI_KEUANGAN_%s_%s.xls", bulanStr, tahunStr)
	c.Header("Content-Type", "application/vnd.ms-excel")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.String(http.StatusOK, html.String())
}

func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
