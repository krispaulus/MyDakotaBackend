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

type UpdateBTTPayload struct {
	NoBTT          string `json:"no_btt" binding:"required"`
	PenerimaNama   string `json:"penerima_nama"`
	PenerimaAlamat string `json:"penerima_alamat"`
	PenerimaTelp   string `json:"penerima_telp"`
	NoSuratJalan   string `json:"no_surat_jalan"`
	Keterangan     string `json:"keterangan"`
	MetodeBayar    string `json:"metode_bayar"`

	// Field Khusus Keuangan / Tarif
	BiayaKirim     float64 `json:"biaya_kirim"`
	BiayaPenerus   float64 `json:"biaya_penerus"`
	BiayaPacking   float64 `json:"biaya_packing"`
	TotalBiaya     float64 `json:"total_biaya"`
	AlasanOverride string  `json:"alasan_override"` // Wajib jika supervisor bypass harga
}

func UpdateBTT(c *gin.Context) {
	db := getTarifCustomerDB(c)

	// Ambil role user dari JWT / Context
	userRole := strings.ToUpper(fmt.Sprintf("%v", c.MustGet("role_akses")))
	userID := fmt.Sprintf("%v", c.MustGet("user_id"))

	var req UpdateBTTPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// 1. Ambil data eksisting BTT untuk perbandingan
	var existingBTT struct {
		BiayaKirim float64
		TotalBiaya float64
	}
	err := db.Table("public.mkt_t_btt").
		Select("biaya_kirim, total_biaya").
		Where("no_btt = ?", req.NoBTT).
		Scan(&existingBTT).Error

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data BTT tidak ditemukan"})
		return
	}

	// 2. Proteksi Wewenang: Jika bukan Supervisor/Manager tapi mencoba ubah harga
	isSupervisor := userRole == "SUPERVISOR" || userRole == "MANAGER" || userRole == "SUPERADMIN"
	priceChanged := (req.BiayaKirim != existingBTT.BiayaKirim) || (req.TotalBiaya != existingBTT.TotalBiaya)

	if priceChanged && !isSupervisor {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Akses Ditolak: Hanya Supervisor yang berhak melakukan bypass/edit harga BTT.",
		})
		return
	}

	tx := db.Begin()

	// 3. Update Data BTT
	updateData := map[string]interface{}{
		"penerima_nama":   req.PenerimaNama,
		"penerima_alamat": req.PenerimaAlamat,
		"penerima_telp":   req.PenerimaTelp,
		"no_surat_jalan":  req.NoSuratJalan,
		"keterangan":      req.Keterangan,
		"metode_bayar":    req.MetodeBayar,
	}

	// Hanya update harga jika Supervisor yang mengubahnya
	if isSupervisor {
		updateData["biaya_kirim"] = req.BiayaKirim
		updateData["biaya_penerus"] = req.BiayaPenerus
		updateData["biaya_packing"] = req.BiayaPacking
		updateData["total_biaya"] = req.TotalBiaya
		updateData["is_price_overridden"] = priceChanged

		// Catat ke Audit Log jika harga diubah manual
		if priceChanged {
			logEntry := map[string]interface{}{
				"no_btt":          req.NoBTT,
				"user_id":         userID,
				"biaya_awal":      existingBTT.BiayaKirim,
				"biaya_baru":      req.BiayaKirim,
				"alasan_override": req.AlasanOverride,
			}
			tx.Table("public.mkt_t_btt_price_audit").Create(&logEntry)
		}
	}

	if err := tx.Table("public.mkt_t_btt").Where("no_btt = ?", req.NoBTT).Updates(updateData).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data BTT berhasil diperbarui",
	})
}

