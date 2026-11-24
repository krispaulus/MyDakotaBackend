package handler

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// Helper internal untuk resolve exact 3-digit kode agen (misal "2" -> "002", "294" -> "294", "1" -> "001")
func resolveExactAgenPrefix(database *gorm.DB, rawAgenParam string) (exactPrefix string, exactAgenID string, isPusat bool) {
	clean := strings.TrimSpace(rawAgenParam)
	upper := strings.ToUpper(clean)

	if clean == "" || upper == "ALL" || upper == "UND" || upper == "DEF" ||
		clean == "1" || clean == "001" || clean == "000" || upper == "PST001" ||
		strings.Contains(upper, "PUSAT") || strings.Contains(upper, "HOLDING") {
		return "001", "001", true
	}

	var agenRow struct {
		AgenID   string `gorm:"column:agen_id"`
		AgenKode string `gorm:"column:agen_kode"`
	}

	// Cari exact match dulu berdasarkan agen_id, agen_kode, atau agen_nama
	err := database.Table("public.glb_m_agen").
		Select("TRIM(agen_id) AS agen_id, TRIM(agen_kode) AS agen_kode").
		Where("TRIM(agen_id) = ? OR UPPER(TRIM(agen_kode)) = ? OR UPPER(TRIM(agen_nama)) = ?", clean, upper, upper).
		Limit(1).
		Scan(&agenRow).Error

	// Fallback ILIKE jika nama agen dikirim sebagian
	if err != nil || agenRow.AgenID == "" {
		database.Table("public.glb_m_agen").
			Select("TRIM(agen_id) AS agen_id, TRIM(agen_kode) AS agen_kode").
			Where("agen_nama ILIKE ?", "%"+clean+"%").
			Order("LENGTH(agen_nama) ASC").
			Limit(1).
			Scan(&agenRow)
	}

	targetID := strings.TrimSpace(agenRow.AgenID)
	if targetID == "" {
		targetID = clean
	}

	if targetID == "1" || targetID == "001" {
		return "001", "001", true
	}

	// Pastikan prefix tepat 3 digit (misal "2" -> "002", "294" -> "294") agar tidak bocor ke agen lain!
	prefix := targetID
	if len(prefix) < 3 {
		prefix = fmt.Sprintf("%03s", prefix)
	} else if len(prefix) > 3 {
		prefix = prefix[:3]
	}

	return prefix, targetID, false
}

