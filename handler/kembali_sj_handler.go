package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 1. GET LIST PENGEMBALIAN SURAT JALAN
func GetKembaliSJList(c *gin.Context) {
	database := getDynamicDB(c)

	tglAwal := strings.TrimSpace(c.Query("tgl_awal"))
	tglAkhir := strings.TrimSpace(c.Query("tgl_akhir"))
	custName := strings.TrimSpace(c.Query("customer_name"))
	docID := strings.TrimSpace(c.Query("document_id"))
	noSJ := strings.TrimSpace(c.Query("no_sj"))

	var results []models.KembaliSJResponse

	query := database.Table("public.mkt_t_kembalisj_h h").
		Select(`
			h.mkt_t_kembalisj_id, 
			h.mkt_t_kembalisj_tanggal, 
			h.mkt_t_kembalisj_custid, 
			c.cust_name, 
			h.mkt_t_kembalisj_keterangan, 
			COALESCE(h.mkt_t_kembalisj_updateid, '-') AS mkt_t_kembalisj_updateid,
			COALESCE(h.mkt_t_kembalisj_diterima, '-') AS mkt_t_kembalisj_diterima, 
			h.mkt_t_kembalisj_tanggalditerima, 
			h.mkt_t_kembalisj_aktifyn
		`).
		Joins("LEFT JOIN public.mkt_m_customer c ON TRIM(h.mkt_t_kembalisj_custid) = TRIM(c.cust_id)").
		Where("h.mkt_t_kembalisj_aktifyn = ?", "Y")

	if tglAwal != "" && tglAkhir != "" {
		query = query.Where("h.mkt_t_kembalisj_tanggal BETWEEN ? AND ?", tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}
	if custName != "" {
		query = query.Where("c.cust_name ILIKE ?", "%"+custName+"%")
	}
	if docID != "" {
		query = query.Where("h.mkt_t_kembalisj_id ILIKE ?", "%"+docID+"%")
	}
	if noSJ != "" {
		query = query.Where("h.mkt_t_kembalisj_id IN (SELECT mkt_t_kembalisj_id_h FROM public.mkt_t_kembalisj_d WHERE mkt_t_kembalisj_nosj ILIKE ?)", "%"+noSJ+"%")
	}

	if err := query.Order("h.mkt_t_kembalisj_tanggal DESC").Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 2. GET DAFTAR SURAT JALAN / BTT / SCAN BARCODE (SESUAI get-KembaliSJ.asp, get-KembaliSJbyBTT.asp, get-KembaliSJbyBTTScan.asp)
func GetAvailableSJ(c *gin.Context) {
	database := getDynamicDB(c)

	customerID := strings.TrimSpace(c.Query("customer_id"))
	mode := strings.ToLower(strings.TrimSpace(c.Query("mode"))) // "sj" | "btt" | "scan"
	tglAwal := strings.TrimSpace(c.Query("tgla"))
	tglAkhir := strings.TrimSpace(c.Query("tgle"))
	bttID := strings.TrimSpace(c.Query("bttid"))
	search := strings.TrimSpace(c.Query("search"))

	type AvailableSJItem struct {
		NoSJ         string  `json:"no_sj"`
		NoBTT        string  `json:"no_btt"`
		TglKirim     string  `json:"tgl_kirim"`
		CustomerID   string  `json:"customer_id"`
		CustomerName string  `json:"customer_name"`
		TglDiterima  string  `json:"tgl_diterima"`
		NamaPenerima string  `json:"nama_penerima"`
		Colly        int     `json:"colly"`
		Berat        float64 `json:"berat"`
		BeratVol     float64 `json:"berat_vol"`
	}

	var list []AvailableSJItem

	query := database.Table("public.opr_t_transportlspb t").
		Select(`
			TRIM(COALESCE(t.dn_od_sj, '')) AS no_sj,
			TRIM(COALESCE(t.nomorbtt, '')) AS no_btt,
			COALESCE(
				TO_CHAR(e.bttt_tanggal, 'DD/MM/YYYY'),
				NULLIF(SPLIT_PART(COALESCE(t.tanggal_handover::text, ''), ' ', 1), ''),
				'-'
			) AS tgl_kirim,
			COALESCE(NULLIF(TRIM(e.bttt_asalcustid), ''), TRIM(t.customerid), '') AS customer_id,
			COALESCE(NULLIF(TRIM(cu.cust_name), ''), NULLIF(TRIM(e.bttt_asalname), ''), '-') AS customer_name,
			COALESCE(NULLIF(SPLIT_PART(COALESCE(t.tanggal_diterima::text, ''), ' ', 1), ''), '-') AS tgl_diterima,
			COALESCE(NULLIF(TRIM(t.nama_penerima), ''), '-') AS nama_penerima,
			COALESCE(e.bttt_jmlunit, 1) AS colly,
			COALESCE(e.bttt_berat, 0) AS berat,
			COALESCE(e.bttt_beratvol, 0) AS berat_vol
		`).
		Joins("LEFT JOIN public.mkt_t_econote e ON UPPER(TRIM(e.bttt_id::text)) = UPPER(TRIM(t.nomorbtt::text))").
		Joins("LEFT JOIN public.mkt_m_customer cu ON TRIM(cu.cust_id::text) = COALESCE(NULLIF(TRIM(e.bttt_asalcustid), ''), TRIM(t.customerid))").
		Where("(t.spb_id IS NULL OR TRIM(t.spb_id) = '' OR UPPER(TRIM(t.spb_id)) = 'NULL')").
		Where("t.dn_od_sj IS NOT NULL AND TRIM(t.dn_od_sj) <> ''").
		Where(`NOT EXISTS (
			SELECT 1 
			FROM public.mkt_t_kembalisj_d d
			JOIN public.mkt_t_kembalisj_h h ON TRIM(h.mkt_t_kembalisj_id) = TRIM(d.mkt_t_kembalisj_id_h)
			WHERE TRIM(d.mkt_t_kembalisj_nosj) = TRIM(t.dn_od_sj)
			  AND COALESCE(h.mkt_t_kembalisj_aktifyn, 'Y') = 'Y'
		)`)

	if customerID != "" {
		query = query.Where("(TRIM(e.bttt_asalcustid) = ? OR TRIM(t.customerid) = ?)", customerID, customerID)
	}

	if mode == "scan" {
		if bttID == "" {
			c.JSON(http.StatusOK, gin.H{"status": "success", "data": []AvailableSJItem{}})
			return
		}
		query = query.Where("(UPPER(TRIM(t.nomorbtt)) = UPPER(?) OR UPPER(TRIM(t.dn_od_sj)) = UPPER(?))", bttID, bttID)
	} else {
		if tglAwal != "" && tglAkhir != "" {
			query = query.Where(`
				COALESCE(
					e.bttt_tanggal::date,
					CASE 
						WHEN t.tanggal_handover::text LIKE '%/%/%' 
							THEN TO_DATE(SPLIT_PART(t.tanggal_handover::text, ' ', 1), 'MM/DD/YYYY')
						WHEN NULLIF(SPLIT_PART(COALESCE(t.tanggal_handover::text, ''), ' ', 1), '') IS NOT NULL 
							THEN SPLIT_PART(t.tanggal_handover::text, ' ', 1)::date
						ELSE NULL
					END
				) BETWEEN ?::date AND ?::date
			`, tglAwal, tglAkhir)
		}
		if search != "" {
			query = query.Where("(t.dn_od_sj ILIKE ? OR t.nomorbtt ILIKE ?)", "%"+search+"%", "%"+search+"%")
		}
	}

	if mode == "btt" {
		query = query.Order("t.nomorbtt ASC, t.dn_od_sj ASC")
	} else {
		query = query.Order("t.dn_od_sj ASC, t.nomorbtt ASC")
	}

	if err := query.Limit(300).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

type SJPair struct {
	NoSJ  string `json:"no_sj"`
	NoBTT string `json:"no_btt"`
}

type CreateKembaliSJReq struct {
	CustomerID string   `json:"customer_id" binding:"required"`
	Tanggal    string   `json:"tanggal" binding:"required"`
	Pengirim   string   `json:"pengirim"`
	Keterangan string   `json:"keterangan"`
	InvYN      string   `json:"inv_yn"`
	LampDO     string   `json:"lamp_do"`
	ListSJ     []SJPair `json:"list_sj"`
	DaftarNoSJ []string `json:"daftar_nosj"`
}

// 3. PROSES SIMPAN PENGEMBALIAN SURAT JALAN (SESUAI p-mkt_t_kembaliSJ_a.asp)
func CreateKembaliSJ(c *gin.Context) {
	database := getDynamicDB(c)

	var req CreateKembaliSJReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid: " + err.Error()})
		return
	}

	// Gabungkan input dari list_sj maupun daftar_nosj agar kompatibel
	itemsToSave := req.ListSJ
	if len(itemsToSave) == 0 && len(req.DaftarNoSJ) > 0 {
		for _, noSJ := range req.DaftarNoSJ {
			cleanSJ := strings.TrimSpace(noSJ)
			if cleanSJ == "" {
				continue
			}
			var noBTT string
			database.Table("public.opr_t_transportlspb").
				Select("TRIM(COALESCE(nomorbtt, ''))").
				Where("TRIM(dn_od_sj) = ?", cleanSJ).
				Limit(1).
				Scan(&noBTT)
			itemsToSave = append(itemsToSave, SJPair{NoSJ: cleanSJ, NoBTT: noBTT})
		}
	}

	if len(itemsToSave) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Silakan pilih/centang minimal 1 Surat Jalan terlebih dahulu"})
		return
	}

	agenID := "001"
	if authAgen, exists := c.Get("agen_id"); exists && authAgen != nil && fmt.Sprintf("%v", authAgen) != "" {
		rawAgen := strings.TrimSpace(fmt.Sprintf("%v", authAgen))
		if len(rawAgen) < 3 {
			agenID = fmt.Sprintf("%03s", rawAgen)
		} else {
			agenID = rawAgen[:3]
		}
	}

	username := "SYSTEM"
	if authUser, exists := c.Get("username"); exists && authUser != nil && fmt.Sprintf("%v", authUser) != "" {
		username = strings.ToUpper(fmt.Sprintf("%v", authUser))
	}

	// Parse tanggal pengembalian (format YYYY-MM-DD) untuk membentuk ekor MMYY + "SJ"
	tglPengembalian, errTgl := time.Parse("2006-01-02", strings.TrimSpace(req.Tanggal))
	if errTgl != nil {
		tglPengembalian = time.Now()
	}
	ekor := fmt.Sprintf("%02d%02dSJ", int(tglPengembalian.Month()), tglPengembalian.Year()%100)

	invYN := strings.ToUpper(strings.TrimSpace(req.InvYN))
	if invYN != "Y" {
		invYN = "N"
	}
	lampDO := strings.TrimSpace(req.LampDO)
	if lampDO == "" {
		lampDO = "0"
	}

	// Pastikan kolom pendukung invyn & lampdo tersedia jika belum ada
	database.Exec(`ALTER TABLE public.mkt_t_kembalisj_h ADD COLUMN IF NOT EXISTS mkt_t_kembalisj_invyn VARCHAR(5) DEFAULT 'N'`)
	database.Exec(`ALTER TABLE public.mkt_t_kembalisj_h ADD COLUMN IF NOT EXISTS mkt_t_kembalisj_lampdo VARCHAR(10) DEFAULT '0'`)

	tx := database.Begin()

	var lastUrut struct {
		Urut string
	}
	tx.Raw(`
		SELECT SUBSTRING(mkt_t_kembalisj_id, 4, 6) AS urut 
		FROM public.mkt_t_kembalisj_h 
		WHERE LEFT(mkt_t_kembalisj_id, 3) = ? AND RIGHT(mkt_t_kembalisj_id, 6) = ? 
		ORDER BY SUBSTRING(mkt_t_kembalisj_id, 4, 6) DESC 
		LIMIT 1
	`, agenID, ekor).Scan(&lastUrut)

	counter := 1
	if lastUrut.Urut != "" {
		if val, err := strconv.Atoi(lastUrut.Urut); err == nil {
			counter = val + 1
		}
	}
	keID := fmt.Sprintf("%s%06d%s", agenID, counter, ekor)

	insertH := `
		INSERT INTO public.mkt_t_kembalisj_h 
			(mkt_t_kembalisj_id, mkt_t_kembalisj_custid, mkt_t_kembalisj_tanggal, mkt_t_kembalisj_keterangan, mkt_t_kembalisj_invyn, mkt_t_kembalisj_lampdo, mkt_t_kembalisj_updateid, mkt_t_kembalisj_updatetime, mkt_t_kembalisj_aktifyn) 
		VALUES 
			(?, ?, ?::timestamp, ?, ?, ?, ?, CURRENT_TIMESTAMP, 'Y')
	`
	if err := tx.Exec(insertH, keID, strings.TrimSpace(req.CustomerID), tglPengembalian.Format("2006-01-02")+" 00:00:00", strings.ToUpper(strings.TrimSpace(req.Keterangan)), invYN, lampDO, username).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan header: " + err.Error()})
		return
	}

	for _, item := range itemsToSave {
		noSJ := strings.TrimSpace(item.NoSJ)
		noBTT := strings.TrimSpace(item.NoBTT)
		if noSJ == "" {
			continue
		}

		if noBTT == "" {
			tx.Table("public.opr_t_transportlspb").
				Select("TRIM(COALESCE(nomorbtt, ''))").
				Where("TRIM(dn_od_sj) = ?", noSJ).
				Limit(1).
				Scan(&noBTT)
		}

		// Cek apakah SJ ini sudah tercatat di pengembalian yang masih aktif
		var existingDocID string
		tx.Raw(`
			SELECT TRIM(d.mkt_t_kembalisj_id_h)
			FROM public.mkt_t_kembalisj_d d
			JOIN public.mkt_t_kembalisj_h h ON TRIM(h.mkt_t_kembalisj_id) = TRIM(d.mkt_t_kembalisj_id_h)
			WHERE TRIM(d.mkt_t_kembalisj_nosj) = ?
			  AND COALESCE(h.mkt_t_kembalisj_aktifyn, 'Y') = 'Y'
			LIMIT 1
		`, noSJ).Scan(&existingDocID)

		if existingDocID != "" {
			tx.Rollback()
			c.JSON(http.StatusConflict, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("No. Surat Jalan %s sudah pernah dibuatkan pengembalian dengan No: %s", noSJ, existingDocID),
			})
			return
		}

		// Bersihkan jika ada sisa record lama yang sudah tidak aktif/yatim agar tidak kena pkey duplicate
		tx.Exec(`DELETE FROM public.mkt_t_kembalisj_d WHERE TRIM(mkt_t_kembalisj_nosj) = ?`, noSJ)

		insertD := `
			INSERT INTO public.mkt_t_kembalisj_d 
				(mkt_t_kembalisj_id_h, mkt_t_kembalisj_nosj, mkt_t_kembalisj_btt) 
			VALUES 
				(?, ?, ?)
		`
		if err := tx.Exec(insertD, keID, noSJ, noBTT).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan detail SJ: " + err.Error()})
			return
		}

		updateLSPB := `
			UPDATE public.opr_t_transportlspb 
			SET spb_id = ?, tanggal_dokumenkembali = ?::timestamp 
			WHERE TRIM(dn_od_sj) = ?
		`
		tx.Exec(updateLSPB, keID, tglPengembalian.Format("2006-01-02")+" 00:00:00", noSJ)
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Pengembalian Surat Jalan berhasil dibuat dengan No: %s", keID),
		"spb_id":  keID,
	})
}

