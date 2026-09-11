package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// DTO Asuransi Item
type AsuransiItem struct {
	NoAsuransi        string  `gorm:"column:noasuransi" json:"noasuransi"`
	TanggalAsuransi   string  `gorm:"column:tanggalasuransi" json:"tanggalasuransi"`
	NamaCust          string  `gorm:"column:namacust" json:"namacust"`
	NamaCustTerima    string  `gorm:"column:namacustterima" json:"namacustterima"`
	TotalBiaya        float64 `gorm:"column:totalbiaya" json:"totalbiaya"`
	JenisBarang       string  `gorm:"column:jenisbarang" json:"jenisbarang"`
	RutePengiriman    string  `gorm:"column:rutepengiriman" json:"rutepengiriman"`
	TanggalPengiriman string  `gorm:"column:tanggalpengiriman" json:"tanggalpengiriman"`
	BTTTID            string  `gorm:"column:bttt_id" json:"bttt_id"`
	AsuransiTjurHno   string  `gorm:"column:asuransi_tjurhno" json:"asuransi_tjurhno"`
	AsuransiBayarYN   string  `gorm:"column:asuransi_bayaryn" json:"asuransi_bayaryn"`
	AsuransiAktifYN   string  `gorm:"column:asuransiaktifyn" json:"asuransiaktifyn"`
}

// Request Create/Update Asuransi
type AsuransiRequest struct {
	NoAsuransi        string  `json:"noasuransi"`
	TanggalAsuransi   string  `json:"tanggalasuransi"`
	NamaCust          string  `json:"namacust" binding:"required"`
	NamaCustTerima    string  `json:"namacustterima" binding:"required"`
	TotalBiaya        float64 `json:"totalbiaya"`
	JenisBarang       string  `json:"jenisbarang" binding:"required"`
	RutePengiriman    string  `json:"rutepengiriman" binding:"required"`
	TanggalPengiriman string  `json:"tanggalpengiriman"`
	BTTTID            string  `json:"bttt_id" binding:"required"`
	AsuransiBayarYN   string  `json:"asuransi_bayaryn"`
}