func GetBTT(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = "C"
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	// 🟢 Tangkap seluruh parameter filter dari React
	agenID := strings.TrimSpace(c.Query("agen_id"))
	cabang := strings.TrimSpace(c.Query("cabang"))
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	bypassTanggal := c.Query("bypass_tanggal") == "Y" || c.Query("bypass_tanggal") == "true"
	jenisBTT := strings.TrimSpace(c.Query("jenis_btt"))
	customer := strings.TrimSpace(c.Query("customer"))
	noBTT := strings.TrimSpace(c.Query("no_btt"))
	kota := strings.TrimSpace(c.Query("kota"))

	// 🌟 CEK APAKAH USER ADALAH PUSAT / HOLDING
	upperAgen := strings.ToUpper(agenID)
	isHoldingUser := upperAgen == "1" || upperAgen == "839" || upperAgen == "001" ||
		strings.Contains(upperAgen, "PUSAT") || strings.Contains(upperAgen, "HOLDING") ||
		agenID == ""

	type BTTGridRow struct {
		ID          string    `json:"id" gorm:"column:id"`
		BtttID      string    `json:"bttt_id" gorm:"column:id"`
		Tanggal     time.Time `json:"tanggal" gorm:"column:tanggal"`
		BtttTanggal string    `json:"bttt_tanggal" gorm:"column:bttt_tanggal_str"`

		// Pengirim (Dua format tag agar tabel & form modal sama-sama terbaca)
		AsalCustID     string `json:"asal_custid" gorm:"column:asal_custid"`
		BtttAsalCustID string `json:"bttt_asalcustid" gorm:"column:asal_custid"`
		AsalName       string `json:"asal_name" gorm:"column:asal_name"`
		BtttAsalName   string `json:"bttt_asalname" gorm:"column:asal_name"`
		AsalAlamat     string `json:"asal_alamat" gorm:"column:asal_alamat"`
		BtttAsalAlamat string `json:"bttt_asalalamat" gorm:"column:asal_alamat"`
		AsalKota       string `json:"asal_kota" gorm:"column:asal_kota"`
		BtttAsalKota   string `json:"bttt_asalkota" gorm:"column:asal_kota"`
		AsalTelp       string `json:"asal_telp" gorm:"column:asal_telp"`
		BtttAsalTelp   string `json:"bttt_asaltelp" gorm:"column:asal_telp"`

		// Penerima
		TujuanCustID     string `json:"tujuan_custid" gorm:"column:tujuan_custid"`
		BtttTujuanCustID string `json:"bttt_tujuancustid" gorm:"column:tujuan_custid"`
		TujuanNama       string `json:"tujuan_nama" gorm:"column:tujuan_nama"`
		BtttTujuanNama   string `json:"bttt_tujuannama" gorm:"column:tujuan_nama"`
		TujuanAlamat     string `json:"tujuan_alamat" gorm:"column:tujuan_alamat"`
		BtttTujuanAlamat string `json:"bttt_tujuanalamat" gorm:"column:tujuan_alamat"`
		TujuanKota       string `json:"tujuan_kota" gorm:"column:tujuan_kota"`
		BtttTujuanKota   string `json:"bttt_tujuankota" gorm:"column:tujuan_kota"`
		TujuanKecamatan  string `json:"tujuan_kecamatan" gorm:"column:tujuan_kecamatan"`
		BtttTujuanKec    string `json:"bttt_tujuankecamatan" gorm:"column:tujuan_kecamatan"`
		TujuanKelurahan  string `json:"tujuan_kelurahan" gorm:"column:tujuan_kelurahan"`
		BtttTujuanKel    string `json:"bttt_tujuankelurahan" gorm:"column:tujuan_kelurahan"`
		TujuanKodePos    string `json:"tujuan_kodepos" gorm:"column:tujuan_kodepos"`
		BtttTujuanKode   string `json:"bttt_tujuankodepos" gorm:"column:tujuan_kodepos"`
		TujuanPropinsi   string `json:"tujuan_propinsi" gorm:"column:tujuan_propinsi"`
		BtttTujuanProp   string `json:"bttt_tujuanpropinsi" gorm:"column:tujuan_propinsi"`
		TujuanTelp       string `json:"tujuan_telp" gorm:"column:tujuan_telp"`
		BtttTujuanTelp   string `json:"bttt_tujuantelp" gorm:"column:tujuan_telp"`
		UP               string `json:"up" gorm:"column:up"`
		BtttUP           string `json:"bttt_up" gorm:"column:up"`

		// Rincian Muatan & Biaya
		NamaBarang         string  `json:"nama_barang" gorm:"column:nama_barang"`
		BtttNamaBarang     string  `json:"bttt_namabarang" gorm:"column:nama_barang"`
		Harga              float64 `json:"harga" gorm:"column:harga"`
		BtttHarga          float64 `json:"bttt_harga" gorm:"column:harga"`
		CreateBy           string  `json:"create_by" gorm:"column:create_by"`
		NoSuratJalan       string  `json:"no_surat_jalan" gorm:"column:no_surat_jalan"`
		BtttNoSuratJalan   string  `json:"bttt_nosuratjalan" gorm:"column:no_surat_jalan"`
		Keterangan         string  `json:"keterangan" gorm:"column:keterangan"`
		BtttKet            string  `json:"bttt_ket" gorm:"column:keterangan"`
		JumlahKoli         int     `json:"jumlah_koli" gorm:"column:jumlah_koli"`
		BtttJmlKoli        int     `json:"bttt_jmlkoli" gorm:"column:jumlah_koli"`
		Berat              float64 `json:"berat" gorm:"column:berat"`
		BtttBerat          float64 `json:"bttt_berat" gorm:"column:berat"`
		BeratVolume        float64 `json:"berat_volume" gorm:"column:berat_volume"`
		BtttBeratVol       float64 `json:"bttt_beratvol" gorm:"column:berat_volume"`
		Kubikasi           float64 `json:"kubikasi" gorm:"column:kubikasi"`
		BiayaPenerus       float64 `json:"biaya_penerus" gorm:"column:biaya_penerus"`
		BiayaPacking       float64 `json:"biaya_packing" gorm:"column:biaya_packing"`
		MetodePembayaran   string  `json:"metode_pembayaran" gorm:"column:metode_pembayaran"`
		JenisLayanan       string  `json:"jenis_layanan" gorm:"column:jenis_layanan"`
		AgenNama           string  `json:"agen_nama" gorm:"column:agen_nama"`
		AsalAgenID         string  `json:"asal_agen_id" gorm:"column:asal_agen_id"`
		BtttAsalAgenID     string  `json:"bttt_asalagenid" gorm:"column:asal_agen_id"`
		TujuanAgenID       string  `json:"tujuan_agen_id" gorm:"column:tujuan_agen_id"`
		BtttTujuanAgenID   string  `json:"bttt_tujuanagenid" gorm:"column:tujuan_agen_id"`
		BtttPaketYN        string  `json:"bttt_paketyn" gorm:"column:bttt_paketyn"`
		BtttCharterJnsKend string  `json:"bttt_charterjnskend" gorm:"column:bttt_charterjnskend"`
	}

	// 🌟 QUERY DETAIL BTT DENGAN AUTO-JOIN MASTER CUSTOMER PENGIRIM
	query := database.Table("public.mkt_t_econote e").
		Select(`
			TRIM(BOTH FROM e.bttt_id::varchar) AS id,
			e.bttt_tanggal AS tanggal,
			TO_CHAR(e.bttt_tanggal, 'YYYY-MM-DD') AS bttt_tanggal_str,
			
			-- 🌟 IDENTITAS PENGIRIM (Fallback otomatis ambil dari master mkt_m_customer)
			COALESCE(NULLIF(e.bttt_asalcustid, ''), c.cust_id, '') AS asal_custid,
			COALESCE(NULLIF(e.bttt_asalname, ''), c.cust_name, 'UMUM') AS asal_name,
			COALESCE(NULLIF(e.bttt_asalalamat, ''), c.cust_alamat1, '') AS asal_alamat,
			COALESCE(NULLIF(e.bttt_asalkota, ''), c.cust_kotaid, 'BEKASI KAB.') AS asal_kota,
			COALESCE(NULLIF(e.bttt_asaltelp, ''), c.cust_telp1, '') AS asal_telp,

			-- IDENTITAS PENERIMA
			COALESCE(e.bttt_tujuancustid, '') AS tujuan_custid,
			COALESCE(e.bttt_tujuannama, '') AS tujuan_nama,
			COALESCE(e.bttt_tujuanalamat, '') AS tujuan_alamat,
			COALESCE(e.bttt_tujuankota, '') AS tujuan_kota,
			COALESCE(e.bttt_tujuankecamatan, '') AS tujuan_kecamatan,
			COALESCE(e.bttt_tujuankelurahan, '') AS tujuan_kelurahan,
			COALESCE(e.bttt_tujuankodepos, '') AS tujuan_kodepos,
			COALESCE(e.bttt_tujuanpulau, '') AS tujuan_propinsi,
			COALESCE(e.bttt_tujuantelp, '') AS tujuan_telp,
			COALESCE(NULLIF(e.bttt_up, ''), c.cust_name, e.bttt_asalname, '') AS up,
			COALESCE(e.bttt_paketyn, 'Y') AS bttt_paketyn,
			COALESCE(e.bttt_charterjnskend, '') AS bttt_charterjnskend,

			-- BARANG & TARIF
			COALESCE(e.bttt_namabarang, '') AS nama_barang,
			COALESCE(e.bttt_harga, 0) AS harga,
			CASE 
				WHEN e.bttt_updateid IS NOT NULL AND e.bttt_updateid <> '' AND e.bttt_updateid !~ '^[0-9]+$' THEN e.bttt_updateid
				WHEN e.bttt_up IS NOT NULL AND e.bttt_up <> '' THEN e.bttt_up
				ELSE 'STAFF'
			END AS create_by,
			COALESCE(e.bttt_nosuratjalan, '') AS no_surat_jalan,
			COALESCE(e.bttt_ket, '') AS keterangan,
			COALESCE(e.bttt_jmlunit, 1) AS jumlah_koli,
			COALESCE(e.bttt_berat, 0) AS berat,
			COALESCE(e.bttt_beratvol, 0) AS berat_volume,
			COALESCE(NULLIF(regexp_replace(COALESCE(e.bttt_ukuran::text, '0'), '[^0-9.]', '', 'g'), '')::numeric, 0) AS kubikasi,
			COALESCE(e.bttt_biayapenerus, 0) AS biaya_penerus,
			COALESCE(p.pck_biaya, 0) AS biaya_packing,
			CASE 
				WHEN e.bttt_jenisharga = '0' THEN 'TUNAI'
				WHEN e.bttt_jenisharga = '2' THEN 'KREDIT'
				ELSE 'TAGIH TUJUAN'
			END AS metode_pembayaran,
			CASE 
				WHEN e.bttt_paketyn = 'Y' THEN 'REGULER'
				ELSE 'EKONOMIS'
			END AS jenis_layanan,
			COALESCE(a.agen_nama, 'DLI PUSAT') AS agen_nama,
			COALESCE(e.bttt_asalagenid, '') AS asal_agen_id,
			COALESCE(e.bttt_tujuanagenid, '') AS tujuan_agen_id
		`).
		Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id::varchar) = TRIM(e.bttt_asalcustid::varchar)"). // 👈 JOIN OTOMATIS DATA PROFIL PENGIRIM
		Joins("LEFT JOIN public.pck_t_packing p ON TRIM(p.pck_id::varchar) = TRIM(e.bttt_packingid::varchar)").
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(e.bttt_asalagenid::varchar)").
		Where("COALESCE(e.bttt_aktifyn, 'Y') = 'Y'")

	// 1. Filter Tanggal
	if !bypassTanggal && startDate != "" && endDate != "" {
		query = query.Where("e.bttt_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}

	// 2. Filter Cabang (Jika memilih cabang tertentu di dropdown)
	if cabang != "" && cabang != "-- SEMUA CABANG --" && cabang != "ALL" {
		query = query.Where(`(
			TRIM(BOTH FROM e.bttt_asalagenid::varchar) = TRIM(?) 
			OR a.agen_kode = ? 
			OR a.agen_nama ILIKE ?
		)`, cabang, cabang, "%"+cabang+"%")
	} else if !isHoldingUser && agenID != "" {
		// Jika BUKAN pusat (misal agen cabang login), batasi ke cabangnya sendiri
		query = query.Where(`(
			TRIM(BOTH FROM e.bttt_asalagenid::varchar) = TRIM(?) 
			OR a.agen_kode = ?
		)`, agenID, agenID)
	}
	// Catatan: Jika isHoldingUser dan memilih "-- SEMUA CABANG --", filter cabang dilewati agar ke-168 data BTT muncul semua!

	// 3. Filter Jenis Pembayaran (0=Tunai, 1=Tagih Turun, 2=Kredit)
	if jenisBTT != "" {
		query = query.Where("e.bttt_jenisharga = ?", jenisBTT)
	}

	// 4. Filter Customer / Pengirim
	if customer != "" {
		query = query.Where("(e.bttt_asalname ILIKE ? OR e.bttt_asalcustid ILIKE ?)", "%"+customer+"%", "%"+customer+"%")
	}

	// 5. Filter Nomor Resi BTT
	if noBTT != "" {
		query = query.Where("e.bttt_id ILIKE ?", "%"+noBTT+"%")
	}

	// 6. Filter Kota Tujuan
	if kota != "" {
		query = query.Where("(e.bttt_tujuankota ILIKE ? OR e.bttt_tujuankecamatan ILIKE ?)", "%"+kota+"%", "%"+kota+"%")
	}

	var list []BTTGridRow
	err := query.Order("e.bttt_tanggal DESC, e.bttt_id DESC").Limit(500).Scan(&list).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

func CheckLockBTT(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PT ID tidak ditemukan di token"})
		return
	}

	agenID := strings.TrimSpace(c.Query("agen_id"))
	if agenID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter agen_id wajib diisi, bro!"})
		return
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	// =========================================================================
	// 🛑 LAPISAN 0: PROTEKSI PUSAT DAKOTA (DENGAN BYPASS TOGGLE PARAMETER)
	// =========================================================================
	upperAgen := strings.ToUpper(agenID)
	var agenNama string
	database.Table("public.glb_m_agen").
		Select("agen_nama").
		Where("glb_agenid = ? OR agen_kode = ?", agenID, agenID).
		Scan(&agenNama)

	upperNamaDb := strings.ToUpper(agenNama)

	isPusat := agenID == "1" || agenID == "839" ||
		upperAgen == "PUSAT DAKOTA" || strings.Contains(upperAgen, "PUSAT") || strings.Contains(upperAgen, "HOLDING") ||
		strings.Contains(upperNamaDb, "PUSAT") || strings.Contains(upperNamaDb, "HOLDING")

	if isPusat {
		// Periksa apakah izin diizinkan dari menu Settings -> Aturan Operasional
		allowPusat := isPusatAllowedCreateBTT(database)
		if !allowPusat {
			c.JSON(http.StatusOK, gin.H{
				"is_locked": true,
				"layer":     0,
				"reason":    "Pusat Dakota (Holding) tidak diizinkan untuk membuat Bukti Tanda Terima (BTT)! Silakan ganti lokasi loket ke Agen / Cabang Operasional atau aktifkan izin di menu Setting Operasional.",
			})
			return
		}
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

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

// POST /api/btt/calculate-tarif
func CalculateTarif(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = "A"
	}
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	var req models.TarifRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input data tidak valid: " + err.Error()})
		return
	}

	cleanAgenID := strings.TrimSpace(req.AgenID)
	if cleanAgenID == "" {
		cleanAgenID = strings.TrimSpace(req.AsalKota)
	}

	cleanKecTujuan := strings.ToUpper(strings.TrimSpace(req.TujuanKec))

	log.Printf("🔍 [CalculateTarif] Mencari tarif mkt_m_eharga dari Agen ID: '%s' ➡️ Kecamatan: '%s'", cleanAgenID, cleanKecTujuan)

	var regulerRow map[string]interface{}
	var ekonomisRow map[string]interface{}

	// 🎯 1. Query Master Tarif Reguler dari mkt_m_eharga (servid = 1)
	_ = database.Table("public.mkt_m_eharga").
		Where("(agenid_asal = ? OR agenid_asal ILIKE ?) AND UPPER(tujuan_kecamatan) = ? AND (servid = '1' OR servid = 'R')",
			cleanAgenID, "%"+cleanAgenID+"%", cleanKecTujuan).
		First(&regulerRow).Error

	// Jika tidak ketemu exact, coba pencarian toleran (LIKE)
	if regulerRow == nil {
		_ = database.Table("public.mkt_m_eharga").
			Where("(agenid_asal = ? OR agenid_asal ILIKE ?) AND UPPER(tujuan_kecamatan) LIKE ?",
				cleanAgenID, "%"+cleanAgenID+"%", "%"+cleanKecTujuan+"%").
			First(&regulerRow).Error
	}

	// 🎯 2. Query Master Tarif Ekonomis (servid = 2 atau tabel pendukung jika ada)
	_ = database.Table("public.mkt_m_eharga").
		Where("(agenid_asal = ? OR agenid_asal ILIKE ?) AND UPPER(tujuan_kecamatan) LIKE ? AND (servid = '2' OR servid = 'E')",
			cleanAgenID, "%"+cleanAgenID+"%", "%"+cleanKecTujuan+"%").
		First(&ekonomisRow).Error

	if regulerRow == nil && ekonomisRow == nil {
		log.Printf("⚠️ [CalculateTarif] Rute tidak ditemukan di mkt_m_eharga")
		c.JSON(http.StatusNotFound, gin.H{
			"status": "error",
			"error":  fmt.Sprintf("Waduh bro, rute pengiriman kargo dari Agen ID %s ke %s belum terdaftar di database tarif!", cleanAgenID, cleanKecTujuan),
		})
		return
	}

	// 🎯 3. Hitung Berat & Biaya
	beratChargeable := req.BeratAsli
	if req.Panjang > 0 && req.Lebar > 0 && req.Tinggi > 0 {
		vol := (req.Panjang * req.Lebar * req.Tinggi) / 4000.0
		if vol > beratChargeable {
			beratChargeable = vol
		}
	}

	targetRow := regulerRow
	if strings.ToUpper(req.JenisLayanan) == "EKONOMIS" && ekonomisRow != nil {
		targetRow = ekonomisRow
	} else if regulerRow == nil && ekonomisRow != nil {
		targetRow = ekonomisRow
	}

	var hargaPerKg, minBerat, biayaPenerus float64

	// Helper konversi interface{} ke float64 aman
	toFloat := func(val interface{}) float64 {
		if val == nil {
			return 0
		}
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int64:
			return float64(v)
		case int:
			return float64(v)
		case string:
			f, _ := strconv.ParseFloat(v, 64)
			return f
		default:
			return 0
		}
	}

	if targetRow != nil {
		hargaPerKg = toFloat(targetRow["hargapokok"])
		minBerat = toFloat(targetRow["minimalkg"])
		biayaPenerus = toFloat(targetRow["biayatambahan"])
	}

	beratFinal := beratChargeable
	if beratChargeable < minBerat {
		beratFinal = minBerat
	}

	totalHarga := beratFinal * hargaPerKg
	grandTotal := totalHarga + biayaPenerus

	c.JSON(http.StatusOK, gin.H{
		"status":           "success",
		"detected_asal":    cleanAgenID,
		"berat_chargeable": beratChargeable,
		"berat_final":      beratFinal,
		"harga_per_kg":     hargaPerKg,
		"minimum_berat":    minBerat,
		"biaya_penerus":    biayaPenerus,
		"grand_total":      grandTotal,
		"reguler_row":      regulerRow,
		"ekonomis_row":     ekonomisRow,
	})
}

