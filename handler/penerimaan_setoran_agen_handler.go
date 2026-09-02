package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type SetoranAgenRow struct {
	BTTHID        string    `json:"btth_id" gorm:"column:btth_id"`
	BTTHDate      time.Time `json:"btth_tanggal" gorm:"column:btth_tanggal"`
	BTTHDateStr   string    `json:"btth_tanggal_str" gorm:"-"`
	AgenID        string    `json:"btth_agenid" gorm:"column:btth_agenid"`
	AgenNama      string    `json:"agen_nama" gorm:"column:agen_nama"`
	KomisiKirim   float64   `json:"agen_komisikirim" gorm:"column:agen_komisikirim"`
	JumlahSetoran float64   `json:"jml_setoran" gorm:"column:jml_setoran"`
	NoPembayaran  string    `json:"no_pembayaran" gorm:"column:no_pembayaran"`
	NoJurnal      string    `json:"no_jurnal" gorm:"column:no_jurnal"`
	PostingYN     string    `json:"posting_yn" gorm:"column:posting_yn"`
	SttBayar      string    `json:"stt_bayar" gorm:"column:stt_bayar"`
}

type SetoranBTTItem struct {
	BTTID         string  `json:"btt_id" gorm:"column:btt_id"`
	TglBTT        string  `json:"btt_tanggal" gorm:"column:btt_tanggal"`
	Penerima      string  `json:"penerima" gorm:"column:penerima"`
	Tujuan        string  `json:"tujuan" gorm:"column:tujuan"`
	Harga         float64 `json:"harga" gorm:"column:harga"`
	BiayaPenerus  float64 `json:"biaya_penerus" gorm:"column:biaya_penerus"`
	BiayaPacking  float64 `json:"biaya_packing" gorm:"column:biaya_packing"`
	BiayaAsuransi float64 `json:"biaya_asuransi" gorm:"column:biaya_asuransi"`
	TotalBTT      float64 `json:"total_btt" gorm:"column:total_btt"`
}

