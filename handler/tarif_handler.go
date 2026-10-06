package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// 1. Struct representasi tabel mkt_m_eharga
type MasterEHarga struct {
	ID                 int64   `gorm:"column:id;primaryKey" json:"id"`
	AgenIDAsal         string  `gorm:"column:agenid_asal" json:"agenid_asal"`
	ServID             string  `gorm:"column:servid" json:"servid"`
	TujuanKecamatan    string  `gorm:"column:tujuan_kecamatan" json:"tujuan_kecamatan"`
	TujuanKabupaten    string  `gorm:"column:tujuan_kabupaten" json:"tujuan_kabupaten"`
	TujuanPropinsi     string  `gorm:"column:tujuan_propinsi" json:"tujuan_propinsi"`
	HargaPokok         float64 `gorm:"column:hargapokok" json:"hargapokok"`
	MinimalKg          float64 `gorm:"column:minimalkg" json:"minimalkg"`
	HargaKgSelanjutnya float64 `gorm:"column:hargakgselanjutnya" json:"hargakgselanjutnya"`
	EstimasiHari       string  `gorm:"column:estimasihari" json:"estimasihari"`
	BiayaTambahan      float64 `gorm:"column:biayatambahan" json:"biayatambahan"`
	FlagDS             string  `gorm:"column:flag_ds" json:"flag_ds"`
	Bypass1Kg          float64 `gorm:"column:bypass1kg" json:"bypass1kg"`
	Harga1Kg           float64 `gorm:"column:harga1kg" json:"harga1kg"`
	Bypass2Kg          float64 `gorm:"column:bypass2kg" json:"bypass2kg"`
	Harga2Kg           float64 `gorm:"column:harga2kg" json:"harga2kg"`
	Bypass3Kg          float64 `gorm:"column:bypass3kg" json:"bypass3kg"`
	Harga3Kg           float64 `gorm:"column:harga3kg" json:"harga3kg"`
	Keterangan         string  `gorm:"column:keterangan" json:"keterangan"`
}

type TarifRequest struct {
	AgenID          string  `json:"agen_id"`
	AsalKota        string  `json:"asal_kota"`
	CustID          string  `json:"cust_id"` // Tambahkan field ini
	TujuanKec       string  `json:"tujuan_kec"`
	TujuanKecamatan string  `json:"tujuan_kecamatan"` // Tambahkan field ini
	BeratAsli       float64 `json:"berat_asli"`
	Panjang         float64 `json:"panjang"`
	Lebar           float64 `json:"lebar"`
	Tinggi          float64 `json:"tinggi"`
	JenisLayanan    string  `json:"jenis_layanan"`
}

type CalculateTarifPayload struct {
	AgenID          string  `json:"agen_id"`
	AsalKota        string  `json:"asal_kota"`
	CustID          string  `json:"cust_id"`
	AsalCustID      string  `json:"bttt_asalcustid"`
	TujuanKec       string  `json:"tujuan_kec"`
	TujuanKecamatan string  `json:"tujuan_kecamatan"`
	BeratAsli       float64 `json:"berat_asli"`
	Panjang         float64 `json:"panjang"`
	Lebar           float64 `json:"lebar"`
	Tinggi          float64 `json:"tinggi"`
	JenisLayanan    string  `json:"jenis_layanan"`
}

func (MasterEHarga) TableName() string {
	return "public.mkt_m_eharga"
}

