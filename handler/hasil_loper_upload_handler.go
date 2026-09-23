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

type HasilLoperCSVRow struct {
	NomorBTT      string `json:"nomor_btt"`
	TanggalTerima string `json:"tanggal_terima"`
	NamaPenerima  string `json:"nama_penerima"`
	StatusKirim   string `json:"status_kirim"`
	Keterangan    string `json:"keterangan"`
}

// 1. PARSE & PREVIEW CSV HASIL LOPERAN SUPIR
// POST /marketing/hasil-loper/parse-csv
func ParseHasilLoperCSV(c *gin.Context) {
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
	reader.LazyQuotes = true

	// Deteksi otomatis pemisah kolom (koma atau titik koma)
	firstLine, err := reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV kosong atau format tidak sesuai"})
		return
	}
	if len(firstLine) == 1 && strings.Contains(firstLine[0], ";") {
		reader.Comma = ';'
	}

	var rows []HasilLoperCSVRow
	rowIndex := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 2 {
			continue
		}
		rowIndex++
		if rowIndex == 1 && strings.Contains(strings.ToUpper(record[0]), "BTT") {
			// Lewati baris header jika ada
			continue
		}

		nomorBTT := strings.TrimSpace(record[0])
		tglTerima := time.Now().Format("2006-01-02")
		if len(record) > 1 && strings.TrimSpace(record[1]) != "" {
			tglTerima = strings.TrimSpace(record[1])
		}

		namaPenerima := "-"
		if len(record) > 2 && strings.TrimSpace(record[2]) != "" {
			namaPenerima = strings.TrimSpace(record[2])
		}

		statusKirim := "DELIVERED"
		if len(record) > 3 && strings.TrimSpace(record[3]) != "" {
			statusKirim = strings.ToUpper(strings.TrimSpace(record[3]))
		}

		keterangan := ""
		if len(record) > 4 {
			keterangan = strings.TrimSpace(record[4])
		}

		rows = append(rows, HasilLoperCSVRow{
			NomorBTT:      nomorBTT,
			TanggalTerima: tglTerima,
			NamaPenerima:  namaPenerima,
			StatusKirim:   statusKirim,
			Keterangan:    keterangan,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"total_rows": len(rows),
		"data":       rows,
	})
}

// 2. SIMPAN BATCH & UPDATE STATUS KE TABEL OPERASIONAL
// POST /marketing/hasil-loper/save-batch
func SaveHasilLoperBatch(c *gin.Context) {
	type SaveReq struct {
		Rows []HasilLoperCSVRow `json:"rows" binding:"required"`
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

	batchID := fmt.Sprintf("POD-%d", time.Now().UnixNano())
	tx := db.DB.Begin()

	insertStaging := `
		INSERT INTO public.opr_t_hasilloper_upload 
			(batch_upload_id, nomor_btt, tanggal_terima, nama_penerima, status_kirim, keterangan, user_upload, created_at)
		VALUES 
			(?, ?, ?::timestamp, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	for _, r := range req.Rows {
		// Validasi format tanggal agar aman saat casting timestamp
		tglVal := r.TanggalTerima
		if parsed, err := time.Parse("01/02/2006", tglVal); err == nil {
			tglVal = parsed.Format("2006-01-02 15:04:05")
		} else if parsed, err := time.Parse("2006-01-02", tglVal); err == nil {
			tglVal = parsed.Format("2006-01-02 15:04:05")
		}

		if err := tx.Exec(insertStaging, batchID, r.NomorBTT, tglVal, r.NamaPenerima, r.StatusKirim, r.Keterangan, username).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan baris: " + err.Error()})
			return
		}

		// Update status di tabel operasional jika diperlukan
		tx.Exec(`
			UPDATE opr_t_hasilloper 
			SET status = ?, penerima = ?, tgl_terima = ?::timestamp, update_id = ?, update_time = CURRENT_TIMESTAMP 
			WHERE no_btt = ?
		`, r.StatusKirim, r.NamaPenerima, tglVal, username, r.NomorBTT)
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  fmt.Sprintf("Berhasil memproses %d data hasil loperan supir", len(req.Rows)),
		"batch_id": batchID,
	})
}
