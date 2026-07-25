package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetBTT(c *gin.Context) {
	var data []models.BTT
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PT ID tidak ditemukan di token"})
		return
	}

	// 🟢 Tangkap parameter filter agen_id dari URL request React (?agen_id=839)
	agenID := c.Query("agen_id")

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	// Buat Query Builder dasar GORM
	query := database.Order("bttt_tanggal desc").Limit(100)

	// 🟢 Jika parameter agen_id dikirim dari React, lakukan penyaringan ketat di query Postgres
	if agenID != "" && agenID != "undefined" && agenID != "null" {
		if _, err := strconv.Atoi(agenID); err == nil {
			// Lolos sensor: agenID terbukti angka murni (e.g. 839, 515), aman masuk ke kolom integer Postgres!
			query = query.Where("bttt_asalagenid = ?", agenID)
		} else {
			// Jika agenID berupa teks nama (e.g. "PUSAT DAKOTA"), otomatis dilewati (bypass) tanpa bikin SQL crash!
			log.Printf("🏢 [Nusantara Guard] Parameter agen_id kustom/non-numeric terdeteksi [%s]. Filter WHERE di-bypass murni.", agenID)
		}
	}

	err := query.Find(&data).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

func CheckLockBTT(c *gin.Context) {
	// 1. Ambil PT ID aman dari context token JWT buatanmu
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PT ID tidak ditemukan di token"})
		return
	}

	// Ambil Agen ID dari parameter query
	agenID := strings.TrimSpace(c.Query("agen_id"))
	if agenID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter agen_id wajib diisi, bro!"})
		return
	}

	// =========================================================================
	// 🛑 LAPISAN 0: PROTEKSI PUSAT DAKOTA (HOLDING)
	// =========================================================================
	upperAgen := strings.ToUpper(agenID)
	if agenID == "1" || upperAgen == "PUSAT DAKOTA" || strings.Contains(upperAgen, "PUSAT") || strings.Contains(upperAgen, "HOLDING") {
		c.JSON(http.StatusOK, gin.H{
			"is_locked": true,
			"layer":     0,
			"reason":    "Pusat Dakota (Holding) tidak diizinkan untuk membuat Bukti Tanda Terima (BTT)! Silakan ganti lokasi loket ke Agen / Cabang Operasional.",
		})
		return
	}

	// Resolve database dinamis sesuai tenant PT
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	// Waktu server saat ini (Zona Asia/Jakarta)
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	// Cek juga ke tabel master glb_m_agen jika agen_id berupa ID/Kode angka
	var agenNama string
	database.Table("public.glb_m_agen").
		Select("agen_nama").
		Where("glb_agenid = ? OR agen_kode = ?", agenID, agenID).
		Scan(&agenNama)

	upperNamaDb := strings.ToUpper(agenNama)
	if strings.Contains(upperNamaDb, "PUSAT") || strings.Contains(upperNamaDb, "HOLDING") {
		c.JSON(http.StatusOK, gin.H{
			"is_locked": true,
			"layer":     0,
			"reason":    "Pusat Dakota (Holding) tidak diizinkan untuk membuat Bukti Tanda Terima (BTT)! Silakan ganti lokasi loket ke Agen / Cabang Operasional.",
		})
		return
	}

	// ==========================================
	// LAPISAN 1: Lock Jam Operasional Global
	// ==========================================
	if now.Hour() >= 22 {
		c.JSON(http.StatusOK, gin.H{
			"is_locked": true,
			"layer":     1,
			"reason":    "Batas waktu input harian sudah habis (Cut-off 22:00 WIB). Silakan hubungi admin pusat.",
		})
		return
	}

	// ==========================================
	// LAPISAN 2: Lock Status Aktif Agen
	// ==========================================
	var statusAktif string
	err := database.Table("public.glb_m_agen").
		Select("glb_aktifyn").
		Where("glb_agenid = ?", agenID).
		Scan(&statusAktif).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal check master agen: " + err.Error()})
		return
	}

	if statusAktif == "N" {
		c.JSON(http.StatusOK, gin.H{
			"is_locked": true,
			"layer":     2,
			"reason":    "ID Agen kamu dideaktivasi oleh sistem pusat. Tidak diizinkan membuat manifest/BTT baru.",
		})
		return
	}

	// ==========================================
	// LAPISAN 3: Proteksi Limit Kredit & Sisa Bayar Invoice
	// ==========================================
	var totalSisaBayar float64
	err = database.Table("public.art_t_invoiceh").
		Select("COALESCE(SUM(artih_sisabayar), 0)").
		Where("artih_custid = ? AND artih_delete = 'N'", agenID).
		Scan(&totalSisaBayar).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal kalkulasi plafon piutang: " + err.Error()})
		return
	}

	if totalSisaBayar > 50000000 {
		c.JSON(http.StatusOK, gin.H{
			"is_locked": true,
			"layer":     3,
			"reason":    fmt.Sprintf("Plafon piutang jebol! Total tunggakan invoice kamu Rp %.2f melebihi batas limit.", totalSisaBayar),
		})
		return
	}

	// ==========================================
	// LAPISAN 4: Proteksi Periode Closing Buku
	// ==========================================
	var isClosed int
	currentPeriode := now.Format("200602") // Format: YYYYMM

	err = database.Table("public.glb_m_closing").
		Select("COUNT(1)").
		Where("closing_periode = ? AND closing_status = 'Y'", currentPeriode).
		Scan(&isClosed).Error

	if err == nil && isClosed > 0 {
		c.JSON(http.StatusOK, gin.H{
			"is_locked": true,
			"layer":     4,
			"reason":    "Periode akuntansi bulan ini sudah ditutup (Closed). Tidak bisa input transaksi baru.",
		})
		return
	}

	// ==========================================
	// SUKSES: Lolos Seluruh Proteksi Lock
	// ==========================================
	c.JSON(http.StatusOK, gin.H{
		"is_locked":   false,
		"message":     "Verifikasi lolos! Agen diizinkan melakukan input BTT.",
		"server_time": now.Format("2006-01-02 15:04:05"),
	})
}

