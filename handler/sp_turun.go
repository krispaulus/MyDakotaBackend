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
// 🔍 1. ENDPOINT AJAX PREVIEW: DETEKSI DOKUMEN & MULTI-DB CHECKER
// =========================================================================
func GetDetailSPNaik(c *gin.Context) {
	// Ambil tenant active dari token context
	ptID, _ := c.Get("pt_id")
	currentTenant := fmt.Sprintf("%v", ptID)
	if currentTenant == "<nil>" || currentTenant == "" {
		currentTenant = "A"
	}

	spID := strings.ToUpper(strings.TrimSpace(c.Query("sp_id")))
	if len(spID) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor SP tidak valid!"})
		return
	}

	// 🎯 ATURAN BISNIS 8.1 & 8.2: Identifikasi Database Asal via Karakter ke-2 Nomor SP
	dbCode := string(spID[1]) // Misal: S'D'BS... atau S'A'BS...
	var targetDB string

	switch dbCode {
	case "A":
		targetDB = "A" // dbs
	case "B":
		targetDB = "B" // dlb
	case "C":
		targetDB = "C" // logistik
	default:
		targetDB = currentTenant // Fallback ke tenant saat ini
	}

	// Koneksi ke Database Asal Dokumen SP
	sourceDatabase, ok := db.ResolveDB(targetDB)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal terhubung ke database asal SP: " + targetDB})
		return
	}

	// Koneksi ke Database Tenant saat ini (Penerima) untuk cek proteksi lokal
	currentDatabase, _ := db.ResolveDB(currentTenant)

	// Ambil Informasi Header SP Naik dari Database Asal
	var headerSP map[string]interface{}
	sourceDatabase.Table("public.opr_t_esp_terima").
		Select("spt_eid, spt_asalagenid, spt_tujuanagenid, spt_nomobil, spt_namasopir, spt_surattugas, spt_transityn").
		Where("spt_eid = ? AND spt_aktifyn = 'Y'", spID).
		Take(&headerSP)

	if len(headerSP) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Dokumen SP Naik tidak ditemukan di cluster database kargo!"})
		return
	}

	// Ambil Rincian Baris Resi BTT dari Database Asal
	var detailsBTT []map[string]interface{}
	sourceDatabase.Table("public.opr_t_esp_terimadetil").
		Select("sptd_bttid AS btt_id").
		Where("sptd_esptid = ?", spID).
		Find(&detailsBTT)

	// 🎯 ATURAN BISNIS 8.3: Filter Proteksi dengan Tabel Berita Acara Barang Lebih (BBL)
	var finalBTTList []map[string]interface{}
	for _, btt := range detailsBTT {
		bttID := fmt.Sprintf("%v", btt["btt_id"])

		var isBBLCount int64
		currentDatabase.Table("public.opr_t_ebbl").
			Where("bbl_bttid = ? AND bbl_aktifyn = 'Y' AND bbl_terimayn = 'Y'", bttID).
			Count(&isBBLCount)

		// Jika sudah diproses BBL Terima Y, lewati resi ini dari manifest
		if isBBLCount > 0 {
			continue
		}

		// Cek apakah resi ini sudah pernah di-SP Turun-kan sebelumnya
		var isAlreadyTurun int64
		currentDatabase.Table("public.opr_t_esp_turun").
			Where("sp_eid = ? AND sp_bttid = ? AND sp_aktifyn = 'Y'", spID, bttID).
			Count(&isAlreadyTurun)

		btt["sudah_turun"] = isAlreadyTurun > 0
		finalBTTList = append(finalBTTList, btt)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"header":  headerSP,
		"details": finalBTTList,
	})
}

