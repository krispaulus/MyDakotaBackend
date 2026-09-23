package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

type CustDOBarcodeItem struct {
	CustomerID string    `json:"customer_id"`
	DNODSJ     string    `json:"dn_od_sj"`
	BarcodeID  string    `json:"barcode_id"`
	Handover   time.Time `json:"handover"`
	Penerima   string    `json:"penerima"`
	Alamat     string    `json:"alamat"`
	Jml        int       `json:"jml"`
	InvoiceYN  string    `json:"invoice_yn"`
	DOAsliYN   string    `json:"do_asli_yn"`
}

// 1. MONITORING DAFTAR DN/OJ BELUM MASUK LSPB
// GET /marketing/dn-upload/outstanding
func GetDNBelumMasukLSPB(c *gin.Context) {

	EnsureDNUploadTables()

	var currentDB, currentUser string
	db.DB.Raw("SELECT current_database(), current_user").Row().Scan(&currentDB, &currentUser)
	fmt.Printf("\n>>> BACKEND TERKONEKSI KE DB: [%s] | USER: [%s] <<<\n\n", currentDB, currentUser)

	customerID := strings.TrimSpace(c.Query("customer_id"))
	tglAwal := strings.TrimSpace(c.Query("tgl_awal"))
	tglAkhir := strings.TrimSpace(c.Query("tgl_akhir"))
	chkTgl := c.Query("chk_tgl") == "true"

	query := db.DB.Table("public.mkt_t_custdobarcode b").
		Select(`
			b.customerid,
			b.dn_od_sj,
			COALESCE(MAX(b.barcodeid), '-') AS barcode_id,
			MAX(b.handover) AS handover,
			COUNT(b.dn_od_sj) AS jml,
			b.penerima,
			b.alamat,
			COALESCE(b.invoiceyn, 'NO') AS invoice_yn,
			COALESCE(b.doasliyn, 'NO') AS do_asli_yn
		`).
		Where("NOT EXISTS (SELECT 1 FROM public.opr_t_transportlspb lspb WHERE lspb.dn_od_sj = b.dn_od_sj)")

	if customerID != "" {
		query = query.Where("b.customerid = ?", customerID)
	}

	query = query.Group("b.penerima, b.alamat, b.dn_od_sj, b.customerid, b.invoiceyn, b.doasliyn")

	if chkTgl && tglAwal != "" && tglAkhir != "" {
		query = query.Having("MAX(b.handover) BETWEEN ? AND ?", tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}

	var results []CustDOBarcodeItem
	if err := query.Order("b.penerima, b.alamat").Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 2. PARSE CSV UPLOAD DN / OJ
// POST /marketing/dn-upload/upload
func UploadDNCSV(c *gin.Context) {
	customerID := strings.TrimSpace(c.PostForm("customer_id"))
	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Pilih Customer terlebih dahulu"})
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
	reader.Comma = ';' // Template standar DN menggunakan titik koma (;)
	reader.LazyQuotes = true

	tx := db.DB.Begin()
	insertSQL := `
		INSERT INTO public.mkt_t_custdobarcode 
			(customerid, dn_od_sj, barcodeid, handover, penerima, alamat, selectedyn, invoiceyn, doasliyn, created_at)
		VALUES 
			(?, ?, ?, ?::timestamp, ?, ?, 'N', ?, ?, CURRENT_TIMESTAMP)
	`

	rowIndex := 0
	totalInserted := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 5 {
			continue
		}
		rowIndex++
		if rowIndex == 1 {
			// Lewati baris header jika kolom pertama mengandung tulisan "DN"
			if strings.Contains(strings.ToUpper(record[0]), "DN") {
				continue
			}
		}

		dnNo := strings.TrimSpace(record[0])
		outID := strings.TrimSpace(record[1])
		tglHandoverStr := strings.TrimSpace(record[2])
		consignee := strings.TrimSpace(record[3])
		address := strings.TrimSpace(record[4])

		markTP := ""
		if len(record) > 5 {
			markTP = strings.ToUpper(strings.TrimSpace(record[5]))
		}

		// Aturan InvoiceYN & DOAsliYN
		invoiceYN := "NO"
		if strings.Contains(markTP, "YES") || strings.Contains(markTP, "Y") || strings.Contains(markTP, "E-INVOICE") {
			invoiceYN = "YES"
		}

		doAsliYN := "NO"
		if !strings.Contains(markTP, "NO") && !strings.Contains(markTP, "N") && markTP != "" {
			doAsliYN = "YES"
		}

		// Konversi format tanggal MM/DD/YYYY ke YYYY-MM-DD
		parsedTgl, err := time.Parse("01/02/2006", tglHandoverStr)
		tglFix := time.Now().Format("2006-01-02 15:04:05")
		if err == nil {
			tglFix = parsedTgl.Format("2006-01-02 15:04:05")
		}

		if err := tx.Exec(insertSQL, customerID, dnNo, outID, tglFix, consignee, address, invoiceYN, doAsliYN).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan baris DN: " + err.Error()})
			return
		}
		totalInserted++
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":         "success",
		"message":        fmt.Sprintf("Berhasil mengimpor %d baris DN / OJ!", totalInserted),
		"total_inserted": totalInserted,
	})
}

func EnsureDNUploadTables() {
	ddl := `
	CREATE TABLE IF NOT EXISTS public.mkt_t_custdobarcode (
		id          BIGSERIAL PRIMARY KEY,
		customerid  TEXT,
		dn_od_sj    TEXT,
		barcodeid   TEXT,
		handover    TIMESTAMP,
		penerima    TEXT,
		alamat      TEXT,
		selectedyn  VARCHAR(5) DEFAULT 'N',
		invoiceyn   VARCHAR(20) DEFAULT 'NO',
		doasliyn    VARCHAR(20) DEFAULT 'NO',
		created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_mkt_t_custdobarcode_dn ON public.mkt_t_custdobarcode (dn_od_sj);
	CREATE INDEX IF NOT EXISTS idx_mkt_t_custdobarcode_cust ON public.mkt_t_custdobarcode (customerid);
	CREATE INDEX IF NOT EXISTS idx_mkt_t_custdobarcode_handover ON public.mkt_t_custdobarcode (handover);

	CREATE TABLE IF NOT EXISTS public.opr_t_transportlspb (
		id                    BIGSERIAL PRIMARY KEY,
		dn_od_sj              TEXT,
		nomorbtt              TEXT,
		spb_id                TEXT,
		customerid            TEXT,
		tanggal_dokumenkembali TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_transportlspb_dn ON public.opr_t_transportlspb (dn_od_sj);
	`
	if err := db.DB.Exec(ddl).Error; err != nil {
		fmt.Printf(">>> GAGAL MEMBUAT TABEL OTOMATIS: %v <<<\n", err)
	} else {
		fmt.Println(">>> [BERHASIL] TABEL mkt_t_custdobarcode & opr_t_transportlspb SUDAH AKTIF DI DATABASE! <<<")
	}
}