// CalculateTarif menghitung berat chargeable dan mencari tarif terbaik dari database
func CalculateTarif(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PT ID tidak ditemukan di token"})
		return
	}

	var req models.TarifRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input data tidak valid: " + err.Error()})
		return
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	var dbAgen models.GlbMAgen
	inputAsalRaw := strings.TrimSpace(req.AsalKota)

	errAgen := database.Table("public.glb_m_agen").
		Select("agen_kotaid").
		Where("TRIM(agen_kode) = TRIM(?)", inputAsalRaw).
		First(&dbAgen).Error

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
		cleanCityName = strings.TrimSpace(cleanCityName)

		// 🌟 FIX POSTGRESQL SYNTAX: Ubah SELECT TOP 1 menjadi LIMIT 1
		queryVerify := `SELECT UPPER(TRIM(asalkota)) FROM public.mkt_m_harga WHERE asalkota ILIKE ? LIMIT 1`
		database.Raw(queryVerify, "%"+cleanCityName+"%").Scan(&kotaAsalMaster)
	}

	if kotaAsalMaster != "" {
		req.AsalKota = kotaAsalMaster
		fmt.Printf("🎯 [RELATIONAL RESOLVE SUCCESS] Agen Kode '%s' -> ID Kota: '%s' -> Terpetakan ke Master Pricing: '%s'\n", inputAsalRaw, dbAgen.AgenKotaID, req.AsalKota)
	} else {
		req.AsalKota = strings.ToUpper(inputAsalRaw)
		fmt.Printf("⚠️ [RELATIONAL RESOLVE FAILED] Gagal menerjemahkan Kode Agen '%s'. Menggunakan data mentah.\n", inputAsalRaw)
	}

	var regulerRow map[string]interface{}
	var ekonomisRow map[string]interface{}

	database.Table("public.mkt_m_harga").
		Where("asalkota = ? AND tujuan_kecamatan LIKE ? AND aktifyn = 'Y'", req.AsalKota, "%"+req.TujuanKec+"%").
		First(&regulerRow)

	if regulerRow == nil {
		database.Table("master_tarif_reguler").
			Where("UPPER(asal_kota) = ? AND UPPER(tujuan_kecamatan) = ?", req.AsalKota, req.TujuanKec).
			First(&regulerRow)
	}

	database.Table("public.mkt_m_hargaekonomis").
		Where("asalkota = ? AND tujuan_kecamatan LIKE ? AND flag_ds = 'N'", req.AsalKota, "%"+req.TujuanKec+"%").
		First(&ekonomisRow)

	var beratVolume float64 = 0
	if req.Panjang > 0 && req.Lebar > 0 && req.Tinggi > 0 {
		beratVolume = (req.Panjang * req.Lebar * req.Tinggi) / 4000.0
	}

	beratChargeable := req.BeratAsli
	if beratVolume > req.BeratAsli {
		beratChargeable = beratVolume
	}

	var hargaPerKg, minBerat, biayaPenerus float64

	if strings.ToUpper(req.JenisLayanan) == "EKONOMIS" && ekonomisRow != nil {
		if val, ok := ekonomisRow["hargapokok"].(float64); ok {
			hargaPerKg = val
		}
		if val, ok := ekonomisRow["minimalkg"].(float64); ok {
			minBerat = val
		}
		if val, ok := ekonomisRow["biayatambahan"].(float64); ok {
			biayaPenerus = val
		}
	} else if regulerRow != nil {
		if val, ok := regulerRow["hargapokok"].(float64); ok {
			hargaPerKg = val
		}
		if val, ok := regulerRow["minimalkg"].(float64); ok {
			minBerat = val
		}
		if val, ok := regulerRow["biayatambahan"].(float64); ok {
			biayaPenerus = val
		}
	}

	beratFinal := beratChargeable
	if beratChargeable < minBerat {
		beratFinal = minBerat
	}

	totalHarga := beratFinal * hargaPerKg
	grandTotal := totalHarga + biayaPenerus

	c.JSON(http.StatusOK, gin.H{
		"status":           "success",
		"detected_asal":    kotaAsalMaster,
		"berat_asli":       req.BeratAsli,
		"berat_volume":     beratVolume,
		"berat_chargeable": beratChargeable,
		"berat_final":      beratFinal,
		"harga_per_kg":     hargaPerKg,
		"minimum_berat":    minBerat,
		"biaya_penerus":    biayaPenerus,
		"total_harga":      totalHarga,
		"grand_total":      grandTotal,
		"reguler_row":      regulerRow,
		"ekonomis_row":     ekonomisRow,
	})
}

