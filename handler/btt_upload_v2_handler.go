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

type BTTUploadV2Row struct {
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
	Service        string  `json:"service"`
	Harga          float64 `json:"harga"`
	Tanggal        string  `json:"tanggal"`
	ReqInvoice     string  `json:"req_invoice"`
	KategoriGRN    string  `json:"kategori_grn"` // "Y" (Copy GRN) atau "N" (Asli GRN)
}

// 1. SUGGEST CUSTOMER
// GET /marketing/btt-upload-v2/suggest-customer?q=xxx
func SuggestCustomerBTTV2(c *gin.Context) {
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

// 2. PARSE CSV BTT UPLOAD V2 (32 / 33 KOLOM)
// POST /marketing/btt-upload-v2/parse-csv
func ParseBTTV2CSV(c *gin.Context) {
	customerID := strings.TrimSpace(c.PostForm("customer_id"))
	delimiterChoice := strings.TrimSpace(c.PostForm("delimiter"))
	tipe := strings.TrimSpace(c.PostForm("tipe")) // 1: Luar Kota, 2: Dalam Kota, 3: Dedicated

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

	var rows []BTTUploadV2Row
	rowIndex := 0

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

	// Klasifikasi aturan Req_Invoice (Copy GRN = Y, Asli GRN = N)
	resolveGRN := func(req string) string {
		clean := strings.ToLower(strings.TrimSpace(req))
		if clean == "t" || clean == "tidak" || clean == "ytfc" {
			return "N"
		}
		return "Y" // default YA / Y / E-Faktur / Null / 0
	}

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
			continue // Skip Header Baris 1
		}

		if len(record) < 15 {
			continue
		}

		noDN := strings.TrimSpace(record[1])
		namaPengirim := strings.TrimSpace(record[2])
		kotaPengirim := strings.TrimSpace(record[4])
		namaPenerima := strings.TrimSpace(record[6])
		alamatPenerima := strings.TrimSpace(record[8])
		kotaTujuan := strings.TrimSpace(record[11])
		koli := parseInt(record[16])
		berat := parseFloat(record[17])
		biayaPacking := parseFloat(record[23])
		nilaiAsuransi := parseFloat(record[25])
		service := strings.TrimSpace(record[29])

		var harga float64 = 0
		var tglKirim, reqInvoice string

		if (tipe == "2" || tipe == "3") && len(record) >= 33 {
			// Template Dalam Kota / Dedicated (33 kolom: indeks 30 adalah Harga)
			harga = parseFloat(record[30])
			tglKirim = strings.TrimSpace(record[31])
			reqInvoice = strings.TrimSpace(record[32])
		} else if len(record) >= 32 {
			// Template Luar Kota (32 kolom)
			tglKirim = strings.TrimSpace(record[30])
			reqInvoice = strings.TrimSpace(record[31])
		}

		rows = append(rows, BTTUploadV2Row{
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
			Service:        service,
			Harga:          harga,
			Tanggal:        tglKirim,
			ReqInvoice:     reqInvoice,
			KategoriGRN:    resolveGRN(reqInvoice),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"total_rows": len(rows),
		"data":       rows,
	})
}

// 3. SIMPAN BATCH KE TABEL STAGING BTT V2
// POST /marketing/btt-upload-v2/save-batch
func SaveBTTV2CSVBatch(c *gin.Context) {
	type SaveReq struct {
		CustomerID string           `json:"customer_id" binding:"required"`
		Tipe       string           `json:"tipe" binding:"required"` // 1, 2, 3
		Rows       []BTTUploadV2Row `json:"rows" binding:"required"`
	}

	var req SaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format payload tidak valid"})
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
	} else if req.Tipe == "3" {
		tipeLabel = "DEDICATED"
	}

	batchID := fmt.Sprintf("BTT-V2-%d", time.Now().UnixNano())
	tx := db.DB.Begin()

	insertSQL := `
		INSERT INTO public.opr_t_btt_upload_v2 
			(batch_id, customer_id, tipe_pengiriman, kode_group_dn, no_dn, nama_pengirim, kota_pengirim, nama_penerima, alamat_penerima, kota_kabupaten, jumlah_koli, berat, biaya_packing, nilai_barang_asuransi, service, harga, req_invoice, kategori_grn, status_proses, user_upload, created_at)
		VALUES 
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'PENDING', ?, CURRENT_TIMESTAMP)
	`

	for _, r := range req.Rows {
		if err := tx.Exec(insertSQL, batchID, req.CustomerID, tipeLabel, r.KodeGroupDN, r.NoDN, r.NamaPengirim, r.KotaPengirim, r.NamaPenerima, r.AlamatPenerima, r.KotaTujuan, r.Koli, r.Berat, r.BiayaPacking, r.NilaiAsuransi, r.Service, r.Harga, r.ReqInvoice, r.KategoriGRN, username).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan baris BTT V2: " + err.Error()})
			return
		}
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  fmt.Sprintf("Berhasil mengimpor %d baris BTT!", len(req.Rows)),
		"batch_id": batchID,
	})
}
