package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type KondisiBTTItem struct {
	TipeDokumen    string     `json:"tipe_dokumen" gorm:"column:tipe_dokumen"`
	NoDokumen      string     `json:"no_dokumen" gorm:"column:no_dokumen"`
	TglDokumen     time.Time  `json:"tgl_dokumen" gorm:"column:tgl_dokumen"`
	CustID         string     `json:"cust_id" gorm:"column:cust_id"`
	CustName       string     `json:"cust_name" gorm:"column:cust_name"`
	PIC            string     `json:"pic" gorm:"column:pic"`
	KotaTujuan     string     `json:"kota_tujuan" gorm:"column:kota_tujuan"`
	Penerima       string     `json:"penerima" gorm:"column:penerima"`
	Koli           float64    `json:"koli" gorm:"column:koli"`
	Berat          float64    `json:"berat" gorm:"column:berat"`
	TotalBiaya     float64    `json:"total_biaya" gorm:"column:total_biaya"`
	MetodeBayar    string     `json:"metode_bayar" gorm:"column:metode_bayar"`
	BTTKembaliYN   string     `json:"btt_kembali_yn" gorm:"column:btt_kembali_yn"`
	SudahInvoiceYN string     `json:"sudah_invoice_yn" gorm:"column:sudah_invoice_yn"`
	NoInvoice      string     `json:"no_invoice" gorm:"column:no_invoice"`
	TerbayarYN     string     `json:"terbayar_yn" gorm:"column:terbayar_yn"`
	TglPelunasan   *time.Time `json:"tgl_pelunasan" gorm:"column:tgl_pelunasan"`
	AgenAsalID     string     `json:"agen_asal_id" gorm:"column:agen_asal_id"`
	AgenAsalNama   string     `json:"agen_asal_nama" gorm:"column:agen_asal_nama"`
}

// 1. GET /api/piutang/kondisi-btt/options (Opsi PIC Marketing)
func GetKondisiBTTOptionsHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var picList []string
	_ = database.Table("public.mkt_m_customer").
		Where("cust_pic IS NOT NULL AND TRIM(cust_pic) <> ''").
		Distinct("cust_pic").
		Order("cust_pic ASC").
		Pluck("cust_pic", &picList)

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"pic_list": picList,
	})
}

// 2. GET /api/piutang/kondisi-btt (List Data Rekonsiliasi)
func GetKondisiBTTListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	cabangAsal := strings.TrimSpace(c.Query("cabang_asal"))
	pic := strings.TrimSpace(c.Query("pic"))
	customer := strings.TrimSpace(c.Query("customer"))
	sudahInvoice := strings.TrimSpace(c.Query("sudah_invoice"))
	bayar := strings.TrimSpace(c.Query("bayar"))
	pembayaran := strings.TrimSpace(c.Query("pembayaran"))
	bypassTanggal := c.Query("bypass_tanggal") == "true" || c.Query("bypass_tanggal") == "1"

	query := database.Table("public.mkt_t_econote e").
		Select(`
			'BTT' AS tipe_dokumen,
			e.bttt_id::varchar AS no_dokumen,
			e.bttt_tanggal AS tgl_dokumen,
			COALESCE(e.bttt_asalcustid::varchar, '') AS cust_id,
			COALESCE(c.cust_name, e.bttt_asalname, '') AS cust_name,
			COALESCE(c.cust_pic, '-') AS pic,
			COALESCE(e.bttt_tujuankota, '') AS kota_tujuan,
			COALESCE(e.bttt_tujuannama, '') AS penerima,
			COALESCE(e.bttt_jmlunit::numeric, 0) AS koli,
			COALESCE(e.bttt_berat::numeric, 0) AS berat,
			(COALESCE(e.bttt_harga::numeric, 0) + COALESCE(e.bttt_biayapenerus::numeric, 0)) AS total_biaya,
			CASE 
				WHEN e.bttt_pembayaran::varchar = '1' THEN 'TUNAI'
				WHEN e.bttt_pembayaran::varchar = '2' THEN 'KREDIT'
				ELSE 'TAGIH TURUN'
			END AS metode_bayar,
			'N' AS btt_kembali_yn,
			CASE WHEN invd.artid_artihid IS NOT NULL THEN 'Y' ELSE 'N' END AS sudah_invoice_yn,
			COALESCE(invd.artid_artihid::varchar, '-') AS no_invoice,
			COALESCE(e.bttt_bayaryn, 'N') AS terbayar_yn,
			NULL::timestamp AS tgl_pelunasan,
			LPAD(LEFT(e.bttt_id::varchar, 3), 3, '0') AS agen_asal_id,
			COALESCE(a.agen_nama, 'DLI PUSAT') AS agen_asal_nama
		`).
		Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id::varchar) = TRIM(e.bttt_asalcustid::varchar)").
		Joins("LEFT JOIN public.glb_m_agen a ON a.agen_id::varchar = LPAD(LEFT(e.bttt_id::varchar, 3), 3, '0')").
		Joins("LEFT JOIN public.art_t_invoiced invd ON TRIM(LOWER(invd.artid_bttid::varchar)) = TRIM(LOWER(e.bttt_id::varchar))").
		Where("COALESCE(e.bttt_aktifyn, 'Y') = 'Y'")

	if !bypassTanggal && startDate != "" && endDate != "" {
		query = query.Where("e.bttt_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if cabangAsal != "" && cabangAsal != "ALL" {
		query = query.Where("LPAD(LEFT(e.bttt_id::varchar, 3), 3, '0') = LPAD(?, 3, '0')", cabangAsal)
	}
	if pic != "" {
		query = query.Where("c.cust_pic ILIKE ?", "%"+pic+"%")
	}
	if customer != "" {
		query = query.Where("(c.cust_name ILIKE ? OR e.bttt_asalname ILIKE ? OR e.bttt_asalcustid::varchar ILIKE ?)", "%"+customer+"%", "%"+customer+"%", "%"+customer+"%")
	}
	if sudahInvoice == "Y" {
		query = query.Where("invd.artid_artihid IS NOT NULL")
	} else if sudahInvoice == "N" {
		query = query.Where("invd.artid_artihid IS NULL")
	}
	if bayar != "" {
		query = query.Where("COALESCE(e.bttt_bayaryn, 'N') = ?", bayar)
	}
	if pembayaran == "TUNAI" {
		query = query.Where("e.bttt_pembayaran::varchar = '1'")
	} else if pembayaran == "KREDIT" {
		query = query.Where("e.bttt_pembayaran::varchar = '2'")
	} else if pembayaran == "TAGIH TURUN" {
		query = query.Where("e.bttt_pembayaran::varchar NOT IN ('1', '2')")
	}

	var list []KondisiBTTItem
	if err := query.Order("e.bttt_tanggal DESC, e.bttt_id DESC").Limit(500).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query kondisi BTT: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}