// 1. GET /api/marketing/asuransi/data
func GetAsuransiList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database resolver gagal"})
		return
	}

	useTanggal := c.Query("use_tanggal") == "true"
	tglAwal := strings.TrimSpace(c.Query("start_date"))
	tglAkhir := strings.TrimSpace(c.Query("end_date"))
	customer := strings.TrimSpace(c.Query("customer"))
	noBtt := strings.TrimSpace(c.Query("nobtt"))
	bayar := strings.TrimSpace(c.Query("bayar"))

	query := `
		SELECT 
			a.noasuransi,
			COALESCE(TO_CHAR(a.tanggalasuransi, 'YYYY-MM-DD'), '') AS tanggalasuransi,
			COALESCE(NULLIF(TRIM(a.namacust), ''), '-') AS namacust,
			COALESCE(NULLIF(TRIM(a.namacustterima), ''), '-') AS namacustterima,
			COALESCE(a.totalbiaya, 0) AS totalbiaya,
			COALESCE(NULLIF(TRIM(a.jenisbarang), ''), '-') AS jenisbarang,
			COALESCE(NULLIF(TRIM(a.rutepengiriman), ''), '-') AS rutepengiriman,
			COALESCE(TO_CHAR(a.tanggalpengiriman, 'YYYY-MM-DD'), '') AS tanggalpengiriman,
			COALESCE(NULLIF(TRIM(a.bttt_id), ''), '-') AS bttt_id,
			COALESCE(NULLIF(TRIM(a.asuransi_tjurhno), ''), '-') AS asuransi_tjurhno,
			COALESCE(NULLIF(TRIM(a.asuransi_bayaryn), ''), 'N') AS asuransi_bayaryn,
			COALESCE(NULLIF(TRIM(a.asuransiaktifyn), ''), 'Y') AS asuransiaktifyn
		FROM public.mkt_t_asuransi a
		WHERE COALESCE(NULLIF(TRIM(a.asuransiaktifyn), ''), 'Y') = 'Y'
	`
	var args []interface{}

	if useTanggal && tglAwal != "" && tglAkhir != "" {
		query += " AND a.tanggalasuransi BETWEEN ? AND ?"
		args = append(args, tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}
	if customer != "" && !strings.Contains(strings.ToUpper(customer), "SEMUA") {
		query += " AND a.namacust ILIKE ?"
		args = append(args, "%"+customer+"%")
	}
	if noBtt != "" {
		query += " AND (a.bttt_id ILIKE ? OR a.noasuransi ILIKE ?)"
		args = append(args, "%"+noBtt+"%", "%"+noBtt+"%")
	}
	if bayar != "" && bayar != "SEMUA" {
		query += " AND a.asuransi_bayaryn = ?"
		args = append(args, bayar)
	}

	query += " ORDER BY a.tanggalasuransi DESC, a.noasuransi DESC LIMIT 500"

	var results []AsuransiItem
	if err := database.Raw(query, args...).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []AsuransiItem{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 2. POST /api/marketing/asuransi/save
func SaveAsuransi(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database resolver gagal"})
		return
	}

	username, _ := c.Get("username")
	var req AsuransiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.TanggalAsuransi == "" {
		req.TanggalAsuransi = time.Now().Format("2006-01-02")
	}
	if req.TanggalPengiriman == "" {
		req.TanggalPengiriman = req.TanggalAsuransi
	}
	if req.AsuransiBayarYN == "" {
		req.AsuransiBayarYN = "N"
	}

	// Generate Nomor Asuransi jika data baru
	if strings.TrimSpace(req.NoAsuransi) == "" {
		req.NoAsuransi = fmt.Sprintf("ASU%s%d", fmt.Sprintf("%v", ptID), time.Now().Unix()%1000000)
		insertSQL := `
			INSERT INTO public.mkt_t_asuransi (
				noasuransi, tanggalasuransi, namacust, namacustterima, totalbiaya,
				jenisbarang, rutepengiriman, tanggalpengiriman, bttt_id,
				asuransi_bayaryn, asuransiaktifyn, updateid, updatetime
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'Y', ?, NOW())
		`
		if err := database.Exec(insertSQL,
			req.NoAsuransi, req.TanggalAsuransi, req.NamaCust, req.NamaCustTerima, req.TotalBiaya,
			req.JenisBarang, req.RutePengiriman, req.TanggalPengiriman, req.BTTTID,
			req.AsuransiBayarYN, fmt.Sprintf("%v", username),
		).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan asuransi: " + err.Error()})
			return
		}
	} else {
		// Update
		updateSQL := `
			UPDATE public.mkt_t_asuransi SET
				tanggalasuransi = ?, namacust = ?, namacustterima = ?, totalbiaya = ?,
				jenisbarang = ?, rutepengiriman = ?, tanggalpengiriman = ?, bttt_id = ?,
				asuransi_bayaryn = ?, updateid = ?, updatetime = NOW()
			WHERE noasuransi = ?
		`
		if err := database.Exec(updateSQL,
			req.TanggalAsuransi, req.NamaCust, req.NamaCustTerima, req.TotalBiaya,
			req.JenisBarang, req.RutePengiriman, req.TanggalPengiriman, req.BTTTID,
			req.AsuransiBayarYN, fmt.Sprintf("%v", username), req.NoAsuransi,
		).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update asuransi: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data asuransi berhasil disimpan", "noasuransi": req.NoAsuransi})
}

// 3. DELETE /api/marketing/asuransi/:id
func DeleteAsuransi(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database resolver gagal"})
		return
	}

	id := c.Param("id")
	username, _ := c.Get("username")

	updateSQL := `UPDATE public.mkt_t_asuransi SET asuransiaktifyn = 'N', updateid = ?, updatetime = NOW() WHERE noasuransi = ?`
	if err := database.Exec(updateSQL, fmt.Sprintf("%v", username), id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus asuransi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data asuransi berhasil dinonaktifkan"})
}