// CreateBTT menghandle INSERT data transaksi BTT baru dari React Form ke mkt_t_econote
func CreateBTT(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PT ID tidak ditemukan di token"})
		return
	}

	var rawPayload map[string]interface{}
	if err := c.ShouldBindJSON(&rawPayload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload input form tidak valid: " + err.Error()})
		return
	}

	// 🛑 PROTEKSI SERVER: BLOKIR INSERT BTT DARI PUSAT DAKOTA / HOLDING
	asalAgenRaw := strings.TrimSpace(fmt.Sprintf("%v", rawPayload["bttt_asalagenid"]))
	upperAsalAgen := strings.ToUpper(asalAgenRaw)

	if asalAgenRaw == "1" || upperAsalAgen == "PUSAT DAKOTA" || strings.Contains(upperAsalAgen, "PUSAT") || strings.Contains(upperAsalAgen, "HOLDING") {
		c.JSON(http.StatusForbidden, gin.H{
			"status": "error",
			"error":  "Akses Ditolak! Pusat Dakota (Holding) tidak diizinkan membuat BTT baru.",
		})
		return
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal resolved"})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	// Ambil ID utama dari React dan cari nomor urut terunik agar tidak bentrok / double
	bttIDStr := getUniqueBttID(database, fmt.Sprintf("%v", rawPayload["id"]), rawPayload, now)

	var agenIDDinamis int = 0
	if rawPayload["bttt_asalagenid"] != nil {
		switch v := rawPayload["bttt_asalagenid"].(type) {
		case float64:
			if v > 0 {
				agenIDDinamis = int(v)
			}
		case string:
			cleanStr := strings.TrimSpace(v)
			if cleanStr != "" && cleanStr != "<nil>" {
				if conv, err := strconv.Atoi(cleanStr); err == nil && conv > 0 {
					agenIDDinamis = conv
				}
			}
		case int:
			if v > 0 {
				agenIDDinamis = v
			}
		}
	}

	isiKirimanRaw := fmt.Sprintf("%v", rawPayload["bttt_isikiriman"])
	if isiKirimanRaw == "<nil>" || strings.TrimSpace(isiKirimanRaw) == "" {
		isiKirimanRaw = "BARANG"
	}

	jmlKoliRaw := fmt.Sprintf("%v", rawPayload["bttt_jmlkoli"])
	if jmlKoliRaw == "<nil>" || strings.TrimSpace(jmlKoliRaw) == "" {
		jmlKoliRaw = "1"
	}

	kodeCabangRaw := fmt.Sprintf("%v", rawPayload["bttt_kodecabangagen"])
	if kodeCabangRaw == "<nil>" || strings.TrimSpace(kodeCabangRaw) == "" {
		kodeCabangRaw = "DK-GENERAL"
	}

	biayaPackingRaw := rawPayload["bttt_biayapacking"]
	biayaPenerusRaw := rawPayload["bttt_biayatambahan"]
	keteranganAsli := fmt.Sprintf("%v", rawPayload["bttt_ket"])
	if keteranganAsli == "<nil>" || strings.TrimSpace(keteranganAsli) == "" {
		keteranganAsli = "SURAT JALAN KEMBALI"
	}

	var nominalPacking float64 = 0
	if biayaPackingRaw != nil {
		if val, ok := biayaPackingRaw.(float64); ok {
			nominalPacking = val
		}
	}

	var nominalPenerus float64 = 0
	if biayaPenerusRaw != nil {
		if val, ok := biayaPenerusRaw.(float64); ok {
			nominalPenerus = val
		}
	}

	pilihCarterRaw := ""
	if rawPayload["bttt_pilihcarter"] != nil {
		pilihCarterRaw = strings.TrimSpace(fmt.Sprintf("%v", rawPayload["bttt_pilihcarter"]))
		if pilihCarterRaw == "<nil>" {
			pilihCarterRaw = ""
		}
	}

	carterSuffix := ""
	if pilihCarterRaw != "" {
		carterSuffix = fmt.Sprintf(" - CARTER: %s", pilihCarterRaw)
	}

	keteranganAsli = fmt.Sprintf(
		"%s (%s KOLI) - CABANG: %s%s - %s [B.PACKING: Rp %.0f, B.PENERUS: Rp %.0f]",
		strings.ToUpper(isiKirimanRaw),
		jmlKoliRaw,
		strings.ToUpper(kodeCabangRaw),
		carterSuffix,
		keteranganAsli,
		nominalPacking,
		nominalPenerus,
	)

	tujuanAlamatRaw := fmt.Sprintf("%v", rawPayload["bttt_tujuanalamat"])
	if tujuanAlamatRaw == "<nil>" {
		tujuanAlamatRaw = ""
	}
	tujuanPropinsiRaw := ""
	if rawPayload["bttt_tujuanpropinsi"] != nil {
		tujuanPropinsiRaw = strings.TrimSpace(fmt.Sprintf("%v", rawPayload["bttt_tujuanpropinsi"]))
		if tujuanPropinsiRaw == "<nil>" {
			tujuanPropinsiRaw = ""
		}
	}
	if tujuanPropinsiRaw != "" {
		if tujuanAlamatRaw != "" {
			tujuanAlamatRaw = tujuanAlamatRaw + ", " + tujuanPropinsiRaw
		} else {
			tujuanAlamatRaw = tujuanPropinsiRaw
		}
	}

	var tujuanAgenID int = 0
	if rawPayload["bttt_tujuanagenid"] != nil {
		tujuanAgenIDRaw := rawPayload["bttt_tujuanagenid"]
		switch v := tujuanAgenIDRaw.(type) {
		case float64:
			tujuanAgenID = int(v)
		case int:
			tujuanAgenID = v
		case string:
			cleanStr := strings.TrimSpace(v)
			if cleanStr != "" && cleanStr != "<nil>" {
				if conv, err := strconv.Atoi(cleanStr); err == nil {
					tujuanAgenID = conv
				} else {
					type Agen struct {
						AgenKode string `gorm:"column:agen_kode"`
					}
					var a Agen
					if err := database.Table("public.glb_m_agen").Select("agen_kode").Where("agen_nama = ?", cleanStr).First(&a).Error; err == nil {
						if code, err := strconv.Atoi(a.AgenKode); err == nil {
							tujuanAgenID = code
						}
					} else {
						queryPattern := "%" + strings.ReplaceAll(cleanStr, " ", "%") + "%"
						if err := database.Table("public.glb_m_agen").Select("agen_kode").Where("agen_nama ILIKE ?", queryPattern).First(&a).Error; err == nil {
							if code, err := strconv.Atoi(a.AgenKode); err == nil {
								tujuanAgenID = code
							}
						}
					}
				}
			}
		}
	}

	dbRow := map[string]interface{}{
		"bttt_id":              cleanStringVal(bttIDStr),
		"bttt_tanggal":         now,
		"bttt_nosuratjalan":    cleanStringVal(rawPayload["bttt_nosuratjalan"]),
		"bttt_ket":             cleanStringVal(keteranganAsli),
		"bttt_nobttmanual":     cleanStringVal(rawPayload["bttt_nobttmanual"]),
		"bttt_dliexpryn":       cleanStringVal(rawPayload["bttt_dliexpryn"]),
		"bttt_promoid":         cleanStringVal(rawPayload["bttt_promoid"]),
		"bttt_asalcustid":      cleanStringVal(rawPayload["bttt_asalcustid"]),
		"bttt_asalname":        cleanStringVal(rawPayload["bttt_asalname"]),
		"bttt_asalalamat":      cleanStringVal(rawPayload["bttt_asalalamat"]),
		"bttt_asalkota":        cleanStringVal(rawPayload["bttt_asalkota"]),
		"bttt_asaltelp":        cleanStringVal(rawPayload["bttt_asaltelp"]),
		"bttt_tujuannama":      cleanStringVal(rawPayload["bttt_tujuannama"]),
		"bttt_up":              cleanStringVal(rawPayload["bttt_up"]),
		"bttt_tujuanalamat":    cleanStringVal(tujuanAlamatRaw),
		"bttt_tujuankota":      cleanStringVal(rawPayload["bttt_tujuankota"]),
		"bttt_tujuankelurahan": cleanStringVal(rawPayload["bttt_tujuankelurahan"]),
		"bttt_tujuankecamatan": cleanStringVal(rawPayload["bttt_tujuankecamatan"]),
		"bttt_tujuankodepos":   cleanStringVal(rawPayload["bttt_tujuankodepos"]),
		"bttt_tujuanemail":     cleanStringVal(rawPayload["bttt_tujuanemail"]),
		"bttt_tujuantelp":      cleanStringVal(rawPayload["bttt_tujuantelp"]),
		"bttt_tujuanagenid":    tujuanAgenID,
		"bttt_paketyn":         cleanStringVal(rawPayload["bttt_paketyn"]),
		"bttt_jenisharga":      cleanStringVal(rawPayload["bttt_jenisharga"]),
		"bttt_berat":           rawPayload["bttt_berat"],
		"bttt_beratvol":        rawPayload["bttt_beratvol"],
		"bttt_ukuran":          cleanStringVal(rawPayload["bttt_ukuran"]),
		"bttt_harga":           rawPayload["bttt_harga"],
		"bttt_spyn":            "Y",
		"bttt_aktifyn":         "Y",
		"bttt_servid":          1,
		"bttt_asalagenid":      agenIDDinamis,
	}

	err := database.Table("public.mkt_t_econote").Create(&dbRow).Error
	if err != nil {
		fmt.Println("❌ [DB INSERT CRASH MELEDAK]:", err.Error())

		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key") {
			c.JSON(http.StatusConflict, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Waduh bro, nomor resi %s sudah pernah terdaftar di database! Harap refresh modal atau naikkan nomor urut manifest buntut kargo lu!", bttIDStr),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan ke database pusat logistik: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data Bukti Tanda Terima (BTT) Berhasil Disimpan ke Database!",
		"btt_no":  bttIDStr,
	})
}