// =========================================================================
// 📥 2. ENDPOINT INITIAL SAVER: COPY MANIFEST KE TABEL SP TURUN
// =========================================================================
func SaveInitialSPTurun(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	currentTenant := fmt.Sprintf("%v", ptID)
	currentDatabase, _ := db.ResolveDB(currentTenant)

	type InitialInput struct {
		SPEID     string   `json:"sp_eid" binding:"required"`
		AgenID    int      `json:"sp_agenid" binding:"required"`
		TransitYN string   `json:"spt_transityn"`
		DaftarBTT []string `json:"daftar_btt" binding:"required"`
	}

	var input InitialInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	username := "SYSTEM_OPERASIONAL"
	if claimsVal, exists := c.Get("user_data"); exists {
		if claims, ok := claimsVal.(jwt.MapClaims); ok {
			if user, ok := claims["username"].(string); ok {
				username = user
			}
		}
	}

	// 🎯 ATURAN BISNIS 8.4: Tentukan status urut tracking eHistory (2 = Transit, 6 = Langsung)
	statusUrutLogistik := 6
	statusText := "Barang telah tiba di cabang tujuan"
	if strings.ToUpper(input.TransitYN) == "Y" {
		statusUrutLogistik = 2
		statusText = "Barang singgah di cabang transit"
	}

	tx := currentDatabase.Begin()

	for _, bttID := range input.DaftarBTT {
		// Cek Duplikasi Baris
		var existingCount int64
		tx.Table("public.opr_t_esp_turun").Where("sp_eid = ? AND sp_bttid = ?", input.SPEID, bttID).Count(&existingCount)

		if existingCount > 0 {
			continue
		}

		// 1. Generate Log Tracking ke Database Sentral ehistorydb
		historyLog := map[string]interface{}{
			"hist_bttid":      bttID,
			"hist_tanggal":    time.Now(),
			"hist_statusurut": statusUrutLogistik,
			"hist_agenid":     input.AgenID,
			"hist_updateid":   username,
			"hist_keterangan": statusText,
		}

		if err := tx.Table("public.mkt_t_ehistory").Create(&historyLog).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menulis log tracking eHistorySentral: " + err.Error()})
			return
		}

		// Ambil ID log yang baru saja ter-insert
		var latestHistID int64
		tx.Table("public.mkt_t_ehistory").Select("hist_id").Where("hist_bttid = ?", bttID).Order("hist_id DESC").Limit(1).Row().Scan(&latestHistID)

		// 2. Insert ke Tabel Utama SP Turun Lokal Tenant
		rowTurun := map[string]interface{}{
			"sp_eid":        input.SPEID,
			"sp_bttid":      bttID,
			"sp_agenid":     input.AgenID,
			"sp_tanggal":    time.Now(),
			"sp_aktifyn":    "Y",
			"sp_jmlterima":  0, // Awal muatan diset 0, nanti di-update realtime via AJAX
			"sp_keterangan": "Bongkar muatan berjalan...",
			"sp_updateid":   username,
			"sp_updatetime": time.Now(),
			"sp_histid":     fmt.Sprintf("%d", latestHistID),
		}

		if err := tx.Table("public.opr_t_esp_turun").Create(&rowTurun).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal inisialisasi manifes SP Turun: " + err.Error()})
			return
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Manifes SP Turun resmi dibuka. Silakan lakukan pencatatan fisik koli!"})
}

// =========================================================================
// ⚡ 3. ENDPOINT REAL-TIME AJAX AUTOSAVE: SIMPAN LANGSUNG PER BARIS RESI
// =========================================================================
func AutoSaveRowSPTurun(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	currentTenant := fmt.Sprintf("%v", ptID)
	currentDatabase, _ := db.ResolveDB(currentTenant)

	type AutoSaveInput struct {
		SPEID        string `json:"sp_eid" binding:"required"`
		SPBTTID      string `json:"sp_bttid" binding:"required"`
		SPJmlTerima  int    `json:"sp_jmlterima"`
		SPKeterangan string `json:"sp_keterangan"`
	}

	var input AutoSaveInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	username := "SYSTEM_OPERASIONAL"
	if claimsVal, exists := c.Get("user_data"); exists {
		if claims, ok := claimsVal.(jwt.MapClaims); ok {
			if user, ok := claims["username"].(string); ok {
				username = user
			}
		}
	}

	// 🎯 ATURAN BISNIS 8.7: Update Data Secara Instan Langsung ke Kolom Baris Target via AJAX
	errUpdate := currentDatabase.Table("public.opr_t_esp_turun").
		Where("sp_eid = ? AND sp_bttid = ?", input.SPEID, input.SPBTTID).
		Updates(map[string]interface{}{
			"sp_jmlterima":  input.SPJmlTerima,
			"sp_keterangan": input.SPKeterangan,
			"sp_updateid":   username,
			"sp_updatetime": time.Now(),
		}).Error

	if errUpdate != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal auto-save data manifest baris resi: " + errUpdate.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Row synchronized dynamically",
		"btt_id":  input.SPBTTID,
	})
}

// =========================================================================
// 🔎 4. ENDPOINT HISTORY LISTER: MENAMPILKAN DATA DENGAN QUERY FILTER DYNAMIC
// =========================================================================
func GetHistorySPTurun(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	currentTenant := fmt.Sprintf("%v", ptID)
	currentDatabase, _ := db.ResolveDB(currentTenant)

	tglAwal := c.Query("tgl_awal")
	tglAkhir := c.Query("tgl_akhir")

	// 🎯 FIX FINAL SAPU JAGAT: casting sp_tanggal ke timestamp agar TO_CHAR tidak mogok!
	query := `SELECT
            t.sp_eid,
            TO_CHAR(t.sp_tanggal::timestamp, 'YYYY-MM-DD HH24:MI:SS') AS sp_tanggal,
            t.sp_aktifyn,
            COUNT(t.sp_bttid) AS jumlah_btt,
            COALESCE(h.spt_transityn, 'N') AS spt_transityn,
            COALESCE(a_asal.agen_nama, 'CABANG ASAL X') AS cabang_asal_nama,
            COALESCE(a_tuj.agen_nama, 'CABANG TUJUAN Y') AS cabang_tujuan_nama
         FROM public.opr_t_esp_turun AS t 
         LEFT JOIN public.opr_t_esp_terima h ON t.sp_eid::text = h.spt_eid::text 
         LEFT JOIN public.glb_m_agen a_asal ON h.spt_asalagenid::text = a_asal.agen_id::text 
         LEFT JOIN public.glb_m_agen a_tuj ON t.sp_agenid::text = a_tuj.agen_id::text 
         WHERE t.sp_tanggal::timestamp::date BETWEEN $1 AND $2 
         GROUP BY t.sp_eid, t.sp_tanggal, t.sp_aktifyn, h.spt_transityn, a_asal.agen_nama, a_tuj.agen_nama 
         ORDER BY t.sp_tanggal DESC`

	var results []map[string]interface{}
	err := currentDatabase.Raw(query, tglAwal, tglAkhir).Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if len(results) == 0 {
		c.JSON(http.StatusOK, []map[string]interface{}{})
		return
	}

	c.JSON(http.StatusOK, results)
}

