package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type BTTTagihTujuanRow struct {
	BTTTurunID         string    `json:"btt_turun_id" gorm:"column:btt_turun_id"`
	BTTTurunTanggal    time.Time `json:"btt_turun_tanggal" gorm:"column:btt_turun_tanggal"`
	BTTTurunCustomerID string    `json:"btt_turun_customer_id" gorm:"column:btt_turun_customer_id"`
	CustTagihName      string    `json:"cust_tagih_name" gorm:"column:cust_tagih_name"`
	BTTTurunTerbayarYN string    `json:"btt_turun_terbayar_yn" gorm:"column:btt_turun_terbayar_yn"`
	BTTTurunNoJurnal   string    `json:"btt_turun_no_jurnal" gorm:"column:btt_turun_no_jurnal"`
	AsalAgenID         string    `json:"asal_agen_id" gorm:"column:asal_agen_id"`
	AsalAgenNama       string    `json:"asal_agen_nama" gorm:"column:asal_agen_nama"`
	AsalCustID         string    `json:"asal_cust_id" gorm:"column:asal_cust_id"`
	AsalCustNama       string    `json:"asal_cust_nama" gorm:"column:asal_cust_nama"`
	AsalAlamat         string    `json:"asal_alamat" gorm:"column:asal_alamat"`
	AsalKota           string    `json:"asal_kota" gorm:"column:asal_kota"`
	AsalTelp           string    `json:"asal_telp" gorm:"column:asal_telp"`
	TujuanAgenID       string    `json:"tujuan_agen_id" gorm:"column:tujuan_agen_id"`
	TujuanNama         string    `json:"tujuan_nama" gorm:"column:tujuan_nama"`
	TujuanAlamat       string    `json:"tujuan_alamat" gorm:"column:tujuan_alamat"`
	TujuanKota         string    `json:"tujuan_kota" gorm:"column:tujuan_kota"`
	TujuanTelp         string    `json:"tujuan_telp" gorm:"column:tujuan_telp"`
	TujuanKelurahan    string    `json:"tujuan_kelurahan" gorm:"column:tujuan_kelurahan"`
	TujuanKecamatan    string    `json:"tujuan_kecamatan" gorm:"column:tujuan_kecamatan"`
	TujuanPulau        string    `json:"tujuan_pulau" gorm:"column:tujuan_pulau"`
	TujuanKodepos      string    `json:"tujuan_kodepos" gorm:"column:tujuan_kodepos"`
	Up                 string    `json:"up" gorm:"column:up"`
	Ket                string    `json:"ket" gorm:"column:ket"`
	NoSuratJalan       string    `json:"no_surat_jalan" gorm:"column:no_surat_jalan"`
	NamaBarang         string    `json:"nama_barang" gorm:"column:nama_barang"`
	JmlUnit            float64   `json:"jml_unit" gorm:"column:jml_unit"`
	JmlPck             float64   `json:"jml_pck" gorm:"column:jml_pck"`
	Berat              float64   `json:"berat" gorm:"column:berat"`
	BeratVol           float64   `json:"berat_vol" gorm:"column:berat_vol"`
	Ukuran             float64   `json:"ukuran" gorm:"column:ukuran"`
	Harga              float64   `json:"harga" gorm:"column:harga"`
	BiayaPenerus       float64   `json:"biaya_penerus" gorm:"column:biaya_penerus"`
	PackingID          float64   `json:"packing_id" gorm:"column:packing_id"`
	TotalTagih         float64   `json:"total_tagih" gorm:"column:total_tagih"`
	ServID             string    `json:"serv_id" gorm:"column:serv_id"`
	BayarYN            string    `json:"bayar_yn" gorm:"column:bayar_yn"`
	PostingYN          string    `json:"posting_yn" gorm:"column:posting_yn"`
	CBYN               string    `json:"cb_yn" gorm:"column:cb_yn"`
	SMUNo              string    `json:"smu_no" gorm:"column:smu_no"`
	SudahInvoice       bool      `json:"sudah_invoice" gorm:"column:sudah_invoice"`
}

type UpdateBTTTagihReq struct {
	BTTTurunID         string `json:"btt_turun_id" binding:"required"`
	BTTTurunCustomerID string `json:"btt_turun_customer_id" binding:"required"`
}

