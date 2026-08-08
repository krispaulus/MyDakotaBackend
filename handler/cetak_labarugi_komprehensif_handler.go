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

// GetLabaRugiKomprehensifHandler Menghitung Laba Rugi Komprehensif (Mode SA / N, Komparasi Tahun Lalu)
func GetLabaRugiKomprehensifHandler(c *gin.Context) {
	bulanStr := c.Query("bulan")
	tahunStr := c.Query("tahun")
	cabangID := c.Query("cabang")
	jnslap := c.Query("jnslap") // "SA" = s/d Periode, "N" = Hanya Bulan Terpilih
	isCompare := c.Query("komparasi") == "Y"

	if bulanStr == "" {
		bulanStr = fmt.Sprintf("%02d", int(time.Now().Month()))
	}
	if tahunStr == "" {
		tahunStr = fmt.Sprintf("%d", time.Now().Year())
	}
	if jnslap == "" {
		jnslap = "SA"
	}

	blInt, _ := strconv.Atoi(bulanStr)
	thInt, _ := strconv.Atoi(tahunStr)
	thLaluStr := fmt.Sprintf("%d", thInt-1)

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.GetDB()
	}

	// 📊 Perhitungan Formula Mutasi Saldo Berdasarkan Mode (SA / N)
	var formulaDebetNow, formulaKreditNow string
	var formulaDebetLalu, formulaKreditLalu string

	if jnslap == "N" {
		// HANYA BULAN TERPILIH
		colDebet := fmt.Sprintf("COALESCE(m.tmsca_saldobln%02dd, 0)", blInt)
		colKredit := fmt.Sprintf("COALESCE(m.tmsca_saldobln%02dk, 0)", blInt)
		formulaDebetNow = colDebet
		formulaKreditNow = colKredit

		colDebetLalu := fmt.Sprintf("COALESCE(ml.tmsca_saldobln%02dd, 0)", blInt)
		colKreditLalu := fmt.Sprintf("COALESCE(ml.tmsca_saldobln%02dk, 0)", blInt)
		formulaDebetLalu = colDebetLalu
		formulaKreditLalu = colKreditLalu
	} else {
		// SAMPAI DENGAN PERIODE TERPILIH (YTD: Saldo Awal + Mutasi Bln 1 s/d Bln X)
		var sumDNow, sumKNow []string
		var sumDLalu, sumKLalu []string

		sumDNow = append(sumDNow, "COALESCE(m.tmsca_saldoawald, 0)")
		sumKNow = append(sumKNow, "COALESCE(m.tmsca_saldoawalk, 0)")
		sumDLalu = append(sumDLalu, "COALESCE(ml.tmsca_saldoawald, 0)")
		sumKLalu = append(sumKLalu, "COALESCE(ml.tmsca_saldoawalk, 0)")

		for i := 1; i <= blInt; i++ {
			sumDNow = append(sumDNow, fmt.Sprintf("COALESCE(m.tmsca_saldobln%02dd, 0)", i))
			sumKNow = append(sumKNow, fmt.Sprintf("COALESCE(m.tmsca_saldobln%02dk, 0)", i))
			sumDLalu = append(sumDLalu, fmt.Sprintf("COALESCE(ml.tmsca_saldobln%02dd, 0)", i))
			sumKLalu = append(sumKLalu, fmt.Sprintf("COALESCE(ml.tmsca_saldobln%02dk, 0)", i))
		}

		formulaDebetNow = strings.Join(sumDNow, " + ")
		formulaKreditNow = strings.Join(sumKNow, " + ")
		formulaDebetLalu = strings.Join(sumDLalu, " + ")
		formulaKreditLalu = strings.Join(sumKLalu, " + ")
	}

	query := fmt.Sprintf(`
		SELECT 
			c.ca_id,
			c.ca_name,
			UPPER(COALESCE(c.ca_jenis, 'D')) AS ca_jenis,
			COALESCE(SUM(%s), 0) AS debet_now,
			COALESCE(SUM(%s), 0) AS kredit_now,
			COALESCE(SUM(%s), 0) AS debet_lalu,
			COALESCE(SUM(%s), 0) AS kredit_lalu
		FROM public.gl_m_chartaccount c
		LEFT JOIN public.gl_t_mutasisaldoca m 
			ON c.ca_id = m.tmsca_caid AND m.tmsca_tahun = $1
		LEFT JOIN public.gl_t_mutasisaldoca ml 
			ON c.ca_id = ml.tmsca_caid AND ml.tmsca_tahun = $2
	`, formulaDebetNow, formulaKreditNow, formulaDebetLalu, formulaKreditLalu)

	var args []interface{}
	args = append(args, tahunStr, thLaluStr)

	if cabangID != "" && cabangID != "1" && cabangID != "PUSAT" && cabangID != "KONSOLIDASI" {
		query += " AND (m.tmsca_agenid IS NULL OR CAST(m.tmsca_agenid AS VARCHAR) = $3)"
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
	var totalPendapatanNow, totalBebanNow float64
	var totalPendapatanLalu, totalBebanLalu float64

	for _, row := range rawResults {
		caID := fmt.Sprintf("%v", row["ca_id"])
		caName := fmt.Sprintf("%v", row["ca_name"])
		caJenis := fmt.Sprintf("%v", row["ca_jenis"])

		// Ambil akun level header saja jika berakhiran '0000'
		if len(caID) >= 10 && !strings.HasSuffix(caID, "0000") && caJenis != "H" {
			continue
		}

		debNow, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["debet_now"]), 64)
		kredNow, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["kredit_now"]), 64)
		debLalu, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["debet_lalu"]), 64)
		kredLalu, _ := strconv.ParseFloat(fmt.Sprintf("%v", row["kredit_lalu"]), 64)

		upperID := strings.ToUpper(caID)
		isParentGroupHeader := caID == "D100000000" || caID == "E100000000" || caID == "4000000000" || caID == "5000000000"

		// 🔹 PENDAPATAN (D, P, 4) -> Normal Kredit
		if strings.HasPrefix(upperID, "D") || strings.HasPrefix(upperID, "P") || strings.HasPrefix(upperID, "4") {
			saldoNow := kredNow - debNow
			saldoLalu := kredLalu - debLalu

			if !isParentGroupHeader {
				totalPendapatanNow += saldoNow
				totalPendapatanLalu += saldoLalu
			}

			pendapatanList = append(pendapatanList, map[string]interface{}{
				"ca_id":      caID,
				"ca_name":    caName,
				"saldo_now":  saldoNow,
				"saldo_lalu": saldoLalu,
			})
		} else if strings.HasPrefix(upperID, "E") || strings.HasPrefix(upperID, "5") ||
			strings.HasPrefix(upperID, "6") || strings.HasPrefix(upperID, "7") {
			// 🔸 BEBAN (E, 5, 6, 7) -> Normal Debet
			saldoNow := debNow - kredNow
			saldoLalu := debLalu - kredLalu

			if !isParentGroupHeader {
				totalBebanNow += saldoNow
				totalBebanLalu += saldoLalu
			}

			bebanList = append(bebanList, map[string]interface{}{
				"ca_id":      caID,
				"ca_name":    caName,
				"saldo_now":  saldoNow,
				"saldo_lalu": saldoLalu,
			})
		}
	}

	labaBersihNow := totalPendapatanNow - totalBebanNow
	labaBersihLalu := totalPendapatanLalu - totalBebanLalu

	c.JSON(http.StatusOK, gin.H{
		"status":                "success",
		"tahun_sekarang":        tahunStr,
		"tahun_lalu":            thLaluStr,
		"bulan":                 bulanStr,
		"jnslap":                jnslap,
		"is_komparasi":          isCompare,
		"pendapatan":            pendapatanList,
		"beban":                 bebanList,
		"total_pendapatan_now":  totalPendapatanNow,
		"total_pendapatan_lalu": totalPendapatanLalu,
		"total_beban_now":       totalBebanNow,
		"total_beban_lalu":      totalBebanLalu,
		"laba_bersih_now":       labaBersihNow,
		"laba_bersih_lalu":      labaBersihLalu,
	})
}
