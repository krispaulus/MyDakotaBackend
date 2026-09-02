package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ProformaHeaderRow struct {
	PIHID         string     `json:"pih_id" gorm:"column:pih_id"`
	PIHTanggal    time.Time  `json:"pih_tanggal" gorm:"column:pih_tanggal"`
	PIHTanggalStr string     `json:"pih_tanggal_str" gorm:"-"`
	PIHCustID     string     `json:"pih_custid" gorm:"column:pih_custid"`
	CustName      string     `json:"cust_name" gorm:"column:cust_name"`
	PIHAgenID     string     `json:"pih_agenid" gorm:"column:pih_agenid"`
	AgenNama      string     `json:"agen_nama" gorm:"column:agen_nama"`
	PIHNoMobil    string     `json:"pih_nomobil" gorm:"column:pih_nomobil"`
	PIHNmSopir    string     `json:"pih_nmsopir" gorm:"column:pih_nmsopir"`
	PIHNoDO       string     `json:"pih_nodo" gorm:"column:pih_nodo"`
	PIHTglPU      *time.Time `json:"pih_tglpu" gorm:"column:pih_tglpu"`
	PIHTglPUStr   string     `json:"pih_tglpu_str" gorm:"-"`
	PIHLokasiPU   string     `json:"pih_lokasipu" gorm:"column:pih_lokasipu"`
	PIHAktifYN    string     `json:"pih_aktifyn" gorm:"column:pih_aktifyn"`
	JumlahBTT     int64      `json:"jbtt" gorm:"column:jbtt"`
	TotalNominal  float64    `json:"total_nominal" gorm:"column:total_nominal"`
}

type ProformaBTTItem struct {
	BTTID         string  `json:"btt_id" gorm:"column:btt_id"`
	Tanggal       string  `json:"bttt_tanggal" gorm:"column:bttt_tanggal"`
	Penerima      string  `json:"penerima" gorm:"column:penerima"`
	Tujuan        string  `json:"tujuan" gorm:"column:tujuan"`
	Harga         float64 `json:"harga" gorm:"column:harga"`
	BiayaPenerus  float64 `json:"biaya_penerus" gorm:"column:biaya_penerus"`
	BiayaPacking  float64 `json:"biaya_packing" gorm:"column:biaya_packing"`
	BiayaAsuransi float64 `json:"biaya_asuransi" gorm:"column:biaya_asuransi"`
	TotalBTT      float64 `json:"total_btt" gorm:"column:total_btt"`
}

