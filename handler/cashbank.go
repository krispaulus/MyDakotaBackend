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
		if strings.TrimSpace(req.BankCode) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Pilihan Bank tidak boleh kosong!"})
			return
		}
		lawanAccCode = strings.TrimSpace(req.BankCode)
		lawanAccName = strings.TrimSpace(req.BankName)

	} else if strings.Contains(sumber, "etoll") || strings.Contains(sumber, "toll") {
		if strings.TrimSpace(req.EtollCode) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Pilihan E-TOLL tidak boleh kosong!"})
			return
		}
		lawanAccCode = strings.TrimSpace(req.EtollCode)
		lawanAccName = strings.TrimSpace(req.EtollName)

	} else if strings.Contains(sumber, "fleet") {
		err := database.Table("public.gl_m_bank").
			Select("COALESCE(NULLIF(bank_acccode, ''), bank_id::varchar) AS code, bank_name").
			Where("bank_name ILIKE '%FLEET%' AND COALESCE(bank_aktifyn, 'Y') = 'Y'").
			Row().Scan(&lawanAccCode, &lawanAccName)

		if err != nil || lawanAccCode == "" {
			err = database.Table("public.gl_m_chartaccount").
				Select("ca_id, ca_name").
				Where("ca_name ILIKE '%FLEET%' AND COALESCE(ca_aktifyn, 'Y') = 'Y'").
				Row().Scan(&lawanAccCode, &lawanAccName)
		}

	} else if strings.Contains(sumber, "lk") {
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

	// 4. Ambil Rincian Kas dari gl_t_cashbankdetil + LEFT JOIN gl_m_item (TANPA HARDCODE)
	type CBDetail struct {
		ItemID      string  `gorm:"column:cbd_itemid"`
		Ket         string  `gorm:"column:cbd_ket"`
		Quantity    float64 `gorm:"column:cbd_quantity"`
		HargaSatuan float64 `gorm:"column:cbd_hargasatuan"`
		AgenID      string  `gorm:"column:cbd_agenid"`
		OwnCAID     string  `gorm:"column:item_owncaid"`
		ItemName    string  `gorm:"column:item_name"`
	}

	var rincian []CBDetail
	if err := database.Table("public.gl_t_cashbankdetil d").
		Select(`
			TRIM(BOTH FROM d.cbd_itemid) AS cbd_itemid,
			TRIM(BOTH FROM d.cbd_ket) AS cbd_ket,
			d.cbd_quantity,
			d.cbd_hargasatuan,
			TRIM(BOTH FROM COALESCE(d.cbd_agenid, '')) AS cbd_agenid,
			TRIM(BOTH FROM COALESCE(i.item_owncaid, '')) AS item_owncaid,
			TRIM(BOTH FROM COALESCE(i.item_name, '')) AS item_name
		`).
		Joins("LEFT JOIN public.gl_m_item i ON TRIM(BOTH FROM d.cbd_itemid) = TRIM(BOTH FROM i.item_id)").
		Where("TRIM(BOTH FROM d.cbd_cbid) = TRIM(?)", req.CBID).
		Scan(&rincian).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membaca rincian kas: " + err.Error()})
		return
	}

	// Validasi 1: Rincian kas tidak boleh kosong (tidak boleh fallback ke akun hardcode)
	if len(rincian) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Transaksi %s belum memiliki rincian item biaya di GL_T_CashBankDetil. Silakan lengkapi rincian terlebih dahulu!", req.CBID),
		})
		return
	}

	// Validasi 2: Setiap item rincian wajib memiliki Kode Item & No. Acc Utama (item_owncaid) di Master Item
	var totalNominal float64
	for _, r := range rincian {
		if strings.TrimSpace(r.ItemID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("Rincian kas pada transaksi %s tidak memiliki Kode Item (cbd_itemid kosong di GL_T_CashBankDetil). Silakan edit dan simpan ulang transaksi!", req.CBID),
			})
			return
		}
		if strings.TrimSpace(r.OwnCAID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("Item '%s' (%s) belum memiliki No. Acc Utama (item_owncaid) di Master Pemasukan & Pengeluaran!", r.ItemName, r.ItemID),
			})
			return
		}
		totalNominal += (r.Quantity * r.HargaSatuan)
	}

	if totalNominal <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Total nominal rincian kas harus lebih dari 0!",
		})
		return
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
	if err := database.Table("public.gl_t_jurnalh").Create(insertJH).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membuat header jurnal: " + err.Error()})
		return
	}

	// 6. Simpan Baris 1: Akun Sumber Dana (Kredit jika Keluar Kas, Debet jika Terima Kas)
	var debetLawan, kreditLawan float64
	if cbHeader.CBTipe == "K" {
		kreditLawan = totalNominal
	} else {
		debetLawan = totalNominal
	}

	if err := database.Table("public.gl_t_jurnald").Create(map[string]interface{}{
		"tjurd_tjurhno":    docNo,
		"tjurd_acccode":    lawanAccCode,
		"tjurd_agenid":     cbHeader.CBAgenID,
		"tjurd_keterangan": fmt.Sprintf("%s : %s", lawanAccName, cbHeader.CBKet),
		"tjurd_debet":      debetLawan,
		"tjurd_kredit":     kreditLawan,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan jurnal sumber dana: " + err.Error()})
		return
	}

	// 7. Simpan Baris 2+: Akun Rincian Biaya dari Master Item (100% dari DB)
	for _, r := range rincian {
		subtotal := r.Quantity * r.HargaSatuan
		var dDebet, dKredit float64
		if cbHeader.CBTipe == "K" {
			dDebet = subtotal
		} else {
			dKredit = subtotal
		}

		ketJurnal := strings.TrimSpace(r.Ket)
		if ketJurnal == "" {
			ketJurnal = strings.TrimSpace(r.ItemName)
		}

		agenBaris := strings.TrimSpace(r.AgenID)
		if agenBaris == "" {
			agenBaris = cbHeader.CBAgenID
		}

		if err := database.Table("public.gl_t_jurnald").Create(map[string]interface{}{
			"tjurd_tjurhno":    docNo,
			"tjurd_acccode":    strings.TrimSpace(r.OwnCAID),
			"tjurd_agenid":     agenBaris,
			"tjurd_keterangan": ketJurnal,
			"tjurd_debet":      dDebet,
			"tjurd_kredit":     dKredit,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan rincian jurnal: " + err.Error()})
			return
		}
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