// 1. GET /api/piutang/btt-tagih-tujuan (List BTT Tagih Turun / Tujuan)
func GetBTTTagihTujuanListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Database corporate tidak terhubung",
		})
		return
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))
	bypassTanggal := c.Query("bypass_tanggal") == "true" || c.Query("bypass_tanggal") == "1"
	tujuanAgenID := strings.TrimSpace(c.Query("tujuan_agen_id"))
	terbayarYN := strings.TrimSpace(c.Query("terbayar_yn"))
	customerName := strings.TrimSpace(c.Query("customer_name"))
	noBTT := strings.TrimSpace(c.Query("no_btt"))
	noSuratJalan := strings.TrimSpace(c.Query("no_surat_jalan"))

	query := database.Table("public.mkt_t_econote e").
		Select(`
			e.bttt_id::varchar AS btt_turun_id,
			COALESCE(e.bttt_tanggal, NOW()) AS btt_turun_tanggal,
			COALESCE(tt.btt_turun_customerid::varchar, '') AS btt_turun_customer_id,
			COALESCE(c_tagih.cust_name, e.bttt_tujuannama, '') AS cust_tagih_name,
			COALESCE(a_asal.agen_nama, 'DLI PUSAT') AS asal_agen_nama,
			COALESCE(e.bttt_asalname::varchar, '') AS asal_cust_nama,
			COALESCE(e.bttt_asalcustid::varchar, '') AS asal_cust_id,
			COALESCE(e.bttt_tujuannama::varchar, '') AS tujuan_nama,
			COALESCE(e.bttt_tujuankota::varchar, '') AS tujuan_kota,
			COALESCE(e.bttt_nosuratjalan::varchar, '') AS no_surat_jalan,
			COALESCE(e.bttt_namabarang::varchar, '') AS nama_barang,
			COALESCE(e.bttt_jmlunit::numeric, 0) AS jml_unit,
			COALESCE(e.bttt_berat::numeric, 0) AS berat,
			(COALESCE(e.bttt_harga::numeric, 0) + COALESCE(e.bttt_biayapenerus::numeric, 0) + COALESCE(p.pck_biaya::numeric, 0)) AS total_tagih,
			CASE WHEN COALESCE(rp.total_terbayar, 0) >= (COALESCE(e.bttt_harga::numeric, 0) + COALESCE(e.bttt_biayapenerus::numeric, 0) + COALESCE(p.pck_biaya::numeric, 0)) THEN 'Y' ELSE 'N' END AS btt_turun_terbayar_yn,
			COALESCE(e.bttt_cbyn, 'N') AS cb_yn,
			CASE WHEN invd.artid_bttid IS NOT NULL THEN true ELSE false END AS sudah_invoice
		`).
		Joins("LEFT JOIN public.mkt_t_econote_tagih tt ON TRIM(tt.btt_turun_id::varchar) = TRIM(e.bttt_id::varchar)").
		Joins("LEFT JOIN public.mkt_m_customer c_tagih ON TRIM(c_tagih.cust_id::varchar) = TRIM(tt.btt_turun_customerid::varchar)").
		Joins("LEFT JOIN public.glb_m_agen a_asal ON a_asal.agen_id::varchar = LPAD(LEFT(e.bttt_id::varchar, 3), 3, '0')").
		Joins("LEFT JOIN public.pck_t_packing p ON TRIM(p.pck_id::varchar) = TRIM(e.bttt_packingid::varchar)").
		Joins(`LEFT JOIN (
			SELECT r.trectddd_nobtt, SUM(r.trectddd_bayar) AS total_terbayar
			FROM public.art_t_receiptdddd r
			GROUP BY r.trectddd_nobtt
		) rp ON TRIM(rp.trectddd_nobtt::varchar) = TRIM(e.bttt_id::varchar)`).
		Joins("LEFT JOIN public.art_t_invoiced invd ON TRIM(invd.artid_bttid::varchar) = TRIM(e.bttt_id::varchar)").
		//Where("COALESCE(e.bttt_aktifyn, 'Y') = 'Y'").
		//Where("TRIM(UPPER(COALESCE(e.bttt_bayaryn, ''))) = 'T'")

		Where("COALESCE(e.bttt_aktifyn, 'Y') <> 'N'").
		Where("(TRIM(UPPER(COALESCE(e.bttt_bayaryn, ''))) IN ('T', 'TT', '3', 'TAGIH') OR TRIM(UPPER(COALESCE(e.bttt_servid, ''))) IN ('T', 'TT', '3'))")

	if !bypassTanggal && startDate != "" && endDate != "" {
		query = query.Where("e.bttt_tanggal BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}

	if tujuanAgenID != "" && tujuanAgenID != "ALL" {
		query = query.Where("(TRIM(e.bttt_tujuanagenid::varchar) = TRIM(?::varchar) OR LPAD(TRIM(e.bttt_tujuanagenid::varchar), 3, '0') = LPAD(?::varchar, 3, '0'))", tujuanAgenID, tujuanAgenID)
	}

	if customerName != "" {
		query = query.Where("(c_tagih.cust_name ILIKE ? OR e.bttt_asalname ILIKE ? OR e.bttt_tujuannama ILIKE ?)", "%"+customerName+"%", "%"+customerName+"%", "%"+customerName+"%")
	}

	if noBTT != "" {
		query = query.Where("e.bttt_id ILIKE ?", "%"+noBTT+"%")
	}

	if noSuratJalan != "" {
		query = query.Where("e.bttt_nosuratjalan ILIKE ?", "%"+noSuratJalan+"%")
	}

	var result []map[string]interface{}
	if err := query.Order("e.bttt_tanggal DESC, e.bttt_id DESC").Limit(500).Scan(&result).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if result == nil {
		result = []map[string]interface{}{}
	}

	if terbayarYN != "" && terbayarYN != "ALL" {
		var filtered []map[string]interface{}
		for _, row := range result {
			if row["btt_turun_terbayar_yn"] == terbayarYN {
				filtered = append(filtered, row)
			}
		}
		result = filtered
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   result,
	})
}