func GetKecamatanByKota(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PT ID tidak ditemukan"})
		return
	}

	namaKota := c.Query("kota")
	if namaKota == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter kota wajib diisi, bro!"})
		return
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	type AreaKecamatan struct {
		Kecamatan string `json:"kecamatan" gorm:"column:kot_kecamatan"`
		KodePos   string `json:"kodepos" gorm:"column:kot_kodepos"`
	}
	var listKecamatan []AreaKecamatan

	err := database.Table("public.glb_m_kota").
		Select("kot_kecamatan, kot_kodepos").
		Where("kot_nama = ? AND kot_aktifyn = 'Y'", namaKota).
		Order("kot_kecamatan asc").
		Find(&listKecamatan).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal fetch data kota: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   listKecamatan,
	})
}

func SearchAreaByKecamatan(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PT ID tidak ditemukan"})
		return
	}

	searchKeyword := c.Query("search")
	if len(searchKeyword) < 3 {
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": []gin.H{}})
		return
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	type AreaData struct {
		KodePos   string `json:"kodepos" gorm:"column:kodepos"`
		Kelurahan string `json:"desakelurahan" gorm:"column:desakelurahan"`
		Kecamatan string `json:"kecamatandistrik" gorm:"column:kecamatandistrik"`
		Kota      string `json:"kotakabupaten" gorm:"column:kotakabupaten"`
		Provinsi  string `json:"propinsi" gorm:"column:propinsi"`
	}
	var results []AreaData

	err := database.Table("public.glb_m_kodepos").
		Select("kodepos, desakelurahan, kecamatandistrik, kotakabupaten, propinsi").
		Where("kecamatandistrik ILIKE ?", "%"+searchKeyword+"%").
		Limit(15).
		Find(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal query data area: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

func ValidateBTTHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PT ID tidak ditemukan di token"})
		return
	}

	var req models.BttValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data validasi tidak komplit: " + err.Error()})
		return
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	re := regexp.MustCompile(`^(08|\+628)\d{8,13}$`)

	if !re.MatchString(req.AsalTelp) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format nomor telepon pengirim tidak valid! Wajib angka, minimal 10 digit, dan diawali 08 atau +62."})
		return
	}
	if !re.MatchString(req.TujuanTelp) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format nomor telepon penerima tidak valid! Wajib angka, minimal 10 digit, dan diawali 08 atau +62."})
		return
	}

	if req.CaraBayar == "1" && req.GrandTotal < 100000 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Metode Tagih Tujuan (COD) ditolak! Total biaya kargo kamu baru Rp %.2f. Syarat minimal wajib Rp 100.000, bro!", req.GrandTotal),
		})
		return
	}

	var totalTunggakan float64
	err := database.Table("public.art_t_invoiceh").
		Select("COALESCE(SUM(artih_sisabayar), 0)").
		Where("artih_custid = ? AND artih_delete = 'N'", req.AsalCustID).
		Scan(&totalTunggakan).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memverifikasi plafon piutang server: " + err.Error()})
		return
	}

	if (totalTunggakan + req.GrandTotal) > 50000000 {
		c.JSON(http.StatusForbidden, gin.H{
			"error": fmt.Sprintf("Transaksi diblokir sistem keuangan! Total piutang berjalan kamu (Rp %.2f) sudah melewati batas limit kredit 50 Juta.", totalTunggakan),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Validasi berlapis tingkat server sukses! Data aman untuk disimpan.",
	})
}

