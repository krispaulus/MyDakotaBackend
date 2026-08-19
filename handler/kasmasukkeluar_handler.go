package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CashBankListModel struct {
	CBID        string  `json:"cb_id" gorm:"column:cb_id"`
	CBTanggal   string  `json:"cb_tanggal" gorm:"column:cb_tanggal"`
	CBTipe      string  `json:"cb_tipe" gorm:"column:cb_tipe"`
	CBKet       string  `json:"cb_ket" gorm:"column:cb_ket"`
	CBAktifYN   string  `json:"cb_aktifyn" gorm:"column:cb_aktifyn"`
	CBNoJurnal  string  `json:"cb_nojurnal" gorm:"column:cb_nojurnal"`
	CBPostYN    string  `json:"cb_postyn" gorm:"column:cb_postyn"`
	AgenNama    string  `json:"agen_nama" gorm:"column:agen_nama"`
	TotalAmount float64 `json:"total_amount" gorm:"column:total_amount"`
}

type CreateCashBankReq struct {
	CBID        string  `json:"cb_id"`
	CBTanggal   string  `json:"cb_tanggal" binding:"required"`
	CBTipe      string  `json:"cb_tipe" binding:"required"` // 'T' = Terima, 'K' = Keluar
	CBTransAgen string  `json:"cb_transagenid"`
	CBKet       string  `json:"cb_ket"`
	CBPostYN    string  `json:"cb_postyn"`
	Nominal     float64 `json:"nominal"`
}

type PostCashBankReq struct {
	CBID         string `json:"cb_id" binding:"required"`
	SumberDana   string `json:"sumber_dana" binding:"required"` // KAS OPS DK, KAS OPS LK, BCA FLEET, BANK, E-TOLL
	BankAccount  string `json:"bank_account"`
	EtollAccount string `json:"etoll_account"`
}

type ItemBiayaModel struct {
	ItemID   string `json:"item_id" gorm:"column:item_id"`
	ItemName string `json:"item_name" gorm:"column:item_name"`
	ItemCAID string `json:"item_caid" gorm:"column:item_caid"`
}

func getCashBankDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/cashbank (READ LIST TRANS KAS / BANK)
// =========================================================================
func GetCashBankListHandler(c *gin.Context) {
	database := getCashBankDB(c)

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tipe := c.Query("tipe")
	noTrans := c.Query("no_trans")
	cabangNama := c.Query("cabang_nama")
	aktifYN := c.Query("aktif_yn")
	postingYN := c.Query("posting_yn")
	noJurnal := c.Query("no_jurnal")
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "500")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 500
	}
	offset := (page - 1) * limit

	query := database.Table("public.gl_t_cashbank cb").
		Select(`
			cb.cb_id, 
			TO_CHAR(cb.cb_tanggal, 'YYYY-MM-DD') AS cb_tanggal, 
			cb.cb_tipe, 
			COALESCE(cb.cb_ket, '-') AS cb_ket, 
			COALESCE(cb.cb_aktifyn, 'Y') AS cb_aktifyn, 
			COALESCE(cb.cb_nojurnal, '-') AS cb_nojurnal, 
			COALESCE(cb.cb_postyn, 'N') AS cb_postyn, 
			COALESCE(a.agen_nama, '-') AS agen_nama, 
			COALESCE(cb.cb_total, SUM(cbd.cbd_quantity * cbd.cbd_hargasatuan), 0) AS total_amount
		`).
		Joins("LEFT JOIN public.gl_t_cashbankdetil cbd ON TRIM(BOTH FROM CAST(cb.cb_id AS VARCHAR)) = TRIM(BOTH FROM CAST(cbd.cbd_cbid AS VARCHAR))").
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(BOTH FROM a.agen_id::varchar) = TRIM(BOTH FROM COALESCE(cb.cb_transagenid, cb.cb_agenid, '1')::varchar)").
		Where("COALESCE(cb.cb_id, '') <> ''")

	if startDate != "" && endDate != "" {
		query = query.Where("cb.cb_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if tipe != "" {
		query = query.Where("cb.cb_tipe = ?", tipe)
	}
	if noTrans != "" {
		query = query.Where("cb.cb_id ILIKE ?", "%"+noTrans+"%")
	}
	if cabangNama != "" {
		query = query.Where("a.agen_nama = ?", cabangNama)
	}
	if aktifYN != "" {
		query = query.Where("cb.cb_aktifyn = ?", aktifYN)
	} else {
		query = query.Where("cb.cb_aktifyn = 'Y'")
	}
	if postingYN != "" {
		query = query.Where("cb.cb_postyn = ?", postingYN)
	}
	if noJurnal != "" {
		query = query.Where("cb.cb_nojurnal ILIKE ?", "%"+noJurnal+"%")
	}

	query = query.Group("cb.cb_id, cb.cb_tanggal, cb.cb_tipe, cb.cb_ket, cb.cb_total, cb.cb_aktifyn, cb.cb_nojurnal, cb.cb_postyn, a.agen_nama")

	var totalRecords int64
	database.Table("(?) AS count_tbl", query).Count(&totalRecords)

	var list []CashBankListModel
	err := query.Order("cb.cb_tanggal DESC, cb.cb_id DESC").Limit(limit).Offset(offset).Scan(&list).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          list,
		"total_records": totalRecords,
	})
}

