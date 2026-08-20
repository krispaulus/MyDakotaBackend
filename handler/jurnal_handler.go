package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/utils"

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
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/jurnal (READ LIST JURNAL)
// =========================================================================
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

	query := database.Table("public.gl_t_jurnalh jh").
		Select(`
			jh.tjurh_no, 
			TO_CHAR(jh.tjurh_tanggal, 'YYYY-MM-DD') AS tjurh_tanggal, 
			jh.tjurh_type, 
			COALESCE(jh.tjurh_keterangan, '-') AS tjurh_keterangan, 
			COALESCE(jh.tjurh_deleteyn, 'N') AS tjurh_deleteyn, 
			COALESCE(jh.tjurh_postyn, 'N') AS tjurh_postyn, 
			COALESCE(a.agen_nama, '-') AS agen_nama, 
			COALESCE(SUM(jd.tjurd_debet), 0) AS jml
		`).
		Joins("LEFT JOIN public.gl_t_jurnald jd ON TRIM(BOTH FROM CAST(jh.tjurh_no AS VARCHAR)) = TRIM(BOTH FROM CAST(jd.tjurd_tjurhno AS VARCHAR))").
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(BOTH FROM a.agen_id::varchar) = TRIM(BOTH FROM SUBSTRING(jh.tjurh_no FROM 5 FOR 3)) OR TRIM(BOTH FROM a.agen_kode::varchar) = TRIM(BOTH FROM SUBSTRING(jh.tjurh_no FROM 5 FOR 6))").
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

	query = query.Group("jh.tjurh_no, jh.tjurh_tanggal, jh.tjurh_type, jh.tjurh_keterangan, jh.tjurh_deleteyn, jh.tjurh_postyn, a.agen_nama")

	var totalRecords int64
	database.Table("(?) AS count_tbl", query).Count(&totalRecords)

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

// =========================================================================
// 2. GET /api/gl/jurnal/detail/:id (AMBIL HEADER & RINCIAN JURNAL LENGKAP)
// =========================================================================
func GetJurnalDetailHandler(c *gin.Context) {
	noJurnal := c.Param("id")
	database := getJurnalDB(c)

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
		Where("jh.tjurh_no = ?", noJurnal).
		Scan(&header).Error

	if errHeader != nil || header.TJurHNo == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Jurnal tidak ditemukan"})
		return
	}

	var details []JurnalDetailItem
	database.Table("public.gl_t_jurnald jd").
		Select(`
			jd.tjurd_acccode,
			COALESCE(ca.ca_name, '-') AS ca_name,
			COALESCE(jd.tjurd_agenid, '1') AS tjurd_agenid,
			COALESCE(a.agen_nama, '-') AS agen_nama,
			COALESCE(jd.tjurd_keterangan, '-') AS tjurd_keterangan,
			COALESCE(jd.tjurd_debet, 0) AS tjurd_debet,
			COALESCE(jd.tjurd_kredit, 0) AS tjurd_kredit
		`).
		Joins("LEFT JOIN public.gl_m_chartaccount ca ON TRIM(BOTH FROM jd.tjurd_acccode::varchar) = TRIM(BOTH FROM ca.ca_id::varchar)").
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(BOTH FROM jd.tjurd_agenid::varchar) = TRIM(BOTH FROM a.agen_id::varchar)").
		Where("jd.tjurd_tjurhno = ?", noJurnal).
		Order("jd.tjurd_acccode ASC"). // 👈 Diurutkan berdasarkan kode akun (tanpa tjurd_nourut)
		Scan(&details)

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
	ptID, _ := c.Get("pt_id")
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

	// Auto generate no jurnal jika kosong: [YY][MM][AGEN_3DIGIT][TIPE][URUTAN_5DIGIT]
	if strings.TrimSpace(req.TJurHNo) == "" {
		ptStr := fmt.Sprintf("%v", ptID)
		docNo, err := utils.GenerateDocNo(database, ptStr, cbID, req.TJurHTanggal, "public.gl_t_jurnalh", "tjurh_no")
		if err != nil {
			tglTrans, _ := time.Parse("2006-01-02", req.TJurHTanggal)
			agen3Digit := fmt.Sprintf("%03s", cbID)
			if len(agen3Digit) > 3 {
				agen3Digit = agen3Digit[len(agen3Digit)-3:]
			}
			docNo = fmt.Sprintf("%s%s%s%05d", tglTrans.Format("0601"), agen3Digit, req.TJurHType, time.Now().Unix()%10000)
		}
		req.TJurHNo = docNo
	}

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

	// Simpan detail rincian jurnal
	for i, d := range req.Details {
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
			"tjurd_nourut":     i + 1,
			"tjurd_acccode":    strings.TrimSpace(d.AccCode),
			"tjurd_agenid":     agenDetail,
			"tjurd_keterangan": ketDetail,
			"tjurd_debet":      d.Debet,
			"tjurd_kredit":     d.Kredit,
			"tjurd_updateid":   fmt.Sprintf("%v", userID),
			"tjurd_updatetime": time.Now(),
		}
		database.Table("public.gl_t_jurnald").Create(insertD)
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

	// Update detail jika array rincian dikirimkan
	if len(req.Details) > 0 {
		database.Table("public.gl_t_jurnald").Where("tjurd_tjurhno = ?", noJurnal).Delete(map[string]interface{}{})

		for i, d := range req.Details {
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
				"tjurd_nourut":     i + 1,
				"tjurd_acccode":    strings.TrimSpace(d.AccCode),
				"tjurd_agenid":     agenDetail,
				"tjurd_keterangan": ketDetail,
				"tjurd_debet":      d.Debet,
				"tjurd_kredit":     d.Kredit,
				"tjurd_updateid":   fmt.Sprintf("%v", userID),
				"tjurd_updatetime": time.Now(),
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
	noJurnal := c.Param("id")
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
