package models

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// KembaliSJHeader mewakili tabel mkt_t_kembalisj_h di pgAdmin
type KembaliSJHeader struct {
	ID              string     `gorm:"column:mkt_t_kembalisj_id;primaryKey" json:"mkt_t_kembalisj_id"`
	CustID          string     `gorm:"column:mkt_t_kembalisj_custid" json:"mkt_t_kembalisj_custid"`
	Tanggal         time.Time  `gorm:"column:mkt_t_kembalisj_tanggal" json:"mkt_t_kembalisj_tanggal"`
	Keterangan      string     `gorm:"column:mkt_t_kembalisj_keterangan" json:"mkt_t_kembalisj_keterangan"`
	Diterima        string     `gorm:"column:mkt_t_kembalisj_diterima" json:"mkt_t_kembalisj_diterima"`
	TanggalDiterima *time.Time `gorm:"column:mkt_t_kembalisj_tanggalditerima" json:"mkt_t_kembalisj_tanggalditerima"`
	UpdateID        string     `gorm:"column:mkt_t_kembalisj_updateid" json:"mkt_t_kembalisj_updateid"`
	UpdateTime      time.Time  `gorm:"column:mkt_t_kembalisj_updatetime" json:"mkt_t_kembalisj_updatetime"`
	AktifYN         string     `gorm:"column:mkt_t_kembalisj_aktifyn;default:Y" json:"mkt_t_kembalisj_aktifyn"`
}

// KembaliSJResponse mewakili hasil LEFT JOIN untuk dirender di React DataTableTemplate
type KembaliSJResponse struct {
	ID              string     `gorm:"column:mkt_t_kembalisj_id" json:"mkt_t_kembalisj_id"`
	Tanggal         time.Time  `gorm:"column:mkt_t_kembalisj_tanggal" json:"mkt_t_kembalisj_tanggal"`
	CustID          string     `gorm:"column:mkt_t_kembalisj_custid" json:"mkt_t_kembalisj_custid"`
	CustName        string     `gorm:"column:cust_name" json:"cust_name"`
	Keterangan      string     `gorm:"column:mkt_t_kembalisj_keterangan" json:"mkt_t_kembalisj_keterangan"`
	UpdateID        string     `gorm:"column:mkt_t_kembalisj_updateid" json:"mkt_t_kembalisj_updateid"`               // Dibuat Oleh
	Diterima        string     `gorm:"column:mkt_t_kembalisj_diterima" json:"mkt_t_kembalisj_diterima"`               // Diterima Oleh
	TanggalDiterima *time.Time `gorm:"column:mkt_t_kembalisj_tanggalditerima" json:"mkt_t_kembalisj_tanggalditerima"` // Tanggal Diterima
	AktifYN         string     `gorm:"column:mkt_t_kembalisj_aktifyn" json:"mkt_t_kembalisj_aktifyn"`                 // Aktif
}

// Struct request untuk endpoint /marketing/kembali-sj/add
type CreateKembaliSJAddReq struct {
	TanggalPengembalian string   `json:"tanggal_pengembalian" binding:"required"`
	Pengirim            string   `json:"pengirim"`
	Keterangan          string   `json:"keterangan"`
	CustomerID          string   `json:"customer_id" binding:"required"`
	DaftarNoSJ          []string `json:"daftar_nosj"`
}

// 5. GET OUTSTANDING SJ BY CUSTOMER (DIPANGGIL MODAL TambahPengembalianSuratJalan.jsx)
// Route: GET /marketing/outstanding-sj/:customer_id
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

	// Tarik data surat jalan dari opr_t_transportlspb yang belum dikembalikan (spb_id kosong/null)
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

// 6. SIMPAN DOKUMEN PENGEMBALIAN SJ (DIPANGGIL MODAL TambahPengembalianSuratJalan.jsx)
// Route: POST /marketing/kembali-sj/add
func CreateKembaliSJAdd(c *gin.Context) {
	var req CreateKembaliSJAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid: " + err.Error()})
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

	// Hitung counter urutan 6 digit terakhir
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

	// 1. Simpan Header ke mkt_t_kembalisj_h
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

	// 2. Simpan Detail ke mkt_t_kembalisj_d & update spb_id di opr_t_transportlspb
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
