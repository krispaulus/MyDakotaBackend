package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// Item hasil verifikasi valid & grouping 24 kolom lengkap
type TransportPlanningGroupedRow struct {
	Key             string  `json:"key"`
	Consignee       string  `json:"consignee"`
	Address         string  `json:"address"`
	Propinsi        string  `json:"propinsi"`
	Kota            string  `json:"kota"`
	Kecamatan       string  `json:"kecamatan"`
	Kelurahan       string  `json:"kelurahan"`
	Kodepos         string  `json:"kodepos"`
	ServID          string  `json:"serv_id"`
	LayananID       string  `json:"layanan_id"`
	TotalColly      int     `json:"total_colly"`
	TotalWeight     float64 `json:"total_weight"`
	TotalVolume     float64 `json:"total_volume"`
	TotalPacking    float64 `json:"total_packing"`
	KodeKendaraan   string  `json:"kode_kendaraan"`
	Harga           float64 `json:"harga"`
	IsDG            string  `json:"is_dg"`
	IsMultidrop     string  `json:"is_multidrop"`
	KodePacking     string  `json:"kode_packing"`
	Up              string  `json:"up"`
	Telp            string  `json:"telp"`
	StatusWilayah   string  `json:"status_wilayah"` // "DALAM KOTA" / "LUAR KOTA"
	Keterangan      string  `json:"keterangan"`
	KodeGroupCarter string  `json:"kode_group_carter"`
	DNList          string  `json:"dn_list"`

	// Field Tambahan Khusus PREVIEW 2
	SumberHarga  string  `json:"sumber_harga"` // "TARIF MASTER", "HARGA CSV", "TIDAK ADA"
	HargaFinal   float64 `json:"harga_final"`
	BiayaPacking float64 `json:"biaya_packing"`
	IsDuplicate  bool    `json:"is_duplicate"`
	ExistingBTT  string  `json:"existing_btt"`
}

type TransportPlanningRejectedRow struct {
	No           int     `json:"no"`
	Consignee    string  `json:"consignee"`
	Address      string  `json:"address"`
	NoDN         string  `json:"no_dn"`
	Colly        int     `json:"colly"`
	Weight       float64 `json:"weight"`
	Volume       float64 `json:"volume"`
	DalamKota    string  `json:"dalam_kota"`
	Keterangan   string  `json:"keterangan"`
	AlasanTolak  string  `json:"alasan_tolak"`
	OriginalLine string  `json:"original_line"`
}

