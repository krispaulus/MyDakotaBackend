package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 1. GET LIST PENGEMBALIAN SURAT JALAN (DENGAN FILTER & 8 KOLOM LENGKAP)
func GetKembaliSJList(c *gin.Context) {
	tglAwal := c.Query("tgl_awal")
	tglAkhir := c.Query("tgl_akhir")
	custName := c.Query("customer_name")
	docID := c.Query("document_id")
	noSJ := c.Query("no_sj")

	var results []models.KembaliSJResponse

	query := db.DB.Table("mkt_t_kembalisj_h h").
		Select(`
			h.mkt_t_kembalisj_id, 
			h.mkt_t_kembalisj_tanggal, 
			h.mkt_t_kembalisj_custid, 
			c.cust_name, 
			h.mkt_t_kembalisj_keterangan, 
			COALESCE(h.mkt_t_kembalisj_updateid, '-') AS mkt_t_kembalisj_updateid,
			COALESCE(h.mkt_t_kembalisj_diterima, '-') AS mkt_t_kembalisj_diterima, 
			h.mkt_t_kembalisj_tanggalditerima, 
			h.mkt_t_kembalisj_aktifyn
		`).
		Joins("LEFT JOIN mkt_m_customer c ON h.mkt_t_kembalisj_custid = c.cust_id").
		Where("h.mkt_t_kembalisj_aktifyn = ?", "Y")

	if tglAwal != "" && tglAkhir != "" {
		query = query.Where("h.mkt_t_kembalisj_tanggal BETWEEN ? AND ?", tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}
	if custName != "" {
		query = query.Where("c.cust_name ILIKE ?", "%"+custName+"%")
	}
	if docID != "" {
		query = query.Where("h.mkt_t_kembalisj_id ILIKE ?", "%"+docID+"%")
	}
	if noSJ != "" {
		query = query.Where("h.mkt_t_kembalisj_id IN (SELECT mkt_t_kembalisj_id_h FROM mkt_t_kembalisj_d WHERE mkt_t_kembalisj_nosj ILIKE ?)", "%"+noSJ+"%")
	}

	if err := query.Order("h.mkt_t_kembalisj_tanggal DESC").Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// Struct request simpan pengembalian
type CreateKembaliSJReq struct {
	CustomerID string   `json:"customer_id" binding:"required"`
	Tanggal    string   `json:"tanggal" binding:"required"`
	Keterangan string   `json:"keterangan"`
	ListSJ     []SJPair `json:"list_sj" binding:"required"`
}

type SJPair struct {
	NoSJ  string `json:"no_sj"`
	NoBTT string `json:"no_btt"`
}

// 2. GET DAFTAR SURAT JALAN / DN YANG SIAP DIKEMBALIKAN (DARI OPR_T_TRANSPORTLSPB)
func GetAvailableSJ(c *gin.Context) {
	customerID := c.Query("customer_id")
	search := c.Query("search")

	type AvailableSJItem struct {
		NoSJ         string `json:"no_sj"`
		NoBTT        string `json:"no_btt"`
		CustomerID   string `json:"customer_id"`
		CustomerName string `json:"customer_name"`
		NamaPenerima string `json:"nama_penerima"`
		TglDiterima  string `json:"tgl_diterima"`
	}

	var list []AvailableSJItem

	query := db.DB.Table("opr_t_transportlspb t").
		Select(`
			COALESCE(t.dn_od_sj, '') AS no_sj,
			COALESCE(t.nomorbtt, '') AS no_btt,
			COALESCE(t.customerid, '') AS customer_id,
			COALESCE(cu.cust_name, t.pickupfrom) AS customer_name,
			COALESCE(t.nama_penerima, '-') AS nama_penerima,
			COALESCE(t.tanggal_diterima, '-') AS tgl_diterima
		`).
		Joins("LEFT JOIN mkt_m_customer cu ON t.customerid = cu.cust_id").
		Where("t.spb_id IS NULL OR t.spb_id = '' OR t.spb_id = 'NULL'")

	if customerID != "" {
		query = query.Where("t.customerid = ? OR cu.cust_name ILIKE ?", customerID, "%"+customerID+"%")
	}
	if search != "" {
		query = query.Where("t.dn_od_sj ILIKE ? OR t.nomorbtt ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if err := query.Order("t.id DESC").Limit(150).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// 3. PROSES SIMPAN PENGEMBALIAN SURAT JALAN (INSERT HEADER, DETAIL, & UPDATE STATUS LSPB)
func CreateKembaliSJ(c *gin.Context) {
	var req CreateKembaliSJReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid"})
		return
	}

	if len(req.ListSJ) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Pilih minimal 1 Surat Jalan"})
		return
	}

	agenID := "001"
	if authAgen, exists := c.Get("agen_id"); exists && authAgen != nil {
		agenID = fmt.Sprintf("%03v", authAgen)
		if len(agenID) > 3 {
			agenID = agenID[:3]
		}
	}

	username := "superdli"
	if authUser, exists := c.Get("username"); exists && authUser != nil {
		username = fmt.Sprintf("%v", authUser)
	}

	// Format penomoran legacy ASP: Cabang(3) + Counter(6) + Bulan(2) + Tahun(2) + "SJ"
	now := time.Now()
	ekor := fmt.Sprintf("%02d%02dSJ", int(now.Month()), now.Year()%100)

	tx := db.DB.Begin()

	var lastUrut struct {
		Urut string
	}
	tx.Raw(`
		SELECT SUBSTRING(mkt_t_kembalisj_id, 4, 6) AS urut 
		FROM mkt_t_kembalisj_h 
		WHERE LEFT(mkt_t_kembalisj_id, 3) = ? AND RIGHT(mkt_t_kembalisj_id, 6) = ? 
		ORDER BY SUBSTRING(mkt_t_kembalisj_id, 4, 6) DESC 
		LIMIT 1
	`, agenID, ekor).Scan(&lastUrut)

	counter := 1
	if lastUrut.Urut != "" {
		if val, err := strconv.Atoi(lastUrut.Urut); err == nil {
			counter = val + 1
		}
	}
	keID := fmt.Sprintf("%s%06d%s", agenID, counter, ekor)

	// Simpan Header
	insertH := `
		INSERT INTO mkt_t_kembalisj_h 
			(mkt_t_kembalisj_id, mkt_t_kembalisj_custid, mkt_t_kembalisj_tanggal, mkt_t_kembalisj_keterangan, mkt_t_kembalisj_updateid, mkt_t_kembalisj_updatetime, mkt_t_kembalisj_aktifyn) 
		VALUES 
			(?, ?, ?::timestamp, ?, ?, CURRENT_TIMESTAMP, 'Y')
	`
	if err := tx.Exec(insertH, keID, req.CustomerID, req.Tanggal+" 00:00:00", req.Keterangan, username).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan header: " + err.Error()})
		return
	}

	// Simpan Detail & Update status dokumen di tabel transport operasional
	for _, sj := range req.ListSJ {
		insertD := `
			INSERT INTO mkt_t_kembalisj_d 
				(mkt_t_kembalisj_id_h, mkt_t_kembalisj_nosj, mkt_t_kembalisj_btt) 
			VALUES 
				(?, ?, ?)
		`
		if err := tx.Exec(insertD, keID, sj.NoSJ, sj.NoBTT).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan detail SJ: " + err.Error()})
			return
		}

		updateLSPB := `
			UPDATE opr_t_transportlspb 
			SET spb_id = ?, tanggal_dokumenkembali = ?::timestamp 
			WHERE dn_od_sj = ?
		`
		tx.Exec(updateLSPB, keID, req.Tanggal+" 00:00:00", sj.NoSJ)
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Pengembalian Surat Jalan berhasil dibuat dengan No: %s", keID),
		"spb_id":  keID,
	})
}

