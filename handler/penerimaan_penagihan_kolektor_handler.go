package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type PenerimaanPenagihanHeaderRow struct {
	ArttihID         string    `json:"arttih_id" gorm:"column:arttih_id"`
	ArttihTanggal    time.Time `json:"arttih_tanggal" gorm:"column:arttih_tanggal"`
	ArttihTanggalStr string    `json:"arttih_tanggal_str" gorm:"-"`
	ArttihKryNIP     string    `json:"arttih_krynip" gorm:"column:arttih_krynip"`
	KryNama          string    `json:"kry_nama" gorm:"column:kry_nama"`
	JumlahInvoice    int64     `json:"jumlah_invoice" gorm:"column:jumlah_invoice"`
	JumlahTerbayar   int64     `json:"jumlah_terbayar" gorm:"column:jumlah_terbayar"`
	TotalTagihan     float64   `json:"total_tagihan" gorm:"column:total_tagihan"`
	TotalTerbayar    float64   `json:"total_terbayar" gorm:"column:total_terbayar"`
	ArttihBatalYN    string    `json:"arttih_batalyn" gorm:"column:arttih_batalyn"`
}

type PenerimaanPenagihanDetailItem struct {
	ArttidID           int64   `json:"arttid_id" gorm:"column:arttid_id"`
	ArttidArttihID     string  `json:"arttid_arttihid" gorm:"column:arttid_arttihid"`
	ArttidArtihID      string  `json:"arttid_artihid" gorm:"column:arttid_artihid"`
	ArtihCustName      string  `json:"artih_custname" gorm:"column:artih_custname"`
	ArtihNoKW          string  `json:"artih_nokw" gorm:"column:artih_nokw"`
	ArtihTotal         float64 `json:"artih_total" gorm:"column:artih_total"`
	ArtihSisaBayar     float64 `json:"artih_sisabayar" gorm:"column:artih_sisabayar"`
	ArttidBayarYN      string  `json:"arttid_bayaryn" gorm:"column:arttid_bayaryn"`
	ArttidBayarNominal float64 `json:"arttid_bayarnominal" gorm:"column:arttid_bayarnominal"`
	ArttidKeterangan   string  `json:"arttid_keterangan" gorm:"column:arttid_keterangan"`
}