// 1. Untuk Tarif Reguler
func GetTarifReguler(c *gin.Context) {
	var data []models.TarifReguler
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))

	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database tidak ditemukan"})
		return
	}

	// 1. Ambil Parameter dari URL (Contoh: ?asal=BEKASI&tujuan=BANYUMANIK)
	asal := c.Query("asal")
	tujuan := c.Query("tujuan")

	// 2. Build Query secara Dinamis
	query := database.Model(&models.TarifReguler{})

	if asal != "" {
		// Pake ILIKE biar search-nya gak sensitif huruf besar/kecil (Postgres Only)
		query = query.Where("asal_kota ILIKE ?", "%"+asal+"%")
	}

	if tujuan != "" {
		query = query.Where("tujuan_kecamatan ILIKE ?", "%"+tujuan+"%")
	}

	// 3. Eksekusi dengan Limit & Offset (Pagination Dasar)
	// Kita batasi 50 data per tarikan biar enteng
	if err := query.Limit(50).Order("id asc").Find(&data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// 2. Untuk Tarif Ekonomis
func GetTarifEkonomis(c *gin.Context) {
	var data []models.TarifEkonomis
	ptID, _ := c.Get("pt_id")

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))

	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database tidak ditemukan"})
		return
	}

	if err := database.Limit(100).Order("id asc").Find(&data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// 3. Untuk Tarif Unit
func GetTarifUnit(c *gin.Context) {
	var data []models.TarifUnit
	ptID, _ := c.Get("pt_id")

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database tidak ditemukan"})
		return
	}

	if err := database.Limit(100).Order("jenis asc").Find(&data).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal query: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func CalculateTarifHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PT ID tidak ditemukan di token"})
		return
	}

	var req CalculateTarifPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload input tidak valid: " + err.Error()})
		return
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	// =========================================================================
	// 🎯 1. RESOLVE INFO AGEN ASAL
	// =========================================================================
	inputAgenRaw := strings.TrimSpace(req.AgenID)
	if inputAgenRaw == "" {
		inputAgenRaw = strings.TrimSpace(req.AsalKota)
	}

	upperInput := strings.ToUpper(inputAgenRaw)
	isPusat := upperInput == "PUSAT DAKOTA" || strings.Contains(upperInput, "PUSAT") || strings.Contains(upperInput, "HOLDING") || inputAgenRaw == "1"

	var dbAgen models.GlbMAgen
	queryAgen := database.Table("public.glb_m_agen")
	if isPusat {
		queryAgen.Where("TRIM(agen_id) = '839' OR TRIM(agen_id) = '1' OR agen_nama ILIKE '%PUSAT%'").
			Order("agen_id ASC").
			Limit(1).
			Find(&dbAgen)
	} else {
		errFind := queryAgen.Where("TRIM(agen_id) = ? OR TRIM(agen_kode) = ?", inputAgenRaw, inputAgenRaw).
			Limit(1).
			Find(&dbAgen).Error

		if errFind != nil || dbAgen.AgenID == "" {
			database.Table("public.glb_m_agen").
				Where("agen_nama ILIKE ?", "%"+inputAgenRaw+"%").
				Limit(1).
				Find(&dbAgen)
		}
	}

	var rawCabangID *string
	if dbAgen.AgenID != "" {
		database.Table("public.glb_m_agen").
			Select("agen_cabangid").
			Where("TRIM(agen_id) = ?", dbAgen.AgenID).
			Scan(&rawCabangID)
	}

	cabangIDStr := ""
	if rawCabangID != nil {
		cabangIDStr = *rawCabangID
	}

	// =========================================================================
	// 🎯 2. HITUNG BERAT CHARGEABLE
	// =========================================================================
	var beratVolume float64 = 0
	if req.Panjang > 0 && req.Lebar > 0 && req.Tinggi > 0 {
		beratVolume = (req.Panjang * req.Lebar * req.Tinggi) / 4000.0
	}

	beratChargeable := req.BeratAsli
	if beratVolume > req.BeratAsli {
		beratChargeable = beratVolume
	}
	if beratChargeable <= 0 {
		beratChargeable = 1
	}

	// Sanitasi Tujuan Kecamatan (hapus - KOTA / - KAB)
	tujuanClean := strings.ToUpper(strings.TrimSpace(req.TujuanKec))
	if tujuanClean == "" {
		tujuanClean = strings.ToUpper(strings.TrimSpace(req.TujuanKecamatan))
	}
	if strings.Contains(tujuanClean, " - ") {
		tujuanClean = strings.TrimSpace(strings.Split(tujuanClean, " - ")[0])
	}

	var regMap map[string]interface{}
	var ekoMap map[string]interface{}
	isCustomerRate := false

	// =========================================================================
	// 🌟 PRIORITAS 1: Cek Tarif Khusus Pelanggan Korporat (Contoh: PT MERCK Tbk)
	// =========================================================================
	cleanCustID := strings.TrimSpace(req.CustID)
	if cleanCustID == "" {
		cleanCustID = strings.TrimSpace(req.AsalCustID)
	}

	log.Printf("👉 [CEK TARIF] CustID: '%s' | TujuanKec: '%s'", cleanCustID, tujuanClean)

	if cleanCustID != "" && strings.ToUpper(cleanCustID) != "UMUM" {
		type CustRateResult struct {
			MinimalKg          float64 `gorm:"column:minimalkg"`
			HargaPokok         float64 `gorm:"column:hargapokok"`
			HargaKgSelanjutnya float64 `gorm:"column:hargakgselanjutnya"`
			EstimasiHari       string  `gorm:"column:estimasihari"`
			BiayaTambahan      float64 `gorm:"column:biayatambahan"`
			Keterangan         string  `gorm:"column:keterangan"`
		}

		var cr CustRateResult
		// Gunakan ILIKE %...% agar tidak gagal karena spasi tersembunyi
		queryCust := `
			SELECT 
				COALESCE(min_charge, 20.0) AS minimalkg,
				(COALESCE(min_charge, 20.0) * COALESCE(tarif_kg, 0)) AS hargapokok,
				COALESCE(tarif_kg, 0) AS hargakgselanjutnya,
				COALESCE(lead_time, '5') AS estimasihari,
				0.0 AS biayatambahan,
				COALESCE(keterangan, 'TARIF KONTRAK') AS keterangan
			FROM public.mkt_m_eharga_customer
			WHERE TRIM(cust_id) = TRIM(?) 
			  AND UPPER(TRIM(tujuan_kecamatan)) ILIKE ?
			ORDER BY id ASC
			LIMIT 1
		`

		errCust := database.Raw(queryCust, cleanCustID, "%"+tujuanClean+"%").Scan(&cr).Error
		if errCust == nil && cr.HargaKgSelanjutnya > 0 {
			isCustomerRate = true
			regMap = map[string]interface{}{
				"minimalkg":          cr.MinimalKg,
				"hargapokok":         cr.HargaPokok,
				"hargakgselanjutnya": cr.HargaKgSelanjutnya,
				"estimasihari":       cr.EstimasiHari,
				"biayatambahan":      cr.BiayaTambahan,
				"keterangan":         cr.Keterangan,
			}
			log.Printf("   ✅ TARIF KONTRAK MERCK DITEMUKAN: MinKg=%.0f, Dasar=Rp %.0f, PerKg=Rp %.0f, LT=%s",
				cr.MinimalKg, cr.HargaPokok, cr.HargaKgSelanjutnya, cr.EstimasiHari)
		} else {
			log.Printf("   ⚠️ Tidak cocok di mkt_m_eharga_customer (Error: %v), fallback ke reguler...", errCust)
		}
	}

	// =========================================================================
	// 🌟 PRIORITAS 2 (FALLBACK): Ambil Tarif Reguler & Ekonomis Umum
	// =========================================================================
	if !isCustomerRate {
		asalCandidates := []string{}
		if dbAgen.AgenID != "" {
			asalCandidates = append(asalCandidates, strings.TrimSpace(dbAgen.AgenID))
		}
		if dbAgen.AgenKode != "" {
			asalCandidates = append(asalCandidates, strings.TrimSpace(dbAgen.AgenKode))
		}
		if dbAgen.AgenKotaID != "" {
			asalCandidates = append(asalCandidates, strings.TrimSpace(dbAgen.AgenKotaID))
		}
		if inputAgenRaw != "" && !isPusat {
			asalCandidates = append(asalCandidates, strings.TrimSpace(inputAgenRaw))
		}
		if isPusat || len(asalCandidates) == 0 {
			asalCandidates = append(asalCandidates, "839", "CGK0100", "001", "PST")
		}

		var regList []map[string]interface{}
		var ekoList []map[string]interface{}

		for _, asal := range asalCandidates {
			if asal == "" {
				continue
			}
			if regMap == nil {
				database.Table("public.mkt_m_eharga").
					Where("(TRIM(agenid_asal) = ? OR agenid_asal ILIKE ?) AND UPPER(TRIM(tujuan_kecamatan)) LIKE ? AND (UPPER(TRIM(servid)) = 'REGULER' OR TRIM(servid) = '1' OR UPPER(TRIM(servid)) = 'R')",
						asal, "%"+asal+"%", "%"+tujuanClean+"%").
					Limit(1).
					Find(&regList)
				if len(regList) > 0 {
					regMap = regList[0]
				}
			}

			if ekoMap == nil {
				database.Table("public.mkt_m_eharga").
					Where("(TRIM(agenid_asal) = ? OR agenid_asal ILIKE ?) AND UPPER(TRIM(tujuan_kecamatan)) LIKE ? AND (UPPER(TRIM(servid)) = 'EKONOMIS' OR TRIM(servid) = '2' OR UPPER(TRIM(servid)) = 'E')",
						asal, "%"+asal+"%", "%"+tujuanClean+"%").
					Limit(1).
					Find(&ekoList)
				if len(ekoList) > 0 {
					ekoMap = ekoList[0]
				}
			}

			if regMap != nil && ekoMap != nil {
				break
			}
		}
	}

	// =========================================================================
	// 🎯 3. HITUNG BARIS LAYANAN (DENGAN RANGELAYANANROW)
	// =========================================================================
	custDiscount := make(map[string]interface{})
	buildLayananRow := rangeLayananRow(regMap, custDiscount, beratChargeable, "REGULER")
	buildEkoRow := rangeLayananRow(ekoMap, custDiscount, beratChargeable, "EKONOMIS")

	// =========================================================================
	// 🎯 4. PENENTUAN GRAND TOTAL
	// =========================================================================
	var finalGrandTotal float64 = 0
	statusHitung := "TARIF TUNAI UMUM"
	if isCustomerRate {
		statusHitung = "TARIF KONTRAK PELANGGAN"
	}

	isEkonomis := strings.ToUpper(req.JenisLayanan) == "EKONOMIS" || req.JenisLayanan == "N"

	if strings.ToUpper(req.JenisLayanan) == "KREDIT" {
		statusHitung = "TARIF KREDIT ACCOUNT"
		if isEkonomis && buildEkoRow != nil {
			if val, ok := buildEkoRow["total_charge"].(float64); ok {
				finalGrandTotal = val
			}
		} else if buildLayananRow != nil {
			if val, ok := buildLayananRow["total_charge"].(float64); ok {
				finalGrandTotal = val
			}
		}
	} else {
		if isEkonomis && buildEkoRow != nil {
			if val, ok := buildEkoRow["total_normal"].(float64); ok {
				finalGrandTotal = val
			}
		} else if buildLayananRow != nil {
			if val, ok := buildLayananRow["total_normal"].(float64); ok {
				finalGrandTotal = val
			}
		}
	}

	// =========================================================================
	// 🚀 5. RESPONSE AKHIR KE REACT
	// =========================================================================
	c.JSON(http.StatusOK, gin.H{
		"status":           "success",
		"status_hitung":    statusHitung,
		"berat_asli":       req.BeratAsli,
		"berat_volume":     beratVolume,
		"berat_chargeable": beratChargeable,
		"grand_total":      finalGrandTotal,
		"reguler_row":      buildLayananRow,
		"ekonomis_row":     buildEkoRow,
		"is_customer_rate": isCustomerRate,
		"kode_kota_asal":   strings.ToUpper(strings.TrimSpace(dbAgen.AgenKotaID)),
		"nomor_urut_agen":  extractThreeDigits(cabangIDStr),
	})
}

