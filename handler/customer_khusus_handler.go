package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// 1. AJAX SUGGEST CUSTOMER KHUSUS (get-cust_csv.asp)
// GET /marketing/customer-khusus/suggest?q=xxx
func SuggestCustomerKhusus(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if len(q) < 2 {
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": []interface{}{}})
		return
	}

	type CustItem struct {
		CustID   string `json:"cust_id"`
		CustName string `json:"cust_name"`
	}
	var list []CustItem

	err := db.DB.Table("mkt_m_customer").
		Select("DISTINCT cust_id, cust_name").
		Where("cust_name ILIKE ?", "%"+q+"%").
		Order("cust_name ASC").
		Limit(25).
		Scan(&list).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// 2. UNGGAH & SIMPAN BERKAS CSV CUSTOMER KHUSUS (mkt_t_econote_csv_upload.asp)
// POST /marketing/customer-khusus/upload
func UploadCSVCustomerKhusus(c *gin.Context) {
	customerID := strings.TrimSpace(c.PostForm("customer_id"))
	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID Pelanggan Khusus wajib diisi"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV wajib diunggah"})
		return
	}

	// Validasi aturan nama file: YYYYMMDDHHmm.csv (12 digit angka)
	baseFileName := filepath.Base(fileHeader.Filename)
	matched, _ := regexp.MatchString(`^\d{12}\.csv$`, strings.ToLower(baseFileName))
	if !matched {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Nama file harus berformat TahunBulanTanggalJamMenit.csv (contoh: 202609221530.csv)",
		})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membaca file: " + err.Error()})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true

	// Deteksi delimiter otomatis
	firstLine, err := reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV kosong atau tidak valid"})
		return
	}
	if len(firstLine) == 1 && strings.Contains(firstLine[0], ";") {
		reader.Comma = ';'
	}

	username := "superdli"
	if authUser, exists := c.Get("username"); exists && authUser != nil {
		username = fmt.Sprintf("%v", authUser)
	}

	tx := db.DB.Begin()

	// 1. Simpan Header Batch
	batchID := fmt.Sprintf("CK-%s-%s", customerID, strings.TrimSuffix(baseFileName, ".csv"))
	insertH := `
		INSERT INTO public.mkt_t_econote_csv_h 
			(econote_csv_id, econote_csv_custid, econote_csv_filename, econote_csv_tanggal, econote_csv_updateid, econote_csv_updatetime, econote_csv_aktifyn) 
		VALUES 
			(?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, 'Y')
		ON CONFLICT (econote_csv_id) DO NOTHING
	`
	if err := tx.Exec(insertH, batchID, customerID, baseFileName, username).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan batch header: " + err.Error()})
		return
	}

	// 2. Loop Baris Detail
	insertD := `
		INSERT INTO public.mkt_t_econote_csv_d 
			(econote_csv_id_h, econote_csv_custid, econote_csv_nosj, econote_csv_penerima, econote_csv_alamat, econote_csv_kota, econote_csv_koli, econote_csv_berat, econote_csv_nobtt, econote_csv_holdyn, created_at) 
		VALUES 
			(?, ?, ?, ?, ?, ?, ?, ?, NULL, 'N', CURRENT_TIMESTAMP)
	`
	totalKoli := 0
	totalBerat := 0.0
	totalRows := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 2 {
			continue
		}

		noSJ := strings.TrimSpace(record[0])
		penerima := strings.TrimSpace(record[1])
		alamat := ""
		if len(record) > 2 {
			alamat = strings.TrimSpace(record[2])
		}
		kota := ""
		if len(record) > 3 {
			kota = strings.TrimSpace(record[3])
		}
		koli := 1
		if len(record) > 4 {
			if val, e := strconv.Atoi(strings.TrimSpace(record[4])); e == nil && val > 0 {
				koli = val
			}
		}
		berat := 1.0
		if len(record) > 5 {
			cleanB := strings.ReplaceAll(strings.TrimSpace(record[5]), ",", ".")
			if val, e := strconv.ParseFloat(cleanB, 64); e == nil && val > 0 {
				berat = val
			}
		}

		if err := tx.Exec(insertD, batchID, customerID, noSJ, penerima, alamat, kota, koli, berat).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan baris detail: " + err.Error()})
			return
		}

		totalKoli += koli
		totalBerat += berat
		totalRows++
	}

	// Update akumulasi total di Header
	tx.Exec("UPDATE public.mkt_t_econote_csv_h SET econote_csv_totalkoli = ?, econote_csv_totalberat = ? WHERE econote_csv_id = ?", totalKoli, totalBerat, batchID)
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"message":     fmt.Sprintf("Berhasil mengunggah %d baris pengiriman!", totalRows),
		"batch_id":    batchID,
		"total_rows":  totalRows,
		"total_koli":  totalKoli,
		"total_berat": totalBerat,
	})
}