// 1. AJAX SUGGEST CUSTOMER PRINCIPAL
func SuggestCustomerPlanning(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = c.Query("pt_id")
		if ptID == "" {
			ptID = "C"
		}
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	q := strings.TrimSpace(c.Query("q"))
	if len(q) < 2 {
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": []interface{}{}})
		return
	}

	type CustItem struct {
		CustID   string `gorm:"column:cust_id" json:"cust_id"`
		CustName string `gorm:"column:cust_name" json:"cust_name"`
	}
	var list []CustItem

	likeParam := "%" + q + "%"
	query := `
		SELECT DISTINCT 
			RTRIM(LTRIM(cust_id)) AS cust_id, 
			RTRIM(LTRIM(cust_name)) AS cust_name 
		FROM public.mkt_m_customer 
		WHERE (UPPER(cust_name) LIKE UPPER(?) OR UPPER(cust_id) LIKE UPPER(?))
		  AND COALESCE(cust_aktifyn, 'Y') = 'Y'
		ORDER BY cust_name ASC 
		LIMIT 60
	`

	if err := database.Raw(query, likeParam, likeParam).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// Helper untuk membersihkan tanda baca dan spasi agar pencocokan nama fleksibel
func normalizeConsigneeName(name string) string {
	s := strings.ToUpper(strings.TrimSpace(name))
	// 1. Hilangkan teks di dalam kurung, misal "(DIANUM)"
	if idx := strings.Index(s, "("); idx != -1 {
		s = strings.TrimSpace(s[:idx])
	}
	// 2. Samakan variasi umum "PT." menjadi "PT "
	s = strings.ReplaceAll(s, "PT.", "PT ")
	// 3. Buang seluruh karakter selain huruf dan angka
	reg := regexp.MustCompile(`[^A-Z0-9]`)
	return reg.ReplaceAllString(s, "")
}

// 2. PARSE CSV TRANSPORT PLANNING (PREVIEW 1) DENGAN AUTO-ENRICH DARI MKT_M_CUSTOMER & HISTORI MERCK
func ParseTransportPlanningCSV(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = c.Query("pt_id")
		if ptID == "" {
			ptID = "C"
		}
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	customerID := strings.TrimSpace(c.PostForm("customer_id"))
	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Customer ID wajib dipilih"})
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

	// 1. Ambil Master Customer dari mkt_m_customer
	type MasterCustData struct {
		Name    string `gorm:"column:cust_name"`
		Alamat1 string `gorm:"column:cust_alamat1"`
		Kota    string `gorm:"column:cust_kotaid"`
		Telp    string `gorm:"column:cust_telp1"`
	}
	var masterCustList []MasterCustData
	database.Raw(`
		SELECT 
			UPPER(TRIM(cust_name)) AS cust_name, 
			COALESCE(TRIM(cust_alamat1), '') AS cust_alamat1, 
			COALESCE(TRIM(cust_kotaid), '') AS cust_kotaid, 
			COALESCE(TRIM(cust_telp1), '') AS cust_telp1
		FROM public.mkt_m_customer 
		WHERE cust_name IS NOT NULL AND TRIM(cust_name) <> ''
	`).Scan(&masterCustList)

	mapMasterCustomer := make(map[string]MasterCustData)
	for _, m := range masterCustList {
		mapMasterCustomer[m.Name] = m
	}

	// 2. Ambil Histori Anak Customer dari mkt_t_econote
	var validChildrenNames []string
	database.Raw(`
		SELECT DISTINCT UPPER(TRIM(bttt_tujuannama)) 
		FROM public.mkt_t_econote 
		WHERE (TRIM(bttt_asalcustid) = TRIM(?) OR TRIM(bttt_asalcustid) = TRIM(?))
		  AND bttt_tujuannama IS NOT NULL AND bttt_tujuannama <> ''
	`, customerID, strings.TrimLeft(customerID, "0")).Scan(&validChildrenNames)

	// -------------------------------------------------------------------------
	// 🌟 SYNTAX 1 DITARUH DI SINI:
	// Membangun Map Exact & Normalized Validasi dari 3 Sumber Sekaligus
	// -------------------------------------------------------------------------
	mapValidConsignees := make(map[string]bool)
	mapNormalizedValid := make(map[string]bool)

	// A. Dari mkt_m_customer
	for _, m := range masterCustList {
		mapValidConsignees[m.Name] = true
		mapNormalizedValid[normalizeConsigneeName(m.Name)] = true
	}

	// B. Dari mkt_t_econote (Histori BTT)
	for _, n := range validChildrenNames {
		mapValidConsignees[n] = true
		mapNormalizedValid[normalizeConsigneeName(n)] = true
	}

	// C. Dari mkt_m_customer_history_merck (Master Excel Merck)
	var merckHistNames []string
	database.Raw(`
		SELECT DISTINCT UPPER(TRIM(penerima_tujuan)) 
		FROM public.mkt_m_customer_history_merck 
		WHERE penerima_tujuan IS NOT NULL AND penerima_tujuan <> ''
	`).Scan(&merckHistNames)

	for _, mh := range merckHistNames {
		mapValidConsignees[mh] = true
		mapNormalizedValid[normalizeConsigneeName(mh)] = true
	}
	// -------------------------------------------------------------------------

	// Deteksi delimiter (koma atau titik koma)
	buf := make([]byte, 1024)
	n, _ := file.Read(buf)
	file.Seek(0, io.SeekStart)

	firstLine := string(buf[:n])
	commaCount := strings.Count(firstLine, ",")
	semiCount := strings.Count(firstLine, ";")

	reader := csv.NewReader(file)
	if semiCount > commaCount {
		reader.Comma = ';'
	} else {
		reader.Comma = ','
	}
	reader.LazyQuotes = true

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

	groupMap := make(map[string]*TransportPlanningGroupedRow)
	var orderedKeys []string
	var rejectedRows []TransportPlanningRejectedRow
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
			continue // Lewati Header Baris 1
		}

		if len(record) < 14 {
			continue
		}

		consignee := strings.TrimSpace(record[8])
		if consignee == "" {
			continue
		}

		dn := strings.TrimSpace(record[3])
		colly := parseInt(record[5])
		address := strings.TrimSpace(record[9])
		weight := parseFloat(record[11])
		volume := parseFloat(record[12])
		dalamKota := strings.ToUpper(strings.TrimSpace(record[13]))
		if dalamKota == "" {
			dalamKota = "DALAM KOTA"
		}

		up := ""
		if len(record) > 14 {
			up = strings.TrimSpace(record[14])
		}
		telp := ""
		if len(record) > 15 {
			telp = strings.TrimSpace(record[15])
		}
		servID := "1"
		if len(record) > 16 && strings.TrimSpace(record[16]) != "" {
			servID = strings.TrimSpace(record[16])
		}
		layananID := "R"
		if len(record) > 17 && strings.TrimSpace(record[17]) != "" {
			layananID = strings.TrimSpace(record[17])
		}
		packing := 0.0
		if len(record) > 18 {
			packing = parseFloat(record[18])
		}
		keterangan := ""
		if len(record) > 19 {
			keterangan = strings.TrimSpace(record[19])
		}
		kodeKendaraan := ""
		if len(record) > 20 {
			kodeKendaraan = strings.TrimSpace(record[20])
		}
		harga := 0.0
		if len(record) > 21 {
			harga = parseFloat(record[21])
		}
		isDG := "N"
		if len(record) > 22 && strings.ToUpper(strings.TrimSpace(record[22])) == "Y" {
			isDG = "Y"
		}
		isMultidrop := "N"
		if len(record) > 23 {
			md := strings.ToUpper(strings.TrimSpace(record[23]))
			if md == "Y" || md == "E" {
				isMultidrop = md
			}
		}
		kodePacking := ""
		if len(record) > 24 {
			kodePacking = strings.TrimSpace(record[24])
		}
		kodeGroupCarter := ""
		if len(record) > 25 {
			kodeGroupCarter = strings.TrimSpace(record[25])
		}

		// ---------------------------------------------------------------------
		// 🌟 SYNTAX 2 DITARUH DI SINI:
		// Validasi Bertingkat (Exact Match ➔ Normalized Match ➔ Prefix Match)
		// ---------------------------------------------------------------------
		upperConsignee := strings.ToUpper(consignee)
		normConsignee := normalizeConsigneeName(consignee)

		isValid := false

		// 1. Cek Exact Match
		if mapValidConsignees[upperConsignee] {
			isValid = true
		}

		// 2. Cek Normalized Match (Abaikan titik, spasi, dan teks dalam kurung)
		if !isValid && mapNormalizedValid[normConsignee] {
			isValid = true
		}

		// 3. Cek Prefix Match (Jika nama di master terpotong, misal 'MUSTIKA SCIENT' vs 'MUSTIKA SCIENTIFIC')
		if !isValid && len(normConsignee) >= 15 {
			for validNorm := range mapNormalizedValid {
				if strings.HasPrefix(normConsignee, validNorm) || strings.HasPrefix(validNorm, normConsignee) {
					isValid = true
					break
				}
			}
		}

		// Jika tetap tidak ditemukan di master manapun, masukkan ke Data Tertolak
		if !isValid {
			rejectedRows = append(rejectedRows, TransportPlanningRejectedRow{
				No:           len(rejectedRows) + 1,
				Consignee:    consignee,
				Address:      address,
				NoDN:         dn,
				Colly:        colly,
				Weight:       weight,
				Volume:       volume,
				DalamKota:    dalamKota,
				Keterangan:   keterangan,
				AlasanTolak:  "Consignee tidak ditemukan di Master Customer maupun Histori BTT",
				OriginalLine: strings.Join(record, ";"),
			})
			continue
		}
		// ---------------------------------------------------------------------

		// 🔄 AUTO-ENRICHMENT: Jika Alamat / Telp kosong di CSV, lengkapi dari data mkt_m_customer
		if masterData, inMaster := mapMasterCustomer[upperConsignee]; inMaster {
			if (address == "" || address == "-") && masterData.Alamat1 != "" {
				address = masterData.Alamat1
			}
			if (telp == "" || telp == "-" || telp == "0") && masterData.Telp != "" && masterData.Telp != "0" {
				telp = masterData.Telp
			}
		}

		// Grouping Key
		var groupKey string
		if strings.ToUpper(layananID) == "C" && kodeGroupCarter != "" {
			groupKey = fmt.Sprintf("%s|%s|C|%s", consignee, address, kodeGroupCarter)
		} else {
			groupKey = fmt.Sprintf("%s|%s", consignee, address)
		}

		if existing, exists := groupMap[groupKey]; exists {
			existing.TotalColly += colly
			existing.TotalWeight += weight
			existing.TotalVolume += volume
			existing.TotalPacking += packing
			if dn != "" {
				if existing.DNList == "" {
					existing.DNList = dn
				} else if !strings.Contains(existing.DNList, dn) {
					existing.DNList = existing.DNList + ", " + dn
				}
			}
		} else {
			// =========================================================================
			// 📍 AUTO-LOOKUP WILAYAH BERLAPIS (PROPINSI, KOTA, KECAMATAN, KELURAHAN, KODEPOS)
			// =========================================================================
			type LocLookup struct {
				Pulau     string `gorm:"column:pulau"`
				Kota      string `gorm:"column:kota"`
				Kecamatan string `gorm:"column:kecamatan"`
				Kelurahan string `gorm:"column:kelurahan"`
				Kodepos   string `gorm:"column:kodepos"`
			}
			var loc LocLookup

			cleanAddr := strings.TrimSpace(strings.ReplaceAll(address, "\n", " "))

			// 1. Prioritas 1: Dari riwayat BTT lama di mkt_t_econote (Penerima + Kemiripan Alamat)
			database.Raw(`
				SELECT 
					bttt_tujuanpulau AS pulau, 
					bttt_tujuankota AS kota, 
					bttt_tujuankecamatan AS kecamatan, 
					bttt_tujuankelurahan AS kelurahan, 
					bttt_tujuankodepos AS kodepos
				FROM public.mkt_t_econote
				WHERE UPPER(TRIM(bttt_tujuannama)) = UPPER(TRIM(?)) 
				  AND UPPER(TRIM(bttt_tujuanalamat)) LIKE UPPER(?) 
				  AND (TRIM(bttt_asalcustid) = TRIM(?) OR TRIM(bttt_asalcustid) = TRIM(?))
				ORDER BY bttt_tanggal DESC LIMIT 1
			`, consignee, "%"+cleanAddr+"%", customerID, strings.TrimLeft(customerID, "0")).Scan(&loc)

			// 2. Prioritas 2: Dari mkt_t_econote hanya berdasarkan Nama Penerima
			if loc.Kecamatan == "" || loc.Kecamatan == "-" {
				database.Raw(`
					SELECT 
						bttt_tujuanpulau AS pulau, 
						bttt_tujuankota AS kota, 
						bttt_tujuankecamatan AS kecamatan, 
						bttt_tujuankelurahan AS kelurahan, 
						bttt_tujuankodepos AS kodepos
					FROM public.mkt_t_econote
					WHERE UPPER(TRIM(bttt_tujuannama)) = UPPER(TRIM(?)) 
					  AND (TRIM(bttt_asalcustid) = TRIM(?) OR TRIM(bttt_asalcustid) = TRIM(?))
					  AND bttt_tujuankecamatan IS NOT NULL AND bttt_tujuankecamatan <> '' AND bttt_tujuankecamatan <> '-'
					ORDER BY bttt_tanggal DESC LIMIT 1
				`, consignee, customerID, strings.TrimLeft(customerID, "0")).Scan(&loc)
			}

			// 3. Prioritas 3 (UTAMA UNTUK DATA BARU MERCK): Dari master mkt_m_customer_history_merck
			if loc.Kecamatan == "" || loc.Kecamatan == "-" {
				database.Raw(`
					SELECT 
						COALESCE(propinsi, '') AS pulau, 
						COALESCE(kota, '') AS kota, 
						COALESCE(kecamatan, '') AS kecamatan, 
						COALESCE(kelurahan, '') AS kelurahan, 
						COALESCE(kodepos, '') AS kodepos
					FROM public.mkt_m_customer_history_merck
					WHERE UPPER(TRIM(penerima_tujuan)) = UPPER(TRIM(?))
					  AND (UPPER(TRIM(tujuan)) LIKE UPPER(?) OR UPPER(?) LIKE UPPER(CONCAT('%', TRIM(tujuan), '%')))
					LIMIT 1
				`, consignee, "%"+cleanAddr+"%", cleanAddr).Scan(&loc)

				if loc.Kecamatan == "" || loc.Kecamatan == "-" {
					database.Raw(`
						SELECT 
							COALESCE(propinsi, '') AS pulau, 
							COALESCE(kota, '') AS kota, 
							COALESCE(kecamatan, '') AS kecamatan, 
							COALESCE(kelurahan, '') AS kelurahan, 
							COALESCE(kodepos, '') AS kodepos
						FROM public.mkt_m_customer_history_merck
						WHERE UPPER(TRIM(penerima_tujuan)) = UPPER(TRIM(?))
						  AND kecamatan IS NOT NULL AND kecamatan <> '' AND kecamatan <> '-'
						ORDER BY id DESC LIMIT 1
					`, consignee).Scan(&loc)
				}
			}

			// 4. Fallback jika masih kosong: Ambil Kota dari mkt_m_customer
			if loc.Kota == "" {
				if masterData, inMaster := mapMasterCustomer[upperConsignee]; inMaster && masterData.Kota != "" {
					loc.Kota = masterData.Kota
				}
			}

			newItem := &TransportPlanningGroupedRow{
				Key:             groupKey,
				Consignee:       consignee,
				Address:         address,
				Propinsi:        strings.TrimSpace(loc.Pulau),
				Kota:            strings.TrimSpace(loc.Kota),
				Kecamatan:       strings.TrimSpace(loc.Kecamatan),
				Kelurahan:       strings.TrimSpace(loc.Kelurahan),
				Kodepos:         strings.TrimSpace(loc.Kodepos),
				ServID:          servID,
				LayananID:       layananID,
				TotalColly:      colly,
				TotalWeight:     weight,
				TotalVolume:     volume,
				TotalPacking:    packing,
				KodeKendaraan:   kodeKendaraan,
				Harga:           harga,
				IsDG:            isDG,
				IsMultidrop:     isMultidrop,
				KodePacking:     kodePacking,
				Up:              up,
				Telp:            telp,
				StatusWilayah:   dalamKota,
				Keterangan:      keterangan,
				KodeGroupCarter: kodeGroupCarter,
				DNList:          dn,
			}

			groupMap[groupKey] = newItem
			orderedKeys = append(orderedKeys, groupKey)
		}
	}

	var validList []TransportPlanningGroupedRow
	for _, k := range orderedKeys {
		row := groupMap[k]
		row.TotalWeight = math.Ceil(row.TotalWeight)
		row.TotalVolume = math.Ceil(row.TotalVolume)
		validList = append(validList, *row)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "success",
		"total_valid":    len(validList),
		"total_rejected": len(rejectedRows),
		"data_valid":     validList,
		"data_rejected":  rejectedRows,
	})
}

