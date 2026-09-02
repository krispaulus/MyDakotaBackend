package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type BTTDetailRevisi struct {
	BTTID         string  `json:"btt_id" gorm:"column:bttt_id"`
	Tanggal       string  `json:"tanggal" gorm:"column:tanggal"`
	CustID        string  `json:"cust_id" gorm:"column:bttt_custid"`
	CustName      string  `json:"cust_name" gorm:"column:cust_name"`
	AgenAsal      string  `json:"agen_asal" gorm:"column:agen_nama"`
	Penerima      string  `json:"penerima" gorm:"column:penerima"`
	Tujuan        string  `json:"tujuan" gorm:"column:tujuan"`
	BeratAsli     float64 `json:"berat_asli" gorm:"column:bttt_berat"`
	BeratVolume   float64 `json:"berat_volume" gorm:"column:bttt_volume"`
	HargaPokok    float64 `json:"harga_pokok" gorm:"column:bttt_harga"`
	BiayaPenerus  float64 `json:"biaya_penerus" gorm:"column:bttt_biayapenerus"`
	BiayaPacking  float64 `json:"biaya_packing" gorm:"column:biaya_packing"`
	BiayaAsuransi float64 `json:"biaya_asuransi" gorm:"column:biaya_asuransi"`
	TotalBTT      float64 `json:"total_btt" gorm:"column:total_btt"`
	IsInvoiced    bool    `json:"is_invoiced"`
	InvoiceNo     string  `json:"invoice_no"`
}