// =========================================================================
// 🖨️ ENDPOINT CETAK MANIFEST: AMBIL DETAIL CETAK SURAT PENGANTAR PENGIRIMAN
// =========================================================================
func GetPrintSPDetail(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	currentTenant := fmt.Sprintf("%v", ptID)
	currentDatabase, _ := db.ResolveDB(currentTenant)

	spID := c.Param("id")

	// 1. Ambil Data Header Manifest SP[cite: 8]
	var header map[string]interface{}
	headerQuery := `
		SELECT 
			l.spt_eid, l.spt_service, l.spt_surattugas, l.spt_notif,
			TO_CHAR(l.spt_tanggal::timestamp, 'YYYY-MM-DD HH24:MI:SS') AS tanggal_sp,
			a_asal.agen_nama AS asal_agen, a_asal.agen_kota AS asal_kota, a_asal.agen_alamat AS asal_alamat,
			a_tujuan.agen_nama AS tujuan_agen, a_tujuan.agen_kota AS tujuan_kota,
			COALESCE(k_sopir.kry_nama, l.spt_namasopir) AS sopir_nama,
			COALESCE(l.spt_nomobil, 'N/A') AS no_mobil
		FROM public.opr_t_esp_terima l
		LEFT JOIN public.glb_m_agen a_asal ON l.spt_asalagenid::text = a_asal.agen_id::text
		LEFT JOIN public.glb_m_agen a_tujuan ON l.spt_tujuanagenid::text = a_tujuan.agen_id::text
		LEFT JOIN public.hrd_m_karyawan k_sopir ON l.spt_nipsopir::text = k_sopir.kry_nip::text
		WHERE l.spt_eid = $1`

	if err := currentDatabase.Raw(headerQuery, spID).Scan(&header).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat header cetakan: " + err.Error()})
		return
	}

	// 2. Ambil Daftar Baris BTT di Dalam Manifest SP Terkait[cite: 8]
	var items []map[string]interface{}
	itemsQuery := `
		SELECT 
			d.sptd_bttid AS btt_id,
			COALESCE(c.bttt_nosuratjalan, '-') AS no_surat_jalan,
			COALESCE(c.bttt_tagihtujuan::numeric, 0) AS nilai_cod,
			CASE 
				WHEN c.bttt_pembayaran = 1 THEN 'TUNAI'
				WHEN c.bttt_pembayaran = 2 THEN 'KREDIT'
				WHEN c.bttt_pembayaran = 3 THEN 'TAGIH TUJUAN'
				WHEN c.bttt_pembayaran = 6 THEN 'TRANSFER'
				ELSE 'N/A'
			END AS tipe_pembayaran,
			CASE 
				WHEN c.bttt_servid = 1 THEN 'Darat'
				WHEN c.bttt_servid = 2 THEN 'Laut'
				WHEN c.bttt_servid = 3 THEN 'Udara'
				ELSE 'N/A'
			END AS jenis_layanan,
			COALESCE(c.bttt_asalname, '-') AS nama_pengirim,
			COALESCE(c.bttt_tujuannama, '-') AS nama_penerima,
			COALESCE(c.bttt_tujuankota, '-') AS kota_penerima,
			COALESCE(c.bttt_jmlunit::integer, 0) AS koli,
			-- Aturan Bisnis: Ambil nilai tertinggi antara berat asli vs berat volume[cite: 8]
			CASE 
				WHEN COALESCE(c.bttt_berat::numeric, 0) > COALESCE(c.bttt_beratvol::numeric, 0) THEN COALESCE(c.bttt_berat::numeric, 0)
				ELSE COALESCE(c.bttt_beratvol::numeric, 0)
			END AS berat_tertinggi,
			COALESCE(c.bttt_harga::numeric, 0) + COALESCE(c.bttt_biayapenerus::numeric, 0) AS biaya_kirim
		FROM public.opr_t_esp_terimadetil d
		LEFT JOIN public.mkt_t_econote c ON d.sptd_bttid::text = c.bttt_id::text
		WHERE d.sptd_esptid = $1
		ORDER BY d.sptd_bttid ASC`

	if err := currentDatabase.Raw(itemsQuery, spID).Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat item cetakan: " + err.Error()})
		return
	}

	// 3. Gabungkan Output Response[cite: 8]
	c.JSON(http.StatusOK, gin.H{
		"header": header,
		"items":  items,
	})
}