func extractThreeDigits(rawCabangID string) string {
	clean := strings.TrimSpace(rawCabangID)
	if len(clean) >= 3 {
		// Ambil 3 karakter terakhir (Contoh: "PST001" -> "001", "SUB002" -> "002")
		return clean[len(clean)-3:]
	}
	return "001" // Fallback aman jika data master belum diisi lengkap oleh tim IT pusat
}

func rangeLayananRow(tarifRow map[string]interface{}, discRow map[string]interface{}, berat float64, jenis string) map[string]interface{} {
	if tarifRow == nil {
		return map[string]interface{}{
			"servid":             jenis,
			"lt":                 "-",
			"dasar":              0.0,
			"kg_min":             0.0,
			"kg_next":            0.0,
			"ambil_sdr":          "N",
			"diskon_1_kg":        0.0,
			"diskon_1_rp":        0.0,
			"diskon_2_kg":        0.0,
			"diskon_2_rp":        0.0,
			"diskon_3_kg":        0.0,
			"diskon_3_rp":        0.0,
			"ket":                "---",
			"hargapokok":         0.0,
			"minimalkg":          0.0,
			"hargakgselanjutnya": 0.0,
			"bypass1kg":          0.0,
			"harga1kg":           0.0,
			"bypass2kg":          0.0,
			"harga2kg":           0.0,
			"bypass3kg":          0.0,
			"harga3kg":           0.0,
			"keterangan":         "---",
			"biayatambahan":      0.0,
			"has_discount":       "N",
			"total_normal":       0.0,
			"total_charge":       0.0,
			"estimasihari":       "-",
		}
	}

	// 1. Parsing data dasar
	hargaPokok := safeParseFloat(tarifRow["hargapokok"])
	if hargaPokok == 0.0 {
		hargaPokok = safeParseFloat(tarifRow["harga_pokok"])
	}

	minKG := safeParseFloat(tarifRow["minimalkg"])
	if minKG == 0.0 {
		minKG = safeParseFloat(tarifRow["minimal_kg"])
	}
	if minKG <= 0 {
		minKG = 1
	}

	hargaNext := safeParseFloat(tarifRow["hargakgselanjutnya"])
	if hargaNext == 0.0 {
		hargaNext = safeParseFloat(tarifRow["harga_kg_selanjutnya"])
	}

	bp1 := safeParseFloat(tarifRow["bypass1kg"])
	hrg1 := safeParseFloat(tarifRow["harga1kg"])
	bp2 := safeParseFloat(tarifRow["bypass2kg"])
	hrg2 := safeParseFloat(tarifRow["harga2kg"])
	bp3 := safeParseFloat(tarifRow["bypass3kg"])
	hrg3 := safeParseFloat(tarifRow["harga3kg"])
	biayaPenerus := safeParseFloat(tarifRow["biayatambahan"])

	lt := fmt.Sprintf("%v", tarifRow["estimasihari"])
	if lt == "<nil>" || strings.TrimSpace(lt) == "" {
		lt = fmt.Sprintf("%v", tarifRow["estimasi_hari"])
	}
	if lt == "<nil>" || strings.TrimSpace(lt) == "" {
		lt = "-"
	}

	ket := fmt.Sprintf("%v", tarifRow["keterangan"])
	if ket == "<nil>" || strings.TrimSpace(ket) == "" {
		ket = "---"
	}

	// 2. Kalkulasi tarif normal
	beratFinal := berat
	if beratFinal < minKG {
		beratFinal = minKG
	}

	totalNormal := 0.0
	if berat <= minKG {
		totalNormal = hargaPokok + biayaPenerus
	} else {
		totalNormal = hargaPokok + ((beratFinal - minKG) * hargaNext) + biayaPenerus
	}

	// 3. Kalkulasi diskon jika ada contract
	var totalCharge float64
	hasDiscount := "N"

	if len(discRow) > 0 {
		hasDiscount = "Y"
		discPokok := safeParseFloat(discRow["DiscountPokok"])
		discLvl1 := safeParseFloat(discRow["DiscountLevel1"])
		discLvl2 := safeParseFloat(discRow["DiscountLevel2"])
		discLvl3 := safeParseFloat(discRow["DiscountLevel3"])

		if val := safeParseFloat(discRow["bypass1kg"]); val > 0 {
			bp1 = val
		}
		if val := safeParseFloat(discRow["bypass2kg"]); val > 0 {
			bp2 = val
		}
		if val := safeParseFloat(discRow["bypass3kg"]); val > 0 {
			bp3 = val
		}
		if val := safeParseFloat(discRow["biayatambahan"]); val > 0 {
			biayaPenerus += val
		}

		hargaPokok = hargaPokok * (1 - discPokok/100)
		hargaNext = hargaNext * (1 - discPokok/100)
		hrg1 = hrg1 * (1 - discLvl1/100)
		hrg2 = hrg2 * (1 - discLvl2/100)
		hrg3 = hrg3 * (1 - discLvl3/100)

		if bp3 > 0 && berat >= bp3 {
			totalCharge = (berat * hrg3) + biayaPenerus
		} else if bp2 > 0 && berat >= bp2 {
			totalCharge = (berat * hrg2) + biayaPenerus
		} else if bp1 > 0 && berat >= bp1 {
			totalCharge = (berat * hrg1) + biayaPenerus
		} else if berat >= minKG {
			totalCharge = (berat * hargaNext) + biayaPenerus
		} else {
			totalCharge = hargaPokok + biayaPenerus
		}
	} else {
		totalCharge = totalNormal
	}

	// 4. Return map payload
	return map[string]interface{}{
		"servid":             jenis,
		"lt":                 lt,
		"dasar":              hargaPokok,
		"kg_min":             minKG,
		"kg_next":            hargaNext,
		"ambil_sdr":          "N",
		"diskon_1_kg":        bp1,
		"diskon_1_rp":        hrg1,
		"diskon_2_kg":        bp2,
		"diskon_2_rp":        hrg2,
		"diskon_3_kg":        bp3,
		"diskon_3_rp":        hrg3,
		"ket":                ket,
		"hargapokok":         hargaPokok,
		"minimalkg":          minKG,
		"hargakgselanjutnya": hargaNext,
		"bypass1kg":          bp1,
		"harga1kg":           hrg1,
		"bypass2kg":          bp2,
		"harga2kg":           hrg2,
		"bypass3kg":          bp3,
		"harga3kg":           hrg3,
		"keterangan":         ket,
		"biayatambahan":      biayaPenerus,
		"has_discount":       hasDiscount,
		"total_normal":       totalNormal,
		"total_charge":       totalCharge,
		"estimasihari":       lt,
	}
}

// 🛠️ Helper Universal untuk Parsing Segala Tipe Data Angka dari DB
func safeParseFloat(v interface{}) float64 {
	if v == nil {
		return 0.0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case int:
		return float64(val)
	case string:
		clean := strings.TrimSpace(val)
		if f, err := strconv.ParseFloat(clean, 64); err == nil {
			return f
		}
	case []uint8:
		clean := strings.TrimSpace(string(val))
		if f, err := strconv.ParseFloat(clean, 64); err == nil {
			return f
		}
	}
	return 0.0
}