// 3. KALKULASI PREVIEW 2 (HARGA FINAL CARTER/REGULER, BIAYA PACKING & CEK DUPLIKAT DN)
// POST /marketing/transport-planning/calculate-preview2
func CalculatePreview2(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = c.Query("pt_id")
		if ptID == "" {
			ptID = "C"
		}
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	// 1. Definisi Struct Request (Hanya SATU KALI deklarasi)
	type CalcReq struct {
		CustomerID string                        `json:"customer_id" binding:"required"`
		Rows       []TransportPlanningGroupedRow `json:"rows" binding:"required"`
	}

	var req CalcReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Printf("❌ [CalculatePreview2] Bind JSON Error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid: " + err.Error()})
		return
	}

	fmt.Printf("\n=======================================================\n")
	fmt.Printf("⚡ [CalculatePreview2] Request Diterima!\n")
	fmt.Printf("   Customer ID : %s\n", req.CustomerID)
	fmt.Printf("   Total Baris : %d BTT\n", len(req.Rows))
	fmt.Printf("=======================================================\n")

	// 2. Kumpulkan seluruh DN dari baris yang diunggah
	var allDNs []string
	for _, r := range req.Rows {
		if r.DNList != "" {
			parts := strings.Split(r.DNList, ",")
			for _, p := range parts {
				cleanP := strings.TrimSpace(p)
				if cleanP != "" {
					allDNs = append(allDNs, cleanP)
				}
			}
		}
	}

	// 3. Query ke OPR_T_TransportLSPB untuk cek duplikasi nomor DN
	mapExistingDN := make(map[string]string)
	if len(allDNs) > 0 {
		type DNBTTItem struct {
			DN  string `gorm:"column:dn_od_sj"`
			BTT string `gorm:"column:nomorbtt"`
		}
		var existingDNs []DNBTTItem

		database.Raw(`
			SELECT COALESCE(dn_od_sj, '') AS dn_od_sj, COALESCE(nomorbtt, '') AS nomorbtt 
			FROM public.opr_t_transportlspb 
			WHERE TRIM(customerid) = TRIM(?) 
			   OR TRIM(customerid) = TRIM(?)
		`, req.CustomerID, strings.TrimLeft(req.CustomerID, "0")).Scan(&existingDNs)

		for _, item := range existingDNs {
			dns := strings.Split(item.DN, ",")
			for _, d := range dns {
				cleanD := strings.TrimSpace(d)
				if cleanD != "" {
					mapExistingDN[cleanD] = strings.TrimSpace(item.BTT)
				}
			}
		}

		fmt.Printf("🔍 [CalculatePreview2] Total Nomor DN Terdaftar di Database: %d\n", len(mapExistingDN))
	}

	var processedRows []TransportPlanningGroupedRow
	totalAkanProses := 0
	totalDilewati := 0
	grandTotalHarga := 0.0
	grandTotalPacking := 0.0

	for _, r := range req.Rows {
		row := r

		// A. Pengecekan Duplikat DN
		isDup := false
		existingBtt := ""
		if row.DNList != "" {
			dnParts := strings.Split(row.DNList, ",")
			for _, d := range dnParts {
				clean := strings.TrimSpace(d)
				if btt, exists := mapExistingDN[clean]; exists {
					isDup = true
					existingBtt = btt
					break
				}
			}
		}
		row.IsDuplicate = isDup
		row.ExistingBTT = existingBtt

		// B. Kalkulasi Biaya Packing
		row.BiayaPacking = 0
		if row.TotalPacking > 0 && row.KodePacking != "" {
			type PackingTarif struct {
				HargaM3  float64 `gorm:"column:harga_m3"`
				BiayaMin float64 `gorm:"column:biaya_min"`
			}
			var pTarif PackingTarif
			database.Raw(`
				SELECT COALESCE(harga_m3, 0) AS harga_m3, COALESCE(biaya_min, 0) AS biaya_min 
				FROM public.mkt_m_eharga_packing 
				WHERE UPPER(TRIM(packing_kode)) = UPPER(TRIM(?)) AND COALESCE(aktifyn, 'Y') = 'Y' LIMIT 1
			`, row.KodePacking).Scan(&pTarif)

			if pTarif.HargaM3 > 0 {
				volM3 := (row.TotalVolume * 4000) / 1000000
				biaya := volM3 * pTarif.HargaM3
				if biaya < pTarif.BiayaMin {
					biaya = pTarif.BiayaMin
				}
				row.BiayaPacking = math.Round(biaya/100) * 100
			}
		}

		// C. Kalkulasi Harga Final (Skema CSV -> Multidrop -> Carter -> Reguler Master)
		var hargaFinal float64 = 0
		var sumberHarga = "TIDAK ADA"

		// 1. PRIORITAS 1: Jika harga sudah tertera eksplisit di CSV
		if row.Harga > 0 {
			hargaFinal = row.Harga
			sumberHarga = "HARGA CSV"

			// 2. PRIORITAS 2: PENGIRIMAN MULTIDROP DALAM KOTA (is_multidrop = 'Y' / 'E')
		} else if strings.ToUpper(strings.TrimSpace(row.IsMultidrop)) == "Y" || strings.ToUpper(strings.TrimSpace(row.IsMultidrop)) == "E" {
			type MultidropTarif struct {
				HargaKg        float64 `gorm:"column:harga_kg"`
				KgMin          float64 `gorm:"column:kg_min"`
				KgMax          float64 `gorm:"column:kg_max"`
				HargaOverKgMax float64 `gorm:"column:hargaoverkgmax"`
				HargaKgDG      float64 `gorm:"column:harga_kg_dg"`
				KgMinDG        float64 `gorm:"column:kg_min_dg"`
				HargaExpressKg float64 `gorm:"column:hargaexpress_kg"`
				KgExpressMin   float64 `gorm:"column:kgexpress_min"`
			}
			var md MultidropTarif

			// A. Cari berdasarkan Kecamatan, fallback ke Kabupaten/Kota
			errMD := database.Raw(`
				SELECT 
					COALESCE(harga_kg, 0) AS harga_kg,
					COALESCE(kg_min, 0) AS kg_min,
					COALESCE(kg_max, 0) AS kg_max,
					COALESCE(hargaoverkgmax, 0) AS hargaoverkgmax,
					COALESCE(harga_kg_dg, 0) AS harga_kg_dg,
					COALESCE(kg_min_dg, 0) AS kg_min_dg,
					COALESCE(hargaexpress_kg, 0) AS hargaexpress_kg,
					COALESCE(kgexpress_min, 0) AS kgexpress_min
				FROM public.mkt_m_eharga_pelanggan_multidrop
				WHERE (TRIM(customerid) = TRIM(?) OR TRIM(customerid) = TRIM(?))
				  AND (
				      UPPER(TRIM(tujuan_kecamatan)) = UPPER(TRIM(?)) 
				      OR UPPER(TRIM(tujuan_kabupaten)) = UPPER(TRIM(?))
				  )
				ORDER BY 
					CASE WHEN UPPER(TRIM(tujuan_kecamatan)) = UPPER(TRIM(?)) THEN 1 ELSE 2 END ASC,
					id DESC
				LIMIT 1
			`, req.CustomerID, strings.TrimLeft(req.CustomerID, "0"), row.Kecamatan, row.Kota, row.Kecamatan).Scan(&md).Error

			// B. Hitung Harga Multidrop Sesuai Aturan (DG B3 vs Reguler vs Express)
			if errMD == nil && (md.HargaKg > 0 || md.HargaKgDG > 0 || md.HargaExpressKg > 0) {
				beratHitung := row.TotalWeight
				if beratHitung <= 0 {
					beratHitung = 1
				}

				tarifPerKg := md.HargaKg
				minimalKg := md.KgMin

				// Jika Kiriman B3 / Dangerous Goods (is_DG = 'Y')
				if strings.ToUpper(strings.TrimSpace(row.IsDG)) == "Y" && md.HargaKgDG > 0 {
					tarifPerKg = md.HargaKgDG
					if md.KgMinDG > 0 {
						minimalKg = md.KgMinDG
					}
					// Jika Kiriman Multidrop Express (is_multidrop = 'E')
				} else if strings.ToUpper(strings.TrimSpace(row.IsMultidrop)) == "E" && md.HargaExpressKg > 0 {
					tarifPerKg = md.HargaExpressKg
					if md.KgExpressMin > 0 {
						minimalKg = md.KgExpressMin
					}
				}

				// Rumus Multidrop Dakota: Berat Minimal x Tarif Per Kg (jika berat aktual < minimal)
				if minimalKg > 0 && beratHitung < minimalKg {
					hargaFinal = minimalKg * tarifPerKg
				} else {
					hargaFinal = beratHitung * tarifPerKg
				}

				sumberHarga = "TARIF MASTER"
			} else {
				// C. Fallback: Cari Histori BTT Terakhir Konsinyee Tersebut
				var historiHarga float64
				database.Raw(`
					SELECT COALESCE(bttt_harga, 0)
					FROM public.mkt_t_econote
					WHERE (TRIM(bttt_asalcustid) = TRIM(?) OR TRIM(bttt_asalcustid) = TRIM(?))
					  AND UPPER(TRIM(bttt_tujuannama)) = UPPER(TRIM(?))
					  AND COALESCE(bttt_harga, 0) > 0
					ORDER BY bttt_tanggal DESC LIMIT 1
				`, req.CustomerID, strings.TrimLeft(req.CustomerID, "0"), row.Consignee).Scan(&historiHarga)

				if historiHarga > 0 {
					hargaFinal = historiHarga
					sumberHarga = "TARIF MASTER"
				}
			}

			// 3. PRIORITAS 3: LAYANAN CARTER ARMADA (layananID = 'C' atau memiliki Kode Kendaraan)
		} else if strings.ToUpper(strings.TrimSpace(row.LayananID)) == "C" || row.KodeKendaraan != "" {
			type CarterTarif struct {
				Harga   float64 `gorm:"column:harga"`
				HargaDG float64 `gorm:"column:harga_dg"`
			}
			var ct CarterTarif

			database.Raw(`
				SELECT 
					COALESCE(harga, 0) AS harga,
					COALESCE(harga_dg, 0) AS harga_dg
				FROM public.mkt_m_eharga_pelanggan_carter
				WHERE (TRIM(customerid) = TRIM(?) OR TRIM(customerid) = TRIM(?))
				  AND UPPER(TRIM(tujuan_kabupaten)) = UPPER(TRIM(?))
				  AND UPPER(TRIM(jns_kend)) = UPPER(TRIM(?))
				LIMIT 1
			`, req.CustomerID, strings.TrimLeft(req.CustomerID, "0"), row.Kota, row.KodeKendaraan).Scan(&ct)

			tarifCarter := ct.Harga
			if strings.ToUpper(strings.TrimSpace(row.IsDG)) == "Y" && ct.HargaDG > 0 {
				tarifCarter = ct.HargaDG
			}

			if tarifCarter == 0 {
				database.Raw(`
					SELECT COALESCE(bttt_harga, 0) 
					FROM public.mkt_t_econote 
					WHERE (TRIM(bttt_asalcustid) = TRIM(?) OR TRIM(bttt_asalcustid) = TRIM(?))
					  AND UPPER(TRIM(bttt_tujuannama)) = UPPER(TRIM(?)) 
					  AND COALESCE(bttt_harga, 0) > 0
					ORDER BY bttt_tanggal DESC LIMIT 1
				`, req.CustomerID, strings.TrimLeft(req.CustomerID, "0"), row.Consignee).Scan(&tarifCarter)
			}

			if tarifCarter > 0 {
				hargaFinal = tarifCarter
				sumberHarga = "TARIF MASTER"
			}

			// 4. PRIORITAS 4: LAYANAN REGULER / LUAR KOTA
		} else {
			type MasterTarifRow struct {
				MinimalKg          float64 `gorm:"column:minimalkg"`
				HargaPokok         float64 `gorm:"column:hargapokok"`
				BypassKg           float64 `gorm:"column:bypasskg"`
				HargaKgSelanjutnya float64 `gorm:"column:hargakgselanjutnya"`
				BiayaTambahan      float64 `gorm:"column:biayatambahan"`
			}
			var mTarif MasterTarifRow

			// Daftar Customer ID Grup Merck
			targetCustIDs := []string{
				req.CustomerID,
				strings.TrimLeft(req.CustomerID, "0"),
			}
			if strings.Contains(req.CustomerID, "0010000219") || strings.Contains(req.CustomerID, "0010000073") || strings.Contains(req.CustomerID, "0010000021") {
				targetCustIDs = append(targetCustIDs, "0010000219", "0010000073", "0010000021", "10000219", "10000073", "10000021")
			}

			// Bersihkan imbuhan wilayah
			regStrip := regexp.MustCompile(`(?i)\b(KAB\.|KABUPATEN|KOTA|ADM\.)\b`)
			cleanKota := strings.TrimSpace(regStrip.ReplaceAllString(row.Kota, ""))
			cleanKecamatan := strings.TrimSpace(regStrip.ReplaceAllString(row.Kecamatan, ""))

			// A. Prioritas Match: Kecamatan + Kota (Cocokkan servid baris, fallback ke servid apa saja)
			errTarif := database.Raw(`
				SELECT 
					COALESCE(minimalkg, 0) AS minimalkg, 
					COALESCE(hargapokok, 0) AS hargapokok, 
					COALESCE(bypasskg, 0) AS bypasskg, 
					COALESCE(hargakgselanjutnya, 0) AS hargakgselanjutnya, 
					COALESCE(biayatambahan, 0) AS biayatambahan
				FROM public.mkt_m_eharga_pelanggan
				WHERE customerid IN (?)
				  AND (TRIM(jnharga::text) = '0' OR UPPER(TRIM(jnharga::text)) = 'BERAT' OR jnharga IS NULL)
				  AND (
				      UPPER(TRIM(tujuan_kecamatan)) LIKE UPPER(?) 
				      OR UPPER(?) LIKE CONCAT('%', UPPER(TRIM(tujuan_kecamatan)), '%')
				  )
				  AND (
				      UPPER(TRIM(tujuan_kabupaten)) LIKE UPPER(?) 
				      OR UPPER(?) LIKE CONCAT('%', UPPER(TRIM(tujuan_kabupaten)), '%')
				  )
				ORDER BY 
					CASE WHEN TRIM(servid::text) = TRIM(?) THEN 1 ELSE 2 END,
					CASE WHEN customerid = ? THEN 1 ELSE 2 END,
					id DESC
				LIMIT 1
			`, targetCustIDs, "%"+cleanKecamatan+"%", cleanKecamatan, "%"+cleanKota+"%", cleanKota, row.ServID, req.CustomerID).Scan(&mTarif).Error

			// B. Fallback: Cocokkan Tingkat Kota/Kabupaten Saja
			if errTarif != nil || mTarif.HargaPokok == 0 {
				database.Raw(`
					SELECT 
						COALESCE(minimalkg, 0) AS minimalkg, 
						COALESCE(hargapokok, 0) AS hargapokok, 
						COALESCE(bypasskg, 0) AS bypasskg, 
						COALESCE(hargakgselanjutnya, 0) AS hargakgselanjutnya, 
						COALESCE(biayatambahan, 0) AS biayatambahan
					FROM public.mkt_m_eharga_pelanggan
					WHERE customerid IN (?)
					  AND (TRIM(jnharga::text) = '0' OR UPPER(TRIM(jnharga::text)) = 'BERAT' OR jnharga IS NULL)
					  AND (
					      UPPER(TRIM(tujuan_kabupaten)) LIKE UPPER(?) 
					      OR UPPER(?) LIKE CONCAT('%', UPPER(TRIM(tujuan_kabupaten)), '%')
					  )
					ORDER BY 
						CASE WHEN TRIM(servid::text) = TRIM(?) THEN 1 ELSE 2 END,
						CASE WHEN customerid = ? THEN 1 ELSE 2 END,
						id DESC
					LIMIT 1
				`, targetCustIDs, "%"+cleanKota+"%", cleanKota, row.ServID, req.CustomerID).Scan(&mTarif)
			}

			// C. Hitung Rumus Tarif Master
			if mTarif.HargaPokok > 0 {
				beratHitung := row.TotalWeight
				if beratHitung <= 0 {
					beratHitung = 1
				}

				biayaKirim := mTarif.HargaPokok

				if mTarif.BypassKg > 0 && beratHitung >= mTarif.BypassKg {
					biayaKirim = beratHitung * mTarif.HargaKgSelanjutnya
				} else if beratHitung > mTarif.MinimalKg {
					biayaKirim = mTarif.HargaPokok + ((beratHitung - mTarif.MinimalKg) * mTarif.HargaKgSelanjutnya)
				}

				hargaFinal = biayaKirim + mTarif.BiayaTambahan
				sumberHarga = "TARIF MASTER"
			}
		}

		row.HargaFinal = hargaFinal
		row.SumberHarga = sumberHarga

		if !isDup {
			totalAkanProses++
			grandTotalHarga += row.HargaFinal
			grandTotalPacking += row.BiayaPacking
		} else {
			totalDilewati++
		}

		processedRows = append(processedRows, row)
	}

	fmt.Printf("✅ [CalculatePreview2] Selesai: %d Akan Diproses, %d Dilewati (Duplikat). Total: Rp %.0f\n\n",
		totalAkanProses, totalDilewati, grandTotalHarga)

	c.JSON(http.StatusOK, gin.H{
		"status":              "success",
		"total_akan_proses":   totalAkanProses,
		"total_dilewati":      totalDilewati,
		"grand_total_harga":   grandTotalHarga,
		"grand_total_packing": grandTotalPacking,
		"rows":                processedRows,
	})
}

