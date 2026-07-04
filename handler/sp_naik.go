package handler

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// =========================================================================
// 🚚 1. ENDPOINT DROPDOWN: AMBIL DATA ARMADA & SOPIR DINAMIS
// =========================================================================
func GetFleetAndDrivers(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	if fmt.Sprintf("%v", ptID) == "<nil>" || fmt.Sprintf("%v", ptID) == "" {
		ptID = "A"
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database tenant gagal"})
		return
	}

	// Ambil data kendaraan aktif
	var vehicles []map[string]interface{}
	database.Table("public.glb_m_kendaraan").
		Select("kend_id, kend_identid AS nopol, kend_merk, kend_type").
		Where("kend_aktifyn = 'Y' OR kend_aktifyn = '1'").
		Order("kend_identid ASC").
		Find(&vehicles)

	// Ambil data sopir/driver aktif murni berdasarkan kode jabatan operasional
	var drivers []map[string]interface{}
	database.Table("public.hrd_m_karyawan").
		Select("kry_nip AS nip, kry_nama AS nama").
		Where("(kry_aktifyn = 'Y' OR kry_aktifyn = '1') AND (kry_jabcode ILIKE '%SPR%' OR kry_jabcode ILIKE '%SOPIR%' OR kry_jabcode ILIKE '%DRIVER%')").
		Order("kry_nama ASC").
		Find(&drivers)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"fleet":   vehicles,
		"drivers": drivers,
	})
}

// =========================================================================
// 📦 2. ENDPOINT POOL RESI: AMBIL DAFTAR BTT YANG SIAP BERANGKAT
// =========================================================================
func GetPoolBTT(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	if fmt.Sprintf("%v", ptID) == "<nil>" || fmt.Sprintf("%v", ptID) == "" {
		ptID = "A"
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database tenant gagal"})
		return
	}

	currentAgen := strings.ToUpper(strings.TrimSpace(c.Query("agen_id")))
	if currentAgen == "" || currentAgen == "ALL" || strings.Contains(currentAgen, "PUSAT") {
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": []interface{}{}})
		return
	}

	// Resolusi mencari 3 huruf inisial depan kode cabang
	var matchPrefix string
	if strings.Contains(currentAgen, " ") {
		currentAgen = strings.Split(currentAgen, " ")[0]
	}

	errPrefix := database.Table("public.glb_m_agen").
		Select("LEFT(agen_id, 3)").
		Where("agen_id LIKE ? OR list_nama_agen ILIKE ? OR agen_nama ILIKE ?", currentAgen+"%", "%"+currentAgen+"%", "%"+currentAgen+"%").
		Limit(1).
		Row().Scan(&matchPrefix)

	if errPrefix != nil || matchPrefix == "" {
		matchPrefix = currentAgen
		if len(matchPrefix) > 3 {
			matchPrefix = matchPrefix[:3]
		}
	}

	// Ambil daftar resi aktif yang belum berasosiasi dengan nomor SP Naik mana pun
	var poolBTT []map[string]interface{}
	database.Table("public.mkt_t_econote").
		Select("btt_id, btt_asal_agenid, btt_tujuan_agenid, btt_service, btt_tanggal, btt_berat").
		Where("btt_id LIKE ? AND btt_id NOT IN (SELECT sptd_bttid FROM public.opr_t_esp_terimadetil)", matchPrefix+"%").
		Order("btt_id DESC").
		Find(&poolBTT)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"prefix": matchPrefix,
		"data":   poolBTT,
	})
}

