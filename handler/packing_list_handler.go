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

// DTO Response List Packing List
type PackingListItem struct {
	PLIID        string  `gorm:"column:pli_id" json:"pli_id"`
	PLIBTTID     string  `gorm:"column:pli_bttid" json:"pli_bttid"`
	Tanggal      string  `gorm:"column:pli_tanggal" json:"pli_tanggal"`
	Customer     string  `gorm:"column:cust_name" json:"cust_name"`
	AsalCustID   string  `gorm:"column:pli_asalcustid" json:"pli_asalcustid"`
	AsalName     string  `gorm:"column:pli_asalname" json:"pli_asalname"`
	TujuanKota   string  `gorm:"column:pli_tujuankota" json:"pli_tujuankota"`
	TujuanNama   string  `gorm:"column:pli_tujuannama" json:"pli_tujuannama"`
	TujuanAlamat string  `gorm:"column:pli_tujuanalamat" json:"pli_tujuanalamat"`
	Colly        int     `gorm:"column:colly" json:"colly"`
	Berat        float64 `gorm:"column:berat" json:"berat"`
	BeratVol     float64 `gorm:"column:beratvol" json:"beratvol"`
	NilaiBarang  float64 `gorm:"column:nilaibarang" json:"nilaibarang"`
}

// Detail Item untuk Print Slip
type PackingListDetailItem struct {
	PLIDID       int64   `gorm:"column:pli_d_id" json:"pli_d_id"`
	NamaBarang   string  `gorm:"column:pli_namabarang" json:"pli_namabarang"`
	JenisKemasan string  `gorm:"column:namakemasan" json:"namakemasan"`
	JmlKemasan   int     `gorm:"column:pli_jmlkemasan" json:"pli_jmlkemasan"`
	BeratAsli    float64 `gorm:"column:pli_beratasli" json:"pli_beratasli"`
	Volume       float64 `gorm:"column:pli_volume" json:"pli_volume"`
	NilaiBarang  float64 `gorm:"column:pli_nilaibarang" json:"pli_nilaibarang"`
}

// Request Payload untuk Konversi Packing List menjadi BTT
type ProcessBTTRequest struct {
	PLIID        string  `json:"pli_id" binding:"required"`
	Tanggal      string  `json:"tanggal" binding:"required"`
	AsalCustID   string  `json:"asal_cust_id"`
	AsalName     string  `json:"asal_name"`
	AsalAlamat   string  `json:"asal_alamat"`
	AsalKota     string  `json:"asal_kota"`
	AsalTelp     string  `json:"asal_telp"`
	TujuanAgen   string  `json:"tujuan_agen_id" binding:"required"`
	TujuanNama   string  `json:"tujuan_nama" binding:"required"`
	TujuanAlamat string  `json:"tujuan_alamat"`
	TujuanKota   string  `json:"tujuan_kota" binding:"required"`
	TujuanTelp   string  `json:"tujuan_telp"`
	IsiBarang    string  `json:"isi_barang" binding:"required"`
	NoSJ         string  `json:"no_sj"`
	Colly        int     `json:"colly"`
	Berat        float64 `json:"berat"`
	Volume       float64 `json:"volume"`
	Service      string  `json:"service"`    // R: REGULER, O: ONS, dll.
	Pembayaran   string  `json:"pembayaran"` // 1: TUNAI, 2: KREDIT, 3: TAGIH
	BiayaKirim   float64 `json:"biaya_kirim"`
	BiayaPenerus float64 `json:"biaya_penerus"`
}

