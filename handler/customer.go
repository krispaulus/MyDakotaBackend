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

func GetMasterCustomerList(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database tenant gagal"})
		return
	}

	// 🧠 AMBIL DATA ROLE AKSES DARI JWT TOKEN (SESSION CONTEXT)
	roleUser := ""
	tokenAgenID := ""
	if claimsVal, exists := c.Get("user_data"); exists {
		if claims, ok := claimsVal.(jwt.MapClaims); ok {
			if r, ok := claims["user_type"].(string); ok {
				roleUser = r
			}
			if a, ok := claims["agent_code"].(string); ok {
				tokenAgenID = a
			}
		}
	}

	if roleUser == "" {
		roleUser = c.Query("role_akses")
	}

	cleanRole := strings.ToUpper(strings.TrimSpace(roleUser))

	// 🛡️ TENTUKAN APAKAH USER ADALAH ADMIN/SUPERADMIN/HOLDING/PUSAT
	isAdmin := cleanRole == "SUPERADMIN" || cleanRole == "SUPERDBS" || cleanRole == "HOLDING" || cleanRole == "S" || strings.Contains(cleanRole, "PUSAT")

	// 🚀 TANGKAP PARAMETER AGEN ID YANG DIKIRIM FRONTEND ATAU TOKEN
	agenID := c.Query("agen_id")
	if !isAdmin {
		if tokenAgenID != "" {
			agenID = tokenAgenID
		}
	}

	query := database.Table("public.mkt_m_customer")

	cleanAgen := strings.ToUpper(strings.TrimSpace(agenID))

	// =========================================================================
	// 🟢 AUTOMATIC ENTERPRISE COMPASS: Deteksi Saringan Lintas Nusantara Murni
	// =========================================================================
	isPusatOrAll := cleanAgen == "ALL" || cleanAgen == "UND" || cleanAgen == "DEF" || cleanAgen == "" ||
		strings.Contains(cleanAgen, "PUSAT") || cleanAgen == "000" || cleanAgen == "PUS" || cleanAgen == "PST001"

	if isAdmin && isPusatOrAll {
		// 👑 JIKA DROP DOWN PUSAT/ALL -> JEBOL SAKTI: TAMPILKAN SELURUH DATA NASIONAL INDONESIA
		fmt.Println("👑 [Access Granted] Akun Eksekutif memuat seluruh data Master Customer berskala Nasional.")
	} else {
		// 🔒 JIKA USER (AGEN / ADMIN) MEMILIH WILAYAH OPERASIONAL SPESIFIK TERTENTU
		var realPrefix string

		// Bersihkan kata jika membawa teks panjang (e.g., "PURWOREJO AGEN" -> "PURWOREJO")
		searchKeyword := cleanAgen
		if strings.Contains(searchKeyword, " ") {
			searchKeyword = strings.Split(searchKeyword, " ")[0]
		}
		wildcardParam := "%" + searchKeyword + "%"

		// 🎯 RELASI SAKTI NUSANTARA: Ambil 3 huruf terdepan dari kolom agen_id atau agen_cabangid yang COCOK!
		errFind := database.Table("public.glb_m_agen").
			Select("LEFT(agen_id, 3)").
			Where("agen_id LIKE ? OR agen_kode LIKE ? OR agen_nama ILIKE ? OR agen_cabangid LIKE ?", cleanAgen+"%", cleanAgen+"%", wildcardParam, cleanAgen+"%").
			Limit(1).
			Row().Scan(&realPrefix)

		if errFind == nil && realPrefix != "" {
			realPrefix = strings.ToUpper(strings.TrimSpace(realPrefix))
			fmt.Printf("🔍 [Compass Success] Dropdown '%s' sukses dikonversi ke Inisial DB murni: '%s'\n", cleanAgen, realPrefix)
			cleanAgen = realPrefix
		} else {
			// 🛡️ ANTI-HALUSINASI SAKTI: Jika relasi data tidak ditemukan, kosongkan total!
			// Jangan menebak-nebak data wilayah lain agar user tidak bingung.
			fmt.Printf("⚠️ [Compass Empty] Inisial tidak ditemukan untuk alias '%s'. Mengosongkan query saringan.\n", cleanAgen)
			cleanAgen = "DATA_TIDAK_DITEMUKAN_SISTEM"
		}

		// Jalankan saringan murni berdasarkan kode inisial yang valid
		query = query.Where("cust_id LIKE ?", cleanAgen+"%")
		fmt.Printf("🔒 [Tenant Filtered] Menyaring master customer murni untuk Hak Wilayah Cabang: '%s%%'\n", cleanAgen)
	}

	// Filter keyword search jika frontend mengirimkannya
	searchKeyword := strings.TrimSpace(c.Query("search"))
	if searchKeyword != "" {
		query = query.Where("cust_name ILIKE ? OR cust_id LIKE ?", "%"+searchKeyword+"%", "%"+searchKeyword+"%")
	}

	var results []map[string]interface{}
	if err := query.Order("cust_id DESC").Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat data master customer: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

