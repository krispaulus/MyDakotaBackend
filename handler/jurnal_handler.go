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

// JurnalListModel Struct Tampilan Header Jurnal
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

// CreateJurnalReq DTO Input/Update Jurnal
type CreateJurnalReq struct {
	TJurHNo         string `json:"tjurh_no"`
	TJurHTanggal    string `json:"tjurh_tanggal" binding:"required"`
	TJurHType       string `json:"tjurh_type" binding:"required"`
	TJurHKeterangan string `json:"tjurh_keterangan"`
	TJurHCBID       string `json:"tjurh_cbid"` // ID Cabang/Agen
}

func getJurnalDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/jurnal (READ LIST JURNAL DENGAN FILTER & PAGINATION)
// =========================================================================
func GetJurnalListHandler(c *gin.Context) {
	database := getJurnalDB(c)

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	cabangNama := c.Query("cabang_nama")
	tipeJurnal := c.Query("tipe_jurnal")
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
		Joins("LEFT JOIN public.glb_m_agen a ON CAST(SUBSTRING(jh.tjurh_no FROM 5 FOR 3) AS INTEGER) = CAST(a.agen_id AS INTEGER)").
		Where("COALESCE(jh.tjurh_no, '') <> ''")

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
// 2. DELETE /api/gl/jurnal/:id (BATALKAN / VOID JURNAL)
// =========================================================================
func DeleteJurnalHandler(c *gin.Context) {
	noJurnal := c.Param("id")
	database := getJurnalDB(c)

	// Tandai status tjurh_deleteyn = 'Y' (Void/Batal)
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

// =========================================================================
// 3. POST /api/gl/jurnal/create (TAMBAH JURNAL BARU)
// =========================================================================
func CreateJurnalHandler(c *gin.Context) {
	database := getJurnalDB(c)
	userID, _ := c.Get("username")

	var req CreateJurnalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// 🌟 Auto-generate Nomor Jurnal jika kosong
	if strings.TrimSpace(req.TJurHNo) == "" {
		cbID := strings.TrimSpace(req.TJurHCBID)
		if cbID == "" {
			cbID = "1"
		}
		cbInt, _ := strconv.Atoi(cbID)

		tglParsed, err := time.Parse("2006-01-02", req.TJurHTanggal)
		if err != nil {
			tglParsed = time.Now()
		}

		// Format penomoran Dakota: YYMM + 3 Digit AgenID + Type + 5 Digit Counter
		prefix := fmt.Sprintf("%s%03d%s", tglParsed.Format("0601"), cbInt, req.TJurHType)

		var lastNo string
		database.Table("public.gl_t_jurnalh").
			Select("tjurh_no").
			Where("tjurh_no LIKE ?", prefix+"%").
			Order("tjurh_no DESC").
			Limit(1).
			Scan(&lastNo)

		counter := 1
		if lastNo != "" && len(lastNo) >= 13 {
			lastCounter, _ := strconv.Atoi(lastNo[len(lastNo)-5:])
			counter = lastCounter + 1
		}

		req.TJurHNo = fmt.Sprintf("%s%05d", prefix, counter)
	}

	insertHeader := map[string]interface{}{
		"tjurh_no":         strings.TrimSpace(req.TJurHNo),
		"tjurh_tanggal":    req.TJurHTanggal + " " + time.Now().Format("15:04:05"),
		"tjurh_type":       req.TJurHType,
		"tjurh_keterangan": req.TJurHKeterangan,
		"tjurh_deleteyn":   "N",
		"tjurh_postyn":     "N",
		"tjurh_updateid":   fmt.Sprintf("%v", userID),
		"tjurh_updatetime": time.Now(),
	}

	if err := database.Table("public.gl_t_jurnalh").Create(insertHeader).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan Header Jurnal: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   fmt.Sprintf("Jurnal Keuangan No %s Berhasil Disimpan!", req.TJurHNo),
		"no_jurnal": req.TJurHNo,
	})
}

// =========================================================================
// 4. POST /api/gl/jurnal/update (UPDATE JURNAL)
// =========================================================================
func UpdateJurnalHandler(c *gin.Context) {
	database := getJurnalDB(c)
	userID, _ := c.Get("username")

	var req CreateJurnalReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if strings.TrimSpace(req.TJurHNo) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Jurnal tidak boleh kosong saat update!"})
		return
	}

	updateHeader := map[string]interface{}{
		"tjurh_tanggal":    req.TJurHTanggal + " " + time.Now().Format("15:04:05"),
		"tjurh_type":       req.TJurHType,
		"tjurh_keterangan": req.TJurHKeterangan,
		"tjurh_updateid":   fmt.Sprintf("%v", userID),
		"tjurh_updatetime": time.Now(),
	}

	err := database.Table("public.gl_t_jurnalh").
		Where("tjurh_no = ?", strings.TrimSpace(req.TJurHNo)).
		Updates(updateHeader).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate Jurnal: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data Jurnal Berhasil Diperbarui!",
	})
}
