package handler

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

	// 🚀 TANGKAP PARAMETER AGEN ID YANG DIKIRIM FRONTEND
	agenID := c.Query("agen_id")

	query := database.Table("public.mkt_m_customer").
		Select("cust_id, cust_name, cust_alamat1, cust_telp1, cust_kotaid")

	// 🛡️ BARRICADE SECURE LOCK: Jika parameter agen_id dikirim dan bukan akun Holding Pusat,
	// maka saring data secara ketat murni berdasarkan agen_id tersebut!
	if agenID != "" && agenID != "ALL" && !strings.Contains(strings.ToUpper(agenID), "PUSAT") {
		// Asumsi di tabel customer lu ada field relasi agen (misal cust_agenid atau inisial cust_id-nya)
		// Kita bandingkan secara aman menggunakan CAST string
		query = query.Where("CAST(cust_agenid AS VARCHAR) = CAST(? AS VARCHAR)", agenID)
	}

	var results []map[string]interface{}
	if err := query.Order("cust_id DESC").Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat data master customer: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
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
	// 1. Ambil Tenant PT ID dari JWT token
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

	// 2. Ambil parameter kata kunci keyword dari URL (?search=...)
	keyword := c.Query("search")
	keyword = strings.TrimSpace(strings.ToUpper(keyword))

	// 3. Buat Struct Penampung yang SINKRON 100% dengan nama kolom di Postgres lu, bro!
	type CustomerRes struct {
		CustID            string  `json:"cust_id" gorm:"column:cust_id"`
		CustName          string  `json:"cust_name" gorm:"column:cust_name"`
		CustAlamat1       string  `json:"cust_alamat1" gorm:"column:cust_alamat1"`
		CustAlamat2       string  `json:"cust_alamat2" gorm:"column:cust_alamat2"`
		CustTelp1         string  `json:"cust_telp1" gorm:"column:cust_telp1"`
		CustKotaID        string  `json:"cust_kotaid" gorm:"column:cust_kotaid"`
		CustTelp2         string  `json:"cust_telp2" gorm:"column:cust_telp2"`
		CustEmail         string  `json:"cust_email" gorm:"column:cust_email"`
		CustNPWP          string  `json:"cust_npwp" gorm:"column:cust_npwp"`
		CustJenisUsaha    string  `json:"cust_jenisusaha" gorm:"column:cust_jenisusaha"`
		CustContactPerson string  `json:"cust_contactperson" gorm:"column:cust_contactperson"`
		CustKreditLimit   float64 `json:"cust_kreditlimit" gorm:"column:cust_kreditlimit"`
		CustKreditHari    int     `json:"cust_kredithari" gorm:"column:cust_kredithari"`
	}

	var listCustomer []CustomerRes

	// 4. JALANKAN LOGIKA QUERY SAKTI ANTI-KOSONG
	query := database.Table("public.mkt_m_customer").Select(
		"cust_id, cust_name, cust_alamat1, cust_alamat2, cust_kotaid, " +
			"cust_telp1, cust_telp2, cust_email, cust_npwp, cust_jenisusaha, " +
			"cust_contactperson, cust_kreditlimit, cust_kredithari",
	)

	// Jika user mengetik keyword pencarian, filter berdasarkan ID atau Nama
	if keyword != "" {
		query = query.Where("cust_id LIKE ? OR cust_name LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	// Ambil data terbaru dan batasi maksimal 100 baris agar performance gesit menjedarrr
	err := query.Order("cust_id DESC").Limit(100).Scan(&listCustomer).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal fetch data: " + err.Error()})
		return
	}

	// Amankan jika datanya nil, paksa jadi array kosong murni []
	if listCustomer == nil {
		listCustomer = []CustomerRes{}
	}

	// 5. Kembalikan response sukses ke React
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   listCustomer,
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