func CreateBTT(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PT ID tidak ditemukan di token"})
		return
	}

	// 1. Ambil identitas user pembuat BTT
	userID, _ := c.Get("username")
	if userID == nil || fmt.Sprintf("%v", userID) == "" {
		userID = "SYSTEM"
	}

	var rawPayload map[string]interface{}
	if err := c.ShouldBindJSON(&rawPayload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload input form tidak valid: " + err.Error()})
		return
	}

	// 2. Resolve database sesuai tenant PT
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal resolved"})
		return
	}

	// 3. Cek apakah asal agen tergolong PUSAT / HOLDING
	asalAgenRaw := strings.TrimSpace(fmt.Sprintf("%v", rawPayload["bttt_asalagenid"]))
	upperAsalAgen := strings.ToUpper(asalAgenRaw)
	asalKotaRaw := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", rawPayload["bttt_asalkota"])))

	isPusat := asalAgenRaw == "1" || asalAgenRaw == "839" ||
		upperAsalAgen == "PUSAT DAKOTA" || strings.Contains(upperAsalAgen, "PUSAT") || strings.Contains(upperAsalAgen, "HOLDING") ||
		strings.Contains(asalKotaRaw, "PUSAT") || strings.Contains(asalKotaRaw, "HOLDING")

	if isPusat {
		allowPusat := isPusatAllowedCreateBTT(database)
		if !allowPusat {
			c.JSON(http.StatusForbidden, gin.H{
				"status": "error",
				"error":  "Akses Ditolak! Pusat Dakota (Holding) tidak diizinkan membuat BTT baru. Silakan aktifkan izinnya di menu Settings -> Aturan Operasional.",
			})
			return
		}
		log.Printf("🔓 [BTT Center Bypass] Unit Pusat/Holding diizinkan menerbitkan BTT berdasarkan parameter glb_m_param.")
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	userAgenID, _ := c.Get("agen_id")

	// Generate nomor BTT dinamis
	bttIDStr, errBttID := getUniqueBttID(database, rawPayload, ptID, userAgenID, now)
	if errBttID != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  errBttID.Error(),
		})
		return
	}

	// =========================================================================
	// 🎯 1. RESOLVE ASAL AGEN ID DINAMIS (STRING / VARCHAR)
	// =========================================================================
	var asalAgenIDFinal string = ""
	if rawPayload["bttt_asalagenid"] != nil {
		valStr := strings.TrimSpace(fmt.Sprintf("%v", rawPayload["bttt_asalagenid"]))
		if valStr != "" && valStr != "<nil>" && valStr != "null" {
			asalAgenIDFinal = valStr
		}
	}
	if asalAgenIDFinal == "" {
		if userAgenID, exists := c.Get("agen_id"); exists {
			asalAgenIDFinal = strings.TrimSpace(fmt.Sprintf("%v", userAgenID))
		}
	}
	if asalAgenIDFinal == "" {
		asalAgenIDFinal = "0"
	}

	// =========================================================================
	// 🎯 2. RESOLVE TUJUAN AGEN ID DINAMIS (STRING / VARCHAR)
	// =========================================================================
	var tujuanAgenIDFinal string = "0"
	if rawPayload["bttt_tujuanagenid"] != nil {
		rawTujuan := strings.TrimSpace(fmt.Sprintf("%v", rawPayload["bttt_tujuanagenid"]))
		if rawTujuan != "" && rawTujuan != "<nil>" && rawTujuan != "null" && rawTujuan != "0" {
			tujuanAgenIDFinal = rawTujuan
		}
	}

	if tujuanAgenIDFinal == "0" {
		tujuanNamaRaw := strings.TrimSpace(fmt.Sprintf("%v", rawPayload["bttt_kodecabangagen"]))
		if tujuanNamaRaw == "" || tujuanNamaRaw == "<nil>" {
			tujuanNamaRaw = strings.TrimSpace(fmt.Sprintf("%v", rawPayload["bttt_tujuannama"]))
		}

		if tujuanNamaRaw != "" && tujuanNamaRaw != "<nil>" {
			type AgenLookup struct {
				AgenID   string `gorm:"column:agen_id"`
				AgenKode string `gorm:"column:agen_kode"`
			}
			var a AgenLookup

			errFind := database.Table("public.glb_m_agen").
				Select("agen_id, agen_kode").
				Where("TRIM(agen_id) = ? OR TRIM(agen_kode) = ? OR TRIM(agen_nama) = ?", tujuanNamaRaw, tujuanNamaRaw, tujuanNamaRaw).
				First(&a).Error

			if errFind != nil {
				queryPattern := "%" + strings.ReplaceAll(tujuanNamaRaw, " ", "%") + "%"
				database.Table("public.glb_m_agen").
					Select("agen_id, agen_kode").
					Where("agen_nama ILIKE ?", queryPattern).
					First(&a)
			}

			if a.AgenID != "" {
				tujuanAgenIDFinal = strings.TrimSpace(a.AgenID)
			} else if a.AgenKode != "" {
				tujuanAgenIDFinal = strings.TrimSpace(a.AgenKode)
			}
		}
	}

	// =========================================================================
	// 🎯 3. RESOLVE SERVID (LAYANAN) SEBAGAI STRING / VARCHAR
	// =========================================================================
	servIDFinal := "REGULER"
	if fmt.Sprintf("%v", rawPayload["bttt_paketyn"]) == "N" {
		servIDFinal = "EKONOMIS"
	}
	if rawPayload["bttt_servid"] != nil {
		strVal := strings.TrimSpace(fmt.Sprintf("%v", rawPayload["bttt_servid"]))
		if strVal != "" && strVal != "<nil>" {
			servIDFinal = strVal
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
		"%s (%s KOLI) - CABANG: %s%s\n%s [B.PACKING: Rp %.0f, B.PENERUS: Rp %.0f]",
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

	// =========================================================================
	// 4. MAP INSERT KE DATABASE (bttt_updateid tersimpan di sini)
	// =========================================================================
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
		"bttt_tujuanagenid":    cleanStringVal(tujuanAgenIDFinal),
		"bttt_paketyn":         cleanStringVal(rawPayload["bttt_paketyn"]),
		"bttt_jenisharga":      cleanStringVal(rawPayload["bttt_jenisharga"]),
		"bttt_berat":           rawPayload["bttt_berat"],
		"bttt_beratvol":        rawPayload["bttt_beratvol"],
		"bttt_ukuran":          cleanStringVal(rawPayload["bttt_ukuran"]),
		"bttt_harga":           rawPayload["bttt_harga"],
		"bttt_spyn":            "Y",
		"bttt_aktifyn":         "Y",
		"bttt_servid":          cleanStringVal(servIDFinal),
		"bttt_asalagenid":      cleanStringVal(asalAgenIDFinal),
		"bttt_updateid":        fmt.Sprintf("%v", userID), // 🌟 Tersimpan langsung di sini
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

// GET /api/btt/generate-custid?kode_agen=DENPASAR%20DLI%20AGEN
func GenerateCustIDHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = "C" // Default DLI
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database tenant gagal"})
		return
	}

	inputAgenParam := strings.TrimSpace(c.Query("kode_agen"))

	var realCustID string

	// 🎯 1. CUST ID MURNI: Ambil agen_id langsung dari tabel public.glb_m_agen
	if inputAgenParam != "" && inputAgenParam != "null" && inputAgenParam != "undefined" {
		database.Table("public.glb_m_agen").
			Select("agen_id").
			Where("UPPER(TRIM(agen_nama)) = UPPER(TRIM(?)) OR UPPER(TRIM(agen_kode)) = UPPER(TRIM(?)) OR UPPER(TRIM(agen_id)) = UPPER(TRIM(?))",
				inputAgenParam, inputAgenParam, inputAgenParam).
			Order("agen_id ASC").
			Limit(1).
			Row().Scan(&realCustID)

		if realCustID == "" {
			cleanSearch := strings.Split(inputAgenParam, " ")[0]
			database.Table("public.glb_m_agen").
				Select("agen_id").
				Where("agen_nama ILIKE ? OR agen_kode ILIKE ?", "%"+cleanSearch+"%", "%"+cleanSearch+"%").
				Order("agen_id ASC").
				Limit(1).
				Row().Scan(&realCustID)
		}
	}

	realCustID = strings.TrimSpace(realCustID)
	if realCustID == "" || strings.Contains(strings.ToUpper(realCustID), "PUSAT") {
		realCustID = "DPS001" // Fallback safety
	}

	// 🎯 2. Kirimkan CUST ID MURNI (Contoh: "DPS001") tanpa embel-ember tanggal/counter
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"cust_id": realCustID, // Nilai ini yang masuk ke input field "CUST ID"
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

func getUniqueBttID(database *gorm.DB, rawPayload map[string]interface{}, ptContext interface{}, agenContext interface{}, now time.Time) (string, error) {
	// =========================================================================
	// 🏢 1. RESOLVE KODE CORPORATE / PT SECARA DINAMIS
	// =========================================================================
	corpCode := ""
	if pt, ok := rawPayload["pt_id"]; ok && fmt.Sprintf("%v", pt) != "" && fmt.Sprintf("%v", pt) != "<nil>" {
		corpCode = strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", pt)))
	} else if ptContext != nil && fmt.Sprintf("%v", ptContext) != "" {
		corpCode = strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", ptContext)))
	}

	if corpCode == "" {
		return "", fmt.Errorf("identitas Corporate/PT tidak terdeteksi dari sesi aktif")
	}

	// =========================================================================
	// 📍 2. RESOLVE KODE AGEN ASAL SECARA DINAMIS
	// =========================================================================
	agenID := ""
	if asal, ok := rawPayload["bttt_asalagenid"]; ok {
		clean := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", asal)))
		if clean != "" && clean != "<nil>" && clean != "0" {
			agenID = clean
		}
	}

	// Fallback ke agen_id user yang sedang login jika payload belum menyertakan
	if agenID == "" && agenContext != nil {
		clean := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", agenContext)))
		if clean != "" && clean != "<nil>" && clean != "0" {
			agenID = clean
		}
	}

	if agenID == "" {
		return "", fmt.Errorf("kode agen asal penerbit BTT tidak valid atau kosong")
	}

	// =========================================================================
	// 🔢 3. GENERATE NOMOR URUT BERDASARKAN PREFIX DINAMIS
	// =========================================================================
	bulanTahun := now.Format("0106") // Format: MMYY (contoh: 0826)

	// Format Standar: [KODE_PT][KODE_AGEN][BULAN_TAHUN] (Contoh: CBDO0040826 / ASUB0010826 / SBKS0020826)
	prefix := fmt.Sprintf("%s%s%s", corpCode, agenID, bulanTahun)

	// Ambil nomor urut transaksi terakhir untuk prefix ini
	var lastBttID string
	database.Table("public.mkt_t_econote").
		Select("bttt_id").
		Where("bttt_id LIKE ?", prefix+"%").
		Order("bttt_id DESC").
		Limit(1).
		Scan(&lastBttID)

	nextUrutan := 1
	if lastBttID != "" && len(lastBttID) >= len(prefix)+5 {
		suffix := lastBttID[len(prefix):]
		if num, err := strconv.Atoi(suffix); err == nil {
			nextUrutan = num + 1
		}
	}

	bttIDStr := fmt.Sprintf("%s%05d", prefix, nextUrutan)

	// Guard loop anti-duplikasi race condition
	for {
		var count int64
		err := database.Table("public.mkt_t_econote").Where("bttt_id = ?", bttIDStr).Count(&count).Error
		if err != nil || count == 0 {
			break
		}
		nextUrutan++
		bttIDStr = fmt.Sprintf("%s%05d", prefix, nextUrutan)
	}

	return bttIDStr, nil
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

	agenID := strings.TrimSpace(c.Query("agen_id"))
	if agenID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter agen_id wajib dikirim!"})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	hariKemarin := time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")

	// 1. Cek apakah ada BTT aktif kemarin
	var totalBttKemarin int64
	database.Table("public.mkt_t_econote").
		Where("(bttt_tanggal::date = ?::date OR bttt_tanggal::text LIKE ?)", hariKemarin, hariKemarin+"%").
		Where(`(
			CAST(bttt_asalagenid AS VARCHAR) = CAST(? AS VARCHAR)
			OR bttt_id ILIKE '%PUSAT DAKOTA%'
			OR bttt_id ILIKE ?
		)`, agenID, "%"+agenID+"%").
		Count(&totalBttKemarin)

	if totalBttKemarin == 0 {
		c.JSON(http.StatusOK, gin.H{"status": "allowed", "message": "Hari kemarin tidak ada transaksi BTT"})
		return
	}

	// 2. 🚀 FIX PENGECEKAN CLOSING: Cocokkan dengan PST001, kode agen, atau no closing /SB kemarin
	var countClosing int64
	cleanPattern := "%" + agenID + "%"

	database.Table("public.art_t_penjualanbtth").
		Where("(btth_tanggal::date = ?::date OR btth_tanggal::text LIKE ?)", hariKemarin, hariKemarin+"%").
		Where(`(
			CAST(btth_agenid AS VARCHAR) = CAST(? AS VARCHAR)
			OR CAST(btth_agenid AS VARCHAR) ILIKE ?
			OR btth_agenid = 'PST001'
			OR btth_id ILIKE '%/SB'
		)`, agenID, cleanPattern).
		Where("btth_activeyn = 'Y'").
		Count(&countClosing)

	// Jika ada BTT yang secara detail sudah masuk ke tabel art_t_penjualanbttd, anggap juga sudah closing
	if countClosing == 0 {
		var detailCount int64
		database.Table("public.art_t_penjualanbttd AS d").
			Joins("JOIN public.art_t_penjualanbtth AS h ON h.btth_id = d.bttd_btthid").
			Where("(h.btth_tanggal::date = ?::date OR h.btth_tanggal::text LIKE ?)", hariKemarin, hariKemarin+"%").
			Count(&detailCount)
		if detailCount > 0 {
			countClosing = detailCount
		}
	}

	if countClosing == 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":  "blocked",
			"message": fmt.Sprintf("BTT kemarin (%s) belum di-closing! Selesaikan closingan terlebih dahulu untuk membuka akses transaksi BTT baru!", hariKemarin),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "allowed", "message": "Akses loket disetujui"})
}

