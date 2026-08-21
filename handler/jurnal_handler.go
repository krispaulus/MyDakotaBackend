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

type JurnalListModel struct {
	TJurHNo         string  `json:"tjurh_no" gorm:"column:tjurh_no"`
	TJurHTanggal    string  `json:"tjurh_tanggal" gorm:"column:tjurh_tanggal"`
	TJurHType       string  `json:"tjurh_type" gorm:"column:tjurh_type"`
	TJurHKeterangan string  `json:"tjurh_keterangan" gorm:"column:tjurh_keterangan"`
	TJurHDeleteYN   string  `json:"tjurh_deleteyn" gorm:"column:tjurh_deleteyn"`
	TJurHPostYN     string  `json:"tjurh_postyn" gorm:"column:tjurh_postyn"`
	AgenNama        string  `json:"agen_nama" gorm:"column:agen_nama"`
	TotalAmount     float64 `json:"total_amount" gorm:"column:jml"`
}

type JurnalDetailItem struct {
	AccCode    string  `json:"tjurd_acccode" gorm:"column:tjurd_acccode"`
	AccName    string  `json:"ca_name" gorm:"column:ca_name"`
	AgenID     string  `json:"tjurd_agenid" gorm:"column:tjurd_agenid"`
	AgenNama   string  `json:"agen_nama" gorm:"column:agen_nama"`
	Keterangan string  `json:"tjurd_keterangan" gorm:"column:tjurd_keterangan"`
	Debet      float64 `json:"tjurd_debet" gorm:"column:tjurd_debet"`
	Kredit     float64 `json:"tjurd_kredit" gorm:"column:tjurd_kredit"`
}

type SaveJurnalFullReq struct {
	TJurHNo         string             `json:"tjurh_no"`
	TJurHTanggal    string             `json:"tjurh_tanggal" binding:"required"`
	TJurHType       string             `json:"tjurh_type" binding:"required"`
	TJurHKeterangan string             `json:"tjurh_keterangan"`
	TJurHCBID       string             `json:"tjurh_cbid"`
	Details         []JurnalDetailItem `json:"details"`
}