// GET /api/piutang/revisi-btt/search?nobtt=...
func SearchBTTForRevisiHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	noBTT := strings.TrimSpace(c.Query("nobtt"))
	if noBTT == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor BTT wajib diisi"})
		return
	}

	// 1. Cek struktur kolom mkt_t_econote secara dinamis
	var ecCols []string
	database.Raw(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE LOWER(table_name) = 'mkt_t_econote'
	`).Scan(&ecCols)

	colCustID := "'' AS bttt_custid"
	colCustJoin := "1=0"
	colAgenJoin := "1=0"
	colPenerima := "'-' AS penerima"
	colTujuan := "'-' AS tujuan"
	colBerat := "0 AS bttt_berat"
	colVolume := "0 AS bttt_volume"

	// Pisahkan expression hitung dan alias kolom
	rawExprHarga := "0"
	rawExprPenerus := "0"
	colPackingJoin := "1=0"

	for _, col := range ecCols {
		cl := strings.ToLower(col)
		// Customer ID
		if cl == "bttt_custid" || cl == "bttt_customerid" || cl == "bttt_cust_id" || cl == "bttt_pengirimid" || cl == "bttt_senderid" {
			colCustID = fmt.Sprintf("COALESCE(ec.%s, '') AS bttt_custid", col)
			colCustJoin = fmt.Sprintf("TRIM(c.cust_id::varchar) = TRIM(ec.%s::varchar)", col)
		}
		// Agen Asal
		if cl == "bttt_agenasalid" || cl == "bttt_agenid" || cl == "bttt_asalagenid" || cl == "bttt_cabangid" {
			colAgenJoin = fmt.Sprintf("TRIM(a.agen_id::varchar) = TRIM(ec.%s::varchar)", col)
		}
		// Penerima
		if cl == "bttt_penerimanama" || cl == "bttt_namapenerima" || cl == "bttt_penerima" {
			colPenerima = fmt.Sprintf("COALESCE(ec.%s, '-') AS penerima", col)
		}
		// Tujuan
		if cl == "bttt_tujuannama" || cl == "bttt_namatujuan" || cl == "bttt_tujuan" {
			colTujuan = fmt.Sprintf("COALESCE(ec.%s, '-') AS tujuan", col)
		}
		// Berat & Volume
		if cl == "bttt_berat" || cl == "bttt_beratkg" {
			colBerat = fmt.Sprintf("COALESCE(ec.%s, 0) AS bttt_berat", col)
		}
		if cl == "bttt_volume" || cl == "bttt_m3" {
			colVolume = fmt.Sprintf("COALESCE(ec.%s, 0) AS bttt_volume", col)
		}
		// Harga Pokok & Penerus
		if cl == "bttt_harga" || cl == "bttt_totalharga" || cl == "bttt_ongkir" {
			rawExprHarga = fmt.Sprintf("COALESCE(ec.%s, 0)", col)
		}
		if cl == "bttt_biayapenerus" || cl == "bttt_penerus" {
			rawExprPenerus = fmt.Sprintf("COALESCE(ec.%s, 0)", col)
		}
		// Packing ID
		if cl == "bttt_packingid" || cl == "bttt_packing_id" {
			colPackingJoin = fmt.Sprintf("pck.pck_id = ec.%s", col)
		}
	}

	rawQuery := fmt.Sprintf(`
		SELECT 
			ec.bttt_id,
			TO_CHAR(ec.bttt_tanggal, 'YYYY-MM-DD') AS tanggal,
			%s,
			COALESCE(c.cust_name, '-') AS cust_name,
			COALESCE(a.agen_nama, '-') AS agen_nama,
			%s,
			%s,
			%s,
			%s,
			%s AS bttt_harga,
			%s AS bttt_biayapenerus,
			COALESCE(pck.pck_biaya, 0) AS biaya_packing,
			COALESCE(asr.totalbiaya, 0) AS biaya_asuransi,
			(
				%s + 
				%s + 
				COALESCE(pck.pck_biaya, 0) + 
				COALESCE(asr.totalbiaya, 0)
			) AS total_btt
		FROM public.mkt_t_econote ec
		LEFT JOIN public.mkt_m_customer c ON %s
		LEFT JOIN public.glb_m_agen a ON %s
		LEFT JOIN public.pck_t_packing pck ON %s
		LEFT JOIN public.mkt_t_asuransi asr ON TRIM(asr.bttt_id) = TRIM(ec.bttt_id)
		WHERE TRIM(ec.bttt_id) = TRIM(?)
		LIMIT 1
	`, colCustID, colPenerima, colTujuan, colBerat, colVolume, rawExprHarga, rawExprPenerus,
		rawExprHarga, rawExprPenerus, colCustJoin, colAgenJoin, colPackingJoin)

	var detail BTTDetailRevisi
	if err := database.Raw(rawQuery, noBTT).Scan(&detail).Error; err != nil || detail.BTTID == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Nomor BTT tidak ditemukan di database"})
		return
	}

	// 2. Cek apakah BTT sudah masuk ke Invoice aktif
	type InvCheck struct {
		InvoiceNo string `gorm:"column:artih_id"`
	}
	var invCheck InvCheck
	checkInvQuery := `
		SELECT ih.artih_id 
		FROM public.art_t_invoiced id
		INNER JOIN public.art_t_invoiceh ih ON TRIM(ih.artih_id) = TRIM(id.artid_artihid)
		WHERE TRIM(id.artid_bttid) = TRIM(?) AND COALESCE(ih.artih_delete, 'N') = 'N'
		LIMIT 1
	`
	database.Raw(checkInvQuery, noBTT).Scan(&invCheck)
	if invCheck.InvoiceNo != "" {
		detail.IsInvoiced = true
		detail.InvoiceNo = invCheck.InvoiceNo
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": detail})
}

// POST /api/piutang/revisi-btt/submit
func SubmitRevisiBTTHargaHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var input struct {
		BTTID        string  `json:"btt_id"`
		CustID       string  `json:"cust_id"`
		HargaLama    float64 `json:"harga_lama"`
		HargaBaru    float64 `json:"harga_baru"`
		PenerusLama  float64 `json:"penerus_lama"`
		PenerusBaru  float64 `json:"penerus_baru"`
		PackingLama  float64 `json:"packing_lama"`
		PackingBaru  float64 `json:"packing_baru"`
		AsuransiLama float64 `json:"asuransi_lama"`
		AsuransiBaru float64 `json:"asuransi_baru"`
		Alasan       string  `json:"alasan"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if input.BTTID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor BTT wajib ditentukan"})
		return
	}

	// Cek nama kolom harga & penerus di mkt_t_econote
	var ecCols []string
	database.Raw(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE LOWER(table_name) = 'mkt_t_econote'
	`).Scan(&ecCols)

	targetColHarga := "bttt_harga"
	targetColPenerus := "bttt_biayapenerus"

	for _, col := range ecCols {
		cl := strings.ToLower(col)
		if cl == "bttt_harga" || cl == "bttt_totalharga" || cl == "bttt_ongkir" {
			targetColHarga = col
		}
		if cl == "bttt_biayapenerus" || cl == "bttt_penerus" {
			targetColPenerus = col
		}
	}

	tx := database.Begin()

	// 1. Update harga pokok & biaya penerus di mkt_t_econote
	updEconote := fmt.Sprintf(`
		UPDATE public.mkt_t_econote 
		SET %s = ?, %s = ? 
		WHERE TRIM(bttt_id) = TRIM(?)
	`, targetColHarga, targetColPenerus)

	if err := tx.Exec(updEconote, input.HargaBaru, input.PenerusBaru, input.BTTID).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update tarif BTT: " + err.Error()})
		return
	}

	// 2. Simpan Riwayat Revisi ke mkt_t_rev_apl_harga
	insLog := `
		INSERT INTO public.mkt_t_rev_apl_harga (
			rev_bttid, rev_custid, rev_hargalama, rev_hargabaru, 
			rev_peneruslama, rev_penerusbaru, rev_packinglama, rev_packingbaru, 
			rev_asuransilama, rev_asuransibaru, rev_alasan, rev_updateid, rev_updatetime
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'USER', NOW())
	`
	if err := tx.Exec(insLog, input.BTTID, input.CustID, input.HargaLama, input.HargaBaru, input.PenerusLama, input.PenerusBaru, input.PackingLama, input.PackingBaru, input.AsuransiLama, input.AsuransiBaru, input.Alasan).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan log audit revisi: " + err.Error()})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Tarif Resi BTT %s berhasil direvisi", input.BTTID),
	})
}

// GET /api/piutang/revisi-btt/history
func GetHistoryRevisiBTTHargaHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	// Auto-create tabel mkt_t_rev_apl_harga jika belum ada
	database.Exec(`
		CREATE TABLE IF NOT EXISTS public.mkt_t_rev_apl_harga (
			rev_id BIGSERIAL PRIMARY KEY,
			rev_bttid VARCHAR(50) NOT NULL,
			rev_custid VARCHAR(50),
			rev_hargalama NUMERIC(18, 2) DEFAULT 0,
			rev_hargabaru NUMERIC(18, 2) DEFAULT 0,
			rev_peneruslama NUMERIC(18, 2) DEFAULT 0,
			rev_penerusbaru NUMERIC(18, 2) DEFAULT 0,
			rev_packinglama NUMERIC(18, 2) DEFAULT 0,
			rev_packingbaru NUMERIC(18, 2) DEFAULT 0,
			rev_asuransilama NUMERIC(18, 2) DEFAULT 0,
			rev_asuransibaru NUMERIC(18, 2) DEFAULT 0,
			rev_alasan TEXT,
			rev_updateid VARCHAR(50) DEFAULT 'USER',
			rev_updatetime TIMESTAMP DEFAULT NOW()
		);
	`)

	type HistoryRow struct {
		RevID       int64   `json:"rev_id" gorm:"column:rev_id"`
		BTTID       string  `json:"btt_id" gorm:"column:rev_bttid"`
		CustID      string  `json:"cust_id" gorm:"column:rev_custid"`
		CustName    string  `json:"cust_name" gorm:"column:cust_name"`
		HargaLama   float64 `json:"harga_lama" gorm:"column:rev_hargalama"`
		HargaBaru   float64 `json:"harga_baru" gorm:"column:rev_hargabaru"`
		PenerusLama float64 `json:"penerus_lama" gorm:"column:rev_peneruslama"`
		PenerusBaru float64 `json:"penerus_baru" gorm:"column:rev_penerusbaru"`
		Alasan      string  `json:"alasan" gorm:"column:rev_alasan"`
		UserUpdate  string  `json:"user_update" gorm:"column:rev_updateid"`
		TanggalStr  string  `json:"tanggal_str" gorm:"column:tanggal_str"`
	}

	rawQuery := `
		SELECT 
			r.rev_id,
			r.rev_bttid,
			r.rev_custid,
			COALESCE(c.cust_name, r.rev_custid, '-') AS cust_name,
			r.rev_hargalama,
			r.rev_hargabaru,
			r.rev_peneruslama,
			r.rev_penerusbaru,
			COALESCE(r.rev_alasan, '-') AS rev_alasan,
			COALESCE(r.rev_updateid, 'USER') AS rev_updateid,
			TO_CHAR(r.rev_updatetime, 'YYYY-MM-DD HH24:MI') AS tanggal_str
		FROM public.mkt_t_rev_apl_harga r
		LEFT JOIN public.mkt_m_customer c ON TRIM(c.cust_id::varchar) = TRIM(r.rev_custid::varchar)
		ORDER BY r.rev_updatetime DESC
		LIMIT 300
	`

	list := make([]HistoryRow, 0)
	database.Raw(rawQuery).Scan(&list)

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}