// 4. SIMPAN BATCH KE DATABASE (FINAL INSERT BTT & LSPB)
func SaveTransportPlanningBatch(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = c.Query("pt_id")
		if ptID == "" {
			ptID = "C"
		}
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	type SaveReq struct {
		CustomerID   string                        `json:"customer_id" binding:"required"`
		CustomerName string                        `json:"customer_name"`
		Rows         []TransportPlanningGroupedRow `json:"rows" binding:"required"`
	}

	var req SaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid"})
		return
	}

	username := "superdli"
	if authUser, exists := c.Get("username"); exists && authUser != nil {
		username = fmt.Sprintf("%v", authUser)
	}

	batchID := fmt.Sprintf("BATCH-%d", time.Now().UnixNano())
	tx := database.Begin()

	insertSQL := `
		INSERT INTO public.opr_t_transport_planning_upload 
			(batch_upload_id, customer_id, customer_name, no_do_dn, nama_penerima, alamat_penerima, 
			 kota_tujuan, koli, berat, volume, status_wilayah, status_proses, delimiter_used, user_upload, created_at)
		VALUES 
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'PENDING', 'AUTO', ?, CURRENT_TIMESTAMP)
	`

	savedCount := 0
	for _, r := range req.Rows {
		// Jangan simpan yang duplikat
		if r.IsDuplicate {
			continue
		}

		if err := tx.Exec(insertSQL, batchID, req.CustomerID, req.CustomerName, r.DNList, r.Consignee, r.Address,
			r.Kota, r.TotalColly, r.TotalWeight, r.TotalVolume, r.StatusWilayah, username).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan batch: " + err.Error()})
			return
		}
		savedCount++
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  fmt.Sprintf("Berhasil memproses %d BTT ke database!", savedCount),
		"batch_id": batchID,
	})
}