func GenerateCustIDUmumHandler(c *gin.Context) {
	kodeAgen := c.Query("kode_agen")
	if kodeAgen == "" {
		kodeAgen = "DKX"
	}

	now := time.Now()
	bulanStr := now.Format("01")
	tahunStr := now.Format("06")

	prefix := kodeAgen + bulanStr + tahunStr

	var count int64
	db.DB.Table("mkt_m_customer").
		Where("cust_id LIKE ?", prefix+"%").
		Count(&count)

	nextCounter := count + 1
	fiveDigitStr := fmt.Sprintf("%05d", nextCounter)

	generatedID := prefix + fiveDigitStr

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"generated_id": generatedID,
	})
}

func GenerateCustIDHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "PT ID tidak ditemukan"})
		return
	}

	kodeAgen, _ := c.Get("kode_agen")
	kodeAgenStr := fmt.Sprintf("%v", kodeAgen)

	if kodeAgenStr == "" || kodeAgenStr == "<nil>" {
		kodeParam := c.Query("kode_agen")
		if kodeParam != "" {
			kodeAgenStr = strings.ToUpper(kodeParam)
		} else {
			kodeAgenStr = "JKT"
		}
	}

	if len(kodeAgenStr) > 3 {
		kodeAgenStr = kodeAgenStr[:3]
	} else {
		kodeAgenStr = fmt.Sprintf("%-3s", kodeAgenStr)
	}
	kodeAgenStr = strings.ToUpper(strings.TrimSpace(kodeAgenStr))

	now := time.Now()
	bulanStr := now.Format("01")
	tahunStr := now.Format("06")

	prefixID := kodeAgenStr + bulanStr + tahunStr

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database tenant gagal resolved"})
		return
	}

	var lastCustID string
	err := database.Table("public.mkt_m_customer").
		Select("cust_id").
		Where("cust_id LIKE ?", prefixID+"%").
		Order("cust_id DESC").
		Limit(1).
		Row().Scan(&lastCustID)

	var nextUrutan int = 1

	if err == nil && len(lastCustID) >= 12 {
		suffix := lastCustID[7:]
		var currentUrutan int
		fmt.Sscanf(suffix, "%d", &currentUrutan)
		nextUrutan = currentUrutan + 1
	}

	fiveDigitStr := fmt.Sprintf("%05d", nextUrutan)
	finalGeneratedID := prefixID + fiveDigitStr

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"generated_id": finalGeneratedID,
	})
}