// 4. BATALKAN / HAPUS PENGEMBALIAN SURAT JALAN (p-mkt_t_kembaliSJ_h.asp)
func DeleteKembaliSJ(c *gin.Context) {
	idSJ := c.Param("id")
	if idSJ == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID Pengembalian wajib diisi"})
		return
	}

	tx := db.DB.Begin()

	// Reset status dokumen balikan di tabel LSPB
	tx.Exec("UPDATE opr_t_transportlspb SET spb_id = NULL, tanggal_dokumenkembali = NULL WHERE spb_id = ?", idSJ)

	// Hapus transaksi pengembalian
	tx.Exec("DELETE FROM mkt_t_kembalisj_d WHERE mkt_t_kembalisj_id_h = ?", idSJ)
	tx.Exec("DELETE FROM mkt_t_kembalisj_h WHERE mkt_t_kembalisj_id = ?", idSJ)

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Pengembalian %s berhasil dibatalkan", idSJ)})
}

// GET /marketing/outstanding-sj/:customer_id
func GetOutstandingSJ(c *gin.Context) {
	customerID := c.Param("customer_id")
	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Customer ID wajib diisi"})
		return
	}

	type SJItem struct {
		NoSJ string `json:"mkt_t_kembalisj_nosj"`
	}

	var results []SJItem

	err := db.DB.Table("opr_t_transportlspb").
		Select("DISTINCT dn_od_sj AS mkt_t_kembalisj_nosj").
		Where("customerid = ? AND (spb_id IS NULL OR spb_id = '' OR spb_id = 'NULL')", customerID).
		Where("dn_od_sj IS NOT NULL AND dn_od_sj <> ''").
		Order("dn_od_sj ASC").
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

