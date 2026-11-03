package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func UploadKimiaFarmaCSVHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	custID := strings.TrimSpace(c.PostForm("cust_id"))
	custName := strings.TrimSpace(c.PostForm("cust_name"))
	delimiterStr := strings.TrimSpace(c.PostForm("delimiter"))
	tipe := strings.TrimSpace(c.PostForm("tipe_pengiriman"))

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV wajib diunggah"})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	if delimiterStr == "," {
		reader.Comma = ','
	} else {
		reader.Comma = ';'
	}
	reader.LazyQuotes = true

	// Baca Header (Baris 1)
	header, err := reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV kosong atau header tidak terbaca"})
		return
	}

	colMap := make(map[string]int)
	for idx, h := range header {
		cleanHeader := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(h), " ", ""))
		colMap[cleanHeader] = idx
	}

	tx := database.Begin()
	totalSuccess := 0

	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(rec) == 0 {
			continue
		}

		getVal := func(keys ...string) string {
			for _, k := range keys {
				upperKey := strings.ToUpper(strings.ReplaceAll(k, " ", ""))
				if idx, ok := colMap[upperKey]; ok && idx < len(rec) {
					return strings.TrimSpace(rec[idx])
				}
			}
			return ""
		}

		toFloat := func(val string) float64 {
			clean := strings.ReplaceAll(strings.TrimSpace(val), ",", ".")
			f, _ := strconv.ParseFloat(clean, 64)
			return f
		}

		noDN := getVal("NoDN", "DN#")
		if noDN == "" {
			continue
		}

		// 🎯 Gunakan custName jika nama pengirim di CSV kosong:
		pengirimName := getVal("Pengirim", "Nama_Pengirim")
		if pengirimName == "" {
			pengirimName = custName
		}

		// Mapping ke kolom econote
		econoteRow := map[string]interface{}{
			"bttt_id":              noDN,
			"bttt_asalcustid":      custID,
			"bttt_asalname":        pengirimName,
			"bttt_asalalamat":      getVal("AlamatPengirim", "Alamat_Pengirim"),
			"bttt_asalkota":        getVal("KotaPengirim", "Kota_Pengirim"),
			"bttt_asaltelp":        getVal("TelpPengirim", "Telp_Pengirim"),
			"bttt_tujuannama":      getVal("Penerima"),
			"bttt_up":              getVal("UpPenerima", "Up_Penerima"),
			"bttt_tujuanalamat":    getVal("AlamatPenerima", "Alamat_Penerima"),
			"bttt_tujuantelp":      getVal("TelpPenerima", "Telp_Penerima"),
			"bttt_tujuanpulau":     getVal("Propinsi"),
			"bttt_tujuankota":      getVal("KotaPenerima", "Kota_Kabupaten"),
			"bttt_tujuankecamatan": getVal("Kecamatan"),
			"bttt_tujuankelurahan": getVal("Kelurahan"),
			"bttt_tujuankodepos":   getVal("Kodepos"),
			"bttt_namabarang":      getVal("IsiKiriman", "Isi_Kiriman"),
			"bttt_jmlunit":         toFloat(getVal("Koli", "Jumlah_Koli")),
			"bttt_berat":           toFloat(getVal("Berat")),
			"bttt_beratvol":        toFloat(getVal("BeratVol", "Berat_Volume")),
			"bttt_ukuran":          toFloat(getVal("Volume")),
			"bttt_biayapenerus":    toFloat(getVal("BiayaPenerus", "Biaya_Penerus")),
			"bttt_harga":           toFloat(getVal("Harga")),
			"bttt_nosuratjalan":    getVal("NoSKB", "No SKB"),
			"bttt_ket":             getVal("Keterangan"),
			"bttt_aktifyn":         "Y",
			"bttt_updatetime":      time.Now(),
		}

		if err := tx.Table("public.mkt_t_econote").
			Where("bttt_id = ?", noDN).
			Assign(econoteRow).
			FirstOrCreate(&map[string]interface{}{}).Error; err != nil {
			_ = tx.Table("public.mkt_t_econote").Create(&econoteRow)
		}

		totalSuccess++
	}

	tx.Commit()

	// 🎯 Variabel custName juga ikut ditampilkan pada balasan response:
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Sukses mengimpor %d pesanan %s (%s).", totalSuccess, custName, tipe),
	})
}