// 3. DAFTAR BARANG BELUM DIBUAT BTT (mkt_t_econote_csv_upload_d.asp)
// GET /marketing/customer-khusus/unprocessed?cust_id=xxx
func GetUnprocessedBTT(c *gin.Context) {
	custID := strings.TrimSpace(c.Query("cust_id"))
	if custID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Customer ID wajib diisi"})
		return
	}

	type UnprocessedItem struct {
		ID        int64     `json:"id"`
		BatchID   string    `json:"batch_id"`
		NoSJ      string    `json:"no_sj"`
		Penerima  string    `json:"penerima"`
		Alamat    string    `json:"alamat"`
		Kota      string    `json:"kota"`
		Koli      int       `json:"koli"`
		Berat     float64   `json:"berat"`
		CreatedAt time.Time `json:"created_at"`
	}

	var list []UnprocessedItem
	err := db.DB.Table("mkt_t_econote_csv_d").
		Select("id, econote_csv_id_h AS batch_id, econote_csv_nosj AS no_sj, econote_csv_penerima AS penerima, econote_csv_alamat AS alamat, econote_csv_kota AS kota, econote_csv_koli AS koli, econote_csv_berat AS berat, created_at").
		Where("econote_csv_custid = ? AND (econote_csv_nobtt IS NULL OR econote_csv_nobtt = '') AND econote_csv_holdyn = 'N'", custID).
		Order("id DESC").
		Limit(300).
		Scan(&list).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// 4. DAFTAR BTT HOLD (mkt_t_econote_csv_upload_Hold.asp)
// GET /marketing/customer-khusus/hold-list?cust_id=xxx
func GetBTTHoldList(c *gin.Context) {
	custID := strings.TrimSpace(c.Query("cust_id"))
	if custID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Customer ID wajib diisi"})
		return
	}

	type BTTHoldItem struct {
		ID         int64     `json:"id"`
		BatchID    string    `json:"batch_id"`
		NoSJ       string    `json:"no_sj"`
		Penerima   string    `json:"penerima"`
		Kota       string    `json:"kota"`
		Koli       int       `json:"koli"`
		Berat      float64   `json:"berat"`
		HoldAlasan string    `json:"hold_alasan"`
		CreatedAt  time.Time `json:"created_at"`
	}

	var list []BTTHoldItem
	err := db.DB.Table("mkt_t_econote_csv_d").
		Select("id, econote_csv_id_h AS batch_id, econote_csv_nosj AS no_sj, econote_csv_penerima AS penerima, econote_csv_kota AS kota, econote_csv_koli AS koli, econote_csv_berat AS berat, COALESCE(econote_csv_holdalasan, 'Ditahan Operasional') AS hold_alasan, created_at").
		Where("econote_csv_custid = ? AND econote_csv_holdyn = 'Y'", custID).
		Order("id DESC").
		Limit(300).
		Scan(&list).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// 5. TOGGLE HOLD STATUS BTT
// POST /marketing/customer-khusus/toggle-hold
func ToggleHoldBTT(c *gin.Context) {
	type HoldReq struct {
		ID     int64  `json:"id" binding:"required"`
		HoldYN string `json:"hold_yn" binding:"required"` // "Y" atau "N"
		Alasan string `json:"alasan"`
	}

	var req HoldReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid"})
		return
	}

	err := db.DB.Table("mkt_t_econote_csv_d").
		Where("id = ?", req.ID).
		Updates(map[string]interface{}{
			"econote_csv_holdyn":     req.HoldYN,
			"econote_csv_holdalasan": req.Alasan,
		}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	msg := "Baris berhasil dilepas dari status Hold"
	if req.HoldYN == "Y" {
		msg = "Baris berhasil dimasukkan ke daftar Hold"
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": msg})
}
