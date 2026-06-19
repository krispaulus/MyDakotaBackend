package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

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
	// 1. Proteksi token JWT
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PT ID tidak ditemukan di token"})
		return
	}

	// 2. Bind JSON body dari React Form
	var req models.TarifRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload input tidak valid: " + err.Error()})
		return
	}

	// 3. Resolve database dinamis sesuai tenant PT
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	// =========================================================================
	// 🧠 3-STAGE RELATIONAL LOOKUP (RESOLVE KOTA ASAL DARI KODE AGEN)
	// =========================================================================
	var dbAgen models.GlbMAgen
	inputAsalRaw := strings.TrimSpace(req.AsalKota)

	errAgen := database.Table("public.glb_m_agen").
		Select("agen_kotaid").
		Where("TRIM(agen_kode) = TRIM(?)", inputAsalRaw).
		First(&dbAgen).Error

	var rawCabangID string
	database.Table("public.glb_m_agen").
		Select("agen_cabangid").
		Where("TRIM(agen_kode) = TRIM(?)", inputAsalRaw).
		Scan(&rawCabangID) // Langsung disadap ke string polosan, kebal dari missing field struct models!

	var namaKotaLengkap string
	if errAgen == nil && dbAgen.AgenKotaID != "" {
		database.Table("public.glb_m_kota").
			Select("kota_nama").
			Where("TRIM(kota_id) = TRIM(?)", dbAgen.AgenKotaID).
			Scan(&namaKotaLengkap)
	}

	var kotaAsalMaster string
	if namaKotaLengkap != "" {
		cleanCityName := strings.ToUpper(strings.TrimSpace(namaKotaLengkap))
		cleanCityName = strings.ReplaceAll(cleanCityName, " KOTA", "")
		cleanCityName = strings.ReplaceAll(cleanCityName, " KABUPATEN", "")
		cleanCityName = strings.ReplaceAll(cleanCityName, " KAB.", "")
		kotaAsalMaster = strings.TrimSpace(cleanCityName)
	}

	if kotaAsalMaster != "" {
		req.AsalKota = kotaAsalMaster
	} else {
		req.AsalKota = strings.ToUpper(inputAsalRaw)
	}

	// =========================================================================
	// 📐 HITUNG BERAT CHARGEABLE (VOLUME VS AKTUAL)
	// =========================================================================
	var beratVolume float64 = 0
	if req.Panjang > 0 && req.Lebar > 0 && req.Tinggi > 0 {
		beratVolume = (req.Panjang * req.Lebar * req.Tinggi) / 4000.0
	}

	beratChargeable := req.BeratAsli
	if beratVolume > req.BeratAsli {
		beratChargeable = beratVolume
	}

	targetTable := "public.mkt_m_harga"
	if fmt.Sprintf("%v", req.JenisLayanan) == "1" || strings.ToUpper(req.JenisLayanan) == "EKONOMIS" {
		targetTable = "public.mkt_m_hargaekonomis"
	}

	tujuanClean := strings.ToUpper(strings.TrimSpace(req.TujuanKec)) // Isinya utuh: "BOGOR BARAT - KOTA"
	asalClean := strings.ToUpper(strings.TrimSpace(req.AsalKota))

	// =========================================================================
	// 🟣 QUERY DATA DASAR DARAT REGULER (MKT_M_HARGA)
	// =========================================================================
	var regMap map[string]interface{}
	var regList []map[string]interface{}

	database.Table("public.mkt_m_harga").
		Where("asalkota LIKE ? AND tujuan_kecamatan LIKE ?", "%"+asalClean+"%", "%"+tujuanClean+"%").
		Limit(1).
		Find(&regList)

	if len(regList) == 0 {
		database.Table(targetTable).
			Where("UPPER(asal_kota) LIKE ? AND UPPER(tujuan_kecamatan) LIKE ?", "%"+asalClean+"%", "%"+tujuanClean+"%").
			Limit(1).
			Find(&regList)
	}

	if len(regList) == 0 {
		database.Table("public.mkt_m_harga").
			Where("asalkota LIKE ? AND tujuan_kecamatan LIKE ?", "%"+asalClean+"%", "%"+tujuanClean+"%").
			Limit(1).
			Find(&regList)
	}

	if len(regList) > 0 {
		regMap = regList[0]
	}

	// =========================================================================
	// 🟢 QUERY DATA DASAR DARAT EKONOMIS (MKT_M_HARGAEKONOMIS)
	// =========================================================================
	var ekoMap map[string]interface{}
	var ekoList []map[string]interface{}

	database.Table("public.mkt_m_hargaekonomis").
		Where("asalkota LIKE ? AND tujuan_kecamatan LIKE ?", "%"+asalClean+"%", "%"+tujuanClean+"%").
		Limit(1).
		Find(&ekoList)

	if len(ekoList) == 0 {
		database.Table("public.mkt_m_hargaekonomis").
			Where("UPPER(asal_kota) LIKE ? AND UPPER(tujuan_kecamatan) LIKE ?", "%"+asalClean+"%", "%"+tujuanClean+"%").
			Limit(1).
			Find(&ekoList)
	}

	if len(ekoList) > 0 {
		ekoMap = ekoList[0]
	}

	// Validasi akhir jika kedua rute layanan di database pusat benar-benar kosong
	if regMap == nil && ekoMap == nil {
		fmt.Printf("⚠️ [Tarif Guard] Rute benar-benar kosong total di DB: %s ke %s\n", req.AsalKota, req.TujuanKec)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("Waduh bro, rute pengiriman kargo dari %s ke %s belum terdaftar di database pusat logistik Dakota!", req.AsalKota, req.TujuanKec),
		})
		return
	}

	// =========================================================================
	// 💎 QUERY MATRIKS DISKON KREDIT CUSTOMER (MKT_M_HargaCustomer)
	// =========================================================================
	var custDiscount map[string]interface{}
	var custDiscountList []map[string]interface{}

	if req.AgenID != "" {
		// Gunakan Limit(1).Find() agar GORM tidak memaksa ORDER BY PK gaib pada objek Map!
		database.Table("public.mkt_m_hargacustomer").
			Where("HargaCust_id = ? AND UPPER(Tujuan_Kecamatan) LIKE ?", req.AgenID, "%"+tujuanClean+"%").
			Limit(1).
			Find(&custDiscountList)

		if len(custDiscountList) > 0 {
			custDiscount = custDiscountList[0]
			fmt.Println("🎯 [Discount Engine] Kontrak harga kredit customer berhasil dimuat!")
		}
	}

	// Fungsi helper internal untuk memetakan row dan kalkulasi diskon
	buildLayananRow := rangeLayananRow(regMap, custDiscount, beratChargeable, "REGULER")
	buildEkoRow := rangeLayananRow(ekoMap, custDiscount, beratChargeable, "EKONOMIS")

	// Penentuan grand total harga final untuk dikirim ke field utama BTT berdasarkan jenis pilihan user
	var finalGrandTotal float64 = 0
	statusHitung := "TARIF TUNAI UMUM"

	if strings.ToUpper(req.JenisLayanan) == "KREDIT" {
		statusHitung = "TARIF KREDIT ACCOUNT"
		if fmt.Sprintf("%v", c.Query("bttt_paketyn")) == "N" {
			finalGrandTotal = buildEkoRow["total_charge"].(float64)
		} else {
			finalGrandTotal = buildLayananRow["total_charge"].(float64)
		}
	} else {
		if fmt.Sprintf("%v", c.Query("bttt_paketyn")) == "N" {
			finalGrandTotal = buildEkoRow["total_normal"].(float64)
		} else {
			finalGrandTotal = buildLayananRow["total_normal"].(float64)
		}
	}

	// 🚀 SEMBURKAN PAYLOAD INTEGRASI FINAl JEDERRR!
	c.JSON(http.StatusOK, gin.H{
		"status":           "success",
		"status_hitung":    statusHitung,
		"berat_asli":       req.BeratAsli,
		"berat_volume":     beratVolume,
		"berat_chargeable": beratChargeable,
		"grand_total":      finalGrandTotal,
		"reguler_row":      buildLayananRow,
		"ekonomis_row":     buildEkoRow,
		"kode_kota_asal":   strings.ToUpper(strings.TrimSpace(dbAgen.AgenKotaID)),

		"nomor_urut_agen": extractThreeDigits(rawCabangID),
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
	res := map[string]interface{}{
		"servid":             99,
		"hargapokok":         0.0,
		"minimalkg":          0.0,
		"hargakgselanjutnya": 0.0,
		"flag_ds":            "N",
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
		"estimasihari":       0, // Lead Time (LT) utama wajib integer
	}

	if tarifRow == nil {
		return res
	}

	// =========================================================================
	// 🟣 1. AMBIL DATA ASLI TARIF NORMAL (SINKRONISASI FIELD DENGAN PGADMIN)
	// =========================================================================

	// Mapping Service ID
	if val, ok := tarifRow["servid"].(int64); ok {
		res["servid"] = val
	} else if val, ok := tarifRow["serv_id"].(int64); ok {
		res["servid"] = val
	} else if val, ok := tarifRow["serv_id"].(int32); ok {
		res["servid"] = int64(val)
	}

	// Mapping Nominal Dasar Tarif
	if val, ok := tarifRow["hargapokok"].(float64); ok {
		res["hargapokok"] = val
	} else if val, ok := tarifRow["harga_pokok"].(float64); ok {
		res["hargapokok"] = val
	}
	if val, ok := tarifRow["minimalkg"].(float64); ok {
		res["minimalkg"] = val
	} else if val, ok := tarifRow["minimal_kg"].(float64); ok {
		res["minimalkg"] = val
	}
	if val, ok := tarifRow["hargakgselanjutnya"].(float64); ok {
		res["hargakgselanjutnya"] = val
	} else if val, ok := tarifRow["harga_kg_selanjutnya"].(float64); ok {
		res["hargakgselanjutnya"] = val
	}
	if val, ok := tarifRow["flag_ds"].(string); ok {
		res["flag_ds"] = val
	}
	if val, ok := tarifRow["biayatambahan"].(float64); ok {
		res["biayatambahan"] = val
	}
	if val, ok := tarifRow["keterangan"].(string); ok {
		res["keterangan"] = val
	}

	// 🚀 FIX MUTLAK SAKRAL LT (LEAD TIME): TYPE SWITCHING TERPADU BERASAL DARI MASTER TARIF UTAMA PUSAT!
	if rawEst, exists := tarifRow["estimasihari"]; exists && rawEst != nil {
		switch v := rawEst.(type) {
		case int32:
			res["estimasihari"] = int(v)
		case int64:
			res["estimasihari"] = int(v)
		case int:
			res["estimasihari"] = v
		case float64:
			res["estimasihari"] = int(v)
		case float32:
			res["estimasihari"] = int(v)
		}
	} else if rawEstDel, exists := tarifRow["estimasi_hari"]; exists && rawEstDel != nil {
		switch v := rawEstDel.(type) {
		case int32:
			res["estimasihari"] = int(v)
		case int64:
			res["estimasihari"] = int(v)
		case int:
			res["estimasihari"] = v
		case float64:
			res["estimasihari"] = int(v)
		}
	}

	// 🚀 FIX SAKRAL BYPASS MASTER VOLUME MASTERING (KOLOM "vol" DARI PGADMIN GAMBAR 5)
	if val, ok := tarifRow["bypass1vol"].(float64); ok {
		res["bypass1kg"] = val
	}
	if val, ok := tarifRow["harga1vol"].(float64); ok {
		res["harga1kg"] = val
	}
	if val, ok := tarifRow["bypass2vol"].(float64); ok {
		res["bypass2kg"] = val
	}
	if val, ok := tarifRow["harga2vol"].(float64); ok {
		res["harga2kg"] = val
	}
	if val, ok := tarifRow["bypass3vol"].(float64); ok {
		res["bypass3kg"] = val
	}
	if val, ok := tarifRow["harga3vol"].(float64); ok {
		res["harga3kg"] = val
	}

	// Variabel lokal pembantu kalkulator rumus kargo
	kgMin := res["minimalkg"].(float64)
	hargaPokok := res["hargapokok"].(float64)
	hargaKgNext := res["hargakgselanjutnya"].(float64)
	biayaPenerus := res["biayatambahan"].(float64)

	// Hitung total normal umum (Flat vs Kumulatif)
	beratFinalNormal := berat
	if berat < kgMin {
		beratFinalNormal = kgMin
	}
	if berat <= kgMin {
		res["total_normal"] = hargaPokok + biayaPenerus
	} else {
		res["total_normal"] = hargaPokok + ((beratFinalNormal - kgMin) * hargaKgNext) + biayaPenerus
	}

	// =========================================================================
	// 💎 2. SUNTIKKAN KALKULASI DISCOUNT JIKA ADA CONTRACT KREDIT CUSTOMER
	// =========================================================================
	if discRow != nil {
		res["has_discount"] = "Y"

		var discPokok, discLvl1, discLvl2, discLvl3 float64
		if val, ok := discRow["DiscountPokok"].(float64); ok {
			discPokok = val
		}
		if val, ok := discRow["DiscountLevel1"].(float64); ok {
			discLvl1 = val
		}
		if val, ok := discRow["DiscountLevel2"].(float64); ok {
			discLvl2 = val
		}
		if val, ok := discRow["DiscountLevel3"].(float64); ok {
			discLvl3 = val
		}

		// Overriding bypass threshold berdasarkan tabel kontrak customer (jika diset khusus)
		if val, ok := discRow["bypass1kg"].(float64); ok && val > 0 {
			res["bypass1kg"] = val
		}
		if val, ok := discRow["bypass2kg"].(float64); ok && val > 0 {
			res["bypass2kg"] = val
		}
		if val, ok := discRow["bypass3kg"].(float64); ok && val > 0 {
			res["bypass3kg"] = val
		}
		if val, ok := discRow["biayatambahan"].(float64); ok {
			res["biayatambahan"] = biayaPenerus + val
		}

		// Overriding lead time khusus customer (jika ada kontrak SLA khusus dari finance)
		if val, ok := discRow["estimasiHari"].(int32); ok && val > 0 {
			res["estimasihari"] = int(val)
		} else if val, ok := discRow["estimasiHari"].(int64); ok && val > 0 {
			res["estimasihari"] = int(val)
		} else if val, ok := discRow["estimasihari"].(float64); ok && val > 0 {
			res["estimasihari"] = int(val)
		}

		bp1 := res["bypass1kg"].(float64)
		bp2 := res["bypass2kg"].(float64)
		bp3 := res["bypass3kg"].(float64)
		biayaPenerusFinal := res["biayatambahan"].(float64)

		// Set visual nominal terdiskon server side
		res["hargapokok"] = hargaPokok * (1 - discPokok/100)
		res["hargakgselanjutnya"] = hargaKgNext * (1 - discPokok/100)
		res["harga1kg"] = res["harga1kg"].(float64) * (1 - discLvl1/100)
		res["harga2kg"] = res["harga2kg"].(float64) * (1 - discLvl2/100)
		res["harga3kg"] = res["harga3kg"].(float64) * (1 - discLvl3/100)

		// Kalkulasi Akhir Tagihan Kredit Account
		if bp3 > 0 && berat >= bp3 {
			res["total_charge"] = (berat * res["harga3kg"].(float64)) + biayaPenerusFinal
		} else if bp2 > 0 && berat >= bp2 {
			res["total_charge"] = (berat * res["harga2kg"].(float64)) + biayaPenerusFinal
		} else if bp1 > 0 && berat >= bp1 {
			res["total_charge"] = (berat * res["harga1kg"].(float64)) + biayaPenerusFinal
		} else if berat >= kgMin {
			discPokokFactor := res["hargakgselanjutnya"].(float64) / hargaKgNext
			res["total_charge"] = (berat * (hargaKgNext * discPokokFactor)) + biayaPenerusFinal
		} else {
			res["total_charge"] = res["hargapokok"].(float64) + biayaPenerusFinal
		}
	} else {
		// Pascasarana default rollback jika pembayaran non-kredit (TUNAI / COD)
		// Tetap pastikan res["estimasihari"] yang sudah dihitung di atas ikut ter-return dengan aman!
		res["total_charge"] = res["total_normal"]
	}

	return res
}
