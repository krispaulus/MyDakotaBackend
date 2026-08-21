package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Model Request Posting Kas ke Jurnal
type PostCashBankReq struct {
	CBID       string `json:"cb_id" binding:"required"`
	SumberDana string `json:"sumber_dana" binding:"required"`
	BankCode   string `json:"bank_code"`
	BankName   string `json:"bank_name"`
	EtollCode  string `json:"etoll_code"`
	EtollName  string `json:"etoll_name"`
}

// GET /api/gl/bank-data (100% Dinamis Langsung dari Database CashBank Aktif)
func GetBankDataHandler(c *gin.Context) {
	database := getCashBankDB(c)
	search := strings.TrimSpace(c.Query("search"))

	type BankRow struct {
		BankID      string `gorm:"column:bank_id"`
		BankAccCode string `gorm:"column:bank_acccode"`
		BankName    string `gorm:"column:bank_name"`
	}

	type BankResult struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}

	var rawList []BankRow
	query := database.Table("public.gl_m_bank").
		Select("bank_id, bank_acccode, bank_name").
		Where("COALESCE(bank_name, '') <> ''").
		Where("COALESCE(bank_aktifyn, 'Y') = 'Y'")

	if search != "" {
		query = query.Where("bank_name ILIKE ? OR bank_acccode ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if err := query.Order("bank_name ASC").Find(&rawList).Error; err != nil {
		fmt.Printf("❌ [ERROR QUERY GL_M_BANK] %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	list := make([]BankResult, 0)
	for _, row := range rawList {
		code := strings.TrimSpace(row.BankAccCode)
		if code == "" {
			code = strings.TrimSpace(row.BankID)
		}
		list = append(list, BankResult{
			Code: code,
			Name: strings.TrimSpace(row.BankName),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// GET /api/gl/etoll-data (List E-Toll untuk Modal Posting)
func GetEtollDataHandler(c *gin.Context) {
	database := getCashBankDB(c)
	search := strings.TrimSpace(c.Query("search"))

	type EtollResult struct {
		Code string `json:"code" gorm:"column:ca_id"`
		Name string `json:"name" gorm:"column:ca_name"`
	}

	var list []EtollResult
	query := database.Table("public.gl_m_chartaccount").
		Select("ca_id, ca_name").
		Where("COALESCE(ca_aktifyn, 'Y') = 'Y' AND (ca_name ILIKE '%TOLL%' OR ca_name ILIKE '%E-TOLL%')")

	if search != "" {
		query = query.Where("ca_id ILIKE ? OR ca_name ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	query.Order("ca_name ASC").Limit(30).Scan(&list)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// POST /api/gl/cashbank/post (Posting Kas Masuk/Keluar ke GL Jurnal)
func PostCashBankHandler(c *gin.Context) {
	database := getCashBankDB(c)
	userID, _ := c.Get("username")

	var req PostCashBankReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// 1. Ambil Header Kas
	var cbHeader struct {
		CBID      string    `gorm:"column:cb_id"`
		CBTanggal time.Time `gorm:"column:cb_tanggal"`
		CBTipe    string    `gorm:"column:cb_tipe"`
		CBKet     string    `gorm:"column:cb_ket"`
		CBAgenID  string    `gorm:"column:cb_agenid"`
		CBTotal   float64   `gorm:"column:cb_total"`
		CBPostYN  string    `gorm:"column:cb_postyn"`
	}

	if err := database.Table("public.gl_t_cashbank").Where("cb_id = ?", req.CBID).First(&cbHeader).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data Kas tidak ditemukan!"})
		return
	}

	// 2. Tentukan Akun Sumber Dana (100% Dinamis Murni dari Database)
	var lawanAccCode, lawanAccName string
	sumber := strings.ToLower(strings.TrimSpace(req.SumberDana))

	if strings.Contains(sumber, "bank") {
		// Kasus 1: Bank (Wajib mengambil dari inputan dropdown hasil query gl_m_bank)
		if strings.TrimSpace(req.BankCode) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Pilihan Bank tidak boleh kosong!"})
			return
		}
		lawanAccCode = strings.TrimSpace(req.BankCode)
		lawanAccName = strings.TrimSpace(req.BankName)

	} else if strings.Contains(sumber, "fleet") {
		// Kasus 2: BCA FLEET (Query ke gl_m_bank atau gl_m_chartaccount)
		err := database.Table("public.gl_m_bank").
			Select("COALESCE(NULLIF(bank_acccode, ''), bank_id::varchar) AS code, bank_name").
			Where("bank_name ILIKE '%FLEET%' AND COALESCE(bank_aktifyn, 'Y') = 'Y'").
			Row().Scan(&lawanAccCode, &lawanAccName)

		if err != nil || lawanAccCode == "" {
			// Jika di tabel bank tidak ada, cari di COA
			err = database.Table("public.gl_m_chartaccount").
				Select("ca_id, ca_name").
				Where("ca_name ILIKE '%FLEET%' AND COALESCE(ca_aktifyn, 'Y') = 'Y'").
				Row().Scan(&lawanAccCode, &lawanAccName)
		}

	} else if strings.Contains(sumber, "lk") {
		// Kasus 3: Kas Operasional LK (Query COA Kas Luar Kota)
		err := database.Table("public.gl_m_chartaccount").
			Select("ca_id, ca_name").
			Where("ca_name ILIKE '%KAS%' AND ca_name ILIKE '%LK%' AND COALESCE(ca_aktifyn, 'Y') = 'Y'").
			Order("ca_id ASC").
			Limit(1).
			Row().Scan(&lawanAccCode, &lawanAccName)

		if err != nil || lawanAccCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Akun perkiraan KAS LK tidak ditemukan di Master COA database!"})
			return
		}

	} else {
		// Kasus 4: Kas Operasional DK (Query COA Kas Dalam Kota / Operasional)
		err := database.Table("public.gl_m_chartaccount").
			Select("ca_id, ca_name").
			Where("ca_name ILIKE '%KAS%' AND (ca_name ILIKE '%DK%' OR ca_name ILIKE '%OPERASIONAL%') AND COALESCE(ca_aktifyn, 'Y') = 'Y'").
			Order("ca_id ASC").
			Limit(1).
			Row().Scan(&lawanAccCode, &lawanAccName)

		if err != nil || lawanAccCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Akun perkiraan KAS OPERASIONAL / DK tidak ditemukan di Master COA database!"})
			return
		}
	}

	// Validasi akhir: pastikan akun lawan berhasil ditemukan dari database
	if strings.TrimSpace(lawanAccCode) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Akun perkiraan untuk sumber dana '%s' tidak ditemukan di database corporate aktif!", req.SumberDana),
		})
		return
	}

	// 3. Generate No Jurnal Format Legacy: [YYMM][AGEN_3DIGIT][TIPE][URUTAN_5DIGIT]
	agen3Digit := fmt.Sprintf("%03s", cbHeader.CBAgenID)
	if len(agen3Digit) > 3 {
		agen3Digit = agen3Digit[len(agen3Digit)-3:]
	}
	prefixJurnal := fmt.Sprintf("%s%s%s", cbHeader.CBTanggal.Format("0601"), agen3Digit, cbHeader.CBTipe)

	var lastJurnal string
	database.Table("public.gl_t_jurnalh").
		Select("tjurh_no").
		Where("tjurh_no LIKE ?", prefixJurnal+"%").
		Order("tjurh_no DESC").
		Limit(1).
		Scan(&lastJurnal)

	nextUrut := 1
	if lastJurnal != "" && len(lastJurnal) >= len(prefixJurnal)+5 {
		if num, err := strconv.Atoi(lastJurnal[len(prefixJurnal):]); err == nil {
			nextUrut = num + 1
		}
	}
	docNo := fmt.Sprintf("%s%05d", prefixJurnal, nextUrut)

	// 4. Ambil Rincian Kas dari gl_t_cashbankdetil
	type CBDetail struct {
		ItemID      string  `gorm:"column:cbd_itemid"`
		Ket         string  `gorm:"column:cbd_ket"`
		Quantity    float64 `gorm:"column:cbd_quantity"`
		HargaSatuan float64 `gorm:"column:cbd_hargasatuan"`
		AgenID      string  `gorm:"column:cbd_agenid"`
	}
	var rincian []CBDetail
	database.Table("public.gl_t_cashbankdetil").Where("TRIM(cbd_cbid) = TRIM(?)", req.CBID).Scan(&rincian)

	var totalNominal float64
	for _, r := range rincian {
		totalNominal += (r.Quantity * r.HargaSatuan)
	}
	if totalNominal <= 0 {
		totalNominal = cbHeader.CBTotal
	}

	// 5. Simpan Header Jurnal (GL_T_JurnalH)
	insertJH := map[string]interface{}{
		"tjurh_no":         docNo,
		"tjurh_tanggal":    cbHeader.CBTanggal,
		"tjurh_type":       cbHeader.CBTipe,
		"tjurh_keterangan": cbHeader.CBKet,
		"tjurh_deleteyn":   "N",
		"tjurh_postyn":     "Y",
		"tjurh_updateid":   fmt.Sprintf("%v", userID),
		"tjurh_updatetime": time.Now(),
	}
	database.Table("public.gl_t_jurnalh").Create(insertJH)

	// 6. Simpan Baris 1: Akun Sumber Dana (Kredit jika Keluar Kas, Debet jika Terima Kas)
	var debetLawan, kreditLawan float64
	if cbHeader.CBTipe == "K" {
		kreditLawan = totalNominal
	} else {
		debetLawan = totalNominal
	}

	// 💡 Kolom murni sesuai skema gl_t_jurnald legacy
	database.Table("public.gl_t_jurnald").Create(map[string]interface{}{
		"tjurd_tjurhno":    docNo,
		"tjurd_acccode":    lawanAccCode,
		"tjurd_agenid":     cbHeader.CBAgenID,
		"tjurd_keterangan": fmt.Sprintf("%s : %s", lawanAccName, cbHeader.CBKet),
		"tjurd_debet":      debetLawan,
		"tjurd_kredit":     kreditLawan,
	})

	// 7. Simpan Baris 2+: Akun Rincian Biaya / Uang Muka
	if len(rincian) > 0 {
		for _, r := range rincian {
			subtotal := r.Quantity * r.HargaSatuan
			var dDebet, dKredit float64
			if cbHeader.CBTipe == "K" {
				dDebet = subtotal
			} else {
				dKredit = subtotal
			}

			itemAccCode := strings.TrimSpace(r.ItemID)
			if itemAccCode == "" {
				itemAccCode = "084000000007"
			}

			database.Table("public.gl_t_jurnald").Create(map[string]interface{}{
				"tjurd_tjurhno":    docNo,
				"tjurd_acccode":    itemAccCode,
				"tjurd_agenid":     cbHeader.CBAgenID,
				"tjurd_keterangan": r.Ket,
				"tjurd_debet":      dDebet,
				"tjurd_kredit":     dKredit,
			})
		}
	} else {
		var dDebet, dKredit float64
		if cbHeader.CBTipe == "K" {
			dDebet = totalNominal
		} else {
			dKredit = totalNominal
		}

		database.Table("public.gl_t_jurnald").Create(map[string]interface{}{
			"tjurd_tjurhno":    docNo,
			"tjurd_acccode":    "084000000007",
			"tjurd_agenid":     cbHeader.CBAgenID,
			"tjurd_keterangan": cbHeader.CBKet,
			"tjurd_debet":      dDebet,
			"tjurd_kredit":     dKredit,
		})
	}

	// 8. Update status Posting & Nomor Jurnal di tabel gl_t_cashbank
	database.Table("public.gl_t_cashbank").Where("cb_id = ?", req.CBID).Updates(map[string]interface{}{
		"cb_postyn":   "Y",
		"cb_nojurnal": docNo,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   fmt.Sprintf("Posting Berhasil! No Jurnal: %s", docNo),
		"no_jurnal": docNo,
	})
}
