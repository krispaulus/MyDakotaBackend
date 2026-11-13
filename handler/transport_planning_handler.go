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
	Tanggal         string  `json:"tanggal"`
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

	StorageCode  string `json:"storage_code"`
	KetDOKembali string `json:"ket_do_kembali"`

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
	_ = parseFloat

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
		originalAddress := strings.TrimSpace(record[9])
		address := originalAddress
		rawWeight := parseIndonesianFloat(record[11])
		rawVolume := parseIndonesianFloat(record[12])
		dalamKota := strings.ToUpper(strings.TrimSpace(record[13]))
		if dalamKota == "" {
			dalamKota = "DALAM KOTA"
		}

		// 🌟 Chargeable Weight per Baris (Sesuai ASP Lawas: Ambil Max lalu bulatkan ke atas per resi jika colly terpisah)
		lineWeight := rawWeight
		if rawVolume > lineWeight {
			lineWeight = rawVolume
		}
		// Pembulatan per baris resi
		roundedLineWeight := math.Ceil(lineWeight)
		if roundedLineWeight <= 0 {
			roundedLineWeight = 1
		}
		// 🌟 Pembacaan Kolom Tambahan dari CSV
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

		// 🌟 1. Ambil StorageCode (Kolom K / index 10) & Mapping ke Keterangan Atas
		storageCode := ""
		if len(record) > 10 {
			storageCode = strings.TrimSpace(record[10])
		}
		ketDOKembaliAtas := MapStorageCodeToDOKembali(storageCode)

		// 🌟 2. Ambil Biaya Packing dari Kolom S (index 18)
		packing := 0.0
		if len(record) > 18 {
			packing = parseIndonesianFloat(record[18])
		}

		// 🌟 3. Ambil Keterangan Bawah langsung dari Kolom T (index 19)
		keterangan := ""
		if len(record) > 19 {
			keterangan = strings.TrimSpace(record[19])
		}
		if keterangan == "" {
			keterangan = "SURAT JALAN KEMBALI"
		}

		kodeKendaraan := ""
		if len(record) > 20 {
			kodeKendaraan = strings.TrimSpace(record[20])
		}
		harga := 0.0
		if len(record) > 21 {
			harga = parseIndonesianFloat(record[21])
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

		// Validasi consignee...
		upperConsignee := strings.ToUpper(consignee)
		normConsignee := normalizeConsigneeName(consignee)

		isValid := false
		if mapValidConsignees[upperConsignee] {
			isValid = true
		}
		if !isValid && mapNormalizedValid[normConsignee] {
			isValid = true
		}
		if !isValid && len(normConsignee) >= 15 {
			for validNorm := range mapNormalizedValid {
				if strings.HasPrefix(normConsignee, validNorm) || strings.HasPrefix(validNorm, normConsignee) {
					isValid = true
					break
				}
			}
		}

		if !isValid {
			rejectedRows = append(rejectedRows, TransportPlanningRejectedRow{
				No:           len(rejectedRows) + 1,
				Consignee:    consignee,
				Address:      address,
				NoDN:         dn,
				Colly:        colly,
				Weight:       rawWeight,
				Volume:       rawVolume,
				DalamKota:    dalamKota,
				Keterangan:   keterangan,
				AlasanTolak:  "Consignee tidak ditemukan di Master Customer maupun Histori BTT",
				OriginalLine: strings.Join(record, ";"),
			})
			continue
		}

		// 🔄 AUTO-ENRICHMENT: Jika Alamat / Telp kosong di CSV, lengkapi dari data mkt_m_customer
		if masterData, inMaster := mapMasterCustomer[upperConsignee]; inMaster {
			if (address == "" || address == "-") && masterData.Alamat1 != "" {
				address = masterData.Alamat1
			}
			if (telp == "" || telp == "-" || telp == "0") && masterData.Telp != "" && masterData.Telp != "0" {
				telp = masterData.Telp
			}
		}

		// 🌟 Grouping Key persis ASP Lawas: Pakai teks asli CSV
		cleanCons := strings.ToUpper(strings.TrimSpace(consignee))
		cleanAddr := strings.ToUpper(strings.TrimSpace(originalAddress))

		var groupKey string
		if strings.ToUpper(layananID) == "C" && kodeGroupCarter != "" {
			groupKey = fmt.Sprintf("%s|%s|C|%s", cleanCons, cleanAddr, strings.ToUpper(strings.TrimSpace(kodeGroupCarter)))
		} else if strings.TrimSpace(servID) == "3" {
			groupKey = fmt.Sprintf("%s|%s|%s|%s|%s", cleanCons, cleanAddr, servID, layananID, strings.ToUpper(isDG))
		} else {
			groupKey = fmt.Sprintf("%s|%s", cleanCons, cleanAddr)
		}

		if existing, exists := groupMap[groupKey]; exists {
			existing.TotalColly += colly
			existing.TotalWeight += roundedLineWeight
			existing.TotalVolume += rawVolume
			existing.TotalPacking += packing
			existing.Harga += harga
			if dn != "" {
				if existing.DNList == "" {
					existing.DNList = dn
				} else if !strings.Contains(existing.DNList, dn) {
					existing.DNList = existing.DNList + ", " + dn
				}
			}
		} else {
			// =========================================================================
			// 📍 AUTO-LOOKUP WILAYAH BERLAPIS (PRIORITAS 1: MASTER HISTORI MERCK TERBARU)
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

			// 1. PRIORITAS UTAMA (1): Ambil dari mkt_m_customer_history_merck (Penerima + Kemiripan Alamat)
			database.Raw(`
				SELECT 
					COALESCE(propinsi, '') AS pulau, 
					COALESCE(kota, '') AS kota, 
					COALESCE(kecamatan, '') AS kecamatan, 
					COALESCE(kelurahan, '') AS kelurahan, 
					COALESCE(kodepos, '') AS kodepos
				FROM public.mkt_m_customer_history_merck
				WHERE UPPER(TRIM(penerima_tujuan)) = UPPER(TRIM(?))
				  AND (
				      UPPER(TRIM(tujuan)) = UPPER(TRIM(?))
				      OR UPPER(TRIM(tujuan)) LIKE UPPER(?) 
				      OR UPPER(?) LIKE CONCAT('%', UPPER(TRIM(tujuan)), '%')
				  )
				ORDER BY id DESC LIMIT 1
			`, consignee, cleanAddr, "%"+cleanAddr+"%", cleanAddr).Scan(&loc)

			// 2. PRIORITAS 2: Dari mkt_m_customer_history_merck berdasarkan Nama Penerima Terakhir
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

			// 3. PRIORITAS 3 (FALLBACK): Dari riwayat transaksi lama di mkt_t_econote (Penerima + Kemiripan Alamat)
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
					  AND UPPER(TRIM(bttt_tujuanalamat)) LIKE UPPER(?) 
					  AND (TRIM(bttt_asalcustid) = TRIM(?) OR TRIM(bttt_asalcustid) = TRIM(?))
					ORDER BY bttt_tanggal DESC LIMIT 1
				`, consignee, "%"+cleanAddr+"%", customerID, strings.TrimLeft(customerID, "0")).Scan(&loc)
			}

			// 4. PRIORITAS 4: Dari mkt_t_econote hanya berdasarkan Nama Penerima
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

			// 5. PRIORITAS 5: Fallback Kota dari mkt_m_customer
			if loc.Kota == "" {
				if masterData, inMaster := mapMasterCustomer[upperConsignee]; inMaster && masterData.Kota != "" {
					loc.Kota = masterData.Kota
				}
			}

			// Ambil tanggal dari CSV (Kolom B / index 1)
			tglCSV := ""
			if len(record) > 1 {
				tglCSV = strings.TrimSpace(record[1])
			}

			newItem := &TransportPlanningGroupedRow{
				Key:             groupKey,
				Tanggal:         tglCSV,
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
				TotalWeight:     roundedLineWeight, // 🌟 Gunakan roundedLineWeight agar baris 1 koli langsung terbulatkan benar
				TotalVolume:     rawVolume,
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
				StorageCode:     storageCode,
				KetDOKembali:    ketDOKembaliAtas,
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
		// Pembulatan ke atas (Ceil) persis ASP lawas
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

	type CalcReq struct {
		CustomerID string                        `json:"customer_id" binding:"required"`
		Rows       []TransportPlanningGroupedRow `json:"rows" binding:"required"`
	}

	var req CalcReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid: " + err.Error()})
		return
	}

	// 1. Cek Duplikasi DN dari Database (Hanya cari DN yang diunggah)
	mapExistingDN := make(map[string]string)
	var allDNs []string
	for _, r := range req.Rows {
		if r.DNList != "" {
			for _, d := range strings.Split(r.DNList, ",") {
				if cleanD := strings.TrimSpace(d); cleanD != "" {
					allDNs = append(allDNs, cleanD)
				}
			}
		}
	}

	if len(allDNs) > 0 {
		type DNBTTItem struct {
			DN  string `gorm:"column:dn_od_sj"`
			BTT string `gorm:"column:nomorbtt"`
		}
		var existingDNs []DNBTTItem

		database.Raw(`
			SELECT COALESCE(dn_od_sj, '') AS dn_od_sj, COALESCE(nomorbtt, '') AS nomorbtt 
			FROM public.opr_t_transportlspb 
			WHERE TRIM(dn_od_sj) IN (?)
			  AND nomorbtt IS NOT NULL 
			  AND TRIM(nomorbtt) <> '' 
			  AND TRIM(nomorbtt) <> '-'
		`, allDNs).Scan(&existingDNs)

		for _, item := range existingDNs {
			for _, d := range strings.Split(item.DN, ",") {
				if cleanD := strings.TrimSpace(d); cleanD != "" {
					mapExistingDN[cleanD] = strings.TrimSpace(item.BTT)
				}
			}
		}
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
			for _, d := range strings.Split(row.DNList, ",") {
				cleanDN := strings.TrimSpace(d)
				if btt, exists := mapExistingDN[cleanDN]; exists && btt != "" && btt != "-" {
					isDup = true
					existingBtt = btt
					break
				}
			}
		}
		row.IsDuplicate = isDup
		row.ExistingBTT = existingBtt

		// B. Kalkulasi Biaya Packing (Utamakan dari kolom CSV Packing jika ada isinya)
		if row.TotalPacking > 0 {
			row.BiayaPacking = row.TotalPacking
		} else if row.KodePacking != "" {
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

		// C. Kalkulasi Harga Final
		var hargaFinal float64 = 0
		var sumberHarga = "TIDAK ADA"

		// 1. PRIORITAS 1: Harga tertera di CSV
		if row.Harga > 0 {
			hargaFinal = row.Harga
			sumberHarga = "HARGA CSV"

			// 2. PRIORITAS 2: Pengiriman Multidrop Dalam Kota
		} else if strings.ToUpper(strings.TrimSpace(row.IsMultidrop)) == "Y" || strings.ToUpper(strings.TrimSpace(row.IsMultidrop)) == "E" {
			type MultidropTarif struct {
				HargaKg        float64 `gorm:"column:harga_kg"`
				KgMin          float64 `gorm:"column:kg_min"`
				KgMax          float64 `gorm:"column:kg_max"`
				HargaOverKgMax float64 `gorm:"column:hargaoverkgmax"`
				HargaKgDG      float64 `gorm:"column:harga_kg_dg"`
				KgMinDG        float64 `gorm:"column:kg_min_dg"`
				KgMaxDG        float64 `gorm:"column:kg_max_dg"`
				HargaOverKgDG  float64 `gorm:"column:hargaoverkgmax_dg"`
				HargaExpressKg float64 `gorm:"column:hargaexpress_kg"`
				KgExpressMin   float64 `gorm:"column:kgexpress_min"`
			}
			var md MultidropTarif

			regNonWord := regexp.MustCompile(`[^\w\s]`)
			tempKota := regNonWord.ReplaceAllString(row.Kota, " ")
			regStrip := regexp.MustCompile(`(?i)\b(KABUPATEN|KAB|KOTA|ADM)\b`)
			cleanKota := strings.ToUpper(strings.TrimSpace(regStrip.ReplaceAllString(tempKota, " ")))

			tempKec := regNonWord.ReplaceAllString(row.Kecamatan, " ")
			cleanKecamatan := strings.ToUpper(strings.TrimSpace(regStrip.ReplaceAllString(tempKec, " ")))

			cleanCustID := strings.TrimSpace(req.CustomerID)

			database.Raw(`
				SELECT 
					COALESCE(harga_kg, 0) AS harga_kg,
					COALESCE(kg_min, 0) AS kg_min,
					COALESCE(kg_max, 0) AS kg_max,
					COALESCE(hargaoverkgmax, 0) AS hargaoverkgmax,
					COALESCE(harga_kg_dg, 0) AS harga_kg_dg,
					COALESCE(kg_min_dg, 0) AS kg_min_dg,
					COALESCE(kg_max_dg, 0) AS kg_max_dg,
					COALESCE(hargaoverkgmax_dg, 0) AS hargaoverkgmax_dg,
					COALESCE(hargaexpress_kg, 0) AS hargaexpress_kg,
					COALESCE(kgexpress_min, 0) AS kgexpress_min
				FROM public.mkt_m_eharga_pelanggan_multidrop
				WHERE (TRIM(customerid) = TRIM(?) OR TRIM(customerid) = TRIM(?))
				  AND (
				      UPPER(TRIM(tujuan_kabupaten)) = UPPER(TRIM(?))
				      OR UPPER(TRIM(tujuan_kabupaten)) LIKE UPPER(?)
				      OR UPPER(?) LIKE CONCAT('%', UPPER(TRIM(tujuan_kabupaten)), '%')
				  )
				  AND (
				      UPPER(TRIM(tujuan_kecamatan)) = UPPER(TRIM(?))
				      OR UPPER(TRIM(tujuan_kecamatan)) LIKE UPPER(?)
				      OR tujuan_kecamatan IS NULL 
				      OR TRIM(tujuan_kecamatan) = ''
				  )
				ORDER BY 
					CASE WHEN UPPER(TRIM(tujuan_kabupaten)) = UPPER(TRIM(?)) THEN 0 ELSE 1 END,
					CASE WHEN UPPER(TRIM(tujuan_kecamatan)) = UPPER(TRIM(?)) THEN 0 ELSE 1 END,
					CASE WHEN (COALESCE(harga_kg_dg, 0) > 0 OR COALESCE(harga_kg, 0) > 0) THEN 0 ELSE 1 END,
					id DESC
				LIMIT 1
			`, cleanCustID, strings.TrimLeft(cleanCustID, "0"), cleanKota, "%"+cleanKota+"%", cleanKota, cleanKecamatan, "%"+cleanKecamatan+"%", cleanKota, cleanKecamatan).Scan(&md)

			if (md.HargaKg == 0 && md.HargaKgDG == 0 && md.HargaExpressKg == 0) && cleanKota != "" {
				database.Raw(`
					SELECT 
						COALESCE(harga_kg, 0) AS harga_kg,
						COALESCE(kg_min, 0) AS kg_min,
						COALESCE(kg_max, 0) AS kg_max,
						COALESCE(hargaoverkgmax, 0) AS hargaoverkgmax,
						COALESCE(harga_kg_dg, 0) AS harga_kg_dg,
						COALESCE(kg_min_dg, 0) AS kg_min_dg,
						COALESCE(kg_max_dg, 0) AS kg_max_dg,
						COALESCE(hargaoverkgmax_dg, 0) AS hargaoverkgmax_dg,
						COALESCE(hargaexpress_kg, 0) AS hargaexpress_kg,
						COALESCE(kgexpress_min, 0) AS kgexpress_min
					FROM public.mkt_m_eharga_pelanggan_multidrop
					WHERE (TRIM(customerid) = TRIM(?) OR TRIM(customerid) = TRIM(?))
					  AND (
					      UPPER(TRIM(tujuan_kabupaten)) = UPPER(TRIM(?))
					      OR UPPER(TRIM(tujuan_kabupaten)) LIKE UPPER(?)
					      OR UPPER(?) LIKE CONCAT('%', UPPER(TRIM(tujuan_kabupaten)), '%')
					  )
					ORDER BY 
						CASE WHEN (COALESCE(harga_kg_dg, 0) > 0 OR COALESCE(harga_kg, 0) > 0) THEN 0 ELSE 1 END,
						id DESC
					LIMIT 1
				`, cleanCustID, strings.TrimLeft(cleanCustID, "0"), cleanKota, "%"+cleanKota+"%", cleanKota).Scan(&md)
			}

			if md.HargaKg > 0 || md.HargaKgDG > 0 || md.HargaExpressKg > 0 {
				berat := row.TotalWeight
				if berat <= 0 {
					berat = 1
				}

				hk := md.HargaKg
				kmin := md.KgMin
				kmax := md.KgMax
				over := md.HargaOverKgMax

				if strings.ToUpper(strings.TrimSpace(row.IsDG)) == "Y" && md.HargaKgDG > 0 {
					hk = md.HargaKgDG
					if md.KgMinDG > 0 {
						kmin = md.KgMinDG
					}
					if md.KgMaxDG > 0 {
						kmax = md.KgMaxDG
					}
					if md.HargaOverKgDG > 0 {
						over = md.HargaOverKgDG
					}
				} else if strings.ToUpper(strings.TrimSpace(row.IsMultidrop)) == "E" && md.HargaExpressKg > 0 {
					hk = md.HargaExpressKg
					if md.KgExpressMin > 0 {
						kmin = md.KgExpressMin
					}
				}

				if over <= 0 {
					over = hk
				}

				if berat <= kmin {
					hargaFinal = hk * kmin
				} else if kmax > 0 && berat > kmax {
					hargaFinal = (hk * kmax) + ((berat - kmax) * over)
				} else {
					hargaFinal = hk * berat
				}
				sumberHarga = "TARIF MASTER"
			}

			// 3. PRIORITAS 3: Layanan Carter Armada
			if strings.ToUpper(strings.TrimSpace(row.LayananID)) == "C" || row.KodeKendaraan != "" {
				type CarterTarif struct {
					Harga   float64 `gorm:"column:harga"`
					HargaDG float64 `gorm:"column:harga_dg"`
				}
				var ct CarterTarif

				cleanCustID := strings.TrimSpace(req.CustomerID)

				regNonWord := regexp.MustCompile(`[^\w\s]`)
				tempKota := regNonWord.ReplaceAllString(row.Kota, " ")
				regStrip := regexp.MustCompile(`(?i)\b(KABUPATEN|KAB|KOTA|ADM)\b`)
				cleanKota := strings.ToUpper(strings.TrimSpace(regStrip.ReplaceAllString(tempKota, " ")))

				jnsKend := strings.ToUpper(strings.TrimSpace(row.KodeKendaraan))
				if jnsKend == "" || jnsKend == "-" {
					grp := strings.ToUpper(strings.TrimSpace(row.KodeGroupCarter))
					regTrailDigits := regexp.MustCompile(`[0-9]+$`)
					jnsKend = strings.TrimSpace(regTrailDigits.ReplaceAllString(grp, ""))
				}

				kendPatterns := []string{"%" + jnsKend + "%"}
				if strings.Contains(jnsKend, "GMAX") || strings.Contains(jnsKend, "GM") {
					kendPatterns = append(kendPatterns, "%GRANMAX%", "%GRAN MAX%", "%GRAND MAX%", "%G-MAX%", "%GM%", "%BLIND%", "%PICK%")
				}

				database.Raw(`
				SELECT 
					COALESCE(harga, 0) AS harga,
					COALESCE(harga_dg, 0) AS harga_dg
				FROM public.mkt_m_eharga_pelanggan_carter
				WHERE (
				      TRIM(customerid) = TRIM(?) 
				      OR TRIM(customerid) = TRIM(?) 
				      OR customerid = '0010000021'
				      OR customerid LIKE '%0021'
				  )
				  AND (
				      UPPER(TRIM(tujuan_kabupaten)) = ?
				      OR UPPER(TRIM(tujuan_kabupaten)) LIKE ?
				      OR UPPER(?) LIKE CONCAT('%', UPPER(TRIM(tujuan_kabupaten)), '%')
				  )
				  AND UPPER(TRIM(jns_kend)) LIKE ANY (?)
				ORDER BY 
					CASE WHEN UPPER(TRIM(jns_kend)) = ? THEN 0 ELSE 1 END,
					id DESC
				LIMIT 1
			`, cleanCustID, strings.TrimLeft(cleanCustID, "0"), cleanKota, "%"+cleanKota+"%", cleanKota, "{"+strings.Join(kendPatterns, ",")+"}", jnsKend).Scan(&ct)

				tarifCarter := ct.Harga
				if strings.ToUpper(strings.TrimSpace(row.IsDG)) == "Y" && ct.HargaDG > 0 {
					tarifCarter = ct.HargaDG
				}

				if tarifCarter > 0 {
					hargaFinal = tarifCarter
					sumberHarga = "TARIF MASTER"
				}
			}

			// 4. PRIORITAS 4: LAYANAN REGULER / LUAR KOTA (MENGGUNAKAN MKT_M_EHARGA_CUSTOMER RESMI 2026)
		} else {
			type CustomerRateRow struct {
				MinCharge float64 `gorm:"column:min_charge"`
				TarifKg   float64 `gorm:"column:tarif_kg"`
			}
			var cRate CustomerRateRow

			weight := row.TotalWeight
			if row.TotalVolume > weight {
				weight = row.TotalVolume
			}
			if weight <= 0 {
				weight = 1
			}

			// 🌟 PEMBERSIH KOTA & KECAMATAN AMAN (Mencegah "SUKABUMI" -> "SUUMI")
			regNonWord := regexp.MustCompile(`[^\w\s]`)
			tempKota := regNonWord.ReplaceAllString(row.Kota, " ")
			regKota := regexp.MustCompile(`(?i)\b(KABUPATEN|KAB|KOTA|ADM)\b`)
			cleanKota := strings.ToUpper(strings.TrimSpace(regKota.ReplaceAllString(tempKota, " ")))

			tempKec := regNonWord.ReplaceAllString(row.Kecamatan, " ")
			regKec := regexp.MustCompile(`(?i)\b(KECAMATAN|KEC)\b`)
			cleanKecamatan := strings.ToUpper(strings.TrimSpace(regKec.ReplaceAllString(tempKec, " ")))

			cleanKelurahan := strings.ToUpper(strings.TrimSpace(row.Kelurahan))
			cleanKodepos := strings.TrimSpace(row.Kodepos)

			// Customer ID & Holding
			cleanCustID := strings.TrimSpace(req.CustomerID)
			targetCustIDs := []string{
				cleanCustID,
				strings.TrimLeft(cleanCustID, "0"),
				fmt.Sprintf("%010s", strings.TrimLeft(cleanCustID, "0")),
			}

			var currentCustName string
			database.Table("public.mkt_m_customer").
				Select("COALESCE(cust_name, '')").
				Where("TRIM(cust_id) = TRIM(?) OR TRIM(cust_id) = TRIM(?)", cleanCustID, strings.TrimLeft(cleanCustID, "0")).
				Limit(1).
				Scan(&currentCustName)

			var searchKeyword string
			regClean := regexp.MustCompile(`(?i)\b(PT|CV|TBK|THE|UD|TOKO|CORP|INC|LTD)\b|[^a-zA-Z0-9\s]`)
			cleanNameWords := strings.Fields(regClean.ReplaceAllString(currentCustName, " "))
			for _, w := range cleanNameWords {
				if len(w) >= 4 {
					searchKeyword = strings.ToUpper(w)
					break
				}
			}

			if searchKeyword != "" {
				var relatedCustIDs []string
				database.Raw(`
					SELECT DISTINCT RTRIM(LTRIM(cust_id)) 
					FROM public.mkt_m_customer 
					WHERE UPPER(cust_name) LIKE ?
					LIMIT 10
				`, "%"+searchKeyword+"%").Scan(&relatedCustIDs)

				for _, rid := range relatedCustIDs {
					if rid != "" {
						targetCustIDs = append(targetCustIDs, rid, strings.TrimLeft(rid, "0"))
					}
				}
			}

			// 🌟 LANGKAH 1: Cari ke mkt_m_eharga_customer (Prioritas Kodepos 43157 & 43357)
			database.Raw(`
				SELECT 
					COALESCE(min_charge, 0) AS min_charge, 
					COALESCE(tarif_kg, 0) AS tarif_kg
				FROM public.mkt_m_eharga_customer
				WHERE (
				      cust_id IN (?) 
				      OR cust_id = '0010000021' 
				      OR cust_id = '0010000073'
				      OR cust_id LIKE '%0021'
				  )
				  AND (
				      (tujuan_kodepos IS NOT NULL AND TRIM(tujuan_kodepos) <> '' AND TRIM(tujuan_kodepos) = ?)
				      OR (
				          (UPPER(TRIM(tujuan_kelurahan)) = ? OR UPPER(TRIM(tujuan_kecamatan)) = ? OR UPPER(TRIM(tujuan_kecamatan)) LIKE ?)
				          AND (UPPER(TRIM(tujuan_kabupaten)) LIKE ? OR UPPER(TRIM(tujuan_kabupaten)) LIKE '%SUKABUMI%')
				      )
				  )
				ORDER BY 
					CASE WHEN TRIM(tujuan_kodepos) = ? THEN 0 ELSE 1 END,
					CASE WHEN UPPER(TRIM(COALESCE(tujuan_kelurahan, ''))) = ? THEN 0 ELSE 1 END,
					id DESC
				LIMIT 1
			`, targetCustIDs, cleanKodepos, cleanKelurahan, cleanKecamatan, "%"+cleanKecamatan+"%", "%"+cleanKota+"%", cleanKodepos, cleanKelurahan).Scan(&cRate)

			// 🌟 LANGKAH 2: Fallback Tingkat Kabupaten / Kota di mkt_m_eharga_customer
			if cRate.TarifKg == 0 && cleanKota != "" && cleanKota != "-" {
				database.Raw(`
					SELECT 
						COALESCE(min_charge, 0) AS min_charge, 
						COALESCE(tarif_kg, 0) AS tarif_kg
					FROM public.mkt_m_eharga_customer
					WHERE (
					      cust_id IN (?) 
					      OR cust_id = '0010000021' 
					      OR cust_id = '0010000073'
					      OR cust_id LIKE '%0021'
					  )
					  AND (
					      UPPER(TRIM(tujuan_kabupaten)) = ? 
					      OR UPPER(TRIM(tujuan_kabupaten)) = ? 
					      OR UPPER(TRIM(tujuan_kabupaten)) LIKE ?
					      OR UPPER(TRIM(tujuan_kabupaten)) LIKE '%SUKABUMI%'
					  )
					ORDER BY id DESC
					LIMIT 1
				`, targetCustIDs, cleanKota, "KOTA "+cleanKota, "%"+cleanKota+"%").Scan(&cRate)
			}

			// 🌟 LANGKAH 3: Hitung Tarif Sesuai Aturan Master Merck 2026 (Min Charge x Tarif Kg)
			if cRate.TarifKg > 0 {
				minCharge := cRate.MinCharge
				if minCharge <= 0 {
					minCharge = 1
				}

				if weight <= minCharge {
					hargaFinal = minCharge * cRate.TarifKg
				} else {
					hargaFinal = weight * cRate.TarifKg
				}
				sumberHarga = "TARIF MASTER"

				// 🌟 LANGKAH 4: Fallback ke Master Tarif Umum Dakota
			} else {
				type UmumTarifRow struct {
					MinimalKg          float64 `gorm:"column:minimalkg"`
					HargaPokok         float64 `gorm:"column:hargapokok"`
					HargaKgSelanjutnya float64 `gorm:"column:hargakgselanjutnya"`
				}
				var uTarif UmumTarifRow

				database.Raw(`
					SELECT 
						COALESCE(minimalkg, 0) AS minimalkg, 
						COALESCE(hargapokok, 0) AS hargapokok, 
						COALESCE(hargakgselanjutnya, 0) AS hargakgselanjutnya
					FROM public.mkt_m_eharga
					WHERE (servid = '1' OR servid IS NULL)
					  AND (
					      UPPER(TRIM(tujuan_kabupaten)) = ? 
					      OR UPPER(TRIM(tujuan_kabupaten)) = ? 
					      OR UPPER(TRIM(tujuan_kabupaten)) LIKE ?
					  )
					ORDER BY 
						CASE WHEN UPPER(TRIM(COALESCE(tujuan_kecamatan, ''))) = ? THEN 0 ELSE 1 END,
						id DESC
					LIMIT 1
				`, cleanKota, "KOTA "+cleanKota, "%"+cleanKota+"%", cleanKecamatan).Scan(&uTarif)

				if uTarif.HargaPokok > 0 {
					if weight < uTarif.MinimalKg {
						hargaFinal = uTarif.HargaPokok
					} else if uTarif.HargaKgSelanjutnya > 0 {
						hargaFinal = uTarif.HargaKgSelanjutnya * weight
					} else {
						hargaFinal = uTarif.HargaPokok
					}
					sumberHarga = "TARIF MASTER"
				}
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

	c.JSON(http.StatusOK, gin.H{
		"status":              "success",
		"total_akan_proses":   totalAkanProses,
		"total_dilewati":      totalDilewati,
		"grand_total_harga":   grandTotalHarga,
		"grand_total_packing": grandTotalPacking,
		"rows":                processedRows,
	})
}

// Helper parsing tanggal + jam dari CSV Excel yang tahan berbagai format
func parseFlexibleCSVDate(raw string, fallback time.Time) time.Time {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return fallback
	}

	// Pisahkan bagian tanggal dan jam (jika ada spasi)
	parts := strings.Fields(clean)
	datePart := parts[0]
	timePart := ""
	if len(parts) > 1 {
		timePart = parts[1]
	}

	// Parse tanggal (DD/MM/YYYY atau YYYY-MM-DD)
	var parsedDate time.Time
	var ok bool
	dateFormats := []string{
		"2/1/2006",
		"02/01/2006",
		"2-1-2006",
		"02-01-2006",
		"2006-01-02",
		"2006/01/02",
	}
	for _, f := range dateFormats {
		if t, err := time.Parse(f, datePart); err == nil {
			parsedDate = t
			ok = true
			break
		}
	}
	if !ok {
		return fallback
	}

	// Parse jam jika ada (misal "7:59" atau "16:07:30"), jika tidak ada pakai jam sekarang
	hour, minute, second := fallback.Hour(), fallback.Minute(), fallback.Second()
	if timePart != "" {
		tParts := strings.Split(timePart, ":")
		if len(tParts) >= 2 {
			if h, err := strconv.Atoi(tParts[0]); err == nil {
				hour = h
			}
			if m, err := strconv.Atoi(tParts[1]); err == nil {
				minute = m
			}
			if len(tParts) >= 3 {
				if s, err := strconv.Atoi(tParts[2]); err == nil {
					second = s
				}
			}
		}
	}

	return time.Date(
		parsedDate.Year(), parsedDate.Month(), parsedDate.Day(),
		hour, minute, second, 0,
		fallback.Location(),
	)
}

// 4. SIMPAN BATCH KE DATABASE (AUTO GENERATE NOMOR BTT & INSERT KE MKT_T_ECONOTE)
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
		AgenID       string                        `json:"agen_id"`
		Rows         []TransportPlanningGroupedRow `json:"rows" binding:"required"`
	}

	var req SaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid"})
		return
	}

	// 🌟 Ambil Profil Lengkap Pengirim dari Master Customer
	type SenderProfile struct {
		Alamat1 string `gorm:"column:cust_alamat1"`
		Kota    string `gorm:"column:cust_kotaid"`
		Telp    string `gorm:"column:cust_telp1"`
	}
	var sender SenderProfile
	database.Table("public.mkt_m_customer").
		Select("cust_alamat1, cust_kotaid, cust_telp1").
		Where("TRIM(cust_id) = TRIM(?) OR TRIM(cust_id) = TRIM(?)", req.CustomerID, strings.TrimLeft(req.CustomerID, "0")).
		Limit(1).
		Scan(&sender)

	username := "superdli"
	if authUser, exists := c.Get("username"); exists && authUser != nil {
		username = fmt.Sprintf("%v", authUser)
	}

	// 🌟 RESOLVE AGEN ID DINAMIS (Mendukung seluruh cabang, format 3 digit misal "001", "004", dst.)
	rawAgen := strings.TrimSpace(req.AgenID)
	if rawAgen == "" {
		if aID, exists := c.Get("agen_id"); exists && aID != nil {
			rawAgen = strings.TrimSpace(fmt.Sprintf("%v", aID))
		}
	}
	if rawAgen == "" || rawAgen == "<nil>" || strings.EqualFold(rawAgen, "PUSAT DAKOTA") || strings.EqualFold(rawAgen, "HOLDING") {
		rawAgen = "001"
	}
	// Normalisasi ke 3 digit jika berupa angka pendek (misal "1" -> "001")
	agenID := rawAgen
	if len(agenID) < 3 {
		agenID = fmt.Sprintf("%03s", agenID)
	}

	// 🌟 Inisialisasi waktu dan batch ID aktif
	now := time.Now()
	tglTrans := now.Format("2006-01-02")
	batchID := fmt.Sprintf("BATCH-%d", now.UnixNano())

	// 🌟 Pastikan kolom pendukung tersedia di database aktif (22.7 / 22.8 / multi-PT)
	database.Exec(`ALTER TABLE public.mkt_t_econote ADD COLUMN IF NOT EXISTS bttt_isikiriman TEXT`)
	database.Exec(`ALTER TABLE public.mkt_t_econote ADD COLUMN IF NOT EXISTS bttt_biayapacking NUMERIC(18,2)`)

	tx := database.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	savedCount := 0

	// 📍 POSISI 1 (SEBELUM LOOP): Ambil nomor urut awal menggunakan generator resmi standar Dakota
	// Contoh hasil: 001112026C000674 (atau 004112026C000001 jika agen 004)
	firstResi := GenerateNomorResiBTTStandard(database, fmt.Sprintf("%v", ptID), agenID, now)
	prefixResi := firstResi[:10] // Contoh: "001112026C"
	nextBTTSeq, _ := strconv.Atoi(firstResi[10:])
	if nextBTTSeq <= 0 {
		nextBTTSeq = 1
	}

	for _, r := range req.Rows {
		// Abaikan baris duplikat
		if r.IsDuplicate {
			continue
		}

		// 📍 POSISI 2 (DI DALAM LOOP): Generate Nomor BTT Resmi Standar Dakota (16 Digit)
		newBTTNo := fmt.Sprintf("%s%06d", prefixResi, nextBTTSeq)
		nextBTTSeq++

		// 🌟 PARSING TANGGAL MASING-MASING BARIS DARI CSV (mendukung format tanggal + jam seperti "11/11/2026 7:59")
		tglBTT := parseFlexibleCSVDate(r.Tanggal, now)
		if r.Tanggal != "" {
			cleanTgl := strings.TrimSpace(r.Tanggal)
			formats := []string{
				"2/1/2006 15:04",
				"02/01/2006 15:04",
				"2/1/2006 15:04:05",
				"02/01/2006 15:04:05",
				"2006-01-02 15:04:05",
				"2006-01-02 15:04",
				"2/1/2006",
				"02/01/2006",
				"2006-01-02",
				"2006/01/02",
				"2-1-2006",
				"02-01-2006",
			}
			for _, fmtStr := range formats {
				if parsedTgl, err := time.Parse(fmtStr, cleanTgl); err == nil {
					if parsedTgl.Hour() == 0 && parsedTgl.Minute() == 0 && !strings.Contains(cleanTgl, ":") {
						tglBTT = time.Date(
							parsedTgl.Year(), parsedTgl.Month(), parsedTgl.Day(),
							now.Hour(), now.Minute(), now.Second(), now.Nanosecond(),
							now.Location(),
						)
					} else {
						tglBTT = parsedTgl
					}
					break
				}
			}
		}

		// 🌟 Fallback untuk UP Pengirim jika di CSV tidak terisi
		namaUP := r.Up
		if strings.TrimSpace(namaUP) == "" || namaUP == "-" {
			namaUP = req.CustomerName
		}

		// 3. Simpan ke Antrean Staging Upload
		insertUploadSQL := `
			INSERT INTO public.opr_t_transport_planning_upload 
				(batch_upload_id, customer_id, customer_name, no_do_dn, nama_penerima, alamat_penerima, 
				 kota_tujuan, koli, berat, volume, status_wilayah, status_proses, delimiter_used, user_upload, created_at)
			VALUES 
				(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'SELESAI', 'AUTO', ?, CURRENT_TIMESTAMP)
		`
		if err := tx.Exec(insertUploadSQL, batchID, req.CustomerID, req.CustomerName, r.DNList, r.Consignee, r.Address,
			r.Kota, r.TotalColly, r.TotalWeight, r.TotalVolume, r.StatusWilayah, username).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan log upload: " + err.Error()})
			return
		}

		// 🌟 4. Potong string nomor DN jika melebihi batas agar aman dari batasan VARCHAR(200)
		cleanDNList := r.DNList
		if len(cleanDNList) > 195 {
			cleanDNList = cleanDNList[:192] + "..."
		}

		// 🌟 Tentukan apakah Paket (R) atau Carter (C) berdasarkan kolom layananID CSV
		paketYN := "Y"
		if strings.ToUpper(strings.TrimSpace(r.LayananID)) == "C" || r.KodeKendaraan != "" {
			paketYN = "N" // N = Carter
		}

		// 🌟 5. Simpan Transaksi BTT ke Tabel Master BTT (public.mkt_t_econote)
		bttRecord := map[string]interface{}{
			"bttt_id":              newBTTNo,
			"bttt_tanggal":         tglBTT,
			"bttt_servid":          r.ServID,
			"bttt_asalagenid":      agenID, // 👈 Dinamis sesuai cabang/agen aktif
			"bttt_asalcustid":      req.CustomerID,
			"bttt_asalname":        req.CustomerName,
			"bttt_asalalamat":      sender.Alamat1,
			"bttt_asalkota":        sender.Kota,
			"bttt_asaltelp":        sender.Telp,
			"bttt_tujuannama":      r.Consignee,
			"bttt_tujuanalamat":    r.Address,
			"bttt_tujuanpulau":     r.Propinsi,
			"bttt_tujuankota":      r.Kota,
			"bttt_tujuankecamatan": r.Kecamatan,
			"bttt_tujuankelurahan": r.Kelurahan,
			"bttt_tujuankodepos":   r.Kodepos,
			"bttt_tujuantelp":      r.Telp,
			"bttt_up":              namaUP,
			"bttt_jmlunit":         r.TotalColly,
			"bttt_berat":           r.TotalWeight,
			"bttt_beratvol":        r.TotalVolume,
			"bttt_ukuran":          fmt.Sprintf("%.2f", r.TotalVolume),
			"bttt_harga":           r.HargaFinal,
			"bttt_packingid":       r.KodePacking,
			"bttt_jmlpck":          int(r.TotalPacking),
			"bttt_biayapacking":    r.BiayaPacking,
			"bttt_biayapenerus":    0,
			"bttt_nosuratjalan":    cleanDNList,
			"bttt_ket":             r.KetDOKembali,
			"bttt_isikiriman":      r.Keterangan,
			"bttt_namabarang":      "PRODUK FARMASI / KESEHATAN",
			"bttt_paketyn":         paketYN,
			"bttt_jenisharga":      "2",
			"bttt_pembayaran":      "KREDIT",
			"bttt_charterjnskend":  r.KodeKendaraan,
			"bttt_spyn":            "Y",
			"bttt_aktifyn":         "Y",
			"bttt_updateid":        username,
			"bttt_updatetime":      now,
		}

		if err := tx.Table("public.mkt_t_econote").Create(&bttRecord).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal insert ke mkt_t_econote: " + err.Error()})
			return
		}

		// 🌟 6. Hubungkan seluruh nomor DN asli ke tabel relasi LSPB
		if r.DNList != "" {
			cleanCustID := req.CustomerID
			if strings.Contains(strings.ToUpper(req.CustomerName), "MERCK") {
				cleanCustID = "0010000021"
			}

			for _, d := range strings.Split(r.DNList, ",") {
				cleanDN := strings.TrimSpace(d)
				if cleanDN == "" {
					continue
				}

				// A. Simpan ke public.opr_t_transportlspb (Bersihkan dulu jika ada DN lama)
				tx.Exec(`DELETE FROM public.opr_t_transportlspb WHERE TRIM(dn_od_sj) = ?`, cleanDN)
				tx.Exec(`
					INSERT INTO public.opr_t_transportlspb (dn_od_sj, customerid, tanggal_handover, nomorbtt)
					VALUES (?, ?, ?, ?)
				`, cleanDN, cleanCustID, tglTrans, newBTTNo)

				// 🌟 B. Sinkronkan ke opr_t_transportlspbwithcarinoutanddatetime
				tx.Exec(`DELETE FROM public.opr_t_transportlspbwithcarinoutanddatetime WHERE TRIM(nodo) = ?`, cleanDN)
				tx.Exec(`
					INSERT INTO public.opr_t_transportlspbwithcarinoutanddatetime (nodo, noskb, kategori, created_at, updated_at)
					VALUES (?, ?, 'LUAR_KOTA', ?, ?)
				`, cleanDN, newBTTNo, now, now)
			}
		}
		savedCount++
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal commit BTT: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  fmt.Sprintf("Berhasil menerbitkan %d BTT ke database!", savedCount),
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

func parseIndonesianFloat(val string) float64 {
	clean := strings.TrimSpace(val)
	if clean == "" || clean == "-" {
		return 0
	}

	// 1. Jika format: 1.089.270 atau 523.150 (titik sebagai pemisah ribuan)
	if strings.Count(clean, ".") > 1 || (strings.Count(clean, ".") == 1 && !strings.Contains(clean, ",")) {
		parts := strings.Split(clean, ".")
		// Jika bagian belakang titik panjangnya tepat 3 digit (contoh: 523.150 atau 1.089.270)
		if len(parts[len(parts)-1]) == 3 {
			clean = strings.ReplaceAll(clean, ".", "")
			clean = strings.ReplaceAll(clean, ",", ".")
		}
	} else {
		// Jika format standar desimal koma (contoh: 12,50)
		clean = strings.ReplaceAll(clean, ".", "")
		clean = strings.ReplaceAll(clean, ",", ".")
	}

	if f, err := strconv.ParseFloat(clean, 64); err == nil {
		return f
	}
	return 0
}

// Helper mapping storagecode ke status DO Kembali (Sesuai DO KEMBALI MERCK.xlsx)
func MapStorageCodeToDOKembali(storageCode string) string {
	cleanCode := strings.ToUpper(strings.TrimSpace(storageCode))
	switch cleanCode {
	case "N", "NO", "N ONS", "NO ONS", "NO COA/PO", "NO COA ASN", "N COA", "NO COA",
		"COA/NO", "ASN/COA/NO", "N PO", "NO /CARD /COA", "NO COA SDS", "COA NO",
		"YTFC", "NO PO", "NO COA PO", "COA PO NO", "PO NO", "N COA PO":
		return "DO Asli Kembali Jatiasih"
	case "Y", "YES", "Y ONS", "YES ONS", "E-INVOICE", "Y /PO", "Y COA", "COA / Y", "0":
		return "DO Copy Kembali Jatiasih"
	default:
		if strings.HasPrefix(cleanCode, "N") {
			return "DO Asli Kembali Jatiasih"
		}
		if strings.HasPrefix(cleanCode, "Y") {
			return "DO Copy Kembali Jatiasih"
		}
		return "DO Asli Kembali Jatiasih"
	}
}
