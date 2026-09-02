package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type TagihInvoiceHeader struct {
	ArttihID         string    `json:"arttih_id" gorm:"column:arttih_id"`
	ArttihTanggal    time.Time `json:"arttih_tanggal" gorm:"column:arttih_tanggal"`
	ArttihTanggalStr string    `json:"arttih_tanggal_str" gorm:"-"`
	KryNIP           string    `json:"arttih_krynip" gorm:"column:arttih_krynip"`
	KryNama          string    `json:"kry_nama" gorm:"column:kry_nama"`
	BatalYN          string    `json:"arttih_batalyn" gorm:"column:arttih_batalyn"`
	JumlahInvoice    int64     `json:"jumlah_invoice" gorm:"column:jumlah_invoice"`
	TotalNominal     float64   `json:"total_nominal" gorm:"column:total_nominal"`
}

type AvailableInvoiceRow struct {
	InvoiceID string    `json:"artih_id" gorm:"column:artih_id"`
	Tanggal   time.Time `json:"artih_tanggal" gorm:"column:artih_tanggal"`
	CustID    string    `json:"artih_custid" gorm:"column:artih_custid"`
	CustName  string    `json:"artih_custname" gorm:"column:artih_custname"`
	NoKW      string    `json:"artih_nokw" gorm:"column:artih_nokw"`
	Total     float64   `json:"artih_total" gorm:"column:artih_total"`
	Dibayar   float64   `json:"artih_dibayar" gorm:"column:artih_dibayar"`
	SisaBayar float64   `json:"artih_sisabayar" gorm:"column:artih_sisabayar"`
}

type TagihInvoiceDetailView struct {
	InvoiceID string  `json:"artih_id" gorm:"column:artih_id"`
	Tanggal   string  `json:"artih_tanggal" gorm:"column:artih_tanggal"`
	CustID    string  `json:"artih_custid" gorm:"column:artih_custid"`
	CustName  string  `json:"artih_custname" gorm:"column:artih_custname"`
	NoKW      string  `json:"artih_nokw" gorm:"column:artih_nokw"`
	Total     float64 `json:"artih_total" gorm:"column:artih_total"`
	SisaBayar float64 `json:"artih_sisabayar" gorm:"column:artih_sisabayar"`
}