func GetMasterCustomerList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	if fmt.Sprintf("%v", ptID) == "<nil>" || fmt.Sprintf("%v", ptID) == "" {
		if altID, ok := c.Get("selected_pt"); ok {
			ptID = altID
		} else {
			ptID = "C"
		}
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database tenant gagal"})
		return
	}

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
	isAdmin := cleanRole == "SUPERADMIN" || cleanRole == "SUPERDBS" || cleanRole == "HOLDING" || cleanRole == "S" || strings.Contains(cleanRole, "PUSAT")

	agenID := strings.TrimSpace(c.Query("agen_id"))
	if !isAdmin && tokenAgenID != "" {
		agenID = tokenAgenID
	}

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

	// 🔒 RESOLVE MAPPING AGEN SECARA TEGAS (3 DIGIT EXACT)
	prefix3, exactAgenID, isPusat := resolveExactAgenPrefix(database, agenID)

	if isPusat {
		// Jika Pusat (001) dipilih, hanya tampilkan customer milik Pusat (001)
		query = query.Where("(TRIM(c.cust_id) LIKE '001%' OR TRIM(c.cust_agenid) IN ('001', '1'))")
	} else {
		// Jika Agen dipilih (misal DLI SURABAYA / 294), KUNCI hanya milik agen tsb & BLOKIR '001%'
		query = query.Where("(TRIM(c.cust_id) LIKE ? OR TRIM(c.cust_agenid) = ? OR TRIM(c.cust_kotaid) = ?)",
			prefix3+"%", exactAgenID, exactAgenID).
			Where("TRIM(c.cust_id) NOT LIKE '001%'")
	}

	// Filter pencarian keyword & tambahan filter frontend
	searchKeyword := strings.TrimSpace(c.Query("search"))
	if searchKeyword != "" {
		query = query.Where("(c.cust_name ILIKE ? OR c.cust_id ILIKE ? OR COALESCE(c.cust_npwp, '') ILIKE ?)",
			"%"+searchKeyword+"%", "%"+searchKeyword+"%", "%"+searchKeyword+"%")
	}
	if kota := strings.TrimSpace(c.Query("kota")); kota != "" {
		query = query.Where("c.cust_kotaid ILIKE ?", "%"+kota+"%")
	}
	if telp := strings.TrimSpace(c.Query("telp")); telp != "" {
		query = query.Where("(c.cust_telp1 ILIKE ? OR c.cust_telp2 ILIKE ?)", "%"+telp+"%", "%"+telp+"%")
	}
	if hasCredit := strings.TrimSpace(c.Query("has_credit_limit")); hasCredit == "Y" {
		query = query.Where("COALESCE(c.cust_kreditlimit, 0) > 0")
	} else if hasCredit == "N" {
		query = query.Where("COALESCE(c.cust_kreditlimit, 0) = 0")
	}

	var results []map[string]interface{}
	if err := query.Order("c.cust_id DESC").Limit(500).Find(&results).Error; err != nil {
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

	type CustomerInput struct {
		CustNama          string  `json:"cust_name"`
		Nama              string  `json:"nama"`
		CustAlamat1       string  `json:"cust_alamat1"`
		Alamat1           string  `json:"alamat1"`
		CustAlamat2       string  `json:"cust_alamat2"`
		Alamat2           string  `json:"alamat2"`
		CustKotaID        string  `json:"cust_kotaid"`
		KotaID            string  `json:"kotaid"`
		CustAgenID        string  `json:"cust_agenid"`
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
		CustKreditLimit   float64 `json:"cust_kreditlimit"`
		CustKreditHari    int     `json:"cust_kredithari"`
		NoPO              string  `json:"nopo"`
	}

	var input CustomerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	valNama := strings.TrimSpace(input.CustNama)
	if valNama == "" {
		valNama = strings.TrimSpace(input.Nama)
	}
	valAlamat1 := strings.TrimSpace(input.CustAlamat1)
	if valAlamat1 == "" {
		valAlamat1 = strings.TrimSpace(input.Alamat1)
	}
	valAlamat2 := strings.TrimSpace(input.CustAlamat2)
	if valAlamat2 == "" {
		valAlamat2 = strings.TrimSpace(input.Alamat2)
	}
	valKotaID := strings.TrimSpace(input.CustKotaID)
	if valKotaID == "" {
		valKotaID = strings.TrimSpace(input.KotaID)
	}
	valTelp1 := strings.TrimSpace(input.CustTelp1)
	if valTelp1 == "" {
		valTelp1 = strings.TrimSpace(input.Telp1)
	}
	valTelp2 := strings.TrimSpace(input.CustTelp2)
	if valTelp2 == "" {
		valTelp2 = strings.TrimSpace(input.Telp2)
	}
	valKontak := strings.TrimSpace(input.CustContactPerson)
	if valKontak == "" {
		valKontak = strings.TrimSpace(input.Kontak)
	}
	valEmail := strings.TrimSpace(input.CustEmail)
	if valEmail == "" {
		valEmail = strings.TrimSpace(input.Email)
	}
	valNPWP := strings.TrimSpace(input.CustNPWP)
	if valNPWP == "" {
		valNPWP = strings.TrimSpace(input.NPWP)
	}
	valTOP := input.CustTOP
	if valTOP == 0 {
		valTOP = input.TOP
	}
	if valTOP == 0 {
		valTOP = input.CustKreditHari
	}

	if valNama == "" || valAlamat1 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nama dan Alamat 1 customer wajib diisi"})
		return
	}

	// 🎯 RESOLVE AGEN AKTIF (Prioritaskan cust_agenid / cust_kotaid dari frontend dropdown)
	rawAgenSource := strings.TrimSpace(input.CustAgenID)
	if rawAgenSource == "" {
		rawAgenSource = valKotaID
	}
	if rawAgenSource == "" && agenExists {
		rawAgenSource = fmt.Sprintf("%v", ctxAgenID)
	}

	prefix3, exactAgenID, isPusat := resolveExactAgenPrefix(database, rawAgenSource)
	if isPusat {
		c.JSON(http.StatusForbidden, gin.H{
			"status":  "error",
			"message": "Cabang PUSAT (001) tidak diperbolehkan membuat customer cabang. Pilih Agen operasional terlebih dahulu!",
		})
		return
	}

	// 🎯 GENERATE CUST ID STANDAR 10 DIGIT: [PREFIX_AGEN_3DIGIT][7_DIGIT_URUT] -> Contoh: 2940000003
	var lastCustID string
	database.Table("public.mkt_m_customer").
		Select("TRIM(cust_id)").
		Where("TRIM(cust_id) LIKE ? AND LENGTH(TRIM(cust_id)) = 10", prefix3+"%").
		Order("cust_id DESC").
		Limit(1).
		Scan(&lastCustID)

	nextUrutan := 1
	if len(lastCustID) == 10 {
		suffixUrutan := lastCustID[3:]
		if n, errN := strconv.Atoi(suffixUrutan); errN == nil {
			nextUrutan = n + 1
		}
	}
	finalCustID := fmt.Sprintf("%s%07d", prefix3, nextUrutan)

	// Pastikan juga customer default "UMUM" dengan ID prefix3 + "0000001" sudah ada untuk agen ini
	defaultUmumID := fmt.Sprintf("%s0000001", prefix3)
	var countUmum int64
	database.Table("public.mkt_m_customer").Where("TRIM(cust_id) = ?", defaultUmumID).Count(&countUmum)
	if countUmum == 0 && finalCustID != defaultUmumID {
		_ = database.Table("public.mkt_m_customer").Create(map[string]interface{}{
			"cust_id":         defaultUmumID,
			"cust_agenid":     exactAgenID,
			"cust_name":       "UMUM",
			"cust_alamat1":    "-",
			"cust_kotaid":     strings.ToUpper(valKotaID),
			"cust_aktifyn":    "Y",
			"cust_approveyn":  "Y",
			"cust_updatetime": time.Now(),
		})
	}

	newCustomer := map[string]interface{}{
		"cust_id":            finalCustID,
		"cust_agenid":        exactAgenID,
		"cust_name":          strings.ToUpper(valNama),
		"cust_alamat1":       strings.ToUpper(valAlamat1),
		"cust_alamat2":       strings.ToUpper(valAlamat2),
		"cust_kotaid":        strings.ToUpper(valKotaID),
		"cust_telp1":         valTelp1,
		"cust_telp2":         valTelp2,
		"cust_fax1":          input.CustFax1,
		"cust_fax2":          input.CustFax2,
		"cust_contactperson": strings.ToUpper(valKontak),
		"cust_memo":          input.CustMemo,
		"cust_kgmin":         input.CustKgMin,
		"cust_email":         valEmail,
		"cust_npwp":          valNPWP,
		"cust_namanpwp":      strings.ToUpper(input.CustNamaNPWP),
		"cust_pic":           input.CustPIC,
		"cust_top":           valTOP,
		"cust_kreditlimit":   input.CustKreditLimit,
		"cust_aktifyn":       "Y",
		"cust_approveyn":     "Y",
		"cust_updatetime":    time.Now(),
	}

	if err := database.Table("public.mkt_m_customer").Create(&newCustomer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan ke database: " + err.Error()})
		return
	}

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

func SearchCustomerHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = "C"
	}

	agenID := strings.TrimSpace(c.Query("agen_id"))
	searchKeyword := strings.TrimSpace(c.Query("search"))

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	query := database.Table("public.mkt_m_customer").
		Where("COALESCE(cust_aktifyn, 'Y') = 'Y'")

	// 🔒 SELALU FILTER BERDASARKAN AGEN YANG DIPILIH DI HEADER (TERMASUK JIKA LOGIN SUPERADMIN)
	prefix3, exactAgenID, isPusat := resolveExactAgenPrefix(database, agenID)

	if isPusat {
		query = query.Where("(TRIM(cust_id) LIKE '001%' OR TRIM(cust_agenid) IN ('001', '1'))")
	} else {
		// Hanya tampilkan customer milik agen aktif, JANGAN PERNAH campur dengan '001%'
		query = query.Where("(TRIM(cust_id) LIKE ? OR TRIM(cust_agenid) = ? OR TRIM(cust_kotaid) = ?)",
			prefix3+"%", exactAgenID, exactAgenID).
			Where("TRIM(cust_id) NOT LIKE '001%'")
	}

	if searchKeyword != "" {
		query = query.Where("(cust_name ILIKE ? OR cust_id ILIKE ?)", "%"+searchKeyword+"%", "%"+searchKeyword+"%")
	}

	type CustomerRes struct {
		CustID      string `json:"cust_id" gorm:"column:cust_id"`
		CustName    string `json:"cust_name" gorm:"column:cust_name"`
		CustAlamat1 string `json:"cust_alamat1" gorm:"column:cust_alamat1"`
		CustTelp1   string `json:"cust_telp1" gorm:"column:cust_telp1"`
		CustTelp2   string `json:"cust_telp2" gorm:"column:cust_telp2"`
		CustEmail   string `json:"cust_email" gorm:"column:cust_email"`
		CustKotaID  string `json:"cust_kotaid" gorm:"column:cust_kotaid"`
	}
	var results []CustomerRes

	if err := query.Order("cust_name ASC").Limit(50).Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

type CustomerModel struct {
	CustID      string `json:"cust_id" gorm:"column:cust_id"`
	CustName    string `json:"cust_name" gorm:"column:cust_name"`
	CustAlamat1 string `json:"cust_alamat1" gorm:"column:cust_alamat1"`
	CustTelp1   string `json:"cust_telp1" gorm:"column:cust_telp1"`
	CustKotaID  string `json:"cust_kotaid" gorm:"column:cust_kotaid"`
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
