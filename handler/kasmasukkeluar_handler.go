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

type BPLKHeaderResponse struct {
	SJHID           string               `json:"sjh_id"`
	SJHKendID       string               `json:"sjh_kendid"`
	SJHKeterangan   string               `json:"sjh_keterangan"`
	AssNama         string               `json:"ass_nama"`
	NmSopir1        string               `json:"nm_sopir1"`
	NmSopir2        string               `json:"nm_sopir2"`
	RuteUtama       string               `json:"rute_utama"`
	RuteCadangan    string               `json:"rute_cadangan"`
	TotalUangMuka   float64              `json:"total_uang_muka"`
	ItemDebet       []BPLKDebetItem      `json:"item_debet"`
	AkunKredit      []BPLKAkunKreditItem `json:"akun_kredit"`
	DaftarKasKeluar []BPLKKasKeluarItem  `json:"daftar_kas_keluar"`
}

type BPLKDebetItem struct {
	ItemName string  `json:"item_name"`
	ItemID   string  `json:"item_id"`
	Nominal  float64 `json:"nominal"`
}

type BPLKAkunKreditItem struct {
	AccCode string `json:"acc_code"`
	AccName string `json:"acc_name"`
}

type BPLKKasKeluarItem struct {
	CBTanggal  string  `json:"cb_tanggal"`
	AgenNama   string  `json:"agen_nama"`
	CBID       string  `json:"cb_id"`
	CBNoJurnal string  `json:"cb_nojurnal"`
	Nominal    float64 `json:"nominal"`
}

type ProsesUangMukaBPLKReq struct {
	SJHID      string            `json:"sjh_id" binding:"required"`
	Tanggal    string            `json:"tanggal"`
	DebetList  []BPLKDebetItem   `json:"debet_list" binding:"required"`
	KreditList []InputKreditItem `json:"kredit_list" binding:"required"`
}

type InputKreditItem struct {
	AccCode string  `json:"acc_code"`
	AccName string  `json:"acc_name"`
	Nominal float64 `json:"nominal"`
}

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

// type PostCashBankReq struct {
// 	CBID         string `json:"cb_id" binding:"required"`
// 	SumberDana   string `json:"sumber_dana" binding:"required"` // KAS OPS DK, KAS OPS LK, BCA FLEET, BANK, E-TOLL
// 	BankAccount  string `json:"bank_account"`
// 	EtollAccount string `json:"etoll_account"`
// }

type ItemBiayaModel struct {
	ItemID   string `json:"item_id" gorm:"column:item_id"`
	ItemName string `json:"item_name" gorm:"column:item_name"`
	ItemCAID string `json:"item_caid" gorm:"column:item_caid"`
}