// POST /marketing/kembali-sj/add
func CreateKembaliSJAdd(c *gin.Context) {
	type CreateKembaliSJAddReq struct {
		TanggalPengembalian string   `json:"tanggal_pengembalian" binding:"required"`
		Pengirim            string   `json:"pengirim"`
		Keterangan          string   `json:"keterangan"`
		CustomerID          string   `json:"customer_id" binding:"required"`
		DaftarNoSJ          []string `json:"daftar_nosj"`
	}

	var req CreateKembaliSJAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid: " + err.Error()})
		return
	}

	if len(req.DaftarNoSJ) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Silakan centang minimal 1 Surat Jalan terlebih dahulu"})
		return
	}

	agenID := "001"
	if authAgen, exists := c.Get("agen_id"); exists && authAgen != nil {
		agenID = fmt.Sprintf("%03v", authAgen)
		if len(agenID) > 3 {
			agenID = agenID[:3]
		}
	}

	username := "superdli"
	if authUser, exists := c.Get("username"); exists && authUser != nil {
		username = fmt.Sprintf("%v", authUser)
	}

	now := time.Now()
	ekor := fmt.Sprintf("%02d%02dSJ", int(now.Month()), now.Year()%100)

	tx := db.DB.Begin()

	var lastUrut struct {
		Urut string
	}
	tx.Raw(`
		SELECT SUBSTRING(mkt_t_kembalisj_id, 4, 6) AS urut 
		FROM mkt_t_kembalisj_h 
		WHERE LEFT(mkt_t_kembalisj_id, 3) = ? AND RIGHT(mkt_t_kembalisj_id, 6) = ? 
		ORDER BY SUBSTRING(mkt_t_kembalisj_id, 4, 6) DESC 
		LIMIT 1
	`, agenID, ekor).Scan(&lastUrut)

	counter := 1
	if lastUrut.Urut != "" {
		if val, err := strconv.Atoi(lastUrut.Urut); err == nil {
			counter = val + 1
		}
	}
	keID := fmt.Sprintf("%s%06d%s", agenID, counter, ekor)

	insertH := `
		INSERT INTO mkt_t_kembalisj_h 
			(mkt_t_kembalisj_id, mkt_t_kembalisj_custid, mkt_t_kembalisj_tanggal, mkt_t_kembalisj_keterangan, mkt_t_kembalisj_updateid, mkt_t_kembalisj_updatetime, mkt_t_kembalisj_aktifyn) 
		VALUES 
			(?, ?, ?::timestamp, ?, ?, CURRENT_TIMESTAMP, 'Y')
	`
	if err := tx.Exec(insertH, keID, req.CustomerID, req.TanggalPengembalian+" 00:00:00", req.Keterangan, username).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan header: " + err.Error()})
		return
	}

	for _, noSJ := range req.DaftarNoSJ {
		var noBTT string
		tx.Table("opr_t_transportlspb").
			Select("nomorbtt").
			Where("dn_od_sj = ?", noSJ).
			Limit(1).
			Scan(&noBTT)

		insertD := `
			INSERT INTO mkt_t_kembalisj_d 
				(mkt_t_kembalisj_id_h, mkt_t_kembalisj_nosj, mkt_t_kembalisj_btt) 
			VALUES 
				(?, ?, ?)
		`
		if err := tx.Exec(insertD, keID, noSJ, noBTT).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan detail SJ: " + err.Error()})
			return
		}

		updateLSPB := `
			UPDATE opr_t_transportlspb 
			SET spb_id = ?, tanggal_dokumenkembali = ?::timestamp 
			WHERE dn_od_sj = ?
		`
		tx.Exec(updateLSPB, keID, req.TanggalPengembalian+" 00:00:00", noSJ)
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Pengembalian Surat Jalan berhasil dibuat dengan No: %s", keID),
		"spb_id":  keID,
	})
}