// GET /api/piutang/penerimaan-setoran-agen
func GetPenerimaanSetoranAgenListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	agenNama := strings.TrimSpace(c.Query("agen_nama"))
	terbayar := strings.TrimSpace(c.Query("terbayar"))
	posting := strings.TrimSpace(c.Query("posting"))
	noClosing := strings.TrimSpace(c.Query("no_closing"))

	// 1. Cek keberadaan kolom komisi pada glb_m_agen secara dinamis
	var aCols []string
	database.Raw(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE LOWER(table_name) = 'glb_m_agen'
	`).Scan(&aCols)

	komisiSelect := "0 AS agen_komisikirim"
	komisiGroup := ""
	for _, col := range aCols {
		cl := strings.ToLower(col)
		if cl == "agen_komisikirim" || cl == "agen_komisi" || cl == "komisi" {
			komisiSelect = fmt.Sprintf("COALESCE(a.%s, 0) AS agen_komisikirim", col)
			komisiGroup = fmt.Sprintf(", a.%s", col)
			break
		}
	}

	rawQuery := fmt.Sprintf(`
		SELECT 
			h.btth_id,
			h.btth_tanggal,
			h.btth_agenid,
			COALESCE(a.agen_nama, h.btth_agenid) AS agen_nama,
			%s,
			SUM(
				COALESCE(CASE WHEN ec.bttt_pembayaran = '1' THEN ROUND(ec.bttt_harga, -3) ELSE ec.bttt_harga END, 0) +
				COALESCE(ec.bttt_biayapenerus, 0) +
				COALESCE(pck.pck_biaya, 0) +
				COALESCE(asr.totalbiaya, 0)
			) AS jml_setoran,
			COALESCE(rdd.trectdd_trectdtrecthno, '') AS no_pembayaran,
			COALESCE(rh.trecth_tjurhno, '') AS no_jurnal,
			COALESCE(rh.trecth_postingyn, 'N') AS posting_yn,
			CASE WHEN rdd.trectdd_trectdtrecthno IS NULL OR TRIM(rdd.trectdd_trectdtrecthno) = '' THEN 'N' ELSE 'Y' END AS stt_bayar
		FROM public.art_t_penjualanbtth h
		INNER JOIN public.art_t_penjualanbttd d ON d.bttd_btthid = h.btth_id
		INNER JOIN public.mkt_t_econote ec ON ec.bttt_id = d.bttd_bttid
		LEFT JOIN public.pck_t_packing pck ON pck.pck_id = ec.bttt_packingid
		LEFT JOIN public.mkt_t_asuransi asr ON asr.bttt_id = ec.bttt_id
		LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(h.btth_agenid::varchar)
		LEFT JOIN public.art_t_receiptdd rdd ON TRIM(rdd.trectdd_nofpum) = TRIM(h.btth_id)
		LEFT JOIN public.art_t_receipth rh ON TRIM(rh.trecth_no) = TRIM(rdd.trectdd_trectdtrecthno)
		WHERE (RIGHT(TRIM(h.btth_id), 2) = 'SB' OR h.btth_id IS NOT NULL)
	`, komisiSelect)

	var conditions []string
	var args []interface{}

	if startDate != "" && endDate != "" {
		conditions = append(conditions, "h.btth_tanggal BETWEEN ? AND ?")
		args = append(args, startDate+" 00:00:00", endDate+" 23:59:59")
	}
	if agenNama != "" && agenNama != "ALL" {
		conditions = append(conditions, "(a.agen_nama ILIKE ? OR h.btth_agenid ILIKE ?)")
		args = append(args, "%"+agenNama+"%", "%"+agenNama+"%")
	}
	if noClosing != "" {
		conditions = append(conditions, "h.btth_id ILIKE ?")
		args = append(args, "%"+noClosing+"%")
	}
	if terbayar != "" && terbayar != "ALL" {
		if terbayar == "Y" {
			conditions = append(conditions, "(rdd.trectdd_trectdtrecthno IS NOT NULL AND TRIM(rdd.trectdd_trectdtrecthno) <> '')")
		} else {
			conditions = append(conditions, "(rdd.trectdd_trectdtrecthno IS NULL OR TRIM(rdd.trectdd_trectdtrecthno) = '')")
		}
	}
	if posting != "" && posting != "ALL" {
		conditions = append(conditions, "COALESCE(rh.trecth_postingyn, 'N') = ?")
		args = append(args, posting)
	}

	if len(conditions) > 0 {
		rawQuery += " AND " + strings.Join(conditions, " AND ")
	}

	rawQuery += fmt.Sprintf(`
		GROUP BY h.btth_id, h.btth_tanggal, h.btth_agenid, a.agen_nama %s, rdd.trectdd_trectdtrecthno, rh.trecth_tjurhno, rh.trecth_postingyn
		ORDER BY h.btth_tanggal DESC, h.btth_id DESC
		LIMIT 300
	`, komisiGroup)

	list := make([]SetoranAgenRow, 0)
	if err := database.Raw(rawQuery, args...).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data setoran agen: " + err.Error()})
		return
	}

	for i := range list {
		list[i].BTTHDateStr = list[i].BTTHDate.Format("2006-01-02")
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// GET /api/piutang/penerimaan-setoran-agen/detail
func GetPenerimaanSetoranAgenDetailHandler(c *gin.Context) {
	btthID := strings.TrimSpace(c.Query("btth_id"))
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	type HeaderClosing struct {
		BTTHID       string    `json:"btth_id"`
		BTTHDate     time.Time `json:"btth_tanggal"`
		AgenID       string    `json:"btth_agenid"`
		AgenNama     string    `json:"agen_nama"`
		KomisiKirim  float64   `json:"agen_komisikirim"`
		NoPembayaran string    `json:"no_pembayaran"`
		PostingYN    string    `json:"posting_yn"`
	}

	var head HeaderClosing
	err := database.Table("public.art_t_penjualanbtth h").
		Select(`
			h.btth_id,
			h.btth_tanggal,
			h.btth_agenid,
			COALESCE(a.agen_nama, h.btth_agenid) AS agen_nama,
			0 AS agen_komisikirim,
			COALESCE(rdd.trectdd_trectdtrecthno, '') AS no_pembayaran,
			COALESCE(rh.trecth_postingyn, 'N') AS posting_yn
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(h.btth_agenid::varchar)").
		Joins("LEFT JOIN public.art_t_receiptdd rdd ON TRIM(rdd.trectdd_nofpum) = TRIM(h.btth_id)").
		Joins("LEFT JOIN public.art_t_receipth rh ON TRIM(rh.trecth_no) = TRIM(rdd.trectdd_trectdtrecthno)").
		Where("TRIM(h.btth_id) = TRIM(?)", btthID).
		Scan(&head).Error

	if err != nil || head.BTTHID == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Dokumen closing agen tidak ditemukan"})
		return
	}

	bttList := make([]SetoranBTTItem, 0)
	rawDetailQuery := `
		SELECT 
			ec.bttt_id AS btt_id,
			TO_CHAR(ec.bttt_tanggal, 'YYYY-MM-DD') AS btt_tanggal,
			COALESCE(ec.bttt_penerimanama, '') AS penerima,
			COALESCE(ec.bttt_tujuannama, '') AS tujuan,
			COALESCE(CASE WHEN ec.bttt_pembayaran = '1' THEN ROUND(ec.bttt_harga, -3) ELSE ec.bttt_harga END, 0) AS harga,
			COALESCE(ec.bttt_biayapenerus, 0) AS biaya_penerus,
			COALESCE(pck.pck_biaya, 0) AS biaya_packing,
			COALESCE(asr.totalbiaya, 0) AS biaya_asuransi,
			(
				COALESCE(CASE WHEN ec.bttt_pembayaran = '1' THEN ROUND(ec.bttt_harga, -3) ELSE ec.bttt_harga END, 0) +
				COALESCE(ec.bttt_biayapenerus, 0) +
				COALESCE(pck.pck_biaya, 0) +
				COALESCE(asr.totalbiaya, 0)
			) AS total_btt
		FROM public.art_t_penjualanbttd d
		INNER JOIN public.mkt_t_econote ec ON ec.bttt_id = d.bttd_bttid
		LEFT JOIN public.pck_t_packing pck ON pck.pck_id = ec.bttt_packingid
		LEFT JOIN public.mkt_t_asuransi asr ON asr.bttt_id = ec.bttt_id
		WHERE TRIM(d.bttd_btthid) = TRIM(?)
		ORDER BY ec.bttt_id ASC
	`
	database.Raw(rawDetailQuery, btthID).Scan(&bttList)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"header": head,
			"items":  bttList,
		},
	})
}