// GET /api/piutang/tagih-invoice
func GetTagihInvoiceListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	kolektor := strings.TrimSpace(c.Query("kolektor"))
	noPenagihan := strings.TrimSpace(c.Query("no_penagihan"))
	noKwitansi := strings.TrimSpace(c.Query("no_kwitansi"))

	query := database.Table("public.art_t_tagihinvoiceh h").
		Select(`
			h.arttih_id,
			h.arttih_tanggal,
			COALESCE(h.arttih_krynip, '') AS arttih_krynip,
			COALESCE(k.kry_nama, h.arttih_krynip, '-') AS kry_nama,
			COALESCE(h.arttih_batalyn, 'N') AS arttih_batalyn,
			COUNT(d.arttid_artihid) AS jumlah_invoice,
			COALESCE(SUM(inv.artih_total * 1.011), 0) AS total_nominal
		`).
		Joins("LEFT JOIN public.hrd_m_karyawan k ON TRIM(k.kry_nip::varchar) = TRIM(h.arttih_krynip::varchar)").
		Joins("LEFT JOIN public.art_t_tagihinvoiced d ON d.arttid_arttihid = h.arttih_id").
		Joins("LEFT JOIN public.art_t_invoiceh inv ON inv.artih_id = d.arttid_artihid")

	// Filter tanggal opsional
	if startDate != "" && endDate != "" {
		query = query.Where("h.arttih_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if kolektor != "" {
		query = query.Where("(k.kry_nama ILIKE ? OR h.arttih_krynip ILIKE ?)", "%"+kolektor+"%", "%"+kolektor+"%")
	}
	if noPenagihan != "" {
		query = query.Where("h.arttih_id ILIKE ?", "%"+noPenagihan+"%")
	}
	if noKwitansi != "" {
		query = query.Where("inv.artih_nokw ILIKE ?", "%"+noKwitansi+"%")
	}

	// Inisialisasi slice agar tidak null di JSON
	list := make([]TagihInvoiceHeader, 0)
	err := query.Group("h.arttih_id, h.arttih_tanggal, h.arttih_krynip, k.kry_nama, h.arttih_batalyn").
		Order("h.arttih_tanggal DESC, h.arttih_id DESC").
		Limit(300).
		Scan(&list).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat daftar penagihan: " + err.Error()})
		return
	}

	for i := range list {
		list[i].ArttihTanggalStr = list[i].ArttihTanggal.Format("2006-01-02")
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// GET /api/piutang/tagih-invoice/available-invoices
func GetAvailableInvoicesForTagih(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	search := strings.TrimSpace(c.Query("search"))

	// Invoice aktif yang belum lunas dan belum berada di manifest penagihan aktif
	query := database.Table("public.art_t_invoiceh i").
		Select(`
			i.artih_id,
			i.artih_tanggal,
			COALESCE(i.artih_custid, '') AS artih_custid,
			COALESCE(i.artih_custname, '') AS artih_custname,
			COALESCE(i.artih_nokw, '') AS artih_nokw,
			COALESCE(i.artih_total, 0) AS artih_total,
			COALESCE(i.artih_dibayar, 0) AS artih_dibayar,
			COALESCE(i.artih_sisabayar, i.artih_total, 0) AS artih_sisabayar
		`).
		Where("COALESCE(i.artih_delete, 'N') = 'N' AND COALESCE(i.artih_terbayar, 'N') = 'N'").
		Where(`NOT EXISTS (
			SELECT 1 FROM public.art_t_tagihinvoiced d 
			INNER JOIN public.art_t_tagihinvoiceh h ON h.arttih_id = d.arttid_arttihid
			WHERE d.arttid_artihid = i.artih_id AND COALESCE(h.arttih_batalyn, 'N') = 'N'
		)`)

	if search != "" {
		query = query.Where("(i.artih_id ILIKE ? OR i.artih_custname ILIKE ? OR i.artih_nokw ILIKE ?)", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var invoices []AvailableInvoiceRow
	if err := query.Order("i.artih_tanggal DESC").Limit(100).Scan(&invoices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query invoice siap tagih: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": invoices})
}

// GET /api/piutang/tagih-invoice/detail/:id
func GetTagihInvoiceDetailByID(c *gin.Context) {
	tagihID := strings.TrimSpace(c.Query("id"))
	if tagihID == "" {
		tagihID = strings.TrimPrefix(c.Param("id"), "/")
	}

	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	type HeaderRes struct {
		ArttihID      string    `json:"arttih_id"`
		ArttihTanggal time.Time `json:"arttih_tanggal"`
		KryNIP        string    `json:"arttih_krynip"`
		KryNama       string    `json:"kry_nama"`
		BatalYN       string    `json:"arttih_batalyn"`
	}

	var head HeaderRes
	err := database.Table("public.art_t_tagihinvoiceh h").
		Select("h.arttih_id, h.arttih_tanggal, COALESCE(h.arttih_krynip, '') AS kry_nip, COALESCE(k.kry_nama, h.arttih_krynip, '-') AS kry_nama, COALESCE(h.arttih_batalyn, 'N') AS batal_yn").
		Joins("LEFT JOIN public.hrd_m_karyawan k ON TRIM(k.kry_nip::varchar) = TRIM(h.arttih_krynip::varchar)").
		Where("TRIM(h.arttih_id) = TRIM(?)", tagihID).
		Scan(&head).Error

	if err != nil || head.ArttihID == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data penagihan tidak ditemukan"})
		return
	}

	var details []TagihInvoiceDetailView
	database.Table("public.art_t_tagihinvoiced d").
		Select(`
			d.arttid_artihid AS artih_id,
			TO_CHAR(i.artih_tanggal, 'YYYY-MM-DD') AS artih_tanggal,
			COALESCE(i.artih_custid, '') AS artih_custid,
			COALESCE(i.artih_custname, '') AS artih_custname,
			COALESCE(i.artih_nokw, '') AS artih_nokw,
			COALESCE(i.artih_total, 0) AS artih_total,
			COALESCE(i.artih_sisabayar, i.artih_total, 0) AS artih_sisabayar
		`).
		Joins("LEFT JOIN public.art_t_invoiceh i ON i.artih_id = d.arttid_artihid").
		Where("TRIM(d.arttid_arttihid) = TRIM(?)", tagihID).
		Scan(&details)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"header":  head,
			"details": details,
		},
	})
}

