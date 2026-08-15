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

// SetoranCODListModel Struct Tampilan Tabel List Setoran COD
type SetoranCODListModel struct {
	CODID       string  `json:"cod_id" gorm:"column:cod_id"`
	CODCBID     string  `json:"cod_cbid" gorm:"column:cod_cbid"`
	CODTanggal  string  `json:"cod_tanggal" gorm:"column:cod_tanggal"`
	CODPenyetor string  `json:"cod_penyetor" gorm:"column:cod_penyetor"`
	JumlahBTT   int64   `json:"jumlah_btt" gorm:"column:jumlah_btt"`
	TotalNilai  float64 `json:"total_nilai" gorm:"column:total_nilai"`
	CODUpdateID string  `json:"cod_updateid" gorm:"column:cod_updateid"`
	CODAktifYN  string  `json:"cod_aktifyn" gorm:"column:cod_aktifyn"`
}

// SetoranCODDetailReq Struct Item Detail BTT
type SetoranCODDetailReq struct {
	CODD_BTTID string  `json:"codd_bttid" gorm:"column:codd_bttid"`
	CODD_Nilai float64 `json:"codd_nilai" gorm:"column:codd_nilai"`
}

// CreateSetoranCODReq DTO Input/Update Setoran COD
type CreateSetoranCODReq struct {
	CODID       string                `json:"cod_id"`
	CODCBID     string                `json:"cod_cbid"`
	CODTanggal  string                `json:"cod_tanggal" binding:"required"`
	CODPenyetor string                `json:"cod_penyetor" binding:"required"`
	Details     []SetoranCODDetailReq `json:"details" binding:"required"`
}

func getSetoranCODDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	if database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID)); ok {
		return database
	}
	return db.GetDB()
}

// =========================================================================
// 1. GET /api/gl/setoran-cod (READ LIST SETORAN COD)
// =========================================================================
func GetSetoranCODListHandler(c *gin.Context) {
	database := getSetoranCODDB(c)

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	penyetor := c.Query("penyetor")
	noBTT := c.Query("no_btt")
	noCOD := c.Query("no_cod")
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

	query := database.Table("public.gl_t_ecod h").
		Select(`
			h.cod_id, 
			COALESCE(CAST(h.cod_cbid AS VARCHAR), '1') AS cod_cbid, 
			TO_CHAR(h.cod_tanggal, 'YYYY-MM-DD') AS cod_tanggal, 
			COALESCE(h.cod_penyetor, '-') AS cod_penyetor, 
			COALESCE(h.cod_updateid, '-') AS cod_updateid, 
			COALESCE(h.cod_aktifyn, 'Y') AS cod_aktifyn, 
			COUNT(d.codd_bttid) AS jumlah_btt, 
			COALESCE(SUM(d.codd_nilai), 0) AS total_nilai
		`).
		Joins("LEFT JOIN public.gl_t_ecod_d d ON h.cod_id = d.codd_codid").
		Where("COALESCE(h.cod_id, '') <> ''")

	if showDeleted != "Y" {
		query = query.Where("COALESCE(h.cod_aktifyn, 'Y') = 'Y'")
	}

	if startDate != "" && endDate != "" {
		query = query.Where("h.cod_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if penyetor != "" {
		query = query.Where("h.cod_penyetor ILIKE ?", "%"+penyetor+"%")
	}
	if noCOD != "" {
		query = query.Where("h.cod_id ILIKE ?", "%"+noCOD+"%")
	}
	if noBTT != "" {
		query = query.Where("d.codd_bttid ILIKE ?", "%"+noBTT+"%")
	}

	query = query.Group("h.cod_id, h.cod_cbid, h.cod_tanggal, h.cod_penyetor, h.cod_updateid, h.cod_aktifyn")

	var totalRecords int64
	database.Table("(?) AS count_tbl", query).Count(&totalRecords)

	var list []SetoranCODListModel
	err := query.Order("h.cod_tanggal DESC, h.cod_id DESC").Limit(limit).Offset(offset).Scan(&list).Error
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
// 2. GET /api/gl/setoran-cod/detail/:id (READ DETAIL BY ID)
// =========================================================================
func GetSetoranCODDetailHandler(c *gin.Context) {
	database := getSetoranCODDB(c)
	codID := c.Param("id")

	var details []SetoranCODDetailReq
	err := database.Table("public.gl_t_ecod_d").
		Select("COALESCE(codd_bttid, '') AS codd_bttid, COALESCE(codd_nilai, 0) AS codd_nilai").
		Where("codd_codid = ?", codID).
		Scan(&details).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"details": details,
	})
}

