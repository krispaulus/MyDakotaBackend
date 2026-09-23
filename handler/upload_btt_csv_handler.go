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

type BTTCSVRow struct {
	KodeGroupDN    string  `json:"kode_group_dn"`
	NoDN           string  `json:"no_dn"`
	NamaPengirim   string  `json:"nama_pengirim"`
	KotaPengirim   string  `json:"kota_pengirim"`
	NamaPenerima   string  `json:"nama_penerima"`
	AlamatPenerima string  `json:"alamat_penerima"`
	KotaTujuan     string  `json:"kota_tujuan"`
	Koli           int     `json:"koli"`
	Berat          float64 `json:"berat"`
	BiayaPacking   float64 `json:"biaya_packing"`
	NilaiAsuransi  float64 `json:"nilai_asuransi"`
	Harga          float64 `json:"harga"`
	Tanggal        string  `json:"tanggal"`
	NoKendaraan    string  `json:"no_kendaraan"`
	NoSKB          string  `json:"no_skb"`
}

// 1. SUGGEST CUSTOMER UNTUK UPLOAD BTT
// GET /marketing/upload-btt/suggest-customer?q=xxx
func SuggestCustomerBTT(c *gin.Context) {
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

// 2. PARSE CSV BTT 37 / 38 KOLOM
// POST /marketing/upload-btt/parse-csv
func ParseBTTCSV(c *gin.Context) {
	customerID := strings.TrimSpace(c.PostForm("customer_id"))
	delimiterChoice := strings.TrimSpace(c.PostForm("delimiter"))
	tipe := strings.TrimSpace(c.PostForm("tipe")) // "1" = Luar Kota, "2" = Dalam Kota

	if customerID == "" || tipe == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Customer dan Tipe Pengiriman wajib dipilih"})
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

	var rows []BTTCSVRow
	rowIndex := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		rowIndex++
		if rowIndex == 1 {
			// Skip Header
			continue
		}

		if len(record) < 15 {
			continue
		}

		// Helper pembersih angka desimal
		parseFloat := func(val string) float64 {
			clean := strings.ReplaceAll(strings.TrimSpace(val), ",", ".")
			if res, e := strconv.ParseFloat(clean, 64); e == nil {
				return res
			}
			return 0
		}

		parseInt := func(val string) int {
			if res, e := strconv.Atoi(strings.TrimSpace(val)); e == nil {
				return res
			}
			return 0
		}

		// Parsing berdasarkan indeks template (Luar Kota 37 kolom, Dalam Kota 38 kolom)
		noDN := strings.TrimSpace(record[1])
		namaPengirim := strings.TrimSpace(record[2])
		kotaPengirim := strings.TrimSpace(record[4])
		namaPenerima := strings.TrimSpace(record[6])
		alamatPenerima := strings.TrimSpace(record[9])
		kotaTujuan := strings.TrimSpace(record[12])
		koli := parseInt(record[17])
		berat := parseFloat(record[18])
		biayaPacking := parseFloat(record[24])
		nilaiAsuransi := parseFloat(record[26])

		var harga float64 = 0
		var tglKirim, noKendaraan, noSKB string

		if tipe == "2" && len(record) >= 38 {
			// Template Dalam Kota (kolom 31 = Harga)
			harga = parseFloat(record[31])
			tglKirim = strings.TrimSpace(record[32])
			noKendaraan = strings.TrimSpace(record[33])
			noSKB = strings.TrimSpace(record[37])
		} else if len(record) >= 37 {
			// Template Luar Kota
			tglKirim = strings.TrimSpace(record[31])
			noKendaraan = strings.TrimSpace(record[32])
			noSKB = strings.TrimSpace(record[36])
		}

		rows = append(rows, BTTCSVRow{
			KodeGroupDN:    strings.TrimSpace(record[0]),
			NoDN:           noDN,
			NamaPengirim:   namaPengirim,
			KotaPengirim:   kotaPengirim,
			NamaPenerima:   namaPenerima,
			AlamatPenerima: alamatPenerima,
			KotaTujuan:     kotaTujuan,
			Koli:           koli,
			Berat:          berat,
			BiayaPacking:   biayaPacking,
			NilaiAsuransi:  nilaiAsuransi,
			Harga:          harga,
			Tanggal:        tglKirim,
			NoKendaraan:    noKendaraan,
			NoSKB:          noSKB,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"total_rows": len(rows),
		"data":       rows,
	})
}

// 3. SIMPAN BATCH KE TABEL STAGING BTT
// POST /marketing/upload-btt/save-batch
func SaveBTTCSVBatch(c *gin.Context) {
	type SaveReq struct {
		CustomerID string      `json:"customer_id" binding:"required"`
		Tipe       string      `json:"tipe" binding:"required"` // "1" / "2"
		Rows       []BTTCSVRow `json:"rows" binding:"required"`
	}

	var req SaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid"})
		return
	}

	if len(req.Rows) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Tidak ada data untuk disimpan"})
		return
	}

	username := "superdli"
	if authUser, exists := c.Get("username"); exists && authUser != nil {
		username = fmt.Sprintf("%v", authUser)
	}

	tipeLabel := "LUAR KOTA"
	if req.Tipe == "2" {
		tipeLabel = "DALAM KOTA"
	}

	batchID := fmt.Sprintf("BATCH-BTT-%d", time.Now().UnixNano())
	tx := db.DB.Begin()

	insertSQL := `
		INSERT INTO public.opr_t_uploadcsv_btt 
			(batch_id, customer_id, tipe_distribusi, kode_group_dn, no_dn, nama_pengirim, kota_pengirim, nama_penerima, alamat_penerima, kota_kabupaten, jumlah_koli, berat, biaya_packing, nilai_barang_asuransi, harga, no_kendaraan, no_skb, status_proses, user_upload, created_at)
		VALUES 
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'PENDING', ?, CURRENT_TIMESTAMP)
	`

	for _, r := range req.Rows {
		if err := tx.Exec(insertSQL, batchID, req.CustomerID, tipeLabel, r.KodeGroupDN, r.NoDN, r.NamaPengirim, r.KotaPengirim, r.NamaPenerima, r.AlamatPenerima, r.KotaTujuan, r.Koli, r.Berat, r.BiayaPacking, r.NilaiAsuransi, r.Harga, r.NoKendaraan, r.NoSKB, username).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan batch BTT: " + err.Error()})
			return
		}
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  fmt.Sprintf("Berhasil memproses %d baris BTT!", len(req.Rows)),
		"batch_id": batchID,
	})
}