// GET /api/piutang/proforma-invoice
func GetProformaInvoiceListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	cabang := strings.TrimSpace(c.Query("cabang"))
	customer := strings.TrimSpace(c.Query("customer"))
	noBTT := strings.TrimSpace(c.Query("nobtt"))
	noPI := strings.TrimSpace(c.Query("nopi"))

	rawQuery := `
		SELECT 
			h.pih_id,
			h.pih_tanggal,
			COALESCE(h.pih_custid, '') AS pih_custid,
			COALESCE(c.cust_name, h.pih_custid, '-') AS cust_name,
			COALESCE(h.pih_agenid, '') AS pih_agenid,
			COALESCE(a.agen_nama, h.pih_agenid, '-') AS agen_nama,
			COALESCE(h.pih_nomobil, '') AS pih_nomobil,
			COALESCE(h.pih_nmsopir, '') AS pih_nmsopir,
			COALESCE(h.pih_nodo, '') AS pih_nodo,
			h.pih_tglpu,
			COALESCE(h.pih_lokasipu, '') AS pih_lokasipu,
			COALESCE(h.pih_aktifyn, 'Y') AS pih_aktifyn,
			COUNT(d.pid_bttid) AS jbtt,
			COALESCE(SUM(
				COALESCE(ec.bttt_harga, 0) + 
				COALESCE(ec.bttt_biayapenerus, 0) + 
				COALESCE(pck.pck_biaya, 0) + 
				COALESCE(asr.totalbiaya, 0)
			), 0) AS total_nominal
		FROM public.art_t_proformah h
		LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id::varchar) = TRIM(h.pih_custid::varchar)
		LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(h.pih_agenid::varchar)
		LEFT JOIN public.art_t_proformad d ON TRIM(d.pid_pihid) = TRIM(h.pih_id)
		LEFT JOIN public.mkt_t_econote ec ON TRIM(ec.bttt_id) = TRIM(d.pid_bttid)
		LEFT JOIN public.pck_t_packing pck ON pck.pck_id = ec.bttt_packingid
		LEFT JOIN public.mkt_t_asuransi asr ON TRIM(asr.bttt_id) = TRIM(ec.bttt_id)
		WHERE 1=1
	`

	var conditions []string
	var args []interface{}

	if startDate != "" && endDate != "" {
		conditions = append(conditions, "h.pih_tanggal BETWEEN ? AND ?")
		args = append(args, startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if cabang != "" && cabang != "ALL" {
		conditions = append(conditions, "(a.agen_nama ILIKE ? OR h.pih_agenid ILIKE ?)")
		args = append(args, "%"+cabang+"%", "%"+cabang+"%")
	}
	if customer != "" {
		conditions = append(conditions, "(c.cust_name ILIKE ? OR h.pih_custid ILIKE ?)")
		args = append(args, "%"+customer+"%", "%"+customer+"%")
	}
	if noPI != "" {
		conditions = append(conditions, "h.pih_id ILIKE ?")
		args = append(args, "%"+noPI+"%")
	}
	if noBTT != "" {
		conditions = append(conditions, "d.pid_bttid ILIKE ?")
		args = append(args, "%"+noBTT+"%")
	}

	if len(conditions) > 0 {
		rawQuery += " AND " + strings.Join(conditions, " AND ")
	}

	rawQuery += `
		GROUP BY h.pih_id, h.pih_tanggal, h.pih_custid, c.cust_name, h.pih_agenid, a.agen_nama, h.pih_nomobil, h.pih_nmsopir, h.pih_nodo, h.pih_tglpu, h.pih_lokasipu, h.pih_aktifyn
		ORDER BY h.pih_tanggal DESC, h.pih_id DESC
		LIMIT 300
	`

	list := make([]ProformaHeaderRow, 0)
	if err := database.Raw(rawQuery, args...).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat proforma invoice: " + err.Error()})
		return
	}

	for i := range list {
		list[i].PIHTanggalStr = list[i].PIHTanggal.Format("2006-01-02")
		if list[i].PIHTglPU != nil {
			list[i].PIHTglPUStr = list[i].PIHTglPU.Format("2006-01-02")
		} else {
			list[i].PIHTglPUStr = "-"
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// GET /api/piutang/proforma-invoice/detail
func GetProformaInvoiceDetailHandler(c *gin.Context) {
	pihID := strings.TrimSpace(c.Query("pih_id"))
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var head ProformaHeaderRow
	err := database.Table("public.art_t_proformah h").
		Select(`
			h.pih_id,
			h.pih_tanggal,
			COALESCE(h.pih_custid, '') AS pih_custid,
			COALESCE(c.cust_name, h.pih_custid, '-') AS cust_name,
			COALESCE(h.pih_agenid, '') AS pih_agenid,
			COALESCE(a.agen_nama, h.pih_agenid, '-') AS agen_nama,
			COALESCE(h.pih_nomobil, '') AS pih_nomobil,
			COALESCE(h.pih_nmsopir, '') AS pih_nmsopir,
			COALESCE(h.pih_nodo, '') AS pih_nodo,
			h.pih_tglpu,
			COALESCE(h.pih_lokasipu, '') AS pih_lokasipu,
			COALESCE(h.pih_aktifyn, 'Y') AS pih_aktifyn
		`).
		Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id::varchar) = TRIM(h.pih_custid::varchar)").
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(h.pih_agenid::varchar)").
		Where("TRIM(h.pih_id) = TRIM(?)", pihID).
		Scan(&head).Error

	if err != nil || head.PIHID == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Dokumen proforma invoice tidak ditemukan"})
		return
	}

	head.PIHTanggalStr = head.PIHTanggal.Format("2006-01-02")
	if head.PIHTglPU != nil {
		head.PIHTglPUStr = head.PIHTglPU.Format("2006-01-02")
	}

	// 1. Cek struktur kolom mkt_t_econote secara dinamis
	var ecCols []string
	database.Raw(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE LOWER(table_name) = 'mkt_t_econote'
	`).Scan(&ecCols)

	colPenerima := "'-' AS penerima"
	colTujuan := "'-' AS tujuan"

	for _, col := range ecCols {
		cl := strings.ToLower(col)
		if cl == "bttt_penerimanama" || cl == "bttt_namapenerima" || cl == "bttt_penerima" {
			colPenerima = fmt.Sprintf("COALESCE(ec.%s, '-') AS penerima", col)
		}
		if cl == "bttt_tujuannama" || cl == "bttt_namatujuan" || cl == "bttt_tujuan" {
			colTujuan = fmt.Sprintf("COALESCE(ec.%s, '-') AS tujuan", col)
		}
	}

	items := make([]ProformaBTTItem, 0)
	rawItems := fmt.Sprintf(`
		SELECT 
			d.pid_bttid AS btt_id,
			TO_CHAR(ec.bttt_tanggal, 'YYYY-MM-DD') AS bttt_tanggal,
			%s,
			%s,
			COALESCE(ec.bttt_harga, 0) AS harga,
			COALESCE(ec.bttt_biayapenerus, 0) AS biaya_penerus,
			COALESCE(pck.pck_biaya, 0) AS biaya_packing,
			COALESCE(asr.totalbiaya, 0) AS biaya_asuransi,
			(
				COALESCE(ec.bttt_harga, 0) + 
				COALESCE(ec.bttt_biayapenerus, 0) + 
				COALESCE(pck.pck_biaya, 0) + 
				COALESCE(asr.totalbiaya, 0)
			) AS total_btt
		FROM public.art_t_proformad d
		LEFT JOIN public.mkt_t_econote ec ON TRIM(ec.bttt_id) = TRIM(d.pid_bttid)
		LEFT JOIN public.pck_t_packing pck ON pck.pck_id = ec.bttt_packingid
		LEFT JOIN public.mkt_t_asuransi asr ON TRIM(asr.bttt_id) = TRIM(ec.bttt_id)
		WHERE TRIM(d.pid_pihid) = TRIM(?)
		ORDER BY d.pid_bttid ASC
	`, colPenerima, colTujuan)

	database.Raw(rawItems, pihID).Scan(&items)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"header": head,
			"items":  items,
		},
	})
}

// POST /api/piutang/proforma-invoice/create
func CreateProformaInvoiceHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var input struct {
		CustID   string   `json:"cust_id"`
		AgenID   string   `json:"agen_id"`
		Tanggal  string   `json:"tanggal"`
		NoMobil  string   `json:"no_mobil"`
		NmSopir  string   `json:"nm_sopir"`
		NoDO     string   `json:"no_do"`
		TglPU    string   `json:"tgl_pu"`
		LokasiPU string   `json:"lokasi_pu"`
		BTTIDs   []string `json:"btt_ids"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	tglParsed, _ := time.Parse("2006-01-02", input.Tanggal)
	year := tglParsed.Format("2006")
	var count int64
	database.Table("public.art_t_proformah").Where("pih_id LIKE ?", "%/PI/%/"+year+"/%").Count(&count)
	piNo := fmt.Sprintf("PI/DLI/001/%s/%08d", year, count+1)

	tx := database.Begin()

	headerQuery := `
		INSERT INTO public.art_t_proformah (
			pih_id, pih_agenid, pih_custid, pih_tanggal, pih_nomobil, pih_nmsopir, pih_nodo, pih_tglpu, pih_lokasipu, pih_aktifyn, pih_updateid, pih_updatetime
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'Y', 'USER', NOW())
	`
	var tglPUVal interface{} = nil
	if input.TglPU != "" {
		tglPUVal = input.TglPU + " 00:00:00"
	}

	if err := tx.Exec(headerQuery, piNo, input.AgenID, input.CustID, input.Tanggal+" 00:00:00", input.NoMobil, input.NmSopir, input.NoDO, tglPUVal, input.LokasiPU).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan header proforma: " + err.Error()})
		return
	}

	for _, btt := range input.BTTIDs {
		trimmedBTT := strings.TrimSpace(btt)
		if trimmedBTT != "" {
			dQuery := `INSERT INTO public.art_t_proformad (pid_pihid, pid_bttid, pid_updateid, pid_updatetime) VALUES (?, ?, 'USER', NOW())`
			if err := tx.Exec(dQuery, piNo, trimmedBTT).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan item BTT proforma: " + err.Error()})
				return
			}
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Proforma Invoice berhasil dibuat", "pih_id": piNo})
}

// POST /api/piutang/proforma-invoice/update
func UpdateProformaInvoiceHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var input struct {
		PIHID    string   `json:"pih_id"`
		CustID   string   `json:"cust_id"`
		AgenID   string   `json:"agen_id"`
		Tanggal  string   `json:"tanggal"`
		NoMobil  string   `json:"no_mobil"`
		NmSopir  string   `json:"nm_sopir"`
		NoDO     string   `json:"no_do"`
		TglPU    string   `json:"tgl_pu"`
		LokasiPU string   `json:"lokasi_pu"`
		BTTIDs   []string `json:"btt_ids"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	tx := database.Begin()

	var tglPUVal interface{} = nil
	if input.TglPU != "" {
		tglPUVal = input.TglPU + " 00:00:00"
	}

	updHeaderQuery := `
		UPDATE public.art_t_proformah 
		SET pih_custid = ?, pih_agenid = ?, pih_tanggal = ?, pih_nomobil = ?, pih_nmsopir = ?, pih_nodo = ?, pih_tglpu = ?, pih_lokasipu = ?, pih_updatetime = NOW()
		WHERE TRIM(pih_id) = TRIM(?)
	`
	if err := tx.Exec(updHeaderQuery, input.CustID, input.AgenID, input.Tanggal+" 00:00:00", input.NoMobil, input.NmSopir, input.NoDO, tglPUVal, input.LokasiPU, input.PIHID).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update header proforma: " + err.Error()})
		return
	}

	if err := tx.Exec("DELETE FROM public.art_t_proformad WHERE TRIM(pid_pihid) = TRIM(?)", input.PIHID).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal reset detail proforma: " + err.Error()})
		return
	}

	for _, btt := range input.BTTIDs {
		trimmedBTT := strings.TrimSpace(btt)
		if trimmedBTT != "" {
			dQuery := `INSERT INTO public.art_t_proformad (pid_pihid, pid_bttid, pid_updateid, pid_updatetime) VALUES (?, ?, 'USER', NOW())`
			if err := tx.Exec(dQuery, input.PIHID, trimmedBTT).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update detail BTT proforma: " + err.Error()})
				return
			}
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Proforma Invoice berhasil diperbarui"})
}

// POST /api/piutang/proforma-invoice/cancel
func CancelProformaInvoiceHandler(c *gin.Context) {
	pihID := strings.TrimSpace(c.Query("pih_id"))
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	if err := database.Exec("UPDATE public.art_t_proformah SET pih_aktifyn = 'N', pih_updatetime = NOW() WHERE TRIM(pih_id) = TRIM(?)", pihID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membatalkan proforma: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Proforma Invoice berhasil dibatalkan"})
}

// GET /api/pelanggan
func GetPelangganDropdownHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	type CustDropdown struct {
		CustID   string `json:"cust_id" gorm:"column:cust_id"`
		CustName string `json:"cust_name" gorm:"column:cust_name"`
	}

	list := make([]CustDropdown, 0)
	rawQuery := `
		SELECT cust_id, cust_name 
		FROM public.mkt_m_customer 
		WHERE cust_aktifyn = 'Y' 
		ORDER BY cust_name ASC 
		LIMIT 1000
	`
	if err := database.Raw(rawQuery).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data pelanggan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}