func getJurnalDB(c *gin.Context) *gorm.DB {
	ptID := strings.TrimSpace(c.Query("pt_id"))
	if ptID == "" {
		if val, exists := c.Get("pt_id"); exists && val != nil {
			ptID = strings.TrimSpace(fmt.Sprintf("%v", val))
		}
	}

	if ptID != "" {
		if database, ok := db.ResolveDB(ptID); ok && database != nil {
			return database
		}
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

	if dliDB, ok := db.ResolveDB("dli"); ok && dliDB != nil {
		return dliDB
	}

	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/jurnal (READ LIST JURNAL)
// =========================================================================
// GET /api/gl/jurnal (READ LIST JURNAL DENGAN QUERY TEROPTIMASI)
func GetJurnalListHandler(c *gin.Context) {
	database := getJurnalDB(c)

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	cabangNama := c.Query("cabang_nama")
	tipeJurnal := c.Query("tipe_jurnal")
	noJurnal := c.Query("no_jurnal")
	showDeleted := c.Query("show_deleted")
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

	// Subquery agregat debet untuk menghindari JOIN perkalian baris
	query := database.Table("public.gl_t_jurnalh jh").
		Select(`
			jh.tjurh_no, 
			TO_CHAR(jh.tjurh_tanggal, 'YYYY-MM-DD') AS tjurh_tanggal, 
			jh.tjurh_type, 
			COALESCE(jh.tjurh_keterangan, '-') AS tjurh_keterangan, 
			COALESCE(jh.tjurh_deleteyn, 'N') AS tjurh_deleteyn, 
			COALESCE(jh.tjurh_postyn, 'N') AS tjurh_postyn, 
			COALESCE(a.agen_nama, 'DLI PUSAT') AS agen_nama, 
			COALESCE((
				SELECT SUM(jd.tjurd_debet) 
				FROM public.gl_t_jurnald jd 
				WHERE TRIM(jd.tjurd_tjurhno) = TRIM(jh.tjurh_no)
			), 0) AS jml
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON a.agen_id::varchar = SUBSTRING(jh.tjurh_no FROM 5 FOR 3)").
		Where("COALESCE(jh.tjurh_no, '') <> ''")

	if showDeleted != "Y" {
		query = query.Where("COALESCE(jh.tjurh_deleteyn, 'N') = 'N'")
	}
	if startDate != "" && endDate != "" {
		query = query.Where("jh.tjurh_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if cabangNama != "" {
		query = query.Where("a.agen_nama = ?", cabangNama)
	}
	if tipeJurnal != "" {
		query = query.Where("jh.tjurh_type = ?", tipeJurnal)
	}
	if noJurnal != "" {
		query = query.Where("jh.tjurh_no ILIKE ?", "%"+noJurnal+"%")
	}

	var totalRecords int64
	database.Table("public.gl_t_jurnalh jh").
		Where("COALESCE(jh.tjurh_no, '') <> '' AND COALESCE(jh.tjurh_deleteyn, 'N') = 'N'").
		Count(&totalRecords)

	var list []JurnalListModel
	err := query.Order("jh.tjurh_tanggal DESC, jh.tjurh_no DESC").Limit(limit).Offset(offset).Scan(&list).Error
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

// GET /api/gl/jurnal/detail/:id (AMBIL HEADER & RINCIAN JURNAL LENGKAP)
func GetJurnalDetailHandler(c *gin.Context) {
	noJurnal := strings.TrimSpace(strings.TrimPrefix(c.Param("id"), "/"))
	database := getJurnalDB(c)

	var currentDB string
	database.Raw("SELECT current_database()").Scan(&currentDB)

	var header JurnalListModel
	errHeader := database.Table("public.gl_t_jurnalh jh").
		Select(`
			jh.tjurh_no, 
			TO_CHAR(jh.tjurh_tanggal, 'YYYY-MM-DD') AS tjurh_tanggal, 
			jh.tjurh_type, 
			COALESCE(jh.tjurh_keterangan, '-') AS tjurh_keterangan, 
			COALESCE(jh.tjurh_deleteyn, 'N') AS tjurh_deleteyn, 
			COALESCE(jh.tjurh_postyn, 'N') AS tjurh_postyn, 
			COALESCE(a.agen_nama, '-') AS agen_nama
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(BOTH FROM a.agen_id::varchar) = TRIM(BOTH FROM SUBSTRING(jh.tjurh_no FROM 5 FOR 3))").
		Where("TRIM(UPPER(jh.tjurh_no)) = TRIM(UPPER(?))", noJurnal).
		Scan(&header).Error

	if errHeader != nil || header.TJurHNo == "" {
		fmt.Printf("❌ [JURNAL DETAIL] Header Jurnal %s TIDAK DITEMUKAN di database: %s\n", noJurnal, currentDB)
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Jurnal tidak ditemukan"})
		return
	}

	details := make([]JurnalDetailItem, 0)
	rawQuery := `
		SELECT 
			jd.tjurd_acccode,
			COALESCE(ca.ca_name, b.bank_name, it.item_name, 'Akun Perkiraan') AS ca_name,
			COALESCE(jd.tjurd_agenid, '1') AS tjurd_agenid,
			COALESCE(a.agen_nama, 'DLI PUSAT') AS agen_nama,
			COALESCE(jd.tjurd_keterangan, '-') AS tjurd_keterangan,
			COALESCE(jd.tjurd_debet, 0) AS tjurd_debet,
			COALESCE(jd.tjurd_kredit, 0) AS tjurd_kredit
		FROM public.gl_t_jurnald jd
		LEFT JOIN public.gl_m_chartaccount ca ON TRIM(BOTH FROM jd.tjurd_acccode::varchar) = TRIM(BOTH FROM ca.ca_id::varchar)
		LEFT JOIN public.gl_m_bank b ON TRIM(BOTH FROM jd.tjurd_acccode::varchar) = TRIM(BOTH FROM b.bank_acccode::varchar)
		LEFT JOIN public.gl_m_item it ON TRIM(BOTH FROM jd.tjurd_acccode::varchar) = TRIM(BOTH FROM it.item_id::varchar)
		LEFT JOIN public.glb_m_agen a ON TRIM(BOTH FROM jd.tjurd_agenid::varchar) = TRIM(BOTH FROM a.agen_id::varchar)
		WHERE TRIM(UPPER(jd.tjurd_tjurhno::varchar)) = TRIM(UPPER(?::varchar))
		ORDER BY jd.tjurd_acccode ASC
	`
	database.Raw(rawQuery, noJurnal).Scan(&details)

	fmt.Printf("🔍 [DEBUG JURNAL DETAIL] DB: [%s] | No Jurnal: %s | Ditemukan: %d baris detail\n", currentDB, noJurnal, len(details))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"header":  header,
		"details": details,
	})
}

// =========================================================================
// 3. GET /api/gl/chart-accounts (AUTOCOMPLETE MASTER KODE PERKIRAAN COA)
// =========================================================================
func GetChartAccountsHandler(c *gin.Context) {
	database := getJurnalDB(c)
	keyword := strings.TrimSpace(c.Query("q"))

	type COAResult struct {
		CAID   string `json:"ca_id" gorm:"column:ca_id"`
		CAName string `json:"ca_name" gorm:"column:ca_name"`
		CAType string `json:"ca_type" gorm:"column:ca_type"`
	}

	var list []COAResult
	query := database.Table("public.gl_m_chartaccount").
		Select("ca_id, ca_name, COALESCE(ca_type, 'D') AS ca_type").
		Where("COALESCE(ca_aktifyn, 'Y') = 'Y'")

	if keyword != "" {
		query = query.Where("ca_id ILIKE ? OR ca_name ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	query.Order("ca_id ASC").Limit(35).Scan(&list)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// =========================================================================
// 4. POST /api/gl/jurnal/create (SIMPAN JURNAL HEADER & DETAIL LENGKAP)
// =========================================================================
func CreateJurnalHandler(c *gin.Context) {
	database := getJurnalDB(c)
	userID, _ := c.Get("username")
	userAgenID, _ := c.Get("agen_id")

	var req SaveJurnalFullReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	cbID := strings.TrimSpace(req.TJurHCBID)
	if cbID == "" && userAgenID != nil {
		cbID = strings.TrimSpace(fmt.Sprintf("%v", userAgenID))
	}
	if cbID == "" {
		cbID = "1"
	}

	// Generate No Jurnal jika kosong: [YYMM][AGEN_3DIGIT][TIPE][URUTAN_5DIGIT]
	if strings.TrimSpace(req.TJurHNo) == "" {
		tglTrans, errDate := time.Parse("2006-01-02", req.TJurHTanggal)
		if errDate != nil {
			tglTrans = time.Now()
		}
		agen3Digit := fmt.Sprintf("%03s", cbID)
		if len(agen3Digit) > 3 {
			agen3Digit = agen3Digit[len(agen3Digit)-3:]
		}
		prefixJurnal := fmt.Sprintf("%s%s%s", tglTrans.Format("0601"), agen3Digit, req.TJurHType)

		var lastJurnal string
		database.Table("public.gl_t_jurnalh").
			Select("tjurh_no").
			Where("tjurh_no LIKE ?", prefixJurnal+"%").
			Order("tjurh_no DESC").
			Limit(1).
			Scan(&lastJurnal)

		nextUrut := 1
		if lastJurnal != "" && len(lastJurnal) >= len(prefixJurnal)+5 {
			if num, errNum := strconv.Atoi(lastJurnal[len(prefixJurnal):]); errNum == nil {
				nextUrut = num + 1
			}
		}
		req.TJurHNo = fmt.Sprintf("%s%05d", prefixJurnal, nextUrut)
	}

	// 1. Simpan Header Jurnal
	insertHeader := map[string]interface{}{
		"tjurh_no":         strings.TrimSpace(req.TJurHNo),
		"tjurh_tanggal":    req.TJurHTanggal + " " + time.Now().Format("15:04:05"),
		"tjurh_type":       req.TJurHType,
		"tjurh_keterangan": req.TJurHKeterangan,
		"tjurh_deleteyn":   "N",
		"tjurh_postyn":     "Y",
		"tjurh_updateid":   fmt.Sprintf("%v", userID),
		"tjurh_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_t_jurnalh").Create(insertHeader).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan Header Jurnal: " + err.Error()})
		return
	}

	// 2. Simpan Baris Rincian Detail
	for _, d := range req.Details {
		agenDetail := strings.TrimSpace(d.AgenID)
		if agenDetail == "" {
			agenDetail = cbID
		}
		ketDetail := strings.TrimSpace(d.Keterangan)
		if ketDetail == "" {
			ketDetail = req.TJurHKeterangan
		}

		insertD := map[string]interface{}{
			"tjurd_tjurhno":    strings.TrimSpace(req.TJurHNo),
			"tjurd_acccode":    strings.TrimSpace(d.AccCode),
			"tjurd_agenid":     agenDetail,
			"tjurd_keterangan": ketDetail,
			"tjurd_debet":      d.Debet,
			"tjurd_kredit":     d.Kredit,
		}
		if err := database.Table("public.gl_t_jurnald").Create(insertD).Error; err != nil {
			fmt.Printf("❌ [ERROR INSERT JURNALD] %v\n", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   fmt.Sprintf("Jurnal Keuangan No %s Berhasil Disimpan!", req.TJurHNo),
		"no_jurnal": req.TJurHNo,
	})
}

// =========================================================================
// 5. POST /api/gl/jurnal/update (UPDATE HEADER & RINCIAN JURNAL)
// =========================================================================
func UpdateJurnalHandler(c *gin.Context) {
	database := getJurnalDB(c)
	userID, _ := c.Get("username")

	var req SaveJurnalFullReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	noJurnal := strings.TrimSpace(req.TJurHNo)
	if noJurnal == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Jurnal tidak boleh kosong!"})
		return
	}

	updateHeader := map[string]interface{}{
		"tjurh_tanggal":    req.TJurHTanggal + " " + time.Now().Format("15:04:05"),
		"tjurh_type":       req.TJurHType,
		"tjurh_keterangan": req.TJurHKeterangan,
		"tjurh_updateid":   fmt.Sprintf("%v", userID),
		"tjurh_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_t_jurnalh").Where("tjurh_no = ?", noJurnal).Updates(updateHeader).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update Header Jurnal: " + err.Error()})
		return
	}

	if len(req.Details) > 0 {
		database.Table("public.gl_t_jurnald").Where("TRIM(tjurd_tjurhno) = TRIM(?)", noJurnal).Delete(map[string]interface{}{})

		for _, d := range req.Details {
			agenDetail := strings.TrimSpace(d.AgenID)
			if agenDetail == "" {
				agenDetail = "1"
			}
			ketDetail := strings.TrimSpace(d.Keterangan)
			if ketDetail == "" {
				ketDetail = req.TJurHKeterangan
			}

			insertD := map[string]interface{}{
				"tjurd_tjurhno":    noJurnal,
				"tjurd_acccode":    strings.TrimSpace(d.AccCode),
				"tjurd_agenid":     agenDetail,
				"tjurd_keterangan": ketDetail,
				"tjurd_debet":      d.Debet,
				"tjurd_kredit":     d.Kredit,
			}
			database.Table("public.gl_t_jurnald").Create(insertD)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Jurnal %s beserta rinciannya berhasil diperbarui!", noJurnal),
	})
}

// =========================================================================
// 6. DELETE /api/gl/jurnal/:id (BATALKAN JURNAL)
// =========================================================================
func DeleteJurnalHandler(c *gin.Context) {
	noJurnal := strings.TrimSpace(strings.TrimPrefix(c.Param("id"), "/"))
	database := getJurnalDB(c)

	err := database.Table("public.gl_t_jurnalh").
		Where("tjurh_no = ?", noJurnal).
		Update("tjurh_deleteyn", "Y").Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membatalkan jurnal: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Jurnal Nomor %s berhasil dibatalkan!", noJurnal),
	})
}