// POST /api/piutang/penerimaan-setoran-agen/proses
func ProcessSetoranAgenHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var input struct {
		BTTHID     string  `json:"btth_id"`
		AgenNama   string  `json:"agen_nama"`
		Tanggal    string  `json:"tanggal"`
		CAID       string  `json:"ca_id"`
		Nominal    float64 `json:"nominal"`
		Keterangan string  `json:"keterangan"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	tglParsed, _ := time.Parse("2006-01-02", input.Tanggal)
	yearMonth := tglParsed.Format("200601")
	var count int64
	database.Table("public.art_t_receipth").Where("trecth_no LIKE ?", "%/REC/"+yearMonth+"/%").Count(&count)
	receiptNo := fmt.Sprintf("001/REC/%s/%04d", yearMonth, count+1)

	tx := database.Begin()

	headerQuery := `
		INSERT INTO public.art_t_receipth (trecth_no, trecth_tanggal, trecth_custid, trecth_custname, trecth_total, trecth_keterangan, trecth_postingyn, trecth_deleteyn, trecth_updateid, trecth_updatetime)
		VALUES (?, ?, ?, ?, ?, ?, 'N', 'N', 'USER', NOW())
	`
	if err := tx.Exec(headerQuery, receiptNo, input.Tanggal+" 10:00:00", input.BTTHID, input.AgenNama, input.Nominal, input.Keterangan).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan header penerimaan setoran: " + err.Error()})
		return
	}

	dQuery := `
		INSERT INTO public.art_t_receiptd (trectd_trecthno, trectd_caid, trectd_keterangan, trectd_nilai, trectd_tipe, trectd_updateid, trectd_updatetime)
		VALUES (?, ?, ?, ?, 'KAS', 'USER', NOW())
	`
	if err := tx.Exec(dQuery, receiptNo, input.CAID, input.Keterangan, input.Nominal).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan rincian kas/bank setoran: " + err.Error()})
		return
	}

	ddQuery := `
		INSERT INTO public.art_t_receiptdd (trectdd_trectdtrecthno, trectdd_nofpum, trectdd_jumlah, trectdd_keterangan, trectdd_updateid, trectdd_updatetime)
		VALUES (?, ?, ?, ?, 'USER', NOW())
	`
	if err := tx.Exec(ddQuery, receiptNo, input.BTTHID, input.Nominal, input.Keterangan).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menautkan closing agen ke kwitansi: " + err.Error()})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"message":       "Penerimaan setoran agen berhasil diproses",
		"no_pembayaran": receiptNo,
	})
}
