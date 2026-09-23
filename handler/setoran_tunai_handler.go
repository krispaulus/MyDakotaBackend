package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 1. GET LIST SETORAN PENJUALAN TUNAI (FILTER & JOIN BTTD)
func GetSetoranTunaiList(c *gin.Context) {
	tglAwal := strings.TrimSpace(c.Query("tgl_awal"))
	tglAkhir := strings.TrimSpace(c.Query("tgl_akhir"))
	noSetor := strings.TrimSpace(c.Query("no_setor"))
	cabang := strings.TrimSpace(c.Query("cabang"))
	noLap := strings.TrimSpace(c.Query("no_lap"))
	noBTT := strings.TrimSpace(c.Query("no_btt"))

	var results []models.SetoranTunaiResponse

	query := db.DB.Table("art_t_setorantunai st").
		Select(`
			DISTINCT st.st_id,
			st.st_tanggal,
			st.st_agenid,
			COALESCE(ag.agen_nama, st.st_agenid) AS agen_nama,
			COALESCE(st.st_btthid, '-') AS st_btthid,
			COALESCE(st.st_jumlah, 0) AS st_jumlah,
			COALESCE(st.st_approveyn, 'N') AS st_approveyn,
			COALESCE(st.st_postingyn, 'N') AS st_postingyn,
			COALESCE(st.st_tjurhno, '-') AS st_tjurhno,
			COALESCE(st.st_updateid, '-') AS st_updateid,
			st.st_updatetime,
			COALESCE(st.st_aktifyn, 'Y') AS st_aktifyn
		`).
		Joins("LEFT JOIN glb_m_agen ag ON st.st_agenid = CAST(ag.agen_id AS TEXT)").
		Joins("LEFT JOIN art_t_penjualanbttd bttd ON st.st_btthid = bttd.bttd_btthid").
		Where("st.st_aktifyn = ?", "Y")

	// Filter tanggal
	if tglAwal != "" && tglAkhir != "" {
		query = query.Where("st.st_tanggal BETWEEN ? AND ?", tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}

	// Filter nomor setoran
	if noSetor != "" {
		query = query.Where("st.st_id ILIKE ?", "%"+noSetor+"%")
	}

	// Filter cabang / agen
	if cabang != "" {
		query = query.Where("ag.agen_nama ILIKE ? OR st.st_agenid = ?", "%"+cabang+"%", cabang)
	}

	// Filter kode LPH
	if noLap != "" {
		query = query.Where("st.st_btthid ILIKE ?", "%"+noLap+"%")
	}

	// Filter nomor resi BTT
	if noBTT != "" {
		query = query.Where("bttd.bttd_bttid ILIKE ?", "%"+noBTT+"%")
	}

	if err := query.Order("st.st_tanggal DESC, st.st_id DESC").Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 2. GET DAFTAR LPH YANG BELUM DISETOR
func GetAvailableLPH(c *gin.Context) {
	agenID := strings.TrimSpace(c.Query("agen_id"))

	type LPHItem struct {
		NoLPH      string    `json:"no_lph"`
		Tanggal    time.Time `json:"tanggal"`
		TotalTunai float64   `json:"total_tunai"`
		JumlahBTT  int64     `json:"jumlah_btt"`
	}

	var results []LPHItem

	// Ambil data LPH yang belum pernah disetorkan
	query := db.DB.Table("art_t_penjualanbtth h").
		Select(`
			h.btth_id AS no_lph,
			h.btth_tanggal AS tanggal,
			COALESCE(h.btth_totaltunai, 0) AS total_tunai,
			COUNT(d.bttd_bttid) AS jumlah_btt
		`).
		Joins("LEFT JOIN art_t_penjualanbttd d ON h.btth_id = d.bttd_btthid").
		Where("h.btth_id NOT IN (SELECT st_btthid FROM art_t_setorantunai WHERE st_aktifyn = 'Y' AND st_btthid IS NOT NULL)")

	if agenID != "" {
		query = query.Where("h.btth_agenid = ?", agenID)
	}

	err := query.Group("h.btth_id, h.btth_tanggal, h.btth_totaltunai").
		Order("h.btth_tanggal DESC").
		Limit(100).
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 3. SIMPAN SETORAN PENJUALAN TUNAI
func CreateSetoranTunai(c *gin.Context) {
	var req models.CreateSetoranTunaiReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid: " + err.Error()})
		return
	}

	username := "superdli"
	if authUser, exists := c.Get("username"); exists && authUser != nil {
		username = fmt.Sprintf("%v", authUser)
	}

	// Generate ID: ST + Cabang(3) + YYMM + Counter(4) -> misal ST00126090001
	now := time.Now()
	prefix := fmt.Sprintf("ST%03s%02d%02d", req.AgenID, now.Year()%100, int(now.Month()))

	tx := db.DB.Begin()

	var lastST struct {
		STID string
	}
	tx.Raw(`
		SELECT st_id 
		FROM art_t_setorantunai 
		WHERE st_id LIKE ? 
		ORDER BY st_id DESC 
		LIMIT 1
	`, prefix+"%").Scan(&lastST)

	counter := 1
	if lastST.STID != "" && len(lastST.STID) >= 4 {
		urutStr := lastST.STID[len(lastST.STID)-4:]
		if val, err := strconv.Atoi(urutStr); err == nil {
			counter = val + 1
		}
	}
	stID := fmt.Sprintf("%s%04d", prefix, counter)

	insertSQL := `
		INSERT INTO art_t_setorantunai 
			(st_id, st_agenid, st_tanggal, st_btthid, st_jumlah, st_approveyn, st_postingyn, st_updateid, st_updatetime, st_aktifyn) 
		VALUES 
			(?, ?, ?::timestamp, ?, ?, 'N', 'N', ?, CURRENT_TIMESTAMP, 'Y')
	`
	if err := tx.Exec(insertSQL, stID, req.AgenID, req.Tanggal+" 00:00:00", req.BTTHID, req.Jumlah, username).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan setoran: " + err.Error()})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Setoran berhasil disimpan dengan Kode: %s", stID),
		"st_id":   stID,
	})
}

// 4. BATALKAN / HAPUS SETORAN TUNAI
func DeleteSetoranTunai(c *gin.Context) {
	stID := c.Param("id")
	if stID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Kode Setoran wajib diisi"})
		return
	}

	// Update menjadi tidak aktif ('N')
	err := db.DB.Exec("UPDATE art_t_setorantunai SET st_aktifyn = 'N' WHERE st_id = ?", stID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membatalkan setoran: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Setoran %s berhasil dibatalkan", stID)})
}