// 2. GET /api/piutang/btt-tagih-tujuan/detail/:id
func GetBTTTagihTujuanDetailHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	bttID := strings.TrimSpace(c.Param("id"))

	var item BTTTagihTujuanRow
	err := database.Table("public.mkt_t_econote e").
		Select(`
			e.bttt_id AS btt_turun_id,
			COALESCE(tt.btt_turun_tanggal, e.bttt_tanggal) AS btt_turun_tanggal,
			COALESCE(tt.btt_turun_customerid, '') AS btt_turun_customer_id,
			COALESCE(ct.cust_name, '') AS cust_tagih_name,
			COALESCE(tt.btt_turun_terbayaryn, 'N') AS btt_turun_terbayar_yn,
			COALESCE(tt.btt_turun_nojurnal, '') AS btt_turun_no_jurnal,
			COALESCE(e.bttt_asalagenid, '') AS asal_agen_id,
			COALESCE(a.agen_nama, 'DLI PUSAT') AS asal_agen_nama,
			COALESCE(e.bttt_asalcustid, '') AS asal_cust_id,
			COALESCE(e.bttt_asalname, '') AS asal_cust_nama,
			COALESCE(e.bttt_asalalamat, '') AS asal_alamat,
			COALESCE(e.bttt_asalkota, '') AS asal_kota,
			COALESCE(e.bttt_asaltelp, '') AS asal_telp,
			COALESCE(e.bttt_tujuanagenid, '') AS tujuan_agen_id,
			COALESCE(e.bttt_tujuannama, '') AS tujuan_nama,
			COALESCE(e.bttt_tujuanalamat, '') AS tujuan_alamat,
			COALESCE(e.bttt_tujuankota, '') AS tujuan_kota,
			COALESCE(e.bttt_tujuantelp, '') AS tujuan_telp,
			COALESCE(e.bttt_tujuankelurahan, '') AS tujuan_kelurahan,
			COALESCE(e.bttt_tujuankecamatan, '') AS tujuan_kecamatan,
			COALESCE(e.bttt_tujuanpulau, '') AS tujuan_pulau,
			COALESCE(e.bttt_tujuankodepos, '') AS tujuan_kodepos,
			COALESCE(e.bttt_up, '') AS up,
			COALESCE(e.bttt_ket, '') AS ket,
			COALESCE(e.bttt_nosuratjalan, '') AS no_surat_jalan,
			COALESCE(e.bttt_namabarang, '') AS nama_barang,
			COALESCE(e.bttt_jmlunit, 0) AS jml_unit,
			COALESCE(e.bttt_jmlpck, 0) AS jml_pck,
			COALESCE(e.bttt_berat, 0) AS berat,
			COALESCE(e.bttt_beratvol, 0) AS berat_vol,
			COALESCE(e.bttt_ukuran, 0) AS ukuran,
			COALESCE(e.bttt_harga, 0) AS harga,
			COALESCE(e.bttt_biayapenerus, 0) AS biaya_penerus,
			COALESCE(e.bttt_packingid, 0) AS packing_id,
			(COALESCE(e.bttt_harga, 0) + COALESCE(e.bttt_biayapenerus, 0) + COALESCE(e.bttt_packingid, 0)) AS total_tagih,
			COALESCE(e.bttt_servid, '1') AS serv_id,
			COALESCE(e.bttt_bayaryn, 'N') AS bayar_yn,
			COALESCE(e.bttt_postingyn, 'N') AS posting_yn,
			COALESCE(e.bttt_cbyn, 'N') AS cb_yn,
			COALESCE(e.bttt_smuno, '') AS smu_no
		`).
		Joins("LEFT JOIN public.mkt_t_econote_tagih tt ON TRIM(tt.btt_turun_id) = TRIM(e.bttt_id)").
		Joins("LEFT JOIN public.glb_m_agen a ON (TRIM(a.agen_id::varchar) = TRIM(e.bttt_asalagenid::varchar) OR a.agen_id::varchar = LPAD(LEFT(e.bttt_id, 3), 3, '0'))").
		Joins("LEFT JOIN public.mkt_m_customer ct ON TRIM(ct.cust_id) = TRIM(tt.btt_turun_customerid)").
		Where("TRIM(e.bttt_id) = TRIM(?)", bttID).
		Take(&item).Error

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data BTT tidak ditemukan: " + err.Error()})
		return
	}

	// Cek apakah sudah dibuat invoice
	var invCount int64
	database.Table("public.art_t_invoiced d").
		Joins("LEFT JOIN public.art_t_invoiceh h ON TRIM(h.artih_id) = TRIM(d.artid_artihid)").
		Where("TRIM(d.artid_bttid) = TRIM(?) AND COALESCE(h.artih_delete, 'N') <> 'Y'", bttID).
		Count(&invCount)

	item.SudahInvoice = invCount > 0

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   item,
	})
}