func SearchHistoryPengirimHandler(c *gin.Context) {
	keyword := c.Query("search")
	if len(keyword) < 1 {
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": []gin.H{}})
		return
	}

	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "PT ID tidak ditemukan"})
		return
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database tenant gagal"})
		return
	}

	type HistoryPengirimRes struct {
		PengirimNama   string `json:"pengirim_nama" gorm:"column:bttt_asalname"`
		PengirimAlamat string `json:"pengirim_alamat" gorm:"column:bttt_asalalamat"`
		PengirimTelp   string `json:"pengirim_telp" gorm:"column:bttt_asaltelp"`
		PengirimEmail  string `json:"pengirim_email" gorm:"column:bttt_asalemail"`
		PengirimKota   string `json:"pengirim_kota" gorm:"column:bttt_asalkota"`
	}

	var data []HistoryPengirimRes

	err := database.Table("public.mkt_t_econote").
		Select("bttt_asalname, bttt_asalalamat, bttt_asaltelp, bttt_asalemail, bttt_asalkota").
		Where("bttt_asalname ILIKE ?", "%"+keyword+"%").
		Group("bttt_asalname, bttt_asalalamat, bttt_asaltelp, bttt_asalemail, bttt_asalkota").
		Limit(5).
		Scan(&data).Error

	if err != nil {
		fmt.Println("🔥 ERROR QUERY Transaksi Econote:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if data == nil {
		data = []HistoryPengirimRes{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": data})
}

func cleanStringVal(val interface{}) string {
	if val == nil {
		return ""
	}
	str := strings.TrimSpace(fmt.Sprintf("%v", val))
	if str == "<nil>" {
		return ""
	}
	return str
}

func getUniqueBttID(database *gorm.DB, rawID string, rawPayload map[string]interface{}, now time.Time) string {
	bttIDStr := strings.TrimSpace(rawID)
	if bttIDStr == "" || bttIDStr == "<nil>" {
		bttIDStr = fmt.Sprintf("A%s%s00001", fmt.Sprintf("%v", rawPayload["bttt_asalagenid"]), now.Format("0106"))
	}

	for {
		var count int64
		err := database.Table("public.mkt_t_econote").Where("bttt_id = ?", bttIDStr).Count(&count).Error
		if err != nil || count == 0 {
			break
		}

		if len(bttIDStr) > 5 {
			prefix := bttIDStr[:len(bttIDStr)-5]
			var maxID string
			errMax := database.Table("public.mkt_t_econote").
				Select("bttt_id").
				Where("bttt_id LIKE ?", prefix+"%").
				Order("bttt_id DESC").
				Limit(1).
				Scan(&maxID).Error

			if errMax == nil && len(maxID) == len(bttIDStr) {
				suffix := maxID[len(maxID)-5:]
				var currentUrutan int
				if _, errScan := fmt.Sscanf(suffix, "%d", &currentUrutan); errScan == nil {
					nextUrutan := currentUrutan + 1
					bttIDStr = fmt.Sprintf("%s%05d", prefix, nextUrutan)
					continue
				}
			}
		}

		bttIDStr = fmt.Sprintf("%s_%d", bttIDStr, time.Now().UnixNano()%1000)
		break
	}

	return bttIDStr
}

func CheckStatusClosingKemarin(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	if fmt.Sprintf("%v", ptID) == "<nil>" || fmt.Sprintf("%v", ptID) == "" {
		if altID, ok := c.Get("selected_pt"); ok {
			ptID = altID
		} else {
			ptID = "A"
		}
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	agenID := c.Query("agen_id")
	if agenID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter agen_id wajib dikirim!"})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	hariKemarin := time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")

	var totalBttKemarin int64
	database.Table("public.mkt_t_econote").
		Where("bttt_tanggal >= ? AND bttt_tanggal <= ? AND bttt_asalagenid = ? AND bttt_aktifyn = 'Y'",
			hariKemarin+" 00:00:00", hariKemarin+" 23:59:59", agenID).
		Count(&totalBttKemarin)

	if totalBttKemarin == 0 {
		c.JSON(http.StatusOK, gin.H{"status": "allowed", "message": "Hari kemarin tidak ada transaksi BTT"})
		return
	}

	var countClosing int64
	database.Table("public.art_t_penjualanbtth").
		Where("btth_tanggal = ? AND btth_agenid = ? AND btth_activeyn = 'Y'", hariKemarin, agenID).
		Count(&countClosing)

	if countClosing == 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":  "blocked",
			"message": fmt.Sprintf("BTT kemarin (%s) belum di-closing! Selesaikan closingan terlebih dahulu untuk membuka akses transaksi BTT baru!", hariKemarin),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "allowed", "message": "Akses loket disetujui"})
}

