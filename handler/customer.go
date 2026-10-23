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

	// 🌟 JOIN LENGKAP KE TABEL KOTA & NO PO
	query := database.Table("public.mkt_m_customer c").
		Select(`
			c.*,
			COALESCE(k.kota_nama, '') AS kota_name,
			COALESCE(k.kota_nama, '') AS kotaname,
			COALESCE(p.nopo, '') AS nopo,
			COALESCE(p.nopo, '') AS no_po,
			COALESCE(h.kry_nama, '') AS pic_nama
		`).
		Joins("LEFT JOIN public.glb_m_kota k ON TRIM(k.kota_id) = TRIM(c.cust_kotaid)").
		Joins("LEFT JOIN public.mkt_m_customer_nopo p ON TRIM(p.cust_id) = TRIM(c.cust_id)").
		Joins("LEFT JOIN public.hrd_m_karyawan h ON TRIM(h.kry_nip) = TRIM(c.cust_pic)")

	cleanAgen := strings.ToUpper(strings.TrimSpace(agenID))

	// =========================================================================
	// 🟢 AUTOMATIC ENTERPRISE COMPASS: Deteksi Saringan Lintas Nusantara Murni
	// =========================================================================
	isPusatOrAll := cleanAgen == "ALL" || cleanAgen == "UND" || cleanAgen == "DEF" || cleanAgen == "" ||
		strings.Contains(cleanAgen, "PUSAT") || cleanAgen == "000" || cleanAgen == "PUS" || cleanAgen == "PST001"

	if isAdmin && isPusatOrAll {
		// 👑 JIKA DROP DOWN PUSAT/ALL -> TAMPILKAN SELURUH DATA NASIONAL
		fmt.Println("👑 [Access Granted] Akun Eksekutif memuat seluruh data Master Customer berskala Nasional.")
	} else {
		// 🔒 JIKA USER (AGEN / ADMIN) MEMILIH WILAYAH OPERASIONAL SPESIFIK TERTENTU
		var realPrefix string

		searchKeyword := cleanAgen
		if strings.Contains(searchKeyword, " ") {
			searchKeyword = strings.Split(searchKeyword, " ")[0]
		}
		wildcardParam := "%" + searchKeyword + "%"

		// 🎯 Relasi agen_id / agen_cabangid
		errFind := database.Table("public.glb_m_agen").
			Select("LEFT(agen_id, 3)").
			Where("agen_id LIKE ? OR agen_kode LIKE ? OR agen_nama ILIKE ? OR agen_cabangid LIKE ?", cleanAgen+"%", cleanAgen+"%", wildcardParam, cleanAgen+"%").
			Limit(1).
			Row().Scan(&realPrefix)

		if errFind == nil && realPrefix != "" {
			realPrefix = strings.ToUpper(strings.TrimSpace(realPrefix))
			cleanAgen = realPrefix
			query = query.Where("c.cust_id LIKE ?", cleanAgen+"%")
		} else {
			query = query.Where("c.cust_id = 'DATA_TIDAK_DITEMUKAN_SISTEM'")
			cleanAgen = "DATA_TIDAK_DITEMUKAN_SISTEM"
		}
	}

	// Filter keyword search
	searchKeyword := strings.TrimSpace(c.Query("search"))
	if searchKeyword != "" {
		query = query.Where("c.cust_name ILIKE ? OR c.cust_id LIKE ?", "%"+searchKeyword+"%", "%"+searchKeyword+"%")
	}

	var results []map[string]interface{}
	if err := query.Order("c.cust_id DESC").Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat data master customer: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