// 1. GET /api/marketing/packing-list/data
func GetPackingListData(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database resolver gagal"})
		return
	}

	useTanggal := c.Query("use_tanggal") == "true"
	tglAwal := strings.TrimSpace(c.Query("start_date"))
	tglAkhir := strings.TrimSpace(c.Query("end_date"))
	custID := strings.TrimSpace(c.Query("cust_id"))

	query := `
		SELECT 
			pli_id,
			COALESCE(pli_bttid, '') AS pli_bttid,
			COALESCE(TO_CHAR(pli_tanggal, 'YYYY-MM-DD'), '') AS pli_tanggal,
			COALESCE(cust_name, pli_asalname, '-') AS cust_name,
			COALESCE(pli_asalcustid, '') AS pli_asalcustid,
			COALESCE(pli_asalname, '-') AS pli_asalname,
			COALESCE(pli_tujuankota, '-') AS pli_tujuankota,
			COALESCE(pli_tujuannama, '-') AS pli_tujuannama,
			COALESCE(pli_tujuanalamat, '-') AS pli_tujuanalamat,
			colly,
			berat,
			beratvol,
			nilaibarang
		FROM public.vw_packinglist
		WHERE COALESCE(NULLIF(TRIM(pli_bttid), ''), '') = ''
	`
	var args []interface{}

	if useTanggal && tglAwal != "" && tglAkhir != "" {
		query += " AND pli_tanggal BETWEEN ? AND ?"
		args = append(args, tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}
	if custID != "" && custID != "SEMUA" {
		query += " AND (pli_asalcustid = ? OR cust_name ILIKE ?)"
		args = append(args, custID, "%"+custID+"%")
	}

	query += " ORDER BY pli_tanggal DESC, pli_id DESC LIMIT 400"

	var results []PackingListItem
	if err := database.Raw(query, args...).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []PackingListItem{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 2. GET /api/marketing/packing-list/detail/:id
func GetPackingListDetail(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database resolver gagal"})
		return
	}

	pliID := c.Param("id")

	var header PackingListItem
	errH := database.Raw(`
		SELECT pli_id, COALESCE(pli_bttid, '') AS pli_bttid, 
		       COALESCE(TO_CHAR(pli_tanggal, 'YYYY-MM-DD'), '') AS pli_tanggal,
		       COALESCE(cust_name, pli_asalname, '-') AS cust_name,
		       COALESCE(pli_asalcustid, '') AS pli_asalcustid,
		       COALESCE(pli_asalname, '-') AS pli_asalname,
		       COALESCE(pli_tujuankota, '-') AS pli_tujuankota,
		       COALESCE(pli_tujuannama, '-') AS pli_tujuannama,
		       COALESCE(pli_tujuanalamat, '-') AS pli_tujuanalamat,
		       colly, berat, beratvol, nilaibarang
		FROM public.vw_packinglist WHERE pli_id = ?
	`, pliID).Scan(&header).Error

	if errH != nil || header.PLIID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data packing list tidak ditemukan"})
		return
	}

	var details []PackingListDetailItem
	database.Raw(`
		SELECT 
			d.pli_d_id,
			COALESCE(d.pli_namabarang, '-') AS pli_namabarang,
			COALESCE(k.namakemasan, d.pli_jeniskemasan, 'DUS') AS namakemasan,
			COALESCE(d.pli_jmlkemasan, 1) AS pli_jmlkemasan,
			COALESCE(d.pli_beratasli, 0) AS pli_beratasli,
			COALESCE(d.pli_volume, 0) AS pli_volume,
			COALESCE(d.pli_nilaibarang, 0) AS pli_nilaibarang
		FROM public.mkt_t_epackinglistd d
		LEFT JOIN public.glb_m_kemasan k ON TRIM(d.pli_jeniskemasan) = TRIM(k.kdjenis)
		WHERE TRIM(d.pli_h_id) = ?
		ORDER BY d.pli_d_id ASC
	`, pliID).Scan(&details)

	if details == nil {
		details = []PackingListDetailItem{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "header": header, "details": details})
}