// GET /api/piutang/penerimaan-penagihan-kolektor
func GetPenerimaanPenagihanListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	kolektor := strings.TrimSpace(c.Query("kolektor"))
	noPenagihan := strings.TrimSpace(c.Query("no_penagihan"))
	noInvoice := strings.TrimSpace(c.Query("no_invoice"))

	// 1. Cek apakah kolom arttid_bayaryn ada di tabel
	var hasBayarCol int64
	database.Raw(`
		SELECT COUNT(1) 
		FROM information_schema.columns 
		WHERE LOWER(table_name) = 'art_t_tagihinvoiced' AND LOWER(column_name) = 'arttid_bayaryn'
	`).Scan(&hasBayarCol)

	var selectTerbayar string
	if hasBayarCol > 0 {
		selectTerbayar = `
			COUNT(CASE WHEN d.arttid_bayaryn = 'Y' THEN 1 END) AS jumlah_terbayar,
			COALESCE(SUM(CASE WHEN d.arttid_bayaryn = 'Y' THEN COALESCE(d.arttid_bayarnominal, inv.artih_total * 1.011) ELSE 0 END), 0) AS total_terbayar
		`
	} else {
		selectTerbayar = `
			0 AS jumlah_terbayar,
			0 AS total_terbayar
		`
	}

	selectClause := fmt.Sprintf(`
		h.arttih_id,
		h.arttih_tanggal,
		COALESCE(h.arttih_krynip, '') AS arttih_krynip,
		COALESCE(k.kry_nama, h.arttih_krynip, '-') AS kry_nama,
		COALESCE(h.arttih_batalyn, 'N') AS arttih_batalyn,
		COUNT(d.arttid_artihid) AS jumlah_invoice,
		COALESCE(SUM(inv.artih_total * 1.011), 0) AS total_tagihan,
		%s
	`, selectTerbayar)

	query := database.Table("public.art_t_tagihinvoiceh h").
		Select(selectClause).
		Joins("LEFT JOIN public.hrd_m_karyawan k ON TRIM(k.kry_nip::varchar) = TRIM(h.arttih_krynip::varchar)").
		Joins("LEFT JOIN public.art_t_tagihinvoiced d ON d.arttid_arttihid = h.arttih_id").
		Joins("LEFT JOIN public.art_t_invoiceh inv ON inv.artih_id = d.arttid_artihid")

	if startDate != "" && endDate != "" {
		query = query.Where("h.arttih_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if kolektor != "" {
		query = query.Where("(k.kry_nama ILIKE ? OR h.arttih_krynip ILIKE ?)", "%"+kolektor+"%", "%"+kolektor+"%")
	}
	if noPenagihan != "" {
		query = query.Where("h.arttih_id ILIKE ?", "%"+noPenagihan+"%")
	}
	if noInvoice != "" {
		query = query.Where("d.arttid_artihid ILIKE ?", "%"+noInvoice+"%")
	}

	list := make([]PenerimaanPenagihanHeaderRow, 0)
	err := query.Group("h.arttih_id, h.arttih_tanggal, h.arttih_krynip, k.kry_nama, h.arttih_batalyn").
		Order("h.arttih_tanggal DESC, h.arttih_id DESC").
		Limit(300).
		Scan(&list).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat penerimaan penagihan: " + err.Error()})
		return
	}

	for i := range list {
		list[i].ArttihTanggalStr = list[i].ArttihTanggal.Format("2006-01-02")
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// GET /api/piutang/penerimaan-penagihan-kolektor/detail
func GetPenerimaanPenagihanDetailHandler(c *gin.Context) {
	tagihID := strings.TrimSpace(c.Query("id"))
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var head PenerimaanPenagihanHeaderRow
	err := database.Table("public.art_t_tagihinvoiceh h").
		Select("h.arttih_id, h.arttih_tanggal, COALESCE(h.arttih_krynip, '') AS arttih_krynip, COALESCE(k.kry_nama, h.arttih_krynip, '-') AS kry_nama, COALESCE(h.arttih_batalyn, 'N') AS arttih_batalyn").
		Joins("LEFT JOIN public.hrd_m_karyawan k ON TRIM(k.kry_nip::varchar) = TRIM(h.arttih_krynip::varchar)").
		Where("TRIM(h.arttih_id) = TRIM(?)", tagihID).
		Scan(&head).Error

	if err != nil || head.ArttihID == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Dokumen penagihan tidak ditemukan"})
		return
	}

	var hasBayarCol int64
	database.Raw(`
		SELECT COUNT(1) 
		FROM information_schema.columns 
		WHERE LOWER(table_name) = 'art_t_tagihinvoiced' AND LOWER(column_name) = 'arttid_bayaryn'
	`).Scan(&hasBayarCol)

	var selectFields string
	if hasBayarCol > 0 {
		selectFields = `
			d.arttid_arttihid,
			d.arttid_artihid,
			COALESCE(i.artih_custname, '') AS artih_custname,
			COALESCE(i.artih_nokw, '') AS artih_nokw,
			COALESCE(i.artih_total * 1.011, 0) AS artih_total,
			COALESCE(i.artih_sisabayar, i.artih_total * 1.011, 0) AS artih_sisabayar,
			COALESCE(d.arttid_bayaryn, 'N') AS arttid_bayaryn,
			COALESCE(d.arttid_bayarnominal, i.artih_total * 1.011, 0) AS arttid_bayarnominal,
			COALESCE(d.arttid_keterangan, '') AS arttid_keterangan
		`
	} else {
		selectFields = `
			d.arttid_arttihid,
			d.arttid_artihid,
			COALESCE(i.artih_custname, '') AS artih_custname,
			COALESCE(i.artih_nokw, '') AS artih_nokw,
			COALESCE(i.artih_total * 1.011, 0) AS artih_total,
			COALESCE(i.artih_sisabayar, i.artih_total * 1.011, 0) AS artih_sisabayar,
			'N' AS arttid_bayaryn,
			COALESCE(i.artih_total * 1.011, 0) AS arttid_bayarnominal,
			'' AS arttid_keterangan
		`
	}

	details := make([]PenerimaanPenagihanDetailItem, 0)
	database.Table("public.art_t_tagihinvoiced d").
		Select(selectFields).
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

// POST /api/piutang/penerimaan-penagihan-kolektor/konfirmasi
func ConfirmPenerimaanPenagihanHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	type ItemConfirm struct {
		InvoiceID  string  `json:"invoice_id"`
		BayarYN    string  `json:"bayar_yn"`
		Nominal    float64 `json:"nominal"`
		Keterangan string  `json:"keterangan"`
	}

	var input struct {
		NoPenagihan string        `json:"no_penagihan"`
		Items       []ItemConfirm `json:"items"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	tx := database.Begin()

	for _, itm := range input.Items {
		updQuery := `
			UPDATE public.art_t_tagihinvoiced 
			SET arttid_bayaryn = ?, 
			    arttid_bayarnominal = ?, 
			    arttid_bayartgl = NOW(), 
			    arttid_keterangan = ?
			WHERE TRIM(arttid_arttihid) = TRIM(?) AND TRIM(arttid_artihid) = TRIM(?)
		`
		if err := tx.Exec(updQuery, itm.BayarYN, itm.Nominal, itm.Keterangan, input.NoPenagihan, itm.InvoiceID).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update hasil tagih: " + err.Error()})
			return
		}

		if itm.BayarYN == "Y" {
			tx.Exec(`
				UPDATE public.art_t_invoiceh 
				SET artih_dibayar = COALESCE(artih_dibayar, 0) + ?,
				    artih_sisabayar = GREATEST(0, COALESCE(artih_total * 1.011, 0) - (COALESCE(artih_dibayar, 0) + ?))
				WHERE TRIM(artih_id) = TRIM(?)
			`, itm.Nominal, itm.Nominal, itm.InvoiceID)
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Konfirmasi penerimaan penagihan berhasil disimpan"})
}