// POST /marketing/transport-planning/save-wilayah-history
// Validasi pintar: Cegah dobel, lengkapi yang kosong, dan simpan data termutakhir
func SaveCustomerWilayahHistory(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = c.Query("pt_id")
		if ptID == "" {
			ptID = "C"
		}
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	var req struct {
		Consignee string `json:"consignee" binding:"required"`
		Address   string `json:"address"`
		Propinsi  string `json:"propinsi" binding:"required"`
		Kota      string `json:"kota" binding:"required"`
		Kecamatan string `json:"kecamatan" binding:"required"`
		Kelurahan string `json:"kelurahan" binding:"required"`
		Kodepos   string `json:"kodepos"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Data wilayah tidak lengkap"})
		return
	}

	cleanConsignee := strings.ToUpper(strings.TrimSpace(req.Consignee))
	cleanAddress := strings.ToUpper(strings.TrimSpace(req.Address))
	cleanProp := strings.ToUpper(strings.TrimSpace(req.Propinsi))
	cleanKota := strings.ToUpper(strings.TrimSpace(req.Kota))
	cleanKec := strings.ToUpper(strings.TrimSpace(req.Kecamatan))
	cleanKel := strings.ToUpper(strings.TrimSpace(req.Kelurahan))
	cleanPos := strings.TrimSpace(req.Kodepos)

	// 1. Cek apakah record sudah ada berdasarkan Penerima & Kemiripan Alamat
	type ExistingRecord struct {
		ID        int    `gorm:"column:id"`
		Kecamatan string `gorm:"column:kecamatan"`
		Kelurahan string `gorm:"column:kelurahan"`
		Kodepos   string `gorm:"column:kodepos"`
	}
	var existing ExistingRecord

	database.Raw(`
		SELECT id, COALESCE(kecamatan, '') AS kecamatan, COALESCE(kelurahan, '') AS kelurahan, COALESCE(kodepos, '') AS kodepos
		FROM public.mkt_m_customer_history_merck
		WHERE UPPER(TRIM(penerima_tujuan)) = ?
		  AND (UPPER(TRIM(tujuan)) = ? 
		       OR UPPER(TRIM(tujuan)) LIKE ? 
		       OR ? LIKE CONCAT('%', UPPER(TRIM(tujuan)), '%'))
		ORDER BY id DESC LIMIT 1
	`, cleanConsignee, cleanAddress, "%"+cleanAddress+"%", cleanAddress).Scan(&existing)

	// 2. Eksekusi Berdasarkan Kondisi Data
	if existing.ID > 0 {
		// KONDISI A: Data sudah ada -> Lakukan UPDATE data terakhir (menghindari duplikasi baris baru)
		err := database.Exec(`
			UPDATE public.mkt_m_customer_history_merck
			SET propinsi = ?,
			    kota = ?,
			    kecamatan = ?,
			    kelurahan = ?,
			    kodepos = ?,
			    tujuan = CASE WHEN tujuan IS NULL OR TRIM(tujuan) = '' THEN ? ELSE tujuan END
			WHERE id = ?
		`, cleanProp, cleanKota, cleanKec, cleanKel, cleanPos, cleanAddress, existing.ID).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui master wilayah"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"action":  "updated",
			"message": "Data wilayah berhasil diperbarui ke record yang ada tanpa duplikasi!",
		})
		return
	}

	// KONDISI B: Data belum ada sama sekali -> Lakukan INSERT baru
	err := database.Exec(`
		INSERT INTO public.mkt_m_customer_history_merck (
			penerima_tujuan, tujuan, propinsi, kota, kecamatan, kelurahan, kodepos
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, cleanConsignee, cleanAddress, cleanProp, cleanKota, cleanKec, cleanKel, cleanPos).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan master wilayah baru"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"action":  "inserted",
		"message": "Data wilayah baru berhasil ditambahkan ke master riwayat!",
	})
}