// 3. POST /api/marketing/packing-list/process-btt
func ProcessPackingListToBTT(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database resolver gagal"})
		return
	}

	username, _ := c.Get("username")
	var req ProcessBTTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Buat No. BTT Baru (Prefix Cabang + PT + Timestamp Suffix)
	prefix := "BTT"
	if len(req.PLIID) >= 3 {
		prefix = req.PLIID[:3]
	}
	newBttID := fmt.Sprintf("%s%s%d", prefix, fmt.Sprintf("%v", ptID), time.Now().Unix()%1000000)

	tx := database.Begin()

	// 1. Insert ke MKT_T_eConote
	insertConote := `
		INSERT INTO public.mkt_t_econote (
			bttt_id, bttt_tanggal, bttt_asalcustid, bttt_asalname, bttt_asalalamat,
			bttt_asalkota, bttt_asaltelp, bttt_tujuanagenid, bttt_tujuannama,
			bttt_tujuanalamat, bttt_tujuankota, bttt_tujuantelp, bttt_nosuratjalan,
			bttt_namabarang, bttt_jmlunit, bttt_berat, bttt_ukuran, bttt_service,
			bttt_pembayaran, bttt_harga, bttt_biayapenerus, bttt_kirimyn, bttt_aktifyn,
			bttt_updateid, bttt_updatetime
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'N', 'Y', ?, NOW())
	`
	if err := tx.Exec(insertConote,
		newBttID, req.Tanggal, req.AsalCustID, req.AsalName, req.AsalAlamat,
		req.AsalKota, req.AsalTelp, req.TujuanAgen, req.TujuanNama,
		req.TujuanAlamat, req.TujuanKota, req.TujuanTelp, req.NoSJ,
		req.IsiBarang, req.Colly, req.Berat, req.Volume, req.Service,
		req.Pembayaran, req.BiayaKirim, req.BiayaPenerus, fmt.Sprintf("%v", username),
	).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat resi BTT: " + err.Error()})
		return
	}

	// 2. Insert Histori Awal (Entry Loket)
	insertHist := `
		INSERT INTO public.mkt_t_ehistory (
			hist_bttid, hist_tanggal, hist_staturut, hist_ket, hist_suratjalan, hist_agenid
		) VALUES (?, NOW(), 0, 'Entry Loket dari Packing List', ?, ?)
	`
	_ = tx.Exec(insertHist, newBttID, req.NoSJ, req.TujuanAgen).Error

	// 3. Update Header Packing List dengan No BTT yang baru
	updatePLI := `UPDATE public.mkt_t_epackinglisth SET pli_bttid = ?, pli_updateid = ?, pli_updatetime = NOW() WHERE pli_id = ?`
	if err := tx.Exec(updatePLI, newBttID, fmt.Sprintf("%v", username), req.PLIID).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update status packing list: " + err.Error()})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Packing List berhasil dikonversi ke Resi BTT",
		"btt_id":  newBttID,
	})
}

// 4. POST /api/marketing/packing-list/upload-csv
func UploadPackingListCSV(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database resolver gagal"})
		return
	}

	custID := c.PostForm("cust_id")
	custName := c.PostForm("cust_name")
	if custID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Customer pengirim wajib dipilih"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Berkas CSV tidak ditemukan"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuka berkas"})
		return
	}
	defer src.Close()

	reader := csv.NewReader(src)
	// Lewati Header
	_, _ = reader.Read()

	tx := database.Begin()
	count := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 6 {
			continue
		}

		// Asumsi Format CSV: NoSJ, Penerima, Alamat, KotaTujuan, NamaBarang, Koli, Berat, Nilai
		noSJ := strings.TrimSpace(record[0])
		penerima := strings.TrimSpace(record[1])
		alamat := strings.TrimSpace(record[2])
		kotaTujuan := strings.TrimSpace(record[3])
		namaBarang := strings.TrimSpace(record[4])
		colly, _ := strconv.Atoi(strings.TrimSpace(record[5]))
		berat, _ := strconv.ParseFloat(strings.TrimSpace(record[6]), 64)
		nilai, _ := strconv.ParseFloat(strings.TrimSpace(record[7]), 64)

		pliID := fmt.Sprintf("PLI%d%04d", time.Now().Unix(), count+1)

		insertH := `
			INSERT INTO public.mkt_t_epackinglisth (
				pli_id, pli_tanggal, pli_asalcustid, pli_asalname, pli_tujuannama,
				pli_tujuanalamat, pli_tujuankota, pli_nosuratjalan, pli_aktifyn
			) VALUES (?, NOW(), ?, ?, ?, ?, ?, ?, 'Y')
		`
		if err := tx.Exec(insertH, pliID, custID, custName, penerima, alamat, kotaTujuan, noSJ).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan baris header CSV"})
			return
		}

		insertD := `
			INSERT INTO public.mkt_t_epackinglistd (
				pli_h_id, pli_namabarang, pli_jeniskemasan, pli_jmlkemasan, pli_beratasli, pli_nilaibarang
			) VALUES (?, ?, 'DUS', ?, ?, ?)
		`
		if err := tx.Exec(insertD, pliID, namaBarang, colly, berat, nilai).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan rincian barang CSV"})
			return
		}

		count++
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Berhasil mengimpor %d dokumen packing list", count),
	})
}