// =========================================================================
// 2. POST /api/gl/cashbank/create (INPUT TRANS KAS / BANK)
// =========================================================================
func CreateCashBankHandler(c *gin.Context) {
	database := getCashBankDB(c)
	userID, _ := c.Get("username")
	userAgenID, _ := c.Get("agen_id")

	var req CreateCashBankReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Resolve Agen ID Dinamis
	agenIDFinal := strings.TrimSpace(req.CBTransAgen)
	if agenIDFinal == "" && userAgenID != nil {
		agenIDFinal = strings.TrimSpace(fmt.Sprintf("%v", userAgenID))
	}
	if agenIDFinal == "" {
		agenIDFinal = "1"
	}

	// Parse Tanggal Transaksi
	tglTrans, errDate := time.Parse("2006-01-02", req.CBTanggal)
	if errDate != nil {
		tglTrans = time.Now()
	}

	// Auto-generate No Transaksi Kas Standar Format Legacy: [AGEN_3DIGIT][URUTAN_6DIGIT]/[MM]/[YYYY]/[BK/BT]
	suffixTipe := "BK"
	if strings.ToUpper(req.CBTipe) == "T" {
		suffixTipe = "BT"
	}

	agen3Digit := fmt.Sprintf("%03s", agenIDFinal)
	if len(agen3Digit) > 3 {
		agen3Digit = agen3Digit[len(agen3Digit)-3:]
	}

	bulanTahun := tglTrans.Format("01/2006")
	patternSearch := fmt.Sprintf("%s%%/%s/%s", agen3Digit, bulanTahun, suffixTipe)

	var nextUrut int = 1
	var lastCBID string
	database.Table("public.gl_t_cashbank").
		Select("cb_id").
		Where("cb_id LIKE ?", patternSearch).
		Order("cb_id DESC").
		Limit(1).
		Scan(&lastCBID)

	if lastCBID != "" && len(lastCBID) >= 9 {
		urutStr := lastCBID[3:9]
		if num, err := strconv.Atoi(urutStr); err == nil {
			nextUrut = num + 1
		}
	}

	if req.CBID == "" {
		req.CBID = fmt.Sprintf("%s%06d/%s/%s", agen3Digit, nextUrut, bulanTahun, suffixTipe)
	}

	insertHeader := map[string]interface{}{
		"cb_id":          strings.TrimSpace(req.CBID),
		"cb_nourut":      nextUrut,
		"cb_tanggal":     req.CBTanggal + " " + time.Now().Format("15:04:05"),
		"cb_tipe":        strings.ToUpper(req.CBTipe),
		"cb_agenid":      agenIDFinal,
		"cb_transagenid": agenIDFinal,
		"cb_ket":         req.CBKet,
		"cb_total":       req.Nominal,
		"cb_postyn":      "N",
		"cb_aktifyn":     "Y",
		"cb_updateid":    fmt.Sprintf("%v", userID),
		"cb_updatetime":  time.Now(),
	}

	if err := database.Table("public.gl_t_cashbank").Create(insertHeader).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan Kas/Bank: " + err.Error()})
		return
	}

	// Insert Detail jika ada nominal (Ke tabel public.gl_t_cashbankdetil)
	if req.Nominal > 0 {
		insertDetail := map[string]interface{}{
			"cbd_cbid":        strings.TrimSpace(req.CBID),
			"cbd_ket":         req.CBKet,
			"cbd_quantity":    1,
			"cbd_hargasatuan": req.Nominal,
			"cbd_agenid":      agenIDFinal,
			"cbd_susutyn":     "N",
		}
		database.Table("public.gl_t_cashbankdetil").Create(insertDetail)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Pencatatan Kas/Bank No %s Berhasil Disimpan!", req.CBID),
		"cb_id":   req.CBID,
	})
}