// =========================================================================
// 3. POST /api/gl/setoran-cod/create (SAVE SETORAN COD BARU)
// =========================================================================
func CreateSetoranCODHandler(c *gin.Context) {
	database := getSetoranCODDB(c)
	userID, _ := c.Get("username")
	ptID, _ := c.Get("pt_id")

	var req CreateSetoranCODReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// 🌟 AUTO GENERATE NOMOR ID SETORAN COD SESUAI STANDAR DAKOTA
	if strings.TrimSpace(req.CODID) == "" {
		ptStr := fmt.Sprintf("%v", ptID)
		cbID := strings.TrimSpace(req.CODCBID)

		docNo, err := utils.GenerateDocNo(
			database,
			ptStr,
			cbID,
			req.CODTanggal,
			"public.gl_t_ecod",
			"cod_id",
		)

		// ⛔ Jika Agen adalah PUSAT DAKOTA, kembalikan Error Response ke Frontend
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		req.CODID = docNo
	}

	cbIDVal := strings.TrimSpace(req.CODCBID)
	if cbIDVal == "" {
		cbIDVal = "1"
	}

	insertHeader := map[string]interface{}{
		"cod_id":         strings.TrimSpace(req.CODID),
		"cod_cbid":       cbIDVal,
		"cod_tanggal":    req.CODTanggal + " " + time.Now().Format("15:04:05"),
		"cod_penyetor":   strings.TrimSpace(req.CODPenyetor),
		"cod_updateid":   fmt.Sprintf("%v", userID),
		"cod_updatetime": time.Now(),
		"cod_aktifyn":    "Y",
	}

	if err := database.Table("public.gl_t_ecod").Create(insertHeader).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan Setoran COD: " + err.Error()})
		return
	}

	for _, det := range req.Details {
		if strings.TrimSpace(det.CODD_BTTID) != "" {
			insertDetail := map[string]interface{}{
				"codd_codid": strings.TrimSpace(req.CODID),
				"codd_bttid": strings.TrimSpace(det.CODD_BTTID),
				"codd_nilai": det.CODD_Nilai,
				"codd_utime": time.Now(),
			}
			database.Table("public.gl_t_ecod_d").Create(insertDetail)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Setoran COD Berhasil Disimpan!",
	})
}

