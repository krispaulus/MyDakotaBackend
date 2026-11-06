package handler

import (
	"dakotagroup/business-insight-be/db"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func UploadKimiaFarmaCSVHandler(c *gin.Context) {
	// 1. Pastikan koneksi database SAMA PERSIS dengan database Marketing BTT
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = "C"
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tenant operasional tidak terhubung"})
		return
	}

	custID := strings.TrimSpace(c.PostForm("cust_id"))
	if custID == "" {
		custID = "0010000073"
	}
	custName := strings.TrimSpace(c.PostForm("cust_name"))
	if custName == "" {
		custName = "PT. KIMIA FARMA TBK - NDC"
	}

	delimiterStr := strings.TrimSpace(c.PostForm("delimiter"))
	tipe := strings.TrimSpace(c.PostForm("tipe_pengiriman"))

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV wajib diunggah"})
		return
	}
	defer file.Close()

	// Delimiter
	chosenDelimiter := ';'
	if delimiterStr == "," {
		chosenDelimiter = ','
	}

	reader := csv.NewReader(file)
	reader.Comma = chosenDelimiter
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV kosong atau format header tidak valid"})
		return
	}

	colMap := make(map[string]int)
	for idx, h := range header {
		cleanHeader := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(h), " ", ""))
		colMap[cleanHeader] = idx
	}

	tx := database.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	totalSuccess := 0
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)

	// 🌟 Tampung nomor resi yang berhasil digenerate untuk dicetak oleh frontend
	var createdBttList []string

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
			clean := strings.TrimSpace(val)
			clean = strings.ReplaceAll(clean, ".", "")
			clean = strings.ReplaceAll(clean, ",", ".")
			f, _ := strconv.ParseFloat(clean, 64)
			return f
		}

		noDN := getVal("NoDN", "DN#", "NODN")
		if noDN == "" {
			continue
		}

		pengirimName := getVal("Pengirim", "Nama_Pengirim")
		if pengirimName == "" {
			pengirimName = custName
		}

		// Parsing Tanggal MM/DD/YYYY
		tglStr := getVal("Tanggal", "Tgl")
		tglTrans := now
		if t, errParse := time.Parse("01/02/2006", tglStr); errParse == nil {
			tglTrans = t
		} else if t, errParse := time.Parse("02/01/2006", tglStr); errParse == nil {
			tglTrans = t
		} else if t, errParse := time.Parse("2006-01-02", tglStr); errParse == nil {
			tglTrans = t
		}

		tujuanAlamat := getVal("AlamatPenerima", "Alamat_Penerima")
		propinsi := getVal("Propinsi", "Provinsi")
		if propinsi != "" && !strings.Contains(tujuanAlamat, propinsi) {
			tujuanAlamat = tujuanAlamat + ", " + propinsi
		}

		volRaw := strings.TrimSpace(getVal("Volume"))
		volClean := strings.ReplaceAll(volRaw, ",", ".")
		if volClean == "" {
			volClean = "0"
		}

		// 🌟 Generate Nomor Resi Resmi Dakota (contoh: 001112026C000145)
		bttNoResmi := GenerateNomorResiBTTStandard(database, fmt.Sprintf("%v", ptID), "001", tglTrans)

		econoteRow := map[string]interface{}{
			"bttt_id":              bttNoResmi,
			"bttt_nosuratjalan":    noDN,
			"bttt_tanggal":         tglTrans,
			"bttt_spyn":            "Y",
			"bttt_aktifyn":         "Y",
			"bttt_asalagenid":      "001",
			"bttt_asalcustid":      custID,
			"bttt_asalname":        pengirimName,
			"bttt_asalalamat":      getVal("AlamatPengirim", "Alamat_Pengirim"),
			"bttt_asalkota":        getVal("KotaPengirim", "Kota_Pengirim"),
			"bttt_asaltelp":        getVal("TelpPengirim", "Telp_Pengirim"),
			"bttt_tujuannama":      getVal("Penerima"),
			"bttt_up":              getVal("UpPenerima", "Up_Penerima"),
			"bttt_tujuanalamat":    tujuanAlamat,
			"bttt_tujuankota":      getVal("KotaPenerima", "Kota_Kabupaten"),
			"bttt_tujuankecamatan": getVal("Kecamatan"),
			"bttt_tujuankelurahan": getVal("Kelurahan"),
			"bttt_tujuankodepos":   getVal("Kodepos"),
			"bttt_tujuantelp":      getVal("TelpPenerima", "Telp_Penerima"),
			"bttt_namabarang":      getVal("IsiKiriman", "Isi_Kiriman"),
			"bttt_jmlunit":         int(toFloat(getVal("Koli", "Jumlah_Koli"))),
			"bttt_berat":           toFloat(getVal("Berat")),
			"bttt_beratvol":        toFloat(getVal("BeratVol", "Berat_Volume")),
			"bttt_ukuran":          volClean,
			"bttt_biayapenerus":    toFloat(getVal("BiayaPenerus", "Biaya_Penerus")),
			"bttt_harga":           toFloat(getVal("Harga")),
			"bttt_ket":             getVal("Keterangan"),
			"bttt_jenisharga":      "2",
			"bttt_paketyn":         "Y",
			"bttt_servid":          "1",
			"bttt_updateid":        "CSV_KIMIA_FARMA",
		}

		// 1. Simpan BTT Resmi ke mkt_t_econote (Wajib pakai tx)
		tx.Table("public.mkt_t_econote").Where("bttt_nosuratjalan = ? OR bttt_id = ?", noDN, bttNoResmi).Delete(&map[string]interface{}{})

		if errSave := tx.Table("public.mkt_t_econote").Create(&econoteRow).Error; errSave != nil {
			tx.Rollback()
			log.Printf("❌ [Kimia Farma Insert Gagal]: %v", errSave)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("Gagal menyimpan baris DN %s: %v", noDN, errSave),
			})
			return
		}

		// 🌟 2. Sinkronisasi ke opr_t_transportlspb (Tabel Relasi Standar)
		lspbRow := map[string]interface{}{
			"dn_od_sj":         noDN,
			"customerid":       custID,
			"tanggal_handover": tglTrans.Format("2006-01-02"),
			"nomorbtt":         bttNoResmi,
		}
		database.Table("public.opr_t_transportlspb").Where("dn_od_sj = ?", noDN).Delete(&map[string]interface{}{})
		if errLSPB := database.Table("public.opr_t_transportlspb").Create(&lspbRow).Error; errLSPB != nil {
			log.Printf("⚠️ [Warning LSPB Insert]: %v", errLSPB)
		}

		// 🌟 3. Sinkronisasi ke opr_t_transportlspbwithcarinoutanddatetime (TABEL SUMBER VIEW vw_transport_lspb)
		lspbWithCarRow := map[string]interface{}{
			"nodo":       noDN,
			"noskb":      bttNoResmi,
			"kategori":   tipe,
			"created_at": tglTrans,
			"updated_at": now,
		}
		database.Table("public.opr_t_transportlspbwithcarinoutanddatetime").Where("nodo = ?", noDN).Delete(&map[string]interface{}{})
		if errCar := database.Table("public.opr_t_transportlspbwithcarinoutanddatetime").Create(&lspbWithCarRow).Error; errCar != nil {
			log.Printf("⚠️ [Warning LSPB Car Insert]: %v", errCar)
		}

		createdBttList = append(createdBttList, bttNoResmi)
		totalSuccess++
	}

	if errCommit := tx.Commit().Error; errCommit != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal commit database: " + errCommit.Error(),
		})
		return
	}

	firstID := ""
	if len(createdBttList) > 0 {
		firstID = createdBttList[0]
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"message":      fmt.Sprintf("Sukses mengimpor %d pesanan %s (%s).", totalSuccess, custName, tipe),
		"btt_list":     createdBttList,
		"first_btt_id": firstID,
	})
}

// Helper konversi aman dari string CSV ke float64 (menghapus tanda koma):
func parseCsvFloat(val string) float64 {
	clean := strings.TrimSpace(val)
	// Ganti koma dengan titik (misal "8,717" -> "8.717")
	clean = strings.ReplaceAll(clean, ",", ".")
	f, _ := strconv.ParseFloat(clean, 64)
	return f
}