func CreateCustomerHandler(c *gin.Context) {
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
		ctxAgenID, agenExists = c.Get("kode_cabang")
	}

	// 1. Struct input diselaraskan dengan kelengkapan form sistem lawas
	type CustomerInput struct {
		CustNama          string  `json:"cust_name"`
		Nama              string  `json:"nama"`
		CustAlamat1       string  `json:"cust_alamat1"`
		Alamat1           string  `json:"alamat1"`
		CustAlamat2       string  `json:"cust_alamat2"`
		Alamat2           string  `json:"alamat2"`
		CustKotaID        string  `json:"cust_kotaid"`
		KotaID            string  `json:"kotaid"`
		CustTelp1         string  `json:"cust_telp1"`
		Telp1             string  `json:"telp1"`
		CustTelp2         string  `json:"cust_telp2"`
		Telp2             string  `json:"telp2"`
		CustFax1          string  `json:"cust_fax1"`
		Fax1              string  `json:"fax1"`
		CustFax2          string  `json:"cust_fax2"`
		Fax2              string  `json:"fax2"`
		CustContactPerson string  `json:"cust_contactperson"`
		Kontak            string  `json:"kontak"`
		CustMemo          string  `json:"cust_memo"`
		Keterangan        string  `json:"keterangan"`
		CustKgMin         float64 `json:"cust_kgmin"`
		MinKg             float64 `json:"minkg"`
		CustEmail         string  `json:"cust_email"`
		Email             string  `json:"email"`
		CustNPWP          string  `json:"cust_npwp"`
		NPWP              string  `json:"npwp"`
		CustNamaNPWP      string  `json:"cust_namanpwp"`
		NamaNPWP          string  `json:"namanpwp"`
		CustPIC           string  `json:"cust_pic"`
		PIC               string  `json:"pic"`
		CustTOP           int     `json:"cust_top"`
		TOP               int     `json:"top"`
		NoPO              string  `json:"nopo"`
	}

	var input CustomerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	// Normalisasi parameter fallback
	valNama := input.CustNama
	if valNama == "" {
		valNama = input.Nama
	}
	valAlamat1 := input.CustAlamat1
	if valAlamat1 == "" {
		valAlamat1 = input.Alamat1
	}
	valAlamat2 := input.CustAlamat2
	if valAlamat2 == "" {
		valAlamat2 = input.Alamat2
	}
	valKotaID := input.CustKotaID
	if valKotaID == "" {
		valKotaID = input.KotaID
	}
	valTelp1 := input.CustTelp1
	if valTelp1 == "" {
		valTelp1 = input.Telp1
	}
	valTelp2 := input.CustTelp2
	if valTelp2 == "" {
		valTelp2 = input.Telp2
	}
	valFax1 := input.CustFax1
	if valFax1 == "" {
		valFax1 = input.Fax1
	}
	valFax2 := input.CustFax2
	if valFax2 == "" {
		valFax2 = input.Fax2
	}
	valKontak := input.CustContactPerson
	if valKontak == "" {
		valKontak = input.Kontak
	}
	valMemo := input.CustMemo
	if valMemo == "" {
		valMemo = input.Keterangan
	}
	valKgMin := input.CustKgMin
	if valKgMin == 0 {
		valKgMin = input.MinKg
	}
	valEmail := input.CustEmail
	if valEmail == "" {
		valEmail = input.Email
	}
	valNPWP := input.CustNPWP
	if valNPWP == "" {
		valNPWP = input.NPWP
	}
	valNamaNPWP := input.CustNamaNPWP
	if valNamaNPWP == "" {
		valNamaNPWP = input.NamaNPWP
	}
	valPIC := input.CustPIC
	if valPIC == "" {
		valPIC = input.PIC
	}
	valTOP := input.CustTOP
	if valTOP == 0 {
		valTOP = input.TOP
	}

	if valNama == "" || valAlamat1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nama dan Alamat 1 customer wajib diisi"})
		return
	}

	// 2. Generate ID Customer Baru
	var agenCabangID string
	if agenExists && fmt.Sprintf("%v", ctxAgenID) != "" && fmt.Sprintf("%v", ctxAgenID) != "PUSAT DAKOTA" {
		_ = database.Table("public.glb_m_agen").
			Select("agen_cabangid").
			Where("agen_id = ? OR agen_kode = ? OR agen_cabangid = ?", ctxAgenID, ctxAgenID, ctxAgenID).
			Limit(1).
			Row().Scan(&agenCabangID)
	}

	if agenCabangID == "" || agenCabangID == "PUSAT DAKOTA" || agenCabangID == "000" {
		if len(valKotaID) >= 3 {
			agenCabangID = strings.ToUpper(valKotaID[:3]) + "001"
		} else {
			agenCabangID = "JKS001"
		}
	}
	agenCabangID = strings.ToUpper(strings.TrimSpace(agenCabangID))

	now := time.Now()
	bulanStr := fmt.Sprintf("%02d", int(now.Month()))
	tahunStr := now.Format("06")
	idPrefixPattern := fmt.Sprintf("%s%s%s", agenCabangID, bulanStr, tahunStr)
	prefixLength := len(idPrefixPattern)

	var lastCustID string
	database.Table("public.mkt_m_customer").
		Select("cust_id").
		Where("cust_id LIKE ?", idPrefixPattern+"%").
		Order("cust_id DESC").
		Limit(1).
		Row().Scan(&lastCustID)

	nextUrutan := 1
	if lastCustID != "" && len(lastCustID) > prefixLength {
		suffixUrutan := lastCustID[prefixLength:]
		var currentNo int
		fmt.Sscanf(suffixUrutan, "%d", &currentNo)
		nextUrutan = currentNo + 1
	}
	finalCustID := fmt.Sprintf("%s%05d", idPrefixPattern, nextUrutan)

	// 3. Mapping data ke tabel public.mkt_m_customer (HANYA KOLOM RESMI YANG ADA DI DB)
	newCustomer := map[string]interface{}{
		"cust_id":            finalCustID,
		"cust_agenid":        1,
		"cust_name":          strings.ToUpper(valNama),
		"cust_alamat1":       strings.ToUpper(valAlamat1),
		"cust_alamat2":       strings.ToUpper(valAlamat2),
		"cust_kotaid":        strings.ToUpper(valKotaID),
		"cust_telp1":         valTelp1,
		"cust_telp2":         valTelp2,
		"cust_fax1":          valFax1,
		"cust_fax2":          valFax2,
		"cust_contactperson": strings.ToUpper(valKontak),
		"cust_memo":          valMemo,
		"cust_kgmin":         valKgMin,
		"cust_email":         valEmail,
		"cust_npwp":          valNPWP,
		"cust_namanpwp":      strings.ToUpper(valNamaNPWP),
		"cust_pic":           valPIC,
		"cust_top":           valTOP,
		"cust_aktifyn":       "Y",
		"cust_approveyn":     "Y",
		"cust_updatetime":    time.Now(),
	}

	if err := database.Table("public.mkt_m_customer").Create(&newCustomer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan ke database: " + err.Error()})
		return
	}

	// 4. Simpan / Sinkronkan Nomor PO Customer (jika diisi)
	if strings.TrimSpace(input.NoPO) != "" {
		_ = database.Exec("DELETE FROM public.mkt_m_customer_nopo WHERE cust_id = ?", finalCustID)
		_ = database.Exec("INSERT INTO public.mkt_m_customer_nopo (cust_id, nopo) VALUES (?, ?)", finalCustID, strings.TrimSpace(input.NoPO))
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

	// 1. Struct input update
	type CustomerUpdateInput struct {
		CustID            string  `json:"cust_id"`
		KdCust            string  `json:"kdcust"`
		CustNama          string  `json:"cust_name"`
		Nama              string  `json:"nama"`
		CustAlamat1       string  `json:"cust_alamat1"`
		Alamat1           string  `json:"alamat1"`
		CustAlamat2       string  `json:"cust_alamat2"`
		Alamat2           string  `json:"alamat2"`
		CustKotaID        string  `json:"cust_kotaid"`
		KotaID            string  `json:"kotaid"`
		CustTelp1         string  `json:"cust_telp1"`
		Telp1             string  `json:"telp1"`
		CustTelp2         string  `json:"cust_telp2"`
		Telp2             string  `json:"telp2"`
		CustFax1          string  `json:"cust_fax1"`
		Fax1              string  `json:"fax1"`
		CustFax2          string  `json:"cust_fax2"`
		Fax2              string  `json:"fax2"`
		CustContactPerson string  `json:"cust_contactperson"`
		Kontak            string  `json:"kontak"`
		CustMemo          string  `json:"cust_memo"`
		Keterangan        string  `json:"keterangan"`
		CustKgMin         float64 `json:"cust_kgmin"`
		MinKg             float64 `json:"minkg"`
		CustEmail         string  `json:"cust_email"`
		Email             string  `json:"email"`
		CustNPWP          string  `json:"cust_npwp"`
		NPWP              string  `json:"npwp"`
		CustNamaNPWP      string  `json:"cust_namanpwp"`
		NamaNPWP          string  `json:"namanpwp"`
		CustPIC           string  `json:"cust_pic"`
		PIC               string  `json:"pic"`
		CustTOP           int     `json:"cust_top"`
		TOP               int     `json:"top"`
		NoPO              string  `json:"nopo"`
	}

	var input CustomerUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input update tidak valid: " + err.Error()})
		return
	}

	custID := strings.TrimSpace(input.CustID)
	if custID == "" {
		custID = strings.TrimSpace(input.KdCust)
	}
	if custID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID Customer wajib disertakan"})
		return
	}

	// Normalisasi parameter fallback
	valNama := input.CustNama
	if valNama == "" {
		valNama = input.Nama
	}
	valAlamat1 := input.CustAlamat1
	if valAlamat1 == "" {
		valAlamat1 = input.Alamat1
	}
	valAlamat2 := input.CustAlamat2
	if valAlamat2 == "" {
		valAlamat2 = input.Alamat2
	}
	valKotaID := input.CustKotaID
	if valKotaID == "" {
		valKotaID = input.KotaID
	}
	valTelp1 := input.CustTelp1
	if valTelp1 == "" {
		valTelp1 = input.Telp1
	}
	valTelp2 := input.CustTelp2
	if valTelp2 == "" {
		valTelp2 = input.Telp2
	}
	valFax1 := input.CustFax1
	if valFax1 == "" {
		valFax1 = input.Fax1
	}
	valFax2 := input.CustFax2
	if valFax2 == "" {
		valFax2 = input.Fax2
	}
	valKontak := input.CustContactPerson
	if valKontak == "" {
		valKontak = input.Kontak
	}
	valMemo := input.CustMemo
	if valMemo == "" {
		valMemo = input.Keterangan
	}
	valKgMin := input.CustKgMin
	if valKgMin == 0 {
		valKgMin = input.MinKg
	}
	valEmail := input.CustEmail
	if valEmail == "" {
		valEmail = input.Email
	}
	valNPWP := input.CustNPWP
	if valNPWP == "" {
		valNPWP = input.NPWP
	}
	valNamaNPWP := input.CustNamaNPWP
	if valNamaNPWP == "" {
		valNamaNPWP = input.NamaNPWP
	}
	valPIC := input.CustPIC
	if valPIC == "" {
		valPIC = input.PIC
	}
	valTOP := input.CustTOP
	if valTOP == 0 {
		valTOP = input.TOP
	}

	// 2. Map data update (KOLOM cust_jenisusaha DIHAPUS AGAR TIDAK ERROR 42703)
	updatedData := map[string]interface{}{
		"cust_name":          strings.ToUpper(valNama),
		"cust_alamat1":       strings.ToUpper(valAlamat1),
		"cust_alamat2":       strings.ToUpper(valAlamat2),
		"cust_kotaid":        strings.ToUpper(valKotaID),
		"cust_telp1":         valTelp1,
		"cust_telp2":         valTelp2,
		"cust_fax1":          valFax1,
		"cust_fax2":          valFax2,
		"cust_contactperson": strings.ToUpper(valKontak),
		"cust_memo":          valMemo,
		"cust_kgmin":         valKgMin,
		"cust_email":         valEmail,
		"cust_npwp":          valNPWP,
		"cust_namanpwp":      strings.ToUpper(valNamaNPWP),
		"cust_pic":           valPIC,
		"cust_top":           valTOP,
		"cust_updatetime":    time.Now(),
	}

	// 3. Eksekusi UPDATE ke public.mkt_m_customer
	err := database.Table("public.mkt_m_customer").
		Where("cust_id = ?", custID).
		Updates(&updatedData).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui data ke database: " + err.Error()})
		return
	}

	// 4. Update Nomor PO ke tabel mkt_m_customer_nopo (sesuai logika p-mkt_m_customer_e.asp)
	nopoClean := strings.TrimSpace(input.NoPO)
	_ = database.Exec("DELETE FROM public.mkt_m_customer_nopo WHERE cust_id = ?", custID)
	if nopoClean != "" {
		_ = database.Exec("INSERT INTO public.mkt_m_customer_nopo (cust_id, nopo) VALUES (?, ?)", custID, nopoClean)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data Master Customer Berhasil Diperbarui!",
		"cust_id": custID,
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

// =========================================================================
// 🟢 FITUR HARI KERJA CUSTOMER (SABTU / MINGGU / LIBUR) VIA AJAX
// =========================================================================
func UpdateCustomerWorkDaysHandler(c *gin.Context) {
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

	type WorkDaysInput struct {
		CustID string `json:"cust_id" binding:"required"`
		Field  string `json:"field" binding:"required"` // sabtuyn, mingguyn, atau liburyn
		Value  string `json:"value" binding:"required"` // Y atau N
	}

	var input WorkDaysInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload tidak valid"})
		return
	}

	cleanField := strings.ToLower(strings.TrimSpace(input.Field))
	cleanValue := strings.ToUpper(strings.TrimSpace(input.Value))

	if cleanField != "sabtuyn" && cleanField != "mingguyn" && cleanField != "liburyn" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nama field hari kerja tidak valid"})
		return
	}

	// Cek apakah data di tabel mkt_m_customer_workdays sudah ada
	var count int64
	database.Table("public.mkt_m_customer_workdays").Where("cust_id = ?", input.CustID).Count(&count)

	if count == 0 {
		// Jika belum ada, lakukan INSERT baru (Default 'N')
		newWorkDays := map[string]interface{}{
			"cust_id":  input.CustID,
			"sabtuyn":  "N",
			"mingguyn": "N",
			"liburyn":  "N",
		}
		newWorkDays[cleanField] = cleanValue

		if err := database.Table("public.mkt_m_customer_workdays").Create(&newWorkDays).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan hari kerja"})
			return
		}
	} else {
		// Jika sudah ada, laksanakan UPDATE pada field terkait saja
		if err := database.Table("public.mkt_m_customer_workdays").
			Where("cust_id = ?", input.CustID).
			Update(cleanField, cleanValue).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate hari kerja"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Hari kerja customer berhasil diperbarui",
	})
}

