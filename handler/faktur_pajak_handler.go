package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type FakturNoModel struct {
	FakturNoHead       string    `json:"fakturno_head" gorm:"column:fakturno_head"`
	FakturNoStart      string    `json:"fakturno_start" gorm:"column:fakturno_start"`
	FakturNoEnd        string    `json:"fakturno_end" gorm:"column:fakturno_end"`
	FakturNoUpdateID   string    `json:"fakturno_updateid" gorm:"column:fakturno_updateid"`
	FakturNoUpdateTime time.Time `json:"fakturno_updatetime" gorm:"column:fakturno_updatetime"`
}

type FakturUsageRow struct {
	NoInvoice    string    `json:"no_invoice" gorm:"column:artih_id"`
	TglInvoice   time.Time `json:"tgl_invoice" gorm:"column:artih_tanggal"`
	CustName     string    `json:"cust_name" gorm:"column:cust_name"`
	FakturPajak  string    `json:"faktur_pajak" gorm:"column:artih_fktpajak"`
	TotalTagihan float64   `json:"total_tagihan" gorm:"column:artih_total"`
}

type UpdateFakturNoReq struct {
	KdHead  string `json:"kdhead" binding:"required"`
	KdStart string `json:"kdstart" binding:"required"`
	KdEnd   string `json:"kdend" binding:"required"`
}

// 1. GET /api/piutang/faktur-pajak (Ambil konfigurasi nomor seri faktur pajak saat ini)
// GET /api/piutang/faktur-pajak
func GetFakturPajakHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var data FakturNoModel
	err := database.Table("public.art_m_fakturno").
		Select("COALESCE(fakturno_head, '') AS fakturno_head, COALESCE(fakturno_start, '') AS fakturno_start, COALESCE(fakturno_end, '') AS fakturno_end, COALESCE(fakturno_updateid, '') AS fakturno_updateid, COALESCE(fakturno_updatetime, NOW()) AS fakturno_updatetime").
		Take(&data).Error

	if err != nil {
		data = FakturNoModel{
			FakturNoHead:  "010.1111",
			FakturNoStart: "00000001",
			FakturNoEnd:   "00010000",
		}
	}

	// Ambil daftar invoice terbaru yang sudah menggunakan Faktur Pajak
	var usageList []FakturUsageRow
	database.Table("public.art_t_invoiceh h").
		Select(`
			h.artih_id,
			h.artih_tanggal,
			COALESCE(c.cust_name, h.artih_custname, '') AS cust_name,
			h.artih_fktpajak,
			COALESCE(h.artih_total::numeric, 0) AS artih_total
		`).
		Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id::varchar) = TRIM(h.artih_custid::varchar)").
		Where("h.artih_fktpajak IS NOT NULL AND TRIM(h.artih_fktpajak) <> '' AND COALESCE(h.artih_delete, 'N') <> 'Y'").
		Order("h.artih_updatetime DESC, h.artih_tanggal DESC").
		Limit(100).
		Scan(&usageList)

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"data":       data,
		"usage_list": usageList,
	})
}

// 2. POST /api/piutang/faktur-pajak/update (Update rentang nomor faktur pajak berjalan)
func UpdateFakturPajakHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	userID, _ := c.Get("username")

	var req UpdateFakturNoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Semua field (Header, Kode Start, Kode End) wajib diisi."})
		return
	}

	kdHead := strings.TrimSpace(req.KdHead)
	kdStart := strings.TrimSpace(req.KdStart)
	kdEnd := strings.TrimSpace(req.KdEnd)

	var count int64
	database.Table("public.art_m_fakturno").Count(&count)

	if count == 0 {
		// Insert jika belum ada data
		if err := database.Table("public.art_m_fakturno").Create(map[string]interface{}{
			"fakturno_head":       kdHead,
			"fakturno_start":      kdStart,
			"fakturno_end":        kdEnd,
			"fakturno_updateid":   userID,
			"fakturno_updatetime": time.Now(),
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan data faktur pajak: " + err.Error()})
			return
		}
	} else {
		// Update baris aktif
		if err := database.Table("public.art_m_fakturno").
			Where("1=1").
			Updates(map[string]interface{}{
				"fakturno_head":       kdHead,
				"fakturno_start":      kdStart,
				"fakturno_end":        kdEnd,
				"fakturno_updateid":   userID,
				"fakturno_updatetime": time.Now(),
			}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update faktur pajak: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Konfigurasi Nomor Faktur Pajak Berjalan berhasil diperbarui.",
	})
}