// 3. POST /api/piutang/btt-tagih-tujuan/save (Simpan Customer Tertagih di Tujuan)
func SaveBTTTagihTujuanHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	userID, _ := c.Get("username")

	var req UpdateBTTTagihReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	bttID := strings.TrimSpace(req.BTTTurunID)
	custID := strings.TrimSpace(req.BTTTurunCustomerID)

	var cbYN string
	database.Table("public.mkt_t_econote").Select("COALESCE(bttt_cbyn, 'N')").Where("TRIM(bttt_id) = TRIM(?)", bttID).Scan(&cbYN)
	if cbYN == "Y" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Transaksi BTT ini sudah di-closing dan tidak dapat diubah lagi."})
		return
	}

	var invCount int64
	database.Table("public.art_t_invoiced d").
		Joins("LEFT JOIN public.art_t_invoiceh h ON TRIM(h.artih_id) = TRIM(d.artid_artihid)").
		Where("TRIM(d.artid_bttid) = TRIM(?) AND COALESCE(h.artih_delete, 'N') <> 'Y'", bttID).
		Count(&invCount)
	if invCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "BTT ini sudah dibuatkan Invoice penagihan piutang."})
		return
	}

	var exists int64
	database.Table("public.mkt_t_econote_tagih").Where("TRIM(btt_turun_id) = TRIM(?)", bttID).Count(&exists)

	if exists > 0 {
		updateMap := map[string]interface{}{
			"btt_turun_customerid": custID,
			"btt_turun_updateid":   fmt.Sprintf("%v", userID),
			"btt_turun_updatetime": time.Now(),
		}
		if err := database.Table("public.mkt_t_econote_tagih").Where("TRIM(btt_turun_id) = TRIM(?)", bttID).Updates(updateMap).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update customer tagih: " + err.Error()})
			return
		}
	} else {
		insertMap := map[string]interface{}{
			"btt_turun_id":         bttID,
			"btt_turun_tanggal":    time.Now(),
			"btt_turun_customerid": custID,
			"btt_turun_terbayaryn": "N",
			"btt_turun_updateid":   fmt.Sprintf("%v", userID),
			"btt_turun_updatetime": time.Now(),
		}
		if err := database.Table("public.mkt_t_econote_tagih").Create(&insertMap).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal simpan customer tagih: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("BTT %s berhasil ditagihkan kepada customer %s.", bttID, custID),
	})
}
