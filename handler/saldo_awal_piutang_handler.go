package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type SaldoAwalCustomerRow struct {
	Tahun     string  `json:"sa_tahun" gorm:"column:sa_tahun"`
	CustID    string  `json:"sa_custid" gorm:"column:sa_custid"`
	CustName  string  `json:"cust_name" gorm:"column:cust_name"`
	AgenID    string  `json:"sa_agenid" gorm:"column:sa_agenid"`
	AgenNama  string  `json:"agen_nama" gorm:"column:agen_nama"`
	SaldoAwal float64 `json:"sa_awal" gorm:"column:sa_awal"`
}

// GET /api/piutang/saldo-awal/list
func GetListSaldoAwalPiutangHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	agenID := strings.TrimSpace(c.Query("agen_id"))
	tahun := strings.TrimSpace(c.Query("tahun"))
	if tahun == "" {
		tahun = strconv.Itoa(time.Now().Year())
	}

	rawQuery := `
		SELECT 
			c.cust_id AS sa_custid,
			c.cust_name,
			COALESCE(c.cust_agenid, a.agen_id::varchar, '1') AS sa_agenid,
			COALESCE(a.agen_nama, '-') AS agen_nama,
			? AS sa_tahun,
			COALESCE(sa.sa_awal, 0) AS sa_awal
		FROM public.mkt_m_customer c
		LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(c.cust_agenid::varchar)
		LEFT JOIN public.ar_t_sapiutang sa ON TRIM(sa.sa_custid) = TRIM(c.cust_id) 
		                                   AND sa.sa_tahun = ?
		WHERE c.cust_aktifyn = 'Y'
	`
	var args []interface{}
	args = append(args, tahun, tahun)

	if agenID != "" && agenID != "ALL" {
		rawQuery += " AND (TRIM(a.agen_id::varchar) = TRIM(?) OR TRIM(a.agen_cabangid::varchar) = TRIM(?) OR TRIM(c.cust_agenid::varchar) = TRIM(?))"
		args = append(args, agenID, agenID, agenID)
	}

	rawQuery += " ORDER BY c.cust_name ASC LIMIT 500"

	list := make([]SaldoAwalCustomerRow, 0)
	if err := database.Raw(rawQuery, args...).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membaca saldo awal: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// POST /api/piutang/saldo-awal/save
func SaveSaldoAwalPiutangHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var input struct {
		AgenID    string  `json:"agen_id"`
		CustID    string  `json:"cust_id"`
		Tahun     string  `json:"tahun"`
		SaldoAwal float64 `json:"saldo_awal"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if input.CustID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Customer wajib dipilih"})
		return
	}

	if input.Tahun == "" {
		input.Tahun = strconv.Itoa(time.Now().Year())
	}

	tx := database.Begin()

	var count int64
	tx.Table("public.ar_t_sapiutang").
		Where("sa_tahun = ? AND TRIM(sa_custid) = TRIM(?)", input.Tahun, input.CustID).
		Count(&count)

	if count == 0 {
		insQuery := `
			INSERT INTO public.ar_t_sapiutang (
				sa_tahun, sa_custid, sa_agenid, sa_awal, 
				sa_bln01, sa_bln02, sa_bln03, sa_bln04, sa_bln05, sa_bln06, 
				sa_bln07, sa_bln08, sa_bln09, sa_bln10, sa_bln11, sa_bln12
			) VALUES (?, ?, ?, ?, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
		`
		if err := tx.Exec(insQuery, input.Tahun, input.CustID, input.AgenID, input.SaldoAwal).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal insert saldo awal: " + err.Error()})
			return
		}
	} else {
		updQuery := `
			UPDATE public.ar_t_sapiutang 
			SET sa_awal = ?, sa_agenid = ? 
			WHERE sa_tahun = ? AND TRIM(sa_custid) = TRIM(?)
		`
		if err := tx.Exec(updQuery, input.SaldoAwal, input.AgenID, input.Tahun, input.CustID).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update saldo awal: " + err.Error()})
			return
		}
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Saldo awal untuk customer %s tahun %s berhasil disimpan", input.CustID, input.Tahun),
	})
}
