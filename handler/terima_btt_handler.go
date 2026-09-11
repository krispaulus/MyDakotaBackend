package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

type TerimaBTTItem struct {
	TglKembali   string  `json:"tb_tglkembali" gorm:"column:tb_tglkembali"`
	TglBTT       string  `json:"bttt_tanggal" gorm:"column:bttt_tanggal"`
	BTTID        string  `json:"tb_bttid" gorm:"column:tb_bttid"`
	CustName     string  `json:"cust_name" gorm:"column:cust_name"`
	TujuanKota   string  `json:"bttt_tujuankota" gorm:"column:bttt_tujuankota"`
	Pembayaran   string  `json:"bttt_pembayaran" gorm:"column:bttt_pembayaran"`
	TotalHarga   float64 `json:"total_harga" gorm:"column:total_harga"`
	NoSuratJalan string  `json:"bttt_nosuratjalan" gorm:"column:bttt_nosuratjalan"`
	AktifYN      string  `json:"tb_aktifyn" gorm:"column:tb_aktifyn"`
}

type TerimaBTTRequest struct {
	BTTID      string `json:"btt_id" binding:"required"`
	TglKembali string `json:"tgl_kembali"`
	AgenID     string `json:"agen_id"`
}

// 1. GET /api/terima-btt/data
func GetTerimaBTTList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database corporate gagal dihubungkan"})
		return
	}

	useTanggal := c.Query("use_tanggal") == "true"
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	pembayaran := strings.TrimSpace(c.Query("pembayaran"))
	noBTT := strings.TrimSpace(c.Query("no_btt"))
	kota := strings.TrimSpace(c.Query("kota"))
	customer := strings.TrimSpace(c.Query("customer"))
	agenID := strings.TrimSpace(c.Query("agen_id"))

	query := `
		SELECT 
			COALESCE(TO_CHAR(tb.tb_tglkembali, 'YYYY-MM-DD'), '') AS tb_tglkembali,
			COALESCE(TO_CHAR(e.bttt_tanggal, 'YYYY-MM-DD'), '') AS bttt_tanggal,
			TRIM(tb.tb_bttid) AS tb_bttid,
			COALESCE(c.cust_name, e.bttt_asalname, '-') AS cust_name,
			COALESCE(e.bttt_tujuankota, '-') AS bttt_tujuankota,
			COALESCE(e.bttt_pembayaran::text, '1') AS bttt_pembayaran,
			(COALESCE(e.bttt_harga, 0) + COALESCE(e.bttt_biayapenerus, 0)) AS total_harga,
			COALESCE(e.bttt_nosuratjalan, '-') AS bttt_nosuratjalan,
			COALESCE(tb.tb_aktifyn, 'Y') AS tb_aktifyn
		FROM public.mkt_t_eterimabtt tb
		LEFT JOIN public.mkt_t_econote e ON TRIM(tb.tb_bttid) = TRIM(e.bttt_id)
		LEFT JOIN public.mkt_m_customer c ON TRIM(e.bttt_asalcustid) = TRIM(c.cust_id)
		WHERE 1=1
	`
	var args []interface{}

	if agenID != "" && !strings.Contains(strings.ToUpper(agenID), "PUSAT") && !strings.Contains(strings.ToUpper(agenID), "HOLDING") && agenID != "001" {
		query += " AND (tb.tb_agenid::text = ? OR tb.tb_agenid::text = TRIM(LEADING '0' FROM ?))"
		args = append(args, agenID, agenID)
	}

	if useTanggal && startDate != "" && endDate != "" {
		query += " AND tb.tb_tglkembali BETWEEN ? AND ?"
		args = append(args, startDate+" 00:00:00", endDate+" 23:59:59")
	}

	if pembayaran != "" && pembayaran != "0" {
		query += " AND e.bttt_pembayaran::text = ?"
		args = append(args, pembayaran)
	}

	if noBTT != "" {
		query += " AND tb.tb_bttid ILIKE ?"
		args = append(args, "%"+noBTT+"%")
	}

	if kota != "" && !strings.Contains(strings.ToUpper(kota), "SEMUA") {
		query += " AND e.bttt_tujuankota ILIKE ?"
		args = append(args, "%"+kota+"%")
	}

	if customer != "" && !strings.Contains(strings.ToUpper(customer), "SEMUA") {
		query += " AND (c.cust_name ILIKE ? OR e.bttt_asalname ILIKE ?)"
		args = append(args, "%"+customer+"%", "%"+customer+"%")
	}

	query += " ORDER BY tb.tb_tglkembali DESC, tb.tb_bttid DESC LIMIT 500"

	var results []TerimaBTTItem
	if err := database.Raw(query, args...).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []TerimaBTTItem{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 2. POST /api/terima-btt/save
func SaveTerimaBTT(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database corporate gagal dihubungkan"})
		return
	}

	var req TerimaBTTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.TglKembali == "" {
		req.TglKembali = time.Now().Format("2006-01-02")
	}

	// Normalisasi agenID jika bernilai teks pusat
	if strings.Contains(strings.ToUpper(req.AgenID), "PUSAT") || req.AgenID == "" {
		req.AgenID = "001"
	}

	var countBTT int64
	database.Table("public.mkt_t_econote").Where("TRIM(bttt_id) = TRIM(?)", req.BTTID).Count(&countBTT)
	if countBTT == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Nomor resi BTT '%s' tidak terdaftar di sistem", req.BTTID)})
		return
	}

	var existing int64
	database.Table("public.mkt_t_eterimabtt").Where("TRIM(tb_bttid) = TRIM(?)", req.BTTID).Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Resi BTT '%s' sudah dicatat penerimaan kembalinya sebelumnya", req.BTTID)})
		return
	}

	insertSQL := `
		INSERT INTO public.mkt_t_eterimabtt (tb_bttid, tb_tglkembali, tb_agenid, tb_aktifyn)
		VALUES (?, ?, ?, 'Y')
	`
	if err := database.Exec(insertSQL, strings.TrimSpace(req.BTTID), req.TglKembali, req.AgenID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data BTT kembali: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Penerimaan BTT kembali berhasil disimpan"})
}

// 3. DELETE /api/terima-btt/:id
func DeleteTerimaBTT(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database corporate gagal dihubungkan"})
		return
	}

	id := c.Param("id")
	updateSQL := `UPDATE public.mkt_t_eterimabtt SET tb_aktifyn = 'N' WHERE TRIM(tb_bttid) = TRIM(?)`
	if err := database.Exec(updateSQL, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menonaktifkan BTT kembali: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data BTT kembali dinonaktifkan"})
}

// 4. GET /api/terima-btt/combo-kota (Ambil kota dari tujuan pengiriman yang aktif di econote)
func GetComboKota(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database corporate gagal dihubungkan"})
		return
	}

	var kotas []struct {
		KotaName string `json:"kota_name" gorm:"column:kota_name"`
	}

	// Mengambil DISTINCT kota tujuan dari resi pengiriman agar datanya valid dan pasti ada
	query := `
		SELECT DISTINCT TRIM(bttt_tujuankota) AS kota_name
		FROM public.mkt_t_econote
		WHERE COALESCE(TRIM(bttt_tujuankota), '') <> ''
		ORDER BY kota_name ASC
		LIMIT 200
	`
	if err := database.Raw(query).Scan(&kotas).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": kotas})
}
