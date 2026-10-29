package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
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

// 1. SUGGEST CUSTOMER PRINCIPAL
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

// 2. PARSE CSV TRANSPORT PLANNING (LENGKAP DENGAN LOOKUP WILAYAH DUA TAHAP)
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

	// 1. Ambil daftar anak customer yang valid dari eConote
	var validChildrenNames []string
	database.Raw(`
		SELECT DISTINCT UPPER(TRIM(bttt_tujuannama)) 
		FROM public.mkt_t_econote 
		WHERE TRIM(bttt_asalcustid) = TRIM(?) AND bttt_tujuannama IS NOT NULL AND bttt_tujuannama <> ''
	`, customerID).Scan(&validChildrenNames)

	mapValidConsignees := make(map[string]bool)
	for _, n := range validChildrenNames {
		mapValidConsignees[n] = true
	}

	// 2. Auto-Detect Delimiter
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
			continue
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

		// 3. Validasi Anak Customer
		upperConsignee := strings.ToUpper(consignee)
		isValid := false

		if len(mapValidConsignees) > 0 {
			if mapValidConsignees[upperConsignee] {
				isValid = true
			}
		} else {
			isValid = true
		}

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
				AlasanTolak:  "Consignee tidak ditemukan di dalam database sebagai Anak Customer",
				OriginalLine: strings.Join(record, ";"),
			})
			continue
		}

		// 4. Grouping Data Valid
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
			// 🌟 LOOKUP WILAYAH DUA TAHAP (Tahap 1: Consignee + Alamat, Tahap 2: Fallback Consignee saja)
			type LocLookup struct {
				Pulau     string `gorm:"column:bttt_tujuanpulau"`
				Kota      string `gorm:"column:bttt_tujuankota"`
				Kecamatan string `gorm:"column:bttt_tujuankecamatan"`
				Kelurahan string `gorm:"column:bttt_tujuankelurahan"`
				Kodepos   string `gorm:"column:bttt_tujuankodepos"`
			}
			var loc LocLookup

			// Coba Tahap 1: Cari yang Nama dan Alamatnya cocok
			cleanAddr := strings.TrimSpace(strings.ReplaceAll(address, "\n", " "))
			database.Raw(`
				SELECT bttt_tujuanpulau, bttt_tujuankota, bttt_tujuankecamatan, bttt_tujuankelurahan, bttt_tujuankodepos
				FROM public.mkt_t_econote
				WHERE UPPER(TRIM(bttt_tujuannama)) = UPPER(TRIM(?)) 
				  AND UPPER(TRIM(bttt_tujuanalamat)) LIKE UPPER(?) 
				  AND TRIM(bttt_asalcustid) = TRIM(?)
				ORDER BY bttt_tanggal DESC LIMIT 1
			`, consignee, "%"+cleanAddr+"%", customerID).Scan(&loc)

			// Coba Tahap 2: Jika alamat belum cocok karena penulisan format, ambil histori terakhir berdasarkan Consignee saja (Sama persis ASP)
			if loc.Kota == "" {
				database.Raw(`
					SELECT bttt_tujuanpulau, bttt_tujuankota, bttt_tujuankecamatan, bttt_tujuankelurahan, bttt_tujuankodepos
					FROM public.mkt_t_econote
					WHERE UPPER(TRIM(bttt_tujuannama)) = UPPER(TRIM(?)) 
					  AND TRIM(bttt_asalcustid) = TRIM(?)
					ORDER BY bttt_tanggal DESC LIMIT 1
				`, consignee, customerID).Scan(&loc)
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
		// Pembulatan ke atas (Ceil) persis sistem logistik Dakota
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

// 3. SIMPAN BATCH KE DATABASE
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

	if len(req.Rows) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Tidak ada baris data valid untuk disimpan"})
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

	for _, r := range req.Rows {
		if err := tx.Exec(insertSQL, batchID, req.CustomerID, req.CustomerName, r.DNList, r.Consignee, r.Address,
			r.Kota, r.TotalColly, r.TotalWeight, r.TotalVolume, r.StatusWilayah, username).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan batch: " + err.Error()})
			return
		}
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  fmt.Sprintf("Berhasil mengunggah %d BTT Transport Planning!", len(req.Rows)),
		"batch_id": batchID,
	})
}
