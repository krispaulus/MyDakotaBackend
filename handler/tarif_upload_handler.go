package handler

import (
	"encoding/csv"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Gunakan func(c *gin.Context) langsung:
func UploadTarifCSVHandler(c *gin.Context) {
	database := getTarifCustomerDB(c)

	custID := strings.TrimSpace(c.PostForm("customer_id"))
	jenisTarif := strings.ToLower(strings.TrimSpace(c.PostForm("jenis_tarif")))

	fileHeader, err := c.FormFile("file_csv")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV wajib diunggah!"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membuka file CSV"})
		return
	}
	defer file.Close()

	var agenIDAsal string
	err = database.Table("public.mkt_m_customer").
		Select("COALESCE(cust_agenid, '1')").
		Where("cust_id = ?", custID).
		Scan(&agenIDAsal).Error

	if err != nil || agenIDAsal == "" {
		agenIDAsal = "1"
	}

	reader := csv.NewReader(file)
	reader.Comma = ','
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format file CSV kosong atau tidak valid"})
		return
	}

	if len(header) == 1 && strings.Contains(header[0], ";") {
		file.Seek(0, 0)
		reader = csv.NewReader(file)
		reader.Comma = ';'
		reader.Read()
	}

	tx := database.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var insertedCount int = 0
	var rejectedRows []string

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(row) < 5 {
			continue
		}

		rowCustID := strings.TrimSpace(row[0])
		if rowCustID != custID {
			rejectedRows = append(rejectedRows, strings.Join(row, ",")+" (Alasan: Customer ID tidak cocok)")
			continue
		}

		if jenisTarif == "charter" {
			jnsKend := strings.TrimSpace(row[1])
			prop := strings.TrimSpace(row[2])
			kota := strings.TrimSpace(row[3])
			harga, _ := strconv.ParseFloat(strings.TrimSpace(row[4]), 64)

			var hargaDG float64
			if len(row) > 5 {
				hargaDG, _ = strconv.ParseFloat(strings.TrimSpace(row[5]), 64)
			}
			var kgMax, hargaOverKg float64
			if len(row) > 7 {
				kgMax, _ = strconv.ParseFloat(strings.TrimSpace(row[6]), 64)
				hargaOverKg, _ = strconv.ParseFloat(strings.TrimSpace(row[7]), 64)
			}
			estimasi := "1-3"
			if len(row) > 10 {
				estimasi = strings.TrimSpace(row[10])
			}

			tx.Exec(`
				DELETE FROM public.mkt_m_eharga_pelanggan_carter 
				WHERE customerid = ? AND jns_kend = ? AND UPPER(tujuan_kabupaten) = UPPER(?)
			`, custID, jnsKend, kota)

			errInsert := tx.Exec(`
				INSERT INTO public.mkt_m_eharga_pelanggan_carter (
					agenid_asal, customerid, jns_kend, tujuan_kecamatan, tujuan_kabupaten, tujuan_propinsi,
					harga, harga_dg, kg_max, hargaoverkg_max, estimasihari
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, agenIDAsal, custID, jnsKend, kota, kota, prop, harga, hargaDG, kgMax, hargaOverKg, estimasi).Error

			if errInsert == nil {
				insertedCount++
			}
		} else {
			servID := strings.TrimSpace(row[1])
			jnharga := strings.TrimSpace(row[2])
			prop := strings.TrimSpace(row[3])
			kota := strings.TrimSpace(row[4])
			kec := strings.TrimSpace(row[5])
			minKg, _ := strconv.ParseFloat(strings.TrimSpace(row[6]), 64)
			hargaMin, _ := strconv.ParseFloat(strings.TrimSpace(row[8]), 64)
			hargaNext, _ := strconv.ParseFloat(strings.TrimSpace(row[9]), 64)

			tx.Exec(`
				DELETE FROM public.mkt_m_eharga_pelanggan
				WHERE customerid = ? AND servid = ? AND jnharga = ? AND UPPER(tujuan_kabupaten) = UPPER(?) AND UPPER(tujuan_kecamatan) = UPPER(?)
			`, custID, servID, jnharga, kota, kec)

			errInsert := tx.Exec(`
				INSERT INTO public.mkt_m_eharga_pelanggan (
					agenid_asal, customerid, servid, jnharga, tujuan_kecamatan, tujuan_kabupaten, tujuan_propinsi,
					minimalkg, hargapokok, hargakgselanjutnya, estimasihari
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '3-5')
			`, agenIDAsal, custID, servID, jnharga, kec, kota, prop, minKg, hargaMin, hargaNext).Error

			if errInsert == nil {
				insertedCount++
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan perubahan ke database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "success",
		"inserted_count": insertedCount,
		"rejected_count": len(rejectedRows),
		"rejected_rows":  rejectedRows,
		"message":        "Proses upload & sinkronisasi tarif berhasil!",
	})
}