func CreateCustomerHandler(c *gin.Context) {
	// 1. Ambil Tenant PT ID dari JWT token login user
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

	ctxAgenID, agenExists := c.Get("active_agen_id")
	if !agenExists {
		// Jika di middleware lu namanya "kode_cabang", kita coba ambil failback-nya
		ctxAgenID, agenExists = c.Get("kode_cabang")
	}

	// 2. Struct Input Diperluas Mengikuti Parameter Form Gambar 4
	type CustomerInput struct {
		CustNama    string `json:"cust_name" binding:"required"`
		CustAlamat1 string `json:"cust_alamat1" binding:"required"`
		CustAlamat2 string `json:"cust_alamat2"`
		CustKotaID  string `json:"cust_kotaid" binding:"required"`
		CustTelp1   string `json:"cust_telp1" binding:"required"`
		CustTelp2   string `json:"cust_telp2"`
		CustEmail   string `json:"cust_email"`
		// Parameter Tambahan Baru Sesuai Gambar 4 🎯
		CustNPWP          string  `json:"cust_npwp"`
		CustJenisUsaha    string  `json:"cust_jenisusaha"`
		CustContactPerson string  `json:"cust_contactperson"`
		CustKreditLimit   float64 `json:"cust_kreditlimit"`
		CustKreditHari    int     `json:"cust_kredithari"`
	}

	var input CustomerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		fmt.Println("❌ EROR BINDING JSON JEDERRR:", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	// ==============================================================
	// 3. 🛡️ FORMULA GENERATOR ID BARU (SOP RULING REVISED)
	// Rumus: agen_cabangid + 4 digit nomor urut global
	// ==============================================================

	// A. Ambil Data Waktu Sekarang (Bulan & Tahun)
	// now := time.Now()
	// bulanStr := fmt.Sprintf("%02d", int(now.Month())) // Hasil: "05" (Mei)
	// tahunStr := now.Format("06")                      // Hasil: "26" (Tahun 2026)

	// searchPattern := fmt.Sprintf("%s%s%s%s%%", prefixKota, kodeAgen, bulanStr, tahunStr)

	var agenCabangID string

	if agenExists && fmt.Sprintf("%v", ctxAgenID) != "" && fmt.Sprintf("%v", ctxAgenID) != "PUSAT DAKOTA" {
		// 🌟 KUNCI 1: Ambil data agen_cabangid asli dari DB (Misal: "JKS001" atau "SUM001")
		errQueryAgen := database.Table("public.glb_m_agen").
			Select("agen_cabangid").
			Where("agen_id = ? OR agen_kode = ? OR agen_cabangid = ?", ctxAgenID, ctxAgenID, ctxAgenID).
			Limit(1).
			Row().Scan(&agenCabangID)

		if errQueryAgen != nil {
			fmt.Println("⚠️ Gagal scan agen cabang, bypass value token...")
			agenCabangID = fmt.Sprintf("%v", ctxAgenID)
		}
	}

	// Ultimate Fallback jika data kosong atau bernilai holding pusat
	if agenCabangID == "" || agenCabangID == "PUSAT DAKOTA" || agenCabangID == "000" {
		if len(input.CustKotaID) >= 3 {
			agenCabangID = strings.ToUpper(input.CustKotaID[:3]) + "001"
		} else {
			agenCabangID = "JKS001" // Default safety net
		}
	}

	// Bersihkan data string agar rapi
	agenCabangID = strings.ToUpper(strings.TrimSpace(agenCabangID))

	// 🏢 Ambil data waktu sekarang (Bulan MM & Tahun YY)
	now := time.Now()
	bulanStr := fmt.Sprintf("%02d", int(now.Month())) // Hasil: "05"
	tahunStr := now.Format("06")                      // Hasil: "26"

	// Kombinasi Pola Dasar 9 Karakter Pertama -> (Contoh: "JKS001" + "05" + "26" = "JKS0010526")
	idPrefixPattern := fmt.Sprintf("%s%s%s", agenCabangID, bulanStr, tahunStr)

	// Hitung panjang prefix untuk pemotongan dinamis di bawah
	prefixLength := len(idPrefixPattern)

	// Cari ID customer terakhir di database yang diawali dengan pola kombinasi bulan ini
	var lastCustID string
	database.Table("public.mkt_m_customer").
		Select("cust_id").
		Where("cust_id LIKE ?", idPrefixPattern+"%").
		Order("cust_id DESC").
		Limit(1).
		Row().Scan(&lastCustID)

	nextUrutan := 1

	// 🌟 KUNCI 2: DYNAMIC SLICING UNTUK URUTAN 5 DIGIT BUNTUT ("00001")
	if lastCustID != "" && len(lastCustID) > prefixLength {
		// Potong string murni tepat setelah karakter prefix selesai berjalan
		suffixUrutan := lastCustID[prefixLength:]

		var currentNo int
		fmt.Sscanf(suffixUrutan, "%d", &currentNo)
		nextUrutan = currentNo + 1 // Naikkan counter 1 angka
	}

	// 🌟 KUNCI 3: UBAH FORMAT SPESIFIKASI DARI %04d MENJADI %05d (MENGUNCI 5 DIGIT URUTAN)
	// Hasil: "JKS0010526" + "00001" = "JKS001052600001" (Total 15 Karakter Sempurna & Presisi!)
	finalCustID := fmt.Sprintf("%s%05d", idPrefixPattern, nextUrutan)

	// 4. MAP DATA DATA KE TABEL POSTGRES (DISESUAIKAN 100% DENGAN SKEMA RESMI LU)
	newCustomer := map[string]interface{}{
		"cust_id":            finalCustID,
		"cust_agenid":        1,
		"cust_name":          strings.ToUpper(input.CustNama), // ✅ Kembalikan ke 'cust_name' sesuai baris 3 DB lu
		"cust_alamat1":       strings.ToUpper(input.CustAlamat1),
		"cust_alamat2":       strings.ToUpper(input.CustAlamat2),
		"cust_kotaid":        strings.ToUpper(input.CustKotaID),
		"cust_telp1":         input.CustTelp1,
		"cust_telp2":         input.CustTelp2,
		"cust_email":         input.CustEmail,
		"cust_aktifyn":       "Y",        // Bawaan default aktif Dakota
		"cust_approveyn":     "Y",        // Bawaan default approve Dakota
		"cust_updatetime":    time.Now(), // ✅ Ganti 'cust_created' menjadi 'cust_updatetime' sesuai baris 18 DB lu!
		"cust_npwp":          input.CustNPWP,
		"cust_jenisusaha":    strings.ToUpper(input.CustJenisUsaha),
		"cust_contactperson": strings.ToUpper(input.CustContactPerson),
		"cust_kreditlimit":   input.CustKreditLimit,
		"cust_kredithari":    input.CustKreditHari,
		"cust_kredityn":      "Y", // Otomatis aktifkan kredit jika ada limitnya
	}

	// 5. EXECUTE INSERT INTO POSTGRESQL
	err := database.Table("public.mkt_m_customer").Create(&newCustomer).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan ke database: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Master Customer Berhasil Ditambahkan!",
		"cust_id": finalCustID,
	})
}

