package handler

import (
	"bytes"
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

// POST /api/master/tarif/import-csv
// Form-data: file (CSV) + tarif_type (CARTER | CUSTOMER | PAKET | PAKET_EKONOMIS | PAKET_UMUM | UNIT | JENIS_KENDARAAN)
func ImportMasterTarifCSVHandler(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = "C"
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database gagal"})
		return
	}

	userID, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", userID)
	if userStr == "" || userStr == "<nil>" {
		userStr = "SYSTEM"
	}

	tarifType := strings.ToUpper(strings.TrimSpace(c.PostForm("tarif_type")))
	if tarifType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter tarif_type wajib diisi"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV wajib dipilih"})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membaca file upload"})
		return
	}
	defer f.Close()

	rawBytes, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membaca isi file CSV"})
		return
	}

	// Bersihkan UTF-8 BOM jika export dari Excel
	rawBytes = bytes.TrimPrefix(rawBytes, []byte{0xEF, 0xBB, 0xBF})

	// Deteksi delimiter otomatis (; atau ,)
	firstLine := string(bytes.SplitN(rawBytes, []byte("\n"), 2)[0])
	delimiter := ','
	if strings.Count(firstLine, ";") > strings.Count(firstLine, ",") {
		delimiter = ';'
	}

	reader := csv.NewReader(bytes.NewReader(rawBytes))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "File CSV kosong atau format tidak valid"})
		return
	}

	// Map nama header -> index kolom (lowercase, tanpa spasi/simbol)
	colIdx := make(map[string]int)
	for i, h := range records[0] {
		key := strings.ToLower(strings.TrimSpace(h))
		colIdx[key] = i
	}

	getVal := func(row []string, keys ...string) string {
		for _, k := range keys {
			if idx, found := colIdx[strings.ToLower(k)]; found && idx < len(row) {
				val := strings.TrimSpace(row[idx])
				if val != "" && val != "-" {
					return val
				}
			}
		}
		return ""
	}

	toFloat := func(s string) float64 {
		if s == "" {
			return 0
		}
		clean := strings.ReplaceAll(s, "Rp", "")
		clean = strings.ReplaceAll(clean, "rp", "")
		clean = strings.TrimSpace(clean)
		// Jika format Indonesia (misal 1.250.000,50)
		if strings.Contains(clean, ".") && strings.Contains(clean, ",") {
			clean = strings.ReplaceAll(clean, ".", "")
			clean = strings.ReplaceAll(clean, ",", ".")
		} else if strings.Count(clean, ".") > 1 {
			clean = strings.ReplaceAll(clean, ".", "")
		} else if strings.Contains(clean, ",") {
			clean = strings.ReplaceAll(clean, ",", ".")
		}
		v, _ := strconv.ParseFloat(clean, 64)
		return v
	}

	tx := database.Begin()
	inserted, updated, skipped := 0, 0, 0
	var rowErrors []string

	for lineNum, row := range records[1:] {
		if len(row) == 0 || strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}

		switch tarifType {

		// 1. TARIF CARTER (mkt_m_eharga_charter atau opr_m_kendjenis)
		case "CARTER":
			agenAsal := getVal(row, "agenid_asal", "agen_id", "kode_agen")
			kendJenis := strings.ToUpper(getVal(row, "kend_jenis", "jenis_kendaraan", "kode_kendaraan"))
			propinsi := strings.ToUpper(getVal(row, "tujuan_propinsi", "propinsi", "provinsi"))
			kabupaten := strings.ToUpper(getVal(row, "tujuan_kabupaten", "kabupaten", "kota_tujuan", "kota"))
			harga := toFloat(getVal(row, "hargapokok", "harga_pokok", "harga", "tarif"))
			ket := getVal(row, "keterangan", "ket")

			if agenAsal == "" || kendJenis == "" || kabupaten == "" {
				skipped++
				rowErrors = append(rowErrors, fmt.Sprintf("Baris %d: agenid_asal, kend_jenis, dan tujuan_kabupaten wajib diisi", lineNum+2))
				continue
			}

			var existID int64
			tx.Table("public.mkt_m_eharga_charter").
				Select("id").
				Where("TRIM(agenid_asal) = ? AND UPPER(TRIM(kend_jenis)) = ? AND UPPER(TRIM(tujuan_kabupaten)) = ?",
					agenAsal, kendJenis, kabupaten).
				Limit(1).
				Scan(&existID)

			if existID > 0 {
				tx.Table("public.mkt_m_eharga_charter").Where("id = ?", existID).Updates(map[string]interface{}{
					"tujuan_propinsi": propinsi,
					"hargapokok":      harga,
					"keterangan":      ket,
					"update_id":       userStr,
					"update_time":     time.Now(),
				})
				updated++
			} else {
				tx.Table("public.mkt_m_eharga_charter").Create(map[string]interface{}{
					"agenid_asal":      agenAsal,
					"kend_jenis":       kendJenis,
					"tujuan_propinsi":  propinsi,
					"tujuan_kabupaten": kabupaten,
					"hargapokok":       harga,
					"keterangan":       ket,
					"update_id":        userStr,
					"update_time":      time.Now(),
				})
				inserted++
			}

		// 2. TARIF CUSTOMER (mkt_m_eharga_customer & mkt_m_eharga_pelanggan_carter jika ada jns_kend)
		case "CUSTOMER":
			custID := getVal(row, "cust_id", "customerid", "kode_customer")
			kabupaten := strings.ToUpper(getVal(row, "tujuan_kabupaten", "kabupaten", "kota"))
			kecamatan := strings.ToUpper(getVal(row, "tujuan_kecamatan", "kecamatan"))
			kelurahan := strings.ToUpper(getVal(row, "tujuan_kelurahan", "kelurahan"))
			kodepos := getVal(row, "tujuan_kodepos", "kodepos")
			minCharge := toFloat(getVal(row, "min_charge", "minimalkg", "min_kg"))
			tarifKg := toFloat(getVal(row, "tarif_kg", "hargapokok", "harga_kg", "harga"))
			jnsKend := strings.ToUpper(getVal(row, "jns_kend", "jenis_kendaraan"))
			hargaDG := toFloat(getVal(row, "harga_dg"))

			if custID == "" || (kabupaten == "" && kecamatan == "" && kodepos == "") {
				skipped++
				rowErrors = append(rowErrors, fmt.Sprintf("Baris %d: cust_id dan wilayah tujuan wajib diisi", lineNum+2))
				continue
			}

			// Jika baris CSV adalah tarif carter customer
			if jnsKend != "" {
				var existCarterID int64
				tx.Table("public.mkt_m_eharga_pelanggan_carter").
					Select("id").
					Where("TRIM(customerid) = ? AND UPPER(TRIM(tujuan_kabupaten)) = ? AND UPPER(TRIM(jns_kend)) = ?",
						custID, kabupaten, jnsKend).
					Limit(1).
					Scan(&existCarterID)

				if existCarterID > 0 {
					tx.Table("public.mkt_m_eharga_pelanggan_carter").Where("id = ?", existCarterID).Updates(map[string]interface{}{
						"harga":    tarifKg,
						"harga_dg": hargaDG,
					})
					updated++
				} else {
					tx.Table("public.mkt_m_eharga_pelanggan_carter").Create(map[string]interface{}{
						"customerid":       custID,
						"tujuan_kabupaten": kabupaten,
						"tujuan_kecamatan": kecamatan,
						"jns_kend":         jnsKend,
						"harga":            tarifKg,
						"harga_dg":         hargaDG,
					})
					inserted++
				}
			} else {
				var existCustTarifID int64
				tx.Table("public.mkt_m_eharga_customer").
					Select("id").
					Where("TRIM(cust_id) = ? AND UPPER(TRIM(COALESCE(tujuan_kabupaten,''))) = ? AND UPPER(TRIM(COALESCE(tujuan_kecamatan,''))) = ? AND TRIM(COALESCE(tujuan_kodepos,'')) = ?",
						custID, kabupaten, kecamatan, kodepos).
					Limit(1).
					Scan(&existCustTarifID)

				if existCustTarifID > 0 {
					tx.Table("public.mkt_m_eharga_customer").Where("id = ?", existCustTarifID).Updates(map[string]interface{}{
						"tujuan_kelurahan": kelurahan,
						"min_charge":       minCharge,
						"tarif_kg":         tarifKg,
					})
					updated++
				} else {
					tx.Table("public.mkt_m_eharga_customer").Create(map[string]interface{}{
						"cust_id":          custID,
						"tujuan_kabupaten": kabupaten,
						"tujuan_kecamatan": kecamatan,
						"tujuan_kelurahan": kelurahan,
						"tujuan_kodepos":   kodepos,
						"min_charge":       minCharge,
						"tarif_kg":         tarifKg,
					})
					inserted++
				}
			}

		// 3, 4, 5. TARIF PAKET (servid='1'), TARIF PAKET EKONOMIS (servid='2'), TARIF PAKET UMUM
		case "PAKET", "PAKET_EKONOMIS", "PAKET_UMUM":
			agenAsal := getVal(row, "agenid_asal", "agen_asal", "asal")
			tujuanKec := strings.ToUpper(getVal(row, "tujuan_kecamatan", "kecamatan"))
			tujuanKab := strings.ToUpper(getVal(row, "tujuan_kabupaten", "tujuan_kota", "kota"))
			tujuanProp := strings.ToUpper(getVal(row, "tujuan_propinsi", "propinsi"))
			hargaPokok := toFloat(getVal(row, "hargapokok", "harga_pokok", "harga"))
			hargaNext := toFloat(getVal(row, "hargakgselanjutnya", "harga_next", "kg_next"))
			minKg := toFloat(getVal(row, "minimalkg", "minimal_kg", "kg_min"))
			biayaTambahan := toFloat(getVal(row, "biayatambahan", "biaya_penerus", "penerus"))
			estimasi := int(toFloat(getVal(row, "estimasihari", "estimasi_hari", "lt")))
			servID := getVal(row, "servid")
			if servID == "" {
				if tarifType == "PAKET_EKONOMIS" {
					servID = "2"
				} else {
					servID = "1"
				}
			}

			if agenAsal == "" || (tujuanKec == "" && tujuanKab == "") {
				skipped++
				rowErrors = append(rowErrors, fmt.Sprintf("Baris %d: agenid_asal dan tujuan wajib diisi", lineNum+2))
				continue
			}

			tableTarget := "public.mkt_m_eharga"
			if tarifType == "PAKET_UMUM" && database.Migrator().HasTable("public.mkt_m_eharga_umum") {
				tableTarget = "public.mkt_m_eharga_umum"
			}

			var existCount int64
			tx.Table(tableTarget).
				Where("TRIM(agenid_asal) = ? AND UPPER(TRIM(COALESCE(tujuan_kecamatan,''))) = ? AND TRIM(COALESCE(servid,'1')) = ?",
					agenAsal, tujuanKec, servID).
				Count(&existCount)

			dataMap := map[string]interface{}{
				"agenid_asal":        agenAsal,
				"tujuan_propinsi":    tujuanProp,
				"tujuan_kabupaten":   tujuanKab,
				"tujuan_kecamatan":   tujuanKec,
				"servid":             servID,
				"hargapokok":         hargaPokok,
				"hargakgselanjutnya": hargaNext,
				"minimalkg":          minKg,
				"biayatambahan":      biayaTambahan,
				"estimasihari":       estimasi,
			}

			if existCount > 0 {
				tx.Table(tableTarget).
					Where("TRIM(agenid_asal) = ? AND UPPER(TRIM(COALESCE(tujuan_kecamatan,''))) = ? AND TRIM(COALESCE(servid,'1')) = ?",
						agenAsal, tujuanKec, servID).
					Updates(dataMap)
				updated++
			} else {
				tx.Table(tableTarget).Create(dataMap)
				inserted++
			}

		// 6. TARIF UNIT (mkt_m_eharga_unit)
		case "UNIT":
			agenAsal := getVal(row, "agenid_asal", "agen_asal")
			tujuanKab := strings.ToUpper(getVal(row, "tujuan_kabupaten", "kota_tujuan", "kota"))
			tujuanKec := strings.ToUpper(getVal(row, "tujuan_kecamatan", "kecamatan"))
			jenisUnit := strings.ToUpper(getVal(row, "jenis_unit", "nama_unit", "unit"))
			hargaUnit := toFloat(getVal(row, "harga", "hargapokok", "harga_unit"))
			ket := getVal(row, "keterangan", "ket")

			if agenAsal == "" || jenisUnit == "" {
				skipped++
				rowErrors = append(rowErrors, fmt.Sprintf("Baris %d: agenid_asal dan jenis_unit wajib diisi", lineNum+2))
				continue
			}

			var existUnitID int64
			tx.Table("public.mkt_m_eharga_unit").
				Select("id").
				Where("TRIM(agenid_asal) = ? AND UPPER(TRIM(COALESCE(tujuan_kabupaten,''))) = ? AND UPPER(TRIM(COALESCE(jenis_unit,''))) = ?",
					agenAsal, tujuanKab, jenisUnit).
				Limit(1).
				Scan(&existUnitID)

			if existUnitID > 0 {
				tx.Table("public.mkt_m_eharga_unit").Where("id = ?", existUnitID).Updates(map[string]interface{}{
					"tujuan_kecamatan": tujuanKec,
					"harga":            hargaUnit,
					"keterangan":       ket,
				})
				updated++
			} else {
				tx.Table("public.mkt_m_eharga_unit").Create(map[string]interface{}{
					"agenid_asal":      agenAsal,
					"tujuan_kabupaten": tujuanKab,
					"tujuan_kecamatan": tujuanKec,
					"jenis_unit":       jenisUnit,
					"harga":            hargaUnit,
					"keterangan":       ket,
				})
				inserted++
			}

		// 7. JENIS KENDARAAN CARTER (opr_m_kendjenis)
		case "JENIS_KENDARAAN":
			jenisID := strings.ToUpper(getVal(row, "jenis_id", "kode_kendaraan", "id"))
			jenisMerk := strings.ToUpper(getVal(row, "jenis_merk", "merk", "nama_kendaraan"))
			jenisModel := strings.ToUpper(getVal(row, "jenis_model", "model", "tipe"))
			hargaSewa := toFloat(getVal(row, "jenis_hargasewa", "harga_sewa", "harga"))
			kapasitas := toFloat(getVal(row, "jenis_kapasitas", "kapasitas_kg", "kapasitas"))

			if jenisID == "" || jenisMerk == "" {
				skipped++
				rowErrors = append(rowErrors, fmt.Sprintf("Baris %d: jenis_id dan jenis_merk wajib diisi", lineNum+2))
				continue
			}

			var existKend int64
			tx.Table("public.opr_m_kendjenis").
				Where("UPPER(TRIM(jenis_id)) = ?", jenisID).
				Count(&existKend)

			if existKend > 0 {
				tx.Table("public.opr_m_kendjenis").Where("UPPER(TRIM(jenis_id)) = ?", jenisID).Updates(map[string]interface{}{
					"jenis_merk":      jenisMerk,
					"jenis_model":     jenisModel,
					"jenis_hargasewa": hargaSewa,
					"jenis_aktifyn":   "Y",
				})
				updated++
			} else {
				tx.Table("public.opr_m_kendjenis").Create(map[string]interface{}{
					"jenis_id":        jenisID,
					"jenis_merk":      jenisMerk,
					"jenis_model":     jenisModel,
					"jenis_hargasewa": hargaSewa,
					"jenis_aktifyn":   "Y",
				})
				_ = kapasitas
				inserted++
			}

		default:
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Tipe tarif tidak dikenali: " + tarifType})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan transaksi CSV: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  fmt.Sprintf("Import CSV %s selesai! Baru: %d, Diperbarui: %d, Dilewati: %d", tarifType, inserted, updated, skipped),
		"inserted": inserted,
		"updated":  updated,
		"skipped":  skipped,
		"errors":   rowErrors,
	})
}