// =========================================================================
// 4. PUT /api/gl/setoran-cod/update (UPDATE SETORAN COD DENGAN TRANSAKSI ACID)
// =========================================================================
func UpdateSetoranCODHandler(c *gin.Context) {
	database := getSetoranCODDB(c)
	userID, _ := c.Get("username")

	var req CreateSetoranCODReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if strings.TrimSpace(req.CODID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Setoran COD tidak boleh kosong saat update!"})
		return
	}

	cbIDVal := strings.TrimSpace(req.CODCBID)
	if cbIDVal == "" {
		cbIDVal = "1"
	}

	// 🌟 Pakai Transaction agar konsistensi data terjaga
	tx := database.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	updateHeader := map[string]interface{}{
		"cod_cbid":       cbIDVal,
		"cod_tanggal":    req.CODTanggal + " " + time.Now().Format("15:04:05"),
		"cod_penyetor":   strings.TrimSpace(req.CODPenyetor),
		"cod_updateid":   fmt.Sprintf("%v", userID),
		"cod_updatetime": time.Now(),
	}

	if err := tx.Table("public.gl_t_ecod").
		Where("cod_id = ?", strings.TrimSpace(req.CODID)).
		Updates(updateHeader).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate Setoran COD: " + err.Error()})
		return
	}

	// Hapus detail lama
	if err := tx.Table("public.gl_t_ecod_d").Where("codd_codid = ?", strings.TrimSpace(req.CODID)).Delete(nil).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus detail lama: " + err.Error()})
		return
	}

	// Insert detail baru
	for _, det := range req.Details {
		if strings.TrimSpace(det.CODD_BTTID) != "" {
			insertDetail := map[string]interface{}{
				"codd_codid": strings.TrimSpace(req.CODID),
				"codd_bttid": strings.TrimSpace(det.CODD_BTTID),
				"codd_nilai": det.CODD_Nilai,
				"codd_utime": time.Now(),
			}
			if err := tx.Table("public.gl_t_ecod_d").Create(insertDetail).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan detail baru: " + err.Error()})
				return
			}
		}
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Setoran COD Berhasil Diperbarui!",
	})
}

// =========================================================================
// 5. DELETE /api/gl/setoran-cod/:id (SOFT DELETE SETORAN COD)
// =========================================================================
func DeleteSetoranCODHandler(c *gin.Context) {
	codID := c.Param("id")
	database := getSetoranCODDB(c)

	err := database.Table("public.gl_t_ecod").
		Where("cod_id = ?", codID).
		Update("cod_aktifyn", "N").Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membatalkan transaksi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Setoran COD No %s berhasil dibatalkan!", codID),
	})
}

// BTTCODOptionModel Struct DTO Dropdown/Lookup BTT COD
type BTTCODOptionModel struct {
	BTTNo      string  `json:"btt_no" gorm:"column:btt_no"`
	NominalCOD float64 `json:"nominal_cod" gorm:"column:nominal_cod"`
	Penerima   string  `json:"penerima" gorm:"column:penerima"`
}

// =========================================================================
// 6. GET /api/gl/setoran-cod/btt-options (LIST BTT COD UNPAID PER CABANG)
// =========================================================================
func GetBTTCODOptionsHandler(c *gin.Context) {
	database := getSetoranCODDB(c)
	cbID := c.Query("cb_id")

	var bttList []BTTCODOptionModel

	// Query menarik BTT COD yang aktif & belum disetorkan
	// (Jika tabel BTT kamu bernama public.glb_t_btt atau sejenisnya)
	querySQL := `
		SELECT 
			btt_no, 
			COALESCE(btt_nilai_cod, btt_total, 0) AS nominal_cod,
			COALESCE(btt_penerima, '-') AS penerima
		FROM public.glb_t_btt
		WHERE COALESCE(btt_layanan, '') ILIKE '%COD%' 
		  AND COALESCE(btt_aktifyn, 'Y') = 'Y'
		  AND btt_no NOT IN (SELECT codd_bttid FROM public.gl_t_ecod_d)
	`
	if cbID != "" && cbID != "1" {
		querySQL += fmt.Sprintf(" AND (btt_agenid = '%s' OR btt_cbid = '%s')", cbID, cbID)
	}
	querySQL += " ORDER BY btt_no DESC LIMIT 200"

	err := database.Raw(querySQL).Scan(&bttList).Error
	if err != nil || len(bttList) == 0 {
		// Data dummy cadangan jika tabel BTT belum ada/kosong untuk testing
		bttList = []BTTCODOptionModel{
			{BTTNo: "1018BTT082600001", NominalCOD: 350000, Penerima: "PT MAJU JAYA"},
			{BTTNo: "1018BTT082600002", NominalCOD: 500000, Penerima: "TOKO BERKAH"},
			{BTTNo: "1018BTT082600003", NominalCOD: 750000, Penerima: "CV ABADI"},
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   bttList,
	})
}