func getCashBankDB(c *gin.Context) *gorm.DB {
	ptID := strings.TrimSpace(c.Query("pt_id"))
	if ptID == "" {
		if val, exists := c.Get("pt_id"); exists && val != nil {
			ptID = strings.TrimSpace(fmt.Sprintf("%v", val))
		}
	}

	// 💡 Mapping kode PT jika resolver kamu mendaftarkan dengan key khusus
	if ptID != "" {
		// Coba langsung dengan ptID (misal "C", "B", "L")
		if database, ok := db.ResolveDB(ptID); ok && database != nil {
			return database
		}
		// Coba mapping lowercase (misal "C" -> "dli", "B" -> "dbs")
		dbMap := map[string]string{
			"c": "dli", "C": "dli", "holding": "dli", "dli": "dli",
			"b": "dbs", "B": "dbs", "dbs": "dbs",
			"l": "dlb", "L": "dlb", "dlb": "dlb",
		}
		if targetKey, found := dbMap[ptID]; found {
			if database, ok := db.ResolveDB(targetKey); ok && database != nil {
				return database
			}
		}
	}

	// Fallback ke DLI jika default belum ketemu
	if dliDB, ok := db.ResolveDB("dli"); ok && dliDB != nil {
		return dliDB
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
// func PostCashBankHandler(c *gin.Context) {
// 	database := getCashBankDB(c)
// 	userID, _ := c.Get("username")

// 	var req PostCashBankReq
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
// 		return
// 	}

// 	// 1. Ambil data Header CashBank menggunakan Map
// 	var cbMap map[string]interface{}
// 	if err := database.Table("public.gl_t_cashbank").Where("cb_id = ?", req.CBID).Take(&cbMap).Error; err != nil {
// 		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data transaksi kas tidak ditemukan"})
// 		return
// 	}

// 	cbNoJurnal := fmt.Sprintf("%v", cbMap["cb_nojurnal"])
// 	cbTipe := fmt.Sprintf("%v", cbMap["cb_tipe"])
// 	cbAgenID := fmt.Sprintf("%v", cbMap["cb_agenid"])
// 	cbKet := fmt.Sprintf("%v", cbMap["cb_ket"])

// 	// Parse Tanggal Transaksi
// 	var tglTrans time.Time
// 	if rawTgl, ok := cbMap["cb_tanggal"].(time.Time); ok {
// 		tglTrans = rawTgl
// 	} else {
// 		tglStr := fmt.Sprintf("%v", cbMap["cb_tanggal"])
// 		tglTrans, _ = time.Parse("2006-01-02 15:04:05", tglStr)
// 		if tglTrans.IsZero() {
// 			tglTrans = time.Now()
// 		}
// 	}

// 	// 2. Generate Nomor Jurnal Otomatis jika belum ada: [YY][MM][AGEN_3DIGIT][TIPE][URUTAN_5DIGIT]
// 	noJurnal := cbNoJurnal
// 	if strings.TrimSpace(noJurnal) == "" || noJurnal == "-" || noJurnal == "<nil>" {
// 		agen3Digit := fmt.Sprintf("%03s", cbAgenID)
// 		if len(agen3Digit) > 3 {
// 			agen3Digit = agen3Digit[len(agen3Digit)-3:]
// 		}

// 		prefixJurnal := fmt.Sprintf("%s%s%s", tglTrans.Format("0601"), agen3Digit, cbTipe)

// 		var lastJurnal string
// 		database.Table("public.gl_t_jurnalh").
// 			Select("tjurh_no").
// 			Where("tjurh_no LIKE ?", prefixJurnal+"%").
// 			Order("tjurh_no DESC").
// 			Limit(1).
// 			Scan(&lastJurnal)

// 		nextUrut := 1
// 		if lastJurnal != "" && len(lastJurnal) >= len(prefixJurnal)+5 {
// 			if num, err := strconv.Atoi(lastJurnal[len(prefixJurnal):]); err == nil {
// 				nextUrut = num + 1
// 			}
// 		}

// 		noJurnal = fmt.Sprintf("%s%05d", prefixJurnal, nextUrut)

// 		// Simpan Header Jurnal Otomatis
// 		insertJurnalH := map[string]interface{}{
// 			"tjurh_no":         noJurnal,
// 			"tjurh_tanggal":    tglTrans,
// 			"tjurh_type":       cbTipe,
// 			"tjurh_keterangan": fmt.Sprintf("%s (SUMBER: %s)", cbKet, req.SumberDana),
// 			"tjurh_postyn":     "Y",
// 			"tjurh_deleteyn":   "N",
// 			"tjurh_updateid":   fmt.Sprintf("%v", userID),
// 			"tjurh_updatetime": time.Now(),
// 		}
// 		database.Table("public.gl_t_jurnalh").Create(insertJurnalH)
// 	}

// 	// 3. Update Status Posting KasBank menjadi 'Y'
// 	updateData := map[string]interface{}{
// 		"cb_postyn":     "Y",
// 		"cb_nojurnal":   noJurnal,
// 		"cb_updateid":   fmt.Sprintf("%v", userID),
// 		"cb_updatetime": time.Now(),
// 	}

// 	if err := database.Table("public.gl_t_cashbank").Where("cb_id = ?", req.CBID).Updates(updateData).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal posting transaksi kas: " + err.Error()})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"status":    "success",
// 		"message":   fmt.Sprintf("Transaksi Kas %s Berhasil Diposting ke Jurnal %s (Sumber: %s)!", req.CBID, noJurnal, req.SumberDana),
// 		"no_jurnal": noJurnal,
// 	})
// }

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

// Struct untuk Laporan Cash Flow
type CashFlowReportItem struct {
	CBDKet         string  `json:"cbd_ket"`
	ItemOwnCAID    string  `json:"item_owncaid"`
	ItemTipe       string  `json:"item_tipe"` // 'K' = Kredit, 'T'/'D' = Debet
	CBDHargaSatuan float64 `json:"cbd_hargasatuan"`
	Debet          float64 `json:"debet"`
	Kredit         float64 `json:"kredit"`
}

type CashFlowReportHeader struct {
	CBID      string               `json:"cb_id"`
	CBTanggal string               `json:"cb_tanggal"`
	CBKet     string               `json:"cb_ket"`
	CBTipe    string               `json:"cb_tipe"`
	SubDebet  float64              `json:"sub_debet"`
	SubKredit float64              `json:"sub_kredit"`
	Details   []CashFlowReportItem `json:"details"`
}

// GET /api/gl/cashbank/laporan-cashflow
func GetLaporanCashFlowHandler(c *gin.Context) {
	database := getCashBankDB(c)
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	cabangNama := strings.TrimSpace(c.Query("cabang_nama"))

	query := database.Table("public.gl_t_cashbank cb").
		Select(`
			TRIM(BOTH FROM CAST(cb.cb_id AS VARCHAR)) AS cb_id, 
			TO_CHAR(cb.cb_tanggal, 'YYYY-MM-DD') AS cb_tanggal, 
			cb.cb_tipe, 
			COALESCE(cb.cb_ket, '-') AS cb_ket
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(BOTH FROM a.agen_id::varchar) = TRIM(BOTH FROM COALESCE(cb.cb_transagenid, cb.cb_agenid, '1')::varchar)").
		Where("COALESCE(cb.cb_aktifyn, 'Y') = 'Y'")

	if startDate != "" && endDate != "" {
		query = query.Where("cb.cb_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if cabangNama != "" && cabangNama != "-- SEMUA CABANG --" {
		query = query.Where("a.agen_nama = ?", cabangNama)
	}

	type RawHeader struct {
		CBID      string `gorm:"column:cb_id"`
		CBTanggal string `gorm:"column:cb_tanggal"`
		CBTipe    string `gorm:"column:cb_tipe"`
		CBKet     string `gorm:"column:cb_ket"`
	}

	var headers []RawHeader
	if err := query.Order("cb.cb_tanggal ASC, cb.cb_id ASC").Scan(&headers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query header: " + err.Error()})
		return
	}

	var reportList []CashFlowReportHeader
	var grandTotalDebet, grandTotalKredit float64

	for _, h := range headers {
		type RawDetail struct {
			CBDKet         string  `gorm:"column:cbd_ket"`
			ItemCAID       string  `gorm:"column:item_caid"`
			CBDQuantity    float64 `gorm:"column:cbd_quantity"`
			CBDHargaSatuan float64 `gorm:"column:cbd_hargasatuan"`
		}

		var rawDetails []RawDetail
		// Query detail aman dengan CAST VARCHAR dan COALESCE
		database.Table("public.gl_t_cashbankdetil cbd").
			Select(`
				COALESCE(cbd.cbd_ket, '') AS cbd_ket,
				COALESCE(it.item_caid, '') AS item_caid,
				COALESCE(cbd.cbd_quantity, 1) AS cbd_quantity,
				COALESCE(cbd.cbd_hargasatuan, 0) AS cbd_hargasatuan
			`).
			Joins("LEFT JOIN public.gl_m_item it ON TRIM(BOTH FROM it.item_id::varchar) = TRIM(BOTH FROM cbd.cbd_itemid::varchar)").
			Where("TRIM(BOTH FROM CAST(cbd.cbd_cbid AS VARCHAR)) = ?", h.CBID).
			Scan(&rawDetails)

		var details []CashFlowReportItem
		var subDebet, subKredit float64

		for _, d := range rawDetails {
			nominal := d.CBDQuantity * d.CBDHargaSatuan
			if nominal <= 0 {
				nominal = d.CBDHargaSatuan
			}

			var debet, kredit float64
			// Sesuai kaidah kas: Tipe 'K' (Keluar Kas) = Kredit, Tipe 'T' (Terima Kas) = Debit
			if strings.ToUpper(h.CBTipe) == "K" {
				kredit = nominal
				debet = 0
			} else {
				debet = nominal
				kredit = 0
			}

			subDebet += debet
			subKredit += kredit

			details = append(details, CashFlowReportItem{
				CBDKet:         d.CBDKet,
				ItemOwnCAID:    d.ItemCAID,
				ItemTipe:       h.CBTipe,
				CBDHargaSatuan: nominal,
				Debet:          debet,
				Kredit:         kredit,
			})
		}

		grandTotalDebet += subDebet
		grandTotalKredit += subKredit

		reportList = append(reportList, CashFlowReportHeader{
			CBID:      h.CBID,
			CBTanggal: h.CBTanggal,
			CBKet:     h.CBKet,
			CBTipe:    h.CBTipe,
			SubDebet:  subDebet,
			SubKredit: subKredit,
			Details:   details,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":             "success",
		"data":               reportList,
		"grand_total_debet":  grandTotalDebet,
		"grand_total_kredit": grandTotalKredit,
		"sisa_kas":           grandTotalDebet - grandTotalKredit,
	})
}

func GetBPLKDetailHandler(c *gin.Context) {
	database := getCashBankDB(c)
	stNumber := strings.TrimSpace(c.Query("st"))
	if stNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Surat Tugas wajib diisi"})
		return
	}

	// 1. Query Data Header Surat Jalan
	type RawHeader struct {
		SJHID         string `gorm:"column:sjh_id"`
		SJHKendID     string `gorm:"column:sjh_kendid"`
		SJHKeterangan string `gorm:"column:sjh_keterangan"`
		TrayekID      string `gorm:"column:sjh_trayekid"`
		AssNama       string `gorm:"column:ass_nama"`
		NmSopir1      string `gorm:"column:nmsopir1"`
		NmSopir2      string `gorm:"column:nmsopir2"`
	}
	var rh RawHeader

	queryHeader := `
		SELECT 
			sj.sjh_id,
			sj.sjh_kendid,
			COALESCE(sj.sjh_keterangan, '-') AS sjh_keterangan,
			sj.sjh_trayekid,
			COALESCE(ass.ass_nama, 'Luar Kota') AS ass_nama,
			COALESCE(s1.kry_nama, '-') AS nmsopir1,
			COALESCE(s2.kry_nama, '') AS nmsopir2
		FROM public.opr_t_esuratjalan sj
		LEFT JOIN public.opr_m_assignment ass ON ass.ass_id = COALESCE(sj.sjh_assid, 'AS009')
		LEFT JOIN public.hrd_m_karyawan s1 ON s1.kry_nip = sj.sjh_sopir1_nip
		LEFT JOIN public.hrd_m_karyawan s2 ON s2.kry_nip = sj.sjh_sopir2_nip
		WHERE UPPER(TRIM(sj.sjh_id)) = UPPER(TRIM(?))
		LIMIT 1
	`
	if err := database.Raw(queryHeader, stNumber).Scan(&rh).Error; err != nil || rh.SJHID == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": fmt.Sprintf("Surat Tugas %s tidak ditemukan!", stNumber)})
		return
	}

	// 2. Query Rute Utama & Rute Cadangan Trayek
	var ruteUtamaList []string
	database.Raw(`
		SELECT ag.agen_nama 
		FROM public.opr_m_trayekd td
		JOIN public.glb_m_agen ag ON ag.agen_id = td.trd_agenid
		WHERE td.trd_hid = ? AND td.trd_jenis = 'U'
		ORDER BY td.trd_status, td.trd_urut
	`, rh.TrayekID).Scan(&ruteUtamaList)

	var ruteCadanganList []string
	database.Raw(`
		SELECT ag.agen_nama 
		FROM public.opr_m_trayekd td
		JOIN public.glb_m_agen ag ON ag.agen_id = td.trd_agenid
		WHERE td.trd_hid = ? AND td.trd_jenis = 'C'
		ORDER BY td.trd_status, td.trd_urut
	`, rh.TrayekID).Scan(&ruteCadanganList)

	// 3. Query Rincian Uang Muka Debet Standar dari OPR_M_TrayekBPLK
	var debetItems []BPLKDebetItem
	database.Raw(`
		SELECT 
			COALESCE(it.item_name, 'Biaya Operasional') AS item_name,
			tb.tri_bplksubid AS item_id,
			COALESCE(tb.tri_nominal, 0) AS nominal
		FROM public.opr_m_trayekbplk tb
		JOIN public.gl_m_item it ON it.item_id = tb.tri_bplksubid
		WHERE tb.tri_hid = ? AND COALESCE(tb.tri_bplkid, '') = ''
		ORDER BY it.item_name ASC
	`, rh.TrayekID).Scan(&debetItems)

	// Fallback jika belum ada di OPR_M_TrayekBPLK, buat template default
	if len(debetItems) == 0 {
		debetItems = []BPLKDebetItem{
			{ItemName: "BBM LK", ItemID: "0840000000137", Nominal: 1695000},
			{ItemName: "Biaya Operasional BPLK", ItemID: "0840000000138", Nominal: 960000},
			{ItemName: "Biaya parkir LK", ItemID: "0840000000144", Nominal: 5000},
			{ItemName: "Biaya Tol LK", ItemID: "0840000000140", Nominal: 840000},
		}
	}

	var totalDebet float64
	for _, d := range debetItems {
		totalDebet += d.Nominal
	}

	// 4. Akun Kredit Standar Pencairan Uang Muka
	akunKredit := []BPLKAkunKreditItem{
		{AccCode: "A101010003", AccName: "KAS OPERASIONAL LK"},
		{AccCode: "A101020008", AccName: "BCA DROPPING-9246"},
		{AccCode: "A101020001", AccName: "GIRO BCA"},
	}

	// 5. Query Riwayat Daftar Kas Keluar yang sudah pernah dibuat untuk Surat Tugas ini
	var kasKeluarList []BPLKKasKeluarItem
	database.Raw(`
		SELECT 
			TO_CHAR(cb.cb_tanggal, 'MM/DD/YYYY') AS cb_tanggal,
			COALESCE(ag.agen_nama, 'DLI PUSAT') AS agen_nama,
			cb.cb_id,
			COALESCE(cb.cb_nojurnal, '-') AS cb_nojurnal,
			COALESCE(cb.cb_total, 0) AS nominal
		FROM public.opr_t_esuratjalancashbank sjc
		JOIN public.gl_t_cashbank cb ON cb.cb_id = sjc.sjc_cbid
		LEFT JOIN public.glb_m_agen ag ON ag.agen_id = cb.cb_agenid
		WHERE sjc.sjc_sjhid = ?
		ORDER BY cb.cb_tanggal ASC
	`, rh.SJHID).Scan(&kasKeluarList)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": BPLKHeaderResponse{
			SJHID:           rh.SJHID,
			SJHKendID:       rh.SJHKendID,
			SJHKeterangan:   rh.SJHKeterangan,
			AssNama:         rh.AssNama,
			NmSopir1:        rh.NmSopir1,
			NmSopir2:        rh.NmSopir2,
			RuteUtama:       strings.Join(ruteUtamaList, " -- "),
			RuteCadangan:    strings.Join(ruteCadanganList, "; "),
			TotalUangMuka:   totalDebet,
			ItemDebet:       debetItems,
			AkunKredit:      akunKredit,
			DaftarKasKeluar: kasKeluarList,
		},
	})
}

func ProsesUangMukaBPLKHandler(c *gin.Context) {
	database := getCashBankDB(c)
	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)
	if userStr == "" || userStr == "<nil>" {
		userStr = "RAHMAT_1"
	}

	var req ProsesUangMukaBPLKReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Hitung Total Kredit Input
	var totalKredit float64
	for _, k := range req.KreditList {
		totalKredit += k.Nominal
	}

	if totalKredit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nominal kredit pencairan belum diisi!"})
		return
	}

	// Ambil Data Surat Jalan untuk nomor kendaraan & tanggal
	var sj struct {
		KendID   string `gorm:"column:sjh_kendid"`
		TrayekID string `gorm:"column:sjh_trayekid"`
	}
	database.Table("public.opr_t_esuratjalan").
		Select("sjh_kendid, sjh_trayekid").
		Where("sjh_id = ?", req.SJHID).
		Scan(&sj)

	tglTrans := time.Now()
	if req.Tanggal != "" {
		if t, err := time.Parse("2006-01-02", req.Tanggal); err == nil {
			tglTrans = t
		}
	}

	tx := database.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Generate No Kas Keluar: [001][URUT_6DIGIT]/[MM]/[YYYY]/BK
	agen3Digit := "001"
	bulanTahun := tglTrans.Format("01/2006")
	patternSearch := fmt.Sprintf("%s%%/%s/BK", agen3Digit, bulanTahun)

	var nextUrut int = 1
	var lastCBID string
	tx.Table("public.gl_t_cashbank").
		Select("cb_id").
		Where("cb_id LIKE ?", patternSearch).
		Order("cb_id DESC").
		Limit(1).
		Scan(&lastCBID)

	if lastCBID != "" && len(lastCBID) >= 9 {
		if num, err := strconv.Atoi(lastCBID[3:9]); err == nil {
			nextUrut = num + 1
		}
	}
	newCBID := fmt.Sprintf("%s%06d/%s/BK", agen3Digit, nextUrut, bulanTahun)

	// 2. Generate No Jurnal: YYMM + 001 + K + [URUT_5DIGIT] (contoh: 2611001K00239)
	prefixJurnal := fmt.Sprintf("%s%sK", tglTrans.Format("0601"), agen3Digit)
	var nextUrutJurnal int = 1
	var lastJurnal string
	tx.Table("public.gl_t_jurnalh").
		Select("tjurh_no").
		Where("tjurh_no LIKE ?", prefixJurnal+"%").
		Order("tjurh_no DESC").
		Limit(1).
		Scan(&lastJurnal)

	if lastJurnal != "" && len(lastJurnal) >= len(prefixJurnal)+5 {
		if num, err := strconv.Atoi(lastJurnal[len(prefixJurnal):]); err == nil {
			nextUrutJurnal = num + 1
		}
	}
	newNoJurnal := fmt.Sprintf("%s%05d", prefixJurnal, nextUrutJurnal)

	keteranganJurnal := fmt.Sprintf("BPLK UANG MUKA %s %s", req.SJHID, sj.KendID)

	// 3. Simpan Header Kas/Bank (gl_t_cashbank)
	cbHeader := map[string]interface{}{
		"cb_id":          newCBID,
		"cb_nourut":      nextUrut,
		"cb_tanggal":     tglTrans,
		"cb_tipe":        "K",
		"cb_agenid":      "1",
		"cb_transagenid": "1",
		"cb_ket":         keteranganJurnal,
		"cb_total":       totalKredit,
		"cb_nojurnal":    newNoJurnal,
		"cb_postyn":      "Y",
		"cb_aktifyn":     "Y",
		"cb_pembuat":     userStr,
		"cb_updateid":    userStr,
		"cb_updatetime":  time.Now(),
	}
	if err := tx.Table("public.gl_t_cashbank").Create(&cbHeader).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan kas: " + err.Error()})
		return
	}

	// 4. Simpan Rincian Biaya Kas (gl_t_cashbankdetil)
	for _, d := range req.DebetList {
		cbDetail := map[string]interface{}{
			"cbd_cbid":        newCBID,
			"cbd_itemid":      d.ItemID,
			"cbd_ket":         d.ItemName,
			"cbd_quantity":    1,
			"cbd_hargasatuan": d.Nominal,
			"cbd_agenid":      "1",
			"cbd_susutyn":     "N",
		}
		if err := tx.Table("public.gl_t_cashbankdetil").Create(&cbDetail).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan rincian biaya: " + err.Error()})
			return
		}
	}

	// 5. Simpan Relasi Surat Jalan ke Kas (opr_t_esuratjalancashbank)
	sjcRow := map[string]interface{}{
		"sjc_sjhid":   req.SJHID,
		"sjc_cbid":    newCBID,
		"sjc_jenis":   "1", // 1 = Uang Muka
		"sjc_primary": "Y",
	}
	tx.Table("public.opr_t_esuratjalancashbank").Create(&sjcRow)

	// 6. Simpan Header Jurnal Memorial (gl_t_jurnalh)
	jhRow := map[string]interface{}{
		"tjurh_no":         newNoJurnal,
		"tjurh_tanggal":    tglTrans,
		"tjurh_type":       "K",
		"tjurh_keterangan": keteranganJurnal,
		"tjurh_postyn":     "Y",
		"tjurh_deleteyn":   "N",
		"tjurh_updateid":   userStr,
		"tjurh_updatetime": time.Now(),
	}
	tx.Table("public.gl_t_jurnalh").Create(&jhRow)

	// 7. Simpan Detail Jurnal (gl_t_jurnald) Sesuai Gambar 8:
	// A. Akun Debet: A104010200 Uang Muka Perjalanan Dinas
	jdDebet := map[string]interface{}{
		"tjurd_tjurhno":    newNoJurnal,
		"tjurd_acccode":    "A104010200",
		"tjurd_agenid":     "1",
		"tjurd_keterangan": keteranganJurnal,
		"tjurd_debet":      totalKredit,
		"tjurd_kredit":     0,
	}
	tx.Table("public.gl_t_jurnald").Create(&jdDebet)

	// B. Akun Kredit: Sesuai rekening yang dipilih user (contoh: A101020001 Giro BCA)
	for _, k := range req.KreditList {
		if k.Nominal > 0 {
			jdKredit := map[string]interface{}{
				"tjurd_tjurhno":    newNoJurnal,
				"tjurd_acccode":    k.AccCode,
				"tjurd_agenid":     "1",
				"tjurd_keterangan": keteranganJurnal,
				"tjurd_debet":      0,
				"tjurd_kredit":     k.Nominal,
			}
			tx.Table("public.gl_t_jurnald").Create(&jdKredit)
		}
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   fmt.Sprintf("Pencairan Uang Muka BPLK berhasil diproses! No Kas: %s, No Jurnal: %s", newCBID, newNoJurnal),
		"cb_id":     newCBID,
		"no_jurnal": newNoJurnal,
		"total":     totalKredit,
	})
}

//===========Kueri Menampilkan Daftar Nomor Surat Tugas Terbaru yang Siap Dipakai
//Kueri ini mengambil data Surat Tugas lengkap dengan nomor kendaraan, nama driver, trayek rute, serta status apakah sudah pernah dicairkan uang mukanya atau belum:

// SELECT
//     sj.sjh_id AS nomor_surat_tugas,
//     TO_CHAR(sj.sjh_tanggal, 'DD/MM/YYYY') AS tanggal_st,
//     sj.sjh_kendid AS no_kendaraan,
//     COALESCE(ass.ass_nama, 'Luar Kota') AS jenis_tugas,
//     COALESCE(s1.kry_nama, '-') AS sopir_1,
//     COALESCE(s2.kry_nama, '-') AS sopir_2,
//     COALESCE(th.trh_name, sj.sjh_trayekid) AS trayek_rute,
//     sj.sjh_keterangan,
//     -- Hitung berapa kali sudah dibuatkan kas keluar / uang muka
//     COUNT(sjc.sjc_cbid) AS jumlah_kas_keluar
// FROM public.opr_t_esuratjalan sj
// LEFT JOIN public.opr_m_assignment ass ON ass.ass_id = COALESCE(sj.sjh_assid, 'AS009')
// LEFT JOIN public.hrd_m_karyawan s1 ON s1.kry_nip = sj.sjh_sopir1_nip
// LEFT JOIN public.hrd_m_karyawan s2 ON s2.kry_nip = sj.sjh_sopir2_nip
// LEFT JOIN public.opr_m_trayekh th ON th.trh_id = sj.sjh_trayekid
// LEFT JOIN public.opr_t_esuratjalancashbank sjc ON sjc.sjc_sjhid = sj.sjh_id
// GROUP BY
//     sj.sjh_id, sj.sjh_tanggal, sj.sjh_kendid, ass.ass_nama,
//     s1.kry_nama, s2.kry_nama, th.trh_name, sj.sjh_trayekid, sj.sjh_keterangan
// ORDER BY sj.sjh_tanggal DESC, sj.sjh_id DESC
// LIMIT 20;

// end

// 2. Kueri Khusus: Surat Tugas yang Belum Pernah Dicairkan Uang Mukanya
// Untuk mencari nomor surat tugas yang belum memiliki pencatatan kas keluar sama sekali:
// SELECT
//     sj.sjh_id AS nomor_surat_tugas,
//     TO_CHAR(sj.sjh_tanggal, 'DD/MM/YYYY') AS tanggal_st,
//     sj.sjh_kendid AS no_kendaraan,
//     COALESCE(s1.kry_nama, '-') AS driver,
//     COALESCE(th.trh_name, sj.sjh_trayekid) AS rute
// FROM public.opr_t_esuratjalan sj
// LEFT JOIN public.hrd_m_karyawan s1 ON s1.kry_nip = sj.sjh_sopir1_nip
// LEFT JOIN public.opr_m_trayekh th ON th.trh_id = sj.sjh_trayekid
// WHERE NOT EXISTS (
//     SELECT 1
//     FROM public.opr_t_esuratjalancashbank sjc
//     WHERE sjc.sjc_sjhid = sj.sjh_id
// )
// ORDER BY sj.sjh_tanggal DESC
// LIMIT 10;

// 3. Kueri Pencarian Berdasarkan Nomor / Kendaraan Tertentu
// SELECT
//     sj.sjh_id AS nomor_surat_tugas,
//     sj.sjh_kendid AS no_kendaraan,
//     sj.sjh_tanggal,
//     s1.kry_nama AS driver,
//     sj.sjh_trayekid
// FROM public.opr_t_esuratjalan sj
// LEFT JOIN public.hrd_m_karyawan s1 ON s1.kry_nip = sj.sjh_sopir1_nip
// WHERE UPPER(sj.sjh_id) LIKE '%2026%'
//    OR UPPER(sj.sjh_kendid) LIKE '%B9340%'
// ORDER BY sj.sjh_id DESC
// LIMIT 5;