type CustomerModel struct {
	CustID      string `json:"cust_id" gorm:"column:cust_id"`
	CustName    string `json:"cust_name" gorm:"column:cust_name"`
	CustAlamat1 string `json:"cust_alamat1" gorm:"column:cust_alamat1"`
	CustTelp1   string `json:"cust_telp1" gorm:"column:cust_telp1"`
	CustKotaID  string `json:"cust_kotaid" gorm:"column:cust_kotaid"`
}

func SearchCustomerHandler(c *gin.Context) {
	// 1. Ambil PT ID dari context token JWT lu
	ptID, exists := c.Get("pt_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "PT ID tidak ditemukan di token"})
		return
	}

	// 🧠 KUNCI SAKTI: Tangkap role akses & agen_id dari query parameter atau context token JWT
	// (Pastikan di middleware login lu sudah menyuntikkan data role ke context, atau bisa ambil param query)
	roleUser := c.Query("role_akses")
	if roleUser == "" {
		// Fallback jika tidak dikirim via parameter, coba intip dari context session token
		if r, ok := c.Get("role_akses"); ok {
			roleUser = fmt.Sprintf("%v", r)
		}
	}

	agenID := c.Query("agen_id")
	searchKeyword := strings.TrimSpace(c.Query("search"))

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	// Buat query builder dasar ke tabel mkt_m_customer
	query := database.Table("public.mkt_m_customer")

	// =========================================================================
	// 🛡️ ALGORITMA FILTER MULTI-TENANT ROLES (ANTI-BOCORET DATA)
	// =========================================================================
	// Ubah teks role ke uppercase untuk menghindari miss match case sensitive
	cleanRole := strings.ToUpper(strings.TrimSpace(roleUser))

	if cleanRole == "SUPERADMIN" || cleanRole == "SUPERDBS" || cleanRole == "HOLDING" {
		// 🟢 KASTA SUPERADMIN: Jangan potong query, biarkan lolos melihat data nasional se-Indonesia!
		fmt.Println("👑 [Access Granted] Akun Superadmin mendeteksi penarikan data customer berskala Nasional.")
	} else {
		// 🔴 KASTA AGEN LOKET OPERASIONAL: Kunci mati query murni berdasarkan 3 digit awalan CUST ID mereka!
		if agenID != "" {
			// Sesuai rumus ID lu (Contoh: GOR01062600001), 3 huruf awal adalah kode agen (GOR)
			// Kita filter menggunakan Operator LIKE 'GOR%'
			query = query.Where("cust_id LIKE ?", agenID+"%")
			fmt.Printf("🔒 [Tenant Locked] Membatasi master customer murni untuk hak Agen ID: '%s'\n", agenID)
		}
	}

	// Filter pencarian teks jika user mengetik di kotak pencarian keyword
	if searchKeyword != "" {
		query = query.Where("cust_name ILIKE ? OR cust_id LIKE ?", "%"+searchKeyword+"%", "%"+searchKeyword+"%")
	}

	type CustomerRes struct {
		CustID      string `json:"cust_id" gorm:"column:cust_id"`
		CustName    string `json:"cust_name" gorm:"column:cust_name"`
		CustAlamat1 string `json:"cust_alamat1" gorm:"column:cust_alamat1"`
		CustTelp1   string `json:"cust_telp1" gorm:"column:cust_telp1"`
		CustKotaID  string `json:"cust_kotaid" gorm:"column:cust_kotaid"`
		CustNama    string `gorm:"column:cust_nama"` // Fallback alias database lamamu
	}
	var results []CustomerRes

	err := query.Limit(100).Find(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Sinkronisasi penamaan properti alias objek map DB lama lu
	for i := 0; i < len(results); i++ {
		if results[i].CustName == "" && results[i].CustNama != "" {
			results[i].CustName = results[i].CustNama
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

// =========================================================================
// 🟢 MESIN UPDATE SAKTI: UPDATE CUSTOMER HANDLER ANTI-404 JEDERRR!
// =========================================================================
func UpdateCustomerHandler(c *gin.Context) {
	// 1. Ambil Tenant PT ID dari JWT token login user
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

	// 2. Struct Binding Khusus Update (Primary Key cust_id WAJIB ADA)
	type CustomerUpdateInput struct {
		CustID            string  `json:"cust_id" binding:"required"` // Kunci utama pencarian
		CustNama          string  `json:"cust_name" binding:"required"`
		CustAlamat1       string  `json:"cust_alamat1" binding:"required"`
		CustAlamat2       string  `json:"cust_alamat2"`
		CustKotaID        string  `json:"cust_kotaid" binding:"required"`
		CustTelp1         string  `json:"cust_telp1" binding:"required"`
		CustTelp2         string  `json:"cust_telp2"`
		CustEmail         string  `json:"cust_email"`
		CustNPWP          string  `json:"cust_npwp"`
		CustJenisUsaha    string  `json:"cust_jenisusaha"`
		CustContactPerson string  `json:"cust_contactperson"`
		CustKreditLimit   float64 `json:"cust_kreditlimit"`
		CustKreditHari    int     `json:"cust_kredithari"`
	}

	var input CustomerUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		fmt.Println("❌ ERROR BINDING JSON UPDATE:", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input update tidak valid: " + err.Error()})
		return
	}

	// 3. Siapkan Map Data Baru (Gunakan data uppercase sesuai SOP Dakota)
	updatedData := map[string]interface{}{
		"cust_name":          strings.ToUpper(input.CustNama),
		"cust_alamat1":       strings.ToUpper(input.CustAlamat1),
		"cust_alamat2":       strings.ToUpper(input.CustAlamat2),
		"cust_kotaid":        strings.ToUpper(input.CustKotaID),
		"cust_telp1":         input.CustTelp1,
		"cust_telp2":         input.CustTelp2,
		"cust_email":         input.CustEmail,
		"cust_npwp":          input.CustNPWP,
		"cust_jenisusaha":    strings.ToUpper(input.CustJenisUsaha),
		"cust_contactperson": strings.ToUpper(input.CustContactPerson),
		"cust_kreditlimit":   input.CustKreditLimit,
		"cust_kredithari":    input.CustKreditHari,
		"cust_updatetime":    time.Now(), // Catat waktu perubahan
	}

	// 4. Jalankan SQL Eksekusi UPDATE dengan klausa WHERE cust_id
	err := database.Table("public.mkt_m_customer").
		Where("cust_id = ?", input.CustID).
		Updates(&updatedData).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui data ke database: " + err.Error()})
		return
	}

	// 5. Kembalikan respons sukses ke React
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data Master Customer Berhasil Diperbarui!",
		"cust_id": input.CustID,
	})
}

// =========================================================================
// 🟢 MESIN DESTRUKSI: DELETE CUSTOMER HANDLER (SAFE POST CLAUSE)
// =========================================================================
func DeleteCustomerHandler(c *gin.Context) {
	// 1. Ambil Tenant PT ID dari JWT token login
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

	// 2. Struct penampung request data (Hanya butuh Primary Key ID)
	type DeleteInput struct {
		CustID string `json:"cust_id" binding:"required"`
	}

	var input DeleteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload input ID tidak valid"})
		return
	}

	// 3. JALANKAN SQL CLAUSE DELETE MURNI BERDASARKAN KUNCI UTAMA CUST_ID
	err := database.Table("public.mkt_m_customer").
		Where("cust_id = ?", input.CustID).
		Delete(map[string]interface{}{}).Error // GORM Delete Engine

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus data di database: " + err.Error()})
		return
	}

	// 4. Kembalikan respons sukses mutlak ke React
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Data Customer ID %s Berhasil Dilenyapkan dari Server!", input.CustID),
	})
}