func SearchMasterGeoBtt(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))

	if len(keyword) < 1 {
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": []interface{}{}})
		return
	}

	ptID, exists := c.Get("pt_id")
	if !exists {
		ptID = "A"
	}
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	var results []map[string]interface{}

	queryRaw := `
		SELECT 
			kodepos as id,
			COALESCE(propinsi, '') as propinsi,
			COALESCE(kotakabupaten, '') as kabupaten,
			COALESCE(kecamatandistrik, '') as kecamatan,
			COALESCE(desakelurahan, '') as kelurahan,
			COALESCE(kodepos, '') as kodepos
		FROM public.glb_m_kodepos
		WHERE 
			public.glb_m_kodepos.kecamatandistrik ILIKE ? OR
			public.glb_m_kodepos.kotakabupaten ILIKE ? OR
			public.glb_m_kodepos.propinsi ILIKE ? OR
			public.glb_m_kodepos.desakelurahan ILIKE ? OR
			kodepos LIKE ?
		ORDER BY kecamatandistrik ASC, desakelurahan ASC
		LIMIT 20`

	likeKeyword := "%" + keyword + "%"

	log.Printf("🔍 [Backend search-geo] Menjalankan SQL ILIKE Postgres untuk keyword: '%s'", likeKeyword)

	if err := database.Raw(queryRaw, likeKeyword, likeKeyword, likeKeyword, likeKeyword, likeKeyword).Scan(&results).Error; err != nil {
		log.Printf("❌ [Backend search-geo] GAGAL LIVE SEARCH BTT GEO: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mencari data wilayah"})
		return
	}

	log.Printf("🎯 [Backend search-geo] SUKSES! Ditemukan %d baris wilayah di glb_m_kodepos", len(results))

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

func GetKelurahanByKecamatan(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists {
		ptID = "A"
	}
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	kecamatan := c.Query("kecamatan")
	if kecamatan == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter kecamatan wajib diisi"})
		return
	}

	type KelurahanRes struct {
		Kelurahan string `json:"kelurahan" gorm:"column:desakelurahan"`
		Kodepos   string `json:"kodepos" gorm:"column:kodepos"`
	}
	var listKelurahan []KelurahanRes

	err := database.Table("public.glb_m_kodepos").
		Select("desakelurahan, kodepos").
		Where("kecamatandistrik = ?", kecamatan).
		Order("desakelurahan ASC").
		Find(&listKelurahan).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   listKelurahan,
	})
}