// 🎯 1. PENCARIAN KECAMATAN (MURNI glb_m_ekodepos)
// GET /api/btt/search-geo?q=SAWANGAN
func SearchMasterGeoBtt(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))
	if len(keyword) < 1 {
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": []interface{}{}})
		return
	}

	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = "A"
	}
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	var results []map[string]interface{}
	likeKeyword := "%" + keyword + "%"

	queryRaw := `
		SELECT DISTINCT ON (kecamatandistrik)
			kodepos as id,
			COALESCE(propinsi, '') as propinsi,
			COALESCE(kotakabupaten, '') as kabupaten,
			COALESCE(kecamatandistrik, '') as kecamatan,
			COALESCE(desakelurahan, '') as kelurahan,
			COALESCE(kodepos, '') as kodepos
		FROM public.glb_m_ekodepos
		WHERE 
			TRIM(COALESCE(kecamatandistrik, '')) != '' AND 
			TRIM(COALESCE(kecamatandistrik, '')) != '-' AND
			(
				kecamatandistrik ILIKE ? OR
				kotakabupaten ILIKE ? OR
				propinsi ILIKE ? OR
				desakelurahan ILIKE ? OR
				kodepos LIKE ?
			)
		ORDER BY kecamatandistrik ASC
		LIMIT 20`

	if err := database.Raw(queryRaw, likeKeyword, likeKeyword, likeKeyword, likeKeyword, likeKeyword).Scan(&results).Error; err != nil {
		log.Printf("❌ [search-geo] Gagal Scan glb_m_ekodepos: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mencari data wilayah"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

// 🎯 2. LIST KELURAHAN BERDASARKAN KECAMATAN (MURNI glb_m_ekodepos)
// GET /api/btt/get-kelurahan?kecamatan=SAWANGAN
func GetKelurahanByKecamatan(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = "A"
	}
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	kecamatan := strings.TrimSpace(c.Query("kecamatan"))
	if kecamatan == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parameter kecamatan wajib diisi"})
		return
	}

	type KelurahanRes struct {
		Kelurahan string `json:"kelurahan" gorm:"column:desakelurahan"`
		Kodepos   string `json:"kodepos" gorm:"column:kodepos"`
	}
	var listKelurahan []KelurahanRes

	err := database.Table("public.glb_m_ekodepos").
		Select("desakelurahan, kodepos").
		Where("UPPER(TRIM(kecamatandistrik)) = UPPER(TRIM(?))", kecamatan).
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

// Helper Pembentuk Nomor BTT Resmi
func generateNomorBTT(database *gorm.DB, ptID string, custID string, now time.Time) string {
	// 1. Corporate Code (C = DLI, A = DBS, B = DLB)
	corpCode := strings.ToUpper(ptID)
	if corpCode == "" {
		corpCode = "C"
	}

	// 2. Format Waktu MMYY
	bulanStr := now.Format("01") // "08"
	tahunStr := now.Format("06") // "26"

	// Prefix Nomor BTT: Corporate + CustID + MM + YY (Contoh: "CDPS0010826")
	prefixBTT := fmt.Sprintf("%s%s%s%s", corpCode, custID, bulanStr, tahunStr)

	// 3. Cari Counter BTT Terakhir di tabel public.mkt_t_econote
	var lastBttID string
	database.Table("public.mkt_t_econote").
		Select("bttt_id").
		Where("bttt_id LIKE ?", prefixBTT+"%").
		Order("bttt_id DESC").
		Limit(1).
		Row().Scan(&lastBttID)

	nextUrutan := 1
	prefixLength := len(prefixBTT)

	if lastBttID != "" && len(lastBttID) > prefixLength {
		suffixUrutan := lastBttID[prefixLength:]
		var currentNo int
		fmt.Sscanf(suffixUrutan, "%d", &currentNo)
		nextUrutan = currentNo + 1
	}

	// Format Final Nomor BTT (Contoh: "CDPS001082600001")
	return fmt.Sprintf("%s%05d", prefixBTT, nextUrutan)
}

// Helper cek apakah Pusat Dakota diizinkan membuat BTT dari tabel glb_m_param
func isPusatAllowedCreateBTT(db *gorm.DB) bool {
	var val string
	err := db.Table("public.glb_m_param").
		Select("COALESCE(set_varvalue, 'N')").
		Where("TRIM(LOWER(set_varname)) = 'allow_pusat_create_btt'").
		Order("set_updatetime DESC NULLS LAST").
		Limit(1).
		Scan(&val).Error

	if err != nil {
		return false
	}
	return strings.ToUpper(strings.TrimSpace(val)) == "Y"
}

// 🌟 GENERATOR NOMOR RESI BTT STANDAR DAKOTA (Contoh: 001112026C000171)
// Format: [KODE_AGEN_3DIGIT][BULAN_2DIGIT][TAHUN_4DIGIT][KODE_PT][URUTAN_6DIGIT]
func GenerateNomorResiBTTStandard(database *gorm.DB, ptID string, agenID string, tgl time.Time) string {
	corpCode := strings.ToUpper(strings.TrimSpace(ptID))
	if corpCode == "" {
		corpCode = "C" // Default DLI
	}

	agenKode := strings.TrimSpace(agenID)
	if agenKode == "" {
		agenKode = "001"
	}
	// Pastikan kode agen 3 digit (contoh: "1" -> "001")
	if len(agenKode) < 3 {
		agenKode = fmt.Sprintf("%03s", agenKode)
	} else if len(agenKode) > 3 {
		agenKode = agenKode[:3]
	}

	bulanStr := tgl.Format("01")   // "11"
	tahunStr := tgl.Format("2006") // "2026"

	// Prefix Resi: Contoh "001112026C"
	prefixResi := fmt.Sprintf("%s%s%s%s", agenKode, bulanStr, tahunStr, corpCode)

	var lastResi string
	database.Table("public.mkt_t_econote").
		Select("bttt_id").
		Where("bttt_id LIKE ?", prefixResi+"%").
		Order("bttt_id DESC").
		Limit(1).
		Scan(&lastResi)

	nextUrut := 1
	prefixLen := len(prefixResi)
	if lastResi != "" && len(lastResi) >= prefixLen+6 {
		suffix := lastResi[prefixLen:]
		if n, errN := strconv.Atoi(suffix); errN == nil {
			nextUrut = n + 1
		}
	}

	res := fmt.Sprintf("%s%06d", prefixResi, nextUrut)

	// Guard loop anti-duplikasi race condition
	for {
		var cnt int64
		err := database.Table("public.mkt_t_econote").Where("bttt_id = ?", res).Count(&cnt).Error
		if err != nil || cnt == 0 {
			break
		}
		nextUrut++
		res = fmt.Sprintf("%s%06d", prefixResi, nextUrut)
	}

	return res
}