// POST /api/piutang/tagih-invoice (Tambah Penugasan Penagihan Kolektor)
func CreateTagihInvoiceHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var input struct {
		Tanggal    string   `json:"tanggal"`
		KryNIP     string   `json:"kry_nip"`
		InvoiceIDs []string `json:"invoice_ids"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if len(input.InvoiceIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Pilih minimal 1 invoice untuk ditugaskan ke kolektor"})
		return
	}

	// Generate Nomor Dokumen Penagihan (Format: 001/TAG/YYYYMM/0001)
	tglParsed, _ := time.Parse("2006-01-02", input.Tanggal)
	yearMonth := tglParsed.Format("200601")
	var count int64
	database.Table("public.art_t_tagihinvoiceh").Where("arttih_id LIKE ?", "%/TAG/"+yearMonth+"/%").Count(&count)
	newTagihID := fmt.Sprintf("001/TAG/%s/%04d", yearMonth, count+1)

	tx := database.Begin()

	// 1. Simpan Header
	headerQuery := `
		INSERT INTO public.art_t_tagihinvoiceh (arttih_id, arttih_tanggal, arttih_krynip, arttih_batalyn, arttih_updateid, arttih_updatetime)
		VALUES (?, ?, ?, 'N', 'USER', NOW())
	`
	if err := tx.Exec(headerQuery, newTagihID, input.Tanggal+" 10:00:00", input.KryNIP).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan header penagihan: " + err.Error()})
		return
	}

	// 2. Simpan Detail Invoice
	for _, invID := range input.InvoiceIDs {
		detailQuery := `INSERT INTO public.art_t_tagihinvoiced (arttid_arttihid, arttid_artihid) VALUES (?, ?)`
		if err := tx.Exec(detailQuery, newTagihID, invID).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan detail invoice: " + err.Error()})
			return
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Penugasan penagihan kolektor berhasil dibuat", "arttih_id": newTagihID})
}

// PUT /api/piutang/tagih-invoice/batal/:id (Membatalkan Manifest Penagihan)
func CancelTagihInvoiceHandler(c *gin.Context) {
	tagihID := strings.TrimSpace(c.Query("id"))
	if tagihID == "" {
		tagihID = strings.TrimPrefix(c.Param("id"), "/")
	}

	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	err := database.Table("public.art_t_tagihinvoiceh").
		Where("TRIM(arttih_id) = TRIM(?)", tagihID).
		Update("arttih_batalyn", "Y").Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membatalkan dokumen penagihan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Dokumen penagihan berhasil dibatalkan"})
}

// GET /api/karyawan/kolektor-dropdown
func GetKolektorDropdownHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	type KolektorOpt struct {
		KryNIP  string `json:"kry_nip" gorm:"column:kry_nip"`
		KryNama string `json:"kry_nama" gorm:"column:kry_nama"`
	}

	var kolektors []KolektorOpt
	database.Table("public.hrd_m_karyawan").
		Select("kry_nip, kry_nama").
		Where("COALESCE(kry_aktifyn, 'Y') = 'Y'").
		Order("kry_nama ASC").
		Scan(&kolektors)

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": kolektors})
}
