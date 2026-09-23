package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// Item hasil parse CSV Transport Planning
type TransportPlanningRow struct {
	NoDODN        string  `json:"no_do_dn"`
	NamaPenerima  string  `json:"nama_penerima"`
	Alamat        string  `json:"alamat"`
	KotaTujuan    string  `json:"kota_tujuan"`
	Koli          int     `json:"koli"`
	Berat         float64 `json:"berat"`
	Volume        float64 `json:"volume"`
	StatusWilayah string  `json:"status_wilayah"` // "DALAM KOTA" / "LUAR KOTA"
}

// 1. AJAX SUGGEST CUSTOMER PRINCIPAL
// GET /marketing/transport-planning/suggest-customer?q=xxx
func SuggestCustomerPlanning(c *gin.Context) {
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
		Limit(20).
		Scan(&list).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// 2. PARSE CSV TRANSPORT PLANNING & REVIEW (VALIDASI + ZONING DALAM/LUAR KOTA)
// POST /marketing/transport-planning/parse-csv
func ParseTransportPlanningCSV(c *gin.Context) {
	customerID := strings.TrimSpace(c.PostForm("customer_id"))
	delimiterChoice := strings.TrimSpace(c.PostForm("delimiter"))

	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Pilih Customer Principal terlebih dahulu"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV wajib diunggah"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membuka file: " + err.Error()})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	if delimiterChoice == "titikkoma" {
		reader.Comma = ';'
	} else {
		reader.Comma = ','
	}
	reader.LazyQuotes = true

	// Ambil daftar anak customer / dalam kota untuk verifikasi zonasi
	var dalamKotaList []string
	db.DB.Table("mkt_m_dalam_kota").Pluck("nama_kota", &dalamKotaList)
	dalamKotaMap := make(map[string]bool)
	for _, k := range dalamKotaList {
		dalamKotaMap[strings.ToUpper(strings.TrimSpace(k))] = true
	}

	var parsedRows []TransportPlanningRow
	rowIndex := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Skip row rusak
		}
		rowIndex++
		if rowIndex == 1 {
			// Skip baris header jika ada
			continue
		}

		if len(record) < 3 {
			continue
		}

		// Asumsi urutan kolom: [0]=No DO/DN, [1]=Nama Penerima, [2]=Alamat, [3]=Kota, [4]=Koli, [5]=Berat, [6]=Volume
		noDO := strings.TrimSpace(record[0])
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

		var berat float64 = 1.0
		if len(record) > 5 {
			cleanBerat := strings.ReplaceAll(strings.TrimSpace(record[5]), ",", ".")
			if val, e := strconv.ParseFloat(cleanBerat, 64); e == nil && val > 0 {
				berat = val
			}
		}

		var volume float64 = 0
		if len(record) > 6 {
			cleanVol := strings.ReplaceAll(strings.TrimSpace(record[6]), ",", ".")
			if val, e := strconv.ParseFloat(cleanVol, 64); e == nil {
				volume = val
			}
		}

		// Deteksi status dalam kota / luar kota
		statusWilayah := "LUAR KOTA"
		if dalamKotaMap[strings.ToUpper(kota)] {
			statusWilayah = "DALAM KOTA"
		}

		parsedRows = append(parsedRows, TransportPlanningRow{
			NoDODN:        noDO,
			NamaPenerima:  penerima,
			Alamat:        alamat,
			KotaTujuan:    kota,
			Koli:          koli,
			Berat:         berat,
			Volume:        volume,
			StatusWilayah: statusWilayah,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"total_rows":  len(parsedRows),
		"customer_id": customerID,
		"data":        parsedRows,
	})
}

// 3. SIMPAN BATCH HASIL UPLOAD TRANSPORT PLANNING KE TABEL STAGING
// POST /marketing/transport-planning/save-batch
func SaveTransportPlanningBatch(c *gin.Context) {
	type SaveReq struct {
		CustomerID   string                 `json:"customer_id" binding:"required"`
		CustomerName string                 `json:"customer_name"`
		Delimiter    string                 `json:"delimiter"`
		Rows         []TransportPlanningRow `json:"rows" binding:"required"`
	}

	var req SaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid"})
		return
	}

	if len(req.Rows) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Tidak ada baris data untuk disimpan"})
		return
	}

	username := "superdli"
	if authUser, exists := c.Get("username"); exists && authUser != nil {
		username = fmt.Sprintf("%v", authUser)
	}

	batchID := fmt.Sprintf("BATCH-%d", time.Now().UnixNano())

	tx := db.DB.Begin()

	insertSQL := `
		INSERT INTO public.opr_t_transport_planning_upload 
			(batch_upload_id, customer_id, customer_name, no_do_dn, nama_penerima, alamat_penerima, kota_tujuan, koli, berat, volume, status_wilayah, status_proses, delimiter_used, user_upload, created_at)
		VALUES 
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'PENDING', ?, ?, CURRENT_TIMESTAMP)
	`

	for _, r := range req.Rows {
		if err := tx.Exec(insertSQL, batchID, req.CustomerID, req.CustomerName, r.NoDODN, r.NamaPenerima, r.Alamat, r.KotaTujuan, r.Koli, r.Berat, r.Volume, r.StatusWilayah, req.Delimiter, username).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan baris: " + err.Error()})
			return
		}
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  fmt.Sprintf("Berhasil mengunggah %d baris Transport Planning!", len(req.Rows)),
		"batch_id": batchID,
	})
}