// =========================================================================
// 3. DELETE /api/gl/cashbank/:id (BATALKAN / VOID TRANS KAS)
// =========================================================================
func DeleteCashBankHandler(c *gin.Context) {
	// Ambil param id dan bersihkan leading slash jika ada
	cbID := strings.TrimPrefix(c.Param("id"), "/")
	database := getCashBankDB(c)

	err := database.Table("public.gl_t_cashbank").
		Where("cb_id = ?", cbID).
		Update("cb_aktifyn", "N").Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membatalkan transaksi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Transaksi Kas/Bank No %s berhasil dibatalkan!", cbID),
	})
}

// =========================================================================
// 4. POST /api/gl/cashbank/update (UPDATE / UNPOSTING TRANS KAS)
// =========================================================================
func UpdateCashBankHandler(c *gin.Context) {
	database := getCashBankDB(c)
	userID, _ := c.Get("username")

	var req CreateCashBankReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if strings.TrimSpace(req.CBID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor transaksi tidak boleh kosong!"})
		return
	}

	postYN := "N"
	if strings.ToUpper(strings.TrimSpace(req.CBPostYN)) == "Y" {
		postYN = "Y"
	}

	updateHeader := map[string]interface{}{
		"cb_tanggal":    req.CBTanggal + " " + time.Now().Format("15:04:05"),
		"cb_tipe":       strings.ToUpper(req.CBTipe),
		"cb_ket":        req.CBKet,
		"cb_total":      req.Nominal,
		"cb_postyn":     postYN,
		"cb_updateid":   fmt.Sprintf("%v", userID),
		"cb_updatetime": time.Now(),
	}

	// 💡 Jika status diubah menjadi UNPOSTING ('N'), kosongkan nomor jurnal terkait
	if postYN == "N" {
		updateHeader["cb_nojurnal"] = "-"
	}

	if err := database.Table("public.gl_t_cashbank").
		Where("cb_id = ?", strings.TrimSpace(req.CBID)).
		Updates(updateHeader).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update transaksi: " + err.Error()})
		return
	}

	// Update rincian nominal di tabel detail
	database.Table("public.gl_t_cashbankdetil").
		Where("cbd_cbid = ?", strings.TrimSpace(req.CBID)).
		Updates(map[string]interface{}{
			"cbd_ket":         req.CBKet,
			"cbd_hargasatuan": req.Nominal,
		})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Transaksi Kas/Bank %s berhasil diupdate (Status Posting: %s)!", req.CBID, postYN),
	})
}