// =========================================================================
// 🟢 MASTER SUPPORT: KOTA & KARYAWAN UNTUK AUTOCOMPLETE CUSTOMER
// =========================================================================
func GetMasterKotaHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "<nil>" || fmt.Sprintf("%v", ptID) == "" {
		if altID, ok := c.Get("selected_pt"); ok {
			ptID = altID
		} else {
			ptID = "A"
		}
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tenant gagal",
		})
		return
	}

	type KotaItem struct {
		KotaID   string `json:"kota_id" gorm:"column:kota_id"`
		KotaName string `json:"kota_name" gorm:"column:kota_name"`
	}

	var results []KotaItem
	// Menggunakan kolom resmi PostgreSQL: kota_id dan kota_nama
	err := database.Table("public.glb_m_kota").
		Select("kota_id, kota_nama AS kota_name").
		Where("COALESCE(kota_aktifyn, 'Y') = ?", "Y").
		Order("kota_nama ASC").
		Find(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data master kota: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

// GetMasterKaryawanHandler mengambil daftar karyawan aktif untuk PIC Marketing (HRD_M_Karyawan)
func GetMasterKaryawanHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "<nil>" || fmt.Sprintf("%v", ptID) == "" {
		if altID, ok := c.Get("selected_pt"); ok {
			ptID = altID
		} else {
			ptID = "A"
		}
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Koneksi database tenant gagal",
		})
		return
	}

	type KaryawanItem struct {
		KryNIP  string `json:"kry_nip" gorm:"column:kry_nip"`
		KryNama string `json:"kry_nama" gorm:"column:kry_nama"`
	}

	var results []KaryawanItem
	err := database.Table("public.hrd_m_karyawan").
		Select("kry_nip, kry_nama").
		Where("kry_aktifyn = ? OR kry_aktifyn = ?", "Y", "A").
		Order("kry_nama ASC").
		Find(&results).Error

	if err != nil {
		_ = database.Table("public.hrd_m_karyawan").
			Select("kry_nip, kry_nama").
			Order("kry_nama ASC").
			Find(&results).Error
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}
