package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// GenerateDocNo Standard Penomoran Resmi Dakota:
// 1 Digit PT (A/B/C) + 6 Digit Kode Agen + MM + YY + 5 Digit Urut
func GenerateDocNo(db *gorm.DB, ptID string, agenID string, tglStr string, tableName string, fieldName string) (string, error) {
	// 1. Map Kode PT (A = DBS, B = DLB, C = DLI)
	ptCode := "C" // Default DLI
	switch strings.ToUpper(strings.TrimSpace(ptID)) {
	case "1", "A", "DBS":
		ptCode = "A"
	case "2", "B", "DLB":
		ptCode = "B"
	case "3", "C", "DLI":
		ptCode = "C"
	}

	// 2. Format / Ambil Kode Agen 6 Digit dari Database
	cbID := strings.TrimSpace(agenID)
	if cbID == "" {
		cbID = "1"
	}

	// 🔒 VALIDASI PUSAT: PUSAT DAKOTA (ID 1) TIDAK BISA MEMBUAT INVOICE / TRANSAKSI
	if cbID == "1" || cbID == "001" || cbID == "000001" {
		return "", fmt.Errorf("PUSAT DAKOTA tidak diizinkan membuat transaksi/invoice secara langsung! Silakan pilih Cabang/Agen operasional.")
	}

	var agenKodeDb string
	if ptCode == "C" {
		// DLI: Ambil dari agen_kode
		db.Table("public.glb_m_agen").
			Select("COALESCE(agen_kode, '')").
			Where("CAST(agen_id AS VARCHAR) = ? OR agen_kode = ?", cbID, cbID).
			Scan(&agenKodeDb)
	} else {
		// Non-DLI (DBS/DLB): Ambil dari agen_id
		db.Table("public.glb_m_agen").
			Select("CAST(agen_id AS VARCHAR)").
			Where("CAST(agen_id AS VARCHAR) = ? OR agen_kode = ?", cbID, cbID).
			Scan(&agenKodeDb)
	}

	if agenKodeDb == "" {
		agenKodeDb = cbID
	}

	// Format Agen Kode menjadi Tepat 6 Digit (Misal: 18 -> "000018", "BDG" -> "BDG000")
	var formattedAgen string
	if _, err := strconv.Atoi(agenKodeDb); err == nil {
		// Jika Angka, pad 6 digit leading zero
		formattedAgen = fmt.Sprintf("%06s", agenKodeDb)
		if len(formattedAgen) > 6 {
			formattedAgen = formattedAgen[len(formattedAgen)-6:]
		}
	} else {
		// Jika Teks Kode, pad kanan hingga 6 char
		formattedAgen = fmt.Sprintf("%-6s", strings.ToUpper(agenKodeDb))
		formattedAgen = strings.ReplaceAll(formattedAgen, " ", "0")
		if len(formattedAgen) > 6 {
			formattedAgen = formattedAgen[:6]
		}
	}

	// 3. Format Tanggal MM & YY
	tglParsed, err := time.Parse("2006-01-02", tglStr)
	if err != nil {
		tglParsed = time.Now()
	}
	mm := tglParsed.Format("01")
	yy := tglParsed.Format("06")

	// 4. Gabungkan Prefix (1 Digit PT + 6 Digit Agen + MM + YY) -> Total 11 Char
	prefix := fmt.Sprintf("%s%s%s%s", ptCode, formattedAgen, mm, yy)

	// 5. Cari Counter Terakhir di Database
	var lastNo string
	db.Table(tableName).
		Select(fieldName).
		Where(fmt.Sprintf("%s LIKE ?", fieldName), prefix+"%").
		Order(fmt.Sprintf("%s DESC", fieldName)).
		Limit(1).
		Scan(&lastNo)

	counter := 1
	if lastNo != "" && len(lastNo) >= len(prefix)+5 {
		lastCounter, _ := strconv.Atoi(lastNo[len(lastNo)-5:])
		counter = lastCounter + 1
	}

	// Result Contoh: C000018082600001 (Panjang 16 Digit)
	finalDocNo := fmt.Sprintf("%s%05d", prefix, counter)
	return finalDocNo, nil
}