// =========================================================================
// 5. POST /api/gl/cashbank/post (POSTING DENGAN KONFIRMASI SUMBER DANA)
// =========================================================================
func PostCashBankHandler(c *gin.Context) {
	database := getCashBankDB(c)
	userID, _ := c.Get("username")

	var req PostCashBankReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// 1. Ambil data Header CashBank menggunakan Map
	var cbMap map[string]interface{}
	if err := database.Table("public.gl_t_cashbank").Where("cb_id = ?", req.CBID).Take(&cbMap).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data transaksi kas tidak ditemukan"})
		return
	}

	cbNoJurnal := fmt.Sprintf("%v", cbMap["cb_nojurnal"])
	cbTipe := fmt.Sprintf("%v", cbMap["cb_tipe"])
	cbAgenID := fmt.Sprintf("%v", cbMap["cb_agenid"])
	cbKet := fmt.Sprintf("%v", cbMap["cb_ket"])

	// Parse Tanggal Transaksi
	var tglTrans time.Time
	if rawTgl, ok := cbMap["cb_tanggal"].(time.Time); ok {
		tglTrans = rawTgl
	} else {
		tglStr := fmt.Sprintf("%v", cbMap["cb_tanggal"])
		tglTrans, _ = time.Parse("2006-01-02 15:04:05", tglStr)
		if tglTrans.IsZero() {
			tglTrans = time.Now()
		}
	}

	// 2. Generate Nomor Jurnal Otomatis jika belum ada: [YY][MM][AGEN_3DIGIT][TIPE][URUTAN_5DIGIT]
	noJurnal := cbNoJurnal
	if strings.TrimSpace(noJurnal) == "" || noJurnal == "-" || noJurnal == "<nil>" {
		agen3Digit := fmt.Sprintf("%03s", cbAgenID)
		if len(agen3Digit) > 3 {
			agen3Digit = agen3Digit[len(agen3Digit)-3:]
		}

		prefixJurnal := fmt.Sprintf("%s%s%s", tglTrans.Format("0601"), agen3Digit, cbTipe)

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

		noJurnal = fmt.Sprintf("%s%05d", prefixJurnal, nextUrut)

		// Simpan Header Jurnal Otomatis
		insertJurnalH := map[string]interface{}{
			"tjurh_no":         noJurnal,
			"tjurh_tanggal":    tglTrans,
			"tjurh_type":       cbTipe,
			"tjurh_keterangan": fmt.Sprintf("%s (SUMBER: %s)", cbKet, req.SumberDana),
			"tjurh_postyn":     "Y",
			"tjurh_deleteyn":   "N",
			"tjurh_updateid":   fmt.Sprintf("%v", userID),
			"tjurh_updatetime": time.Now(),
		}
		database.Table("public.gl_t_jurnalh").Create(insertJurnalH)
	}

	// 3. Update Status Posting KasBank menjadi 'Y'
	updateData := map[string]interface{}{
		"cb_postyn":     "Y",
		"cb_nojurnal":   noJurnal,
		"cb_updateid":   fmt.Sprintf("%v", userID),
		"cb_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_t_cashbank").Where("cb_id = ?", req.CBID).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal posting transaksi kas: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   fmt.Sprintf("Transaksi Kas %s Berhasil Diposting ke Jurnal %s (Sumber: %s)!", req.CBID, noJurnal, req.SumberDana),
		"no_jurnal": noJurnal,
	})
}

// =========================================================================
// 6. POST /api/gl/cashbank/unpost (UNPOSTING TRANS KAS)
// =========================================================================
func UnpostCashBankHandler(c *gin.Context) {
	database := getCashBankDB(c)
	userID, _ := c.Get("username")

	var req struct {
		CBID string `json:"cb_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	updateData := map[string]interface{}{
		"cb_postyn":     "N",
		"cb_updateid":   fmt.Sprintf("%v", userID),
		"cb_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_t_cashbank").Where("cb_id = ?", req.CBID).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal unposting transaksi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Transaksi Kas %s berhasil di-unposting!", req.CBID),
	})
}

// =========================================================================
// 7. GET /api/gl/cashbank/items (PENCARIAN AUTOCOMPLETE MASTER BIAYA)
// =========================================================================
func GetMasterItemBiayaHandler(c *gin.Context) {
	database := getCashBankDB(c)
	keyword := strings.TrimSpace(c.Query("q"))

	type ItemResult struct {
		ItemID   string `json:"item_id" gorm:"column:item_id"`
		ItemName string `json:"item_name" gorm:"column:item_name"`
	}

	var list []ItemResult
	query := database.Table("public.gl_m_item").
		Select("item_id, item_name").
		Where("COALESCE(item_aktifyn, 'Y') = 'Y'")

	if keyword != "" {
		query = query.Where("item_name ILIKE ? OR item_id ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	if err := query.Order("item_name ASC").Limit(30).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}