// =========================================================================
// 🚀 3. ENDPOINT EXECUTE: TRANSACTION MAKER SP NAIK (ANTI-HARDCODE)
// =========================================================================
func CreateSPNaikHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "PT ID Token tidak ditemukan"})
		return
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database tenant gagal"})
		return
	}

	type SPInput struct {
		AsalAgenNama   string   `json:"spt_asal_agen_nama" binding:"required"`
		TujuanAgenNama string   `json:"spt_tujuan_agen_nama" binding:"required"`
		TransitYN      string   `json:"spt_transityn" binding:"required"`
		NamaSopir      string   `json:"spt_namasopir" binding:"required"`
		NoMobil        string   `json:"spt_nomobil" binding:"required"`
		SuratTugas     string   `json:"spt_surattugas"`
		BoronganYN     string   `json:"spt_boronganyn"`
		Service        int      `json:"spt_service" binding:"required"`
		DaftarBTT      []string `json:"daftar_btt" binding:"required"`
	}

	var input SPInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload input tidak valid: " + err.Error()})
		return
	}

	if len(input.DaftarBTT) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Rincian barang kosong! Wajib memasukkan minimal 1 nomor resi/BTT."})
		return
	}

	// A. RESOLUSI KODE ASAL AGEN & KODE CABANG DB MURNI
	var asalAgenID, tujuanAgenID int
	var asalCabangID string

	cleanAsal := input.AsalAgenNama
	if strings.Contains(cleanAsal, " ") {
		cleanAsal = strings.Split(cleanAsal, " ")[0]
	}

	cleanTujuan := input.TujuanAgenNama
	if strings.Contains(cleanTujuan, " ") {
		cleanTujuan = strings.Split(cleanTujuan, " ")[0]
	}

	// Cari ID dan kode cabang asal
	database.Table("public.glb_m_agen").
		Select("agen_id, LEFT(agen_id, 3)").
		Where("agen_nama ILIKE ? OR agen_id LIKE ?", "%"+cleanAsal+"%", cleanAsal+"%").
		Row().Scan(&asalAgenID, &asalCabangID)

	// Cari ID cabang tujuan
	database.Table("public.glb_m_agen").
		Select("agen_id").
		Where("agen_nama ILIKE ? OR agen_id LIKE ?", "%"+cleanTujuan+"%", cleanTujuan+"%").
		Row().Scan(&tujuanAgenID)

	// Safety fallback jika tidak terelasi
	asalCabangID = strings.ToUpper(strings.TrimSpace(asalCabangID))
	if len(asalCabangID) > 3 {
		asalCabangID = asalCabangID[:3]
	}
	if asalCabangID == "" {
		asalCabangID = "DBS"
	}

	// B. FORMULA NOMOR SP GENERATOR OTOMATIS BERDASARKAN SOP RESMI
	now := time.Now()
	bulanTahunStr := now.Format("012006") // Format: MMYYYY (Contoh: "062026")

	// Pola Prefix Utama (Contoh: "SDBS" + "URW" + "062026" = "SDBSURW062026")
	spPrefixPattern := fmt.Sprintf("S%s%s%s", fmt.Sprintf("%v", ptID), asalCabangID, bulanTahunStr)

	// Validasi Periode Closing Akuntansi Lintas Wilayah Nusantara
	var isClosed string
	database.Table("public.glb_m_closing").
		Select("closing_yn").
		Where("periode_bulan = ? AND periode_tahun = ? AND (closing_yn = 'Y' OR closing_yn = '1')", int(now.Month()), now.Year()).
		Limit(1).
		Row().Scan(&isClosed)

	if isClosed == "Y" || isClosed == "1" {
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Transaksi diblokir! Periode akuntansi untuk bulan ini sudah dikunci/closing oleh pusat."})
		return
	}

	// Hitung urutan counter nomor terakhir dari database
	var lastSPID string
	database.Table("public.opr_t_esp_terima").
		Select("spt_eid").
		Where("spt_eid LIKE ?", spPrefixPattern+"%").
		Order("spt_eid DESC").
		Limit(1).
		Row().Scan(&lastSPID)

	nextCounter := 1
	prefixLen := len(spPrefixPattern)

	if lastSPID != "" && len(lastSPID) > prefixLen {
		suffixStr := lastSPID[prefixLen:]
		var currentNo int
		fmt.Sscanf(suffixStr, "%d", &currentNo)
		nextCounter = currentNo + 1
	}

	// Hasil Akhir Nomor SP Komplit Sesuai Dokumen (Contoh: SDBSURW0620260001)
	finalSPID := fmt.Sprintf("%s%04d", spPrefixPattern, nextCounter)

	// C. EKSEKUSI DATABASE TRANSACTION (INSERT HEADER & DETAIL RESI)
	tx := database.Begin()

	username := "SYSTEM_OPERASIONAL"
	if claimsVal, exists := c.Get("user_data"); exists {
		if claims, ok := claimsVal.(jwt.MapClaims); ok {
			if user, ok := claims["username"].(string); ok {
				username = user
			}
		}
	}

	headerSP := map[string]interface{}{
		"spt_eid":          finalSPID,
		"spt_asalagenid":   asalAgenID,
		"spt_tujuanagenid": tujuanAgenID,
		"spt_transityn":    strings.ToUpper(input.TransitYN),
		"spt_tanggal":      time.Now(),
		"spt_namasopir":    strings.ToUpper(input.NamaSopir),
		"spt_nomobil":      strings.ToUpper(input.NoMobil),
		"spt_surattugas":   strings.ToUpper(input.SuratTugas),
		"spt_postingyn":    "N",
		"spt_aktifyn":      "Y",
		"spt_updateid":     username,
		"spt_updatetime":   time.Now(),
		"spt_boronganyn":   strings.ToUpper(input.BoronganYN),
		"spt_service":      input.Service,
	}

	if err := tx.Table("public.opr_t_esp_terima").Create(&headerSP).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan header SP Naik: " + err.Error()})
		return
	}

	// Looping insert detail manifest BTT pengiriman barang
	for _, bttID := range input.DaftarBTT {
		detailSP := map[string]interface{}{
			"sptd_esptid": finalSPID,
			"sptd_bttid":  strings.ToUpper(strings.TrimSpace(bttID)),
		}
		if err := tx.Table("public.opr_t_esp_terimadetil").Create(&detailSP).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan rincian manifes BTT: " + err.Error()})
			return
		}
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Dokumen Surat Pengantar Naik (SP Naik) Berhasil Diterbitkan!",
		"spt_eid": finalSPID,
	})
}