// 4. BATALKAN / HAPUS PENGEMBALIAN SURAT JALAN
func DeleteKembaliSJ(c *gin.Context) {
	database := getDynamicDB(c)

	idSJ := strings.TrimSpace(c.Param("id"))
	if idSJ == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID Pengembalian wajib diisi"})
		return
	}

	tx := database.Begin()
	tx.Exec("UPDATE public.opr_t_transportlspb SET spb_id = NULL, tanggal_dokumenkembali = NULL WHERE TRIM(spb_id) = ?", idSJ)
	tx.Exec("DELETE FROM public.mkt_t_kembalisj_d WHERE TRIM(mkt_t_kembalisj_id_h) = ?", idSJ)
	tx.Exec("DELETE FROM public.mkt_t_kembalisj_h WHERE TRIM(mkt_t_kembalisj_id) = ?", idSJ)
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("Pengembalian %s berhasil dibatalkan", idSJ)})
}

// GET /marketing/outstanding-sj/:customer_id (Kompatibilitas route lama)
func GetOutstandingSJ(c *gin.Context) {
	database := getDynamicDB(c)
	customerID := strings.TrimSpace(c.Param("customer_id"))
	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Customer ID wajib diisi"})
		return
	}

	type SJItem struct {
		NoSJ string `json:"mkt_t_kembalisj_nosj"`
	}

	var results []SJItem
	err := database.Table("public.opr_t_transportlspb").
		Select("DISTINCT TRIM(dn_od_sj) AS mkt_t_kembalisj_nosj").
		Where("TRIM(customerid) = ? AND (spb_id IS NULL OR TRIM(spb_id) = '' OR UPPER(TRIM(spb_id)) = 'NULL')", customerID).
		Where("dn_od_sj IS NOT NULL AND TRIM(dn_od_sj) <> ''").
		Order("mkt_t_kembalisj_nosj ASC").
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

// POST /marketing/kembali-sj/add (Kompatibilitas route lama)
func CreateKembaliSJAdd(c *gin.Context) {
	CreateKembaliSJ(c)
}
