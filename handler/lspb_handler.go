package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// GET /api/operasional/laporan-lspb/customers
// Mengambil daftar Customer aktif seperti di aplikasi ASP lawas
func GetLSPBCustomers(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = c.Query("pt_id")
		if ptID == "" {
			ptID = "C"
		}
	}
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	var customers []map[string]interface{}
	err := database.Raw(`
		SELECT DISTINCT TRIM(cust_id) AS cust_id, TRIM(cust_name) AS cust_name
		FROM public.mkt_m_customer
		WHERE UPPER(COALESCE(cust_aktifyn, 'Y')) = 'Y'
		  AND TRIM(cust_id) LIKE '001%'
		  AND TRIM(COALESCE(cust_name, '')) <> ''
		ORDER BY TRIM(cust_name) ASC
	`).Scan(&customers).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": customers})
}

// GET API: Laporan lspb / Informasi LSPB (Mendukung 37 Kolom Penuh + Flag INV/DO)
func GetLSPBReportList(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = c.Query("pt_id")
		if ptID == "" {
			ptID = "C"
		}
	}
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	tglAwal := strings.TrimSpace(c.Query("tgla"))
	tglAkhir := strings.TrimSpace(c.Query("tgle"))
	customer := strings.TrimSpace(c.Query("customer"))
	nobtt := strings.TrimSpace(c.Query("nobtt"))
	spb := strings.TrimSpace(c.Query("spb"))
	nosj := strings.TrimSpace(c.Query("nosj"))
	tujuan := strings.TrimSpace(c.Query("tujuan"))
	service := strings.TrimSpace(c.Query("service"))
	status := strings.TrimSpace(c.Query("status"))
	kirim := strings.TrimSpace(c.Query("kirim"))
	bayar := strings.TrimSpace(c.Query("bayar"))

	var baseConditions []string
	var args []interface{}

	// 1. Filter utama langsung di sumber view agar cepat (< 150ms)
	if tglAwal != "" && tglAkhir != "" {
		baseConditions = append(baseConditions, `
			(
				CASE 
					WHEN v."Tanggal_HandOver"::text LIKE '%/%/%' THEN TO_DATE(SPLIT_PART(v."Tanggal_HandOver"::text, ' ', 1), 'MM/DD/YYYY')
					WHEN NULLIF(SPLIT_PART(COALESCE(v."Tanggal_HandOver"::text, ''), ' ', 1), '') IS NOT NULL THEN SPLIT_PART(v."Tanggal_HandOver"::text, ' ', 1)::date
					ELSE NULL
				END BETWEEN ?::date AND ?::date
			)
		`)
		args = append(args, tglAwal, tglAkhir)
	}
	if nobtt != "" {
		baseConditions = append(baseConditions, `v."nomorbtt" ILIKE ?`)
		args = append(args, "%"+nobtt+"%")
	}
	if spb != "" {
		baseConditions = append(baseConditions, `v.spb_id ILIKE ?`)
		args = append(args, "%"+spb+"%")
	}
	if nosj != "" {
		baseConditions = append(baseConditions, `v."DN_OD_SJ" ILIKE ?`)
		args = append(args, "%"+nosj+"%")
	}
	if service != "" {
		baseConditions = append(baseConditions, `TRIM(COALESCE(v."bttt_servid"::text, '')) = ?`)
		args = append(args, service)
	}

	baseWhereSQL := ""
	if len(baseConditions) > 0 {
		baseWhereSQL = " AND " + strings.Join(baseConditions, " AND ")
	}

	queryStr := fmt.Sprintf(`
		WITH base_lspb AS (
			SELECT 
				TRIM(v."DN_OD_SJ") AS dn_od_sj,
				COALESCE(NULLIF(TRIM(m.cust_name), ''), NULLIF(TRIM(e.bttt_asalname), ''), v."Cust_Name", '-') AS cust_name,
				COALESCE(TRIM(t.customerid), TRIM(e.bttt_asalcustid), '') AS customer_id,
				CASE 
					WHEN v."Tanggal_HandOver"::text LIKE '%%/%%/%%' THEN TO_DATE(SPLIT_PART(v."Tanggal_HandOver"::text, ' ', 1), 'MM/DD/YYYY')
					WHEN NULLIF(SPLIT_PART(COALESCE(v."Tanggal_HandOver"::text, ''), ' ', 1), '') IS NOT NULL THEN SPLIT_PART(v."Tanggal_HandOver"::text, ' ', 1)::date
					ELSE NULL
				END AS dt_handover,
				COALESCE(TRIM(v."nomorbtt"), '') AS nomorbtt,
				COALESCE(NULLIF(TRIM(v."PickupFrom"), ''), 'WH MARUNDA') AS pickup_from,
				COALESCE(NULLIF(TRIM(e.bttt_tujuannama), ''), NULLIF(TRIM(v."BTTT_TujuanNama"), ''), '-') AS bttt_tujuannama,
				COALESCE(NULLIF(TRIM(e.bttt_tujuankota), ''), NULLIF(TRIM(v."BTTT_TujuanKota"), ''), '-') AS bttt_tujuan_kota,
				UPPER(COALESCE(NULLIF(TRIM(s.serv_name), ''), NULLIF(TRIM(v."Serv_Name"), ''), 'DARAT')) AS serv_name,

				COALESCE(v."Darat", 0)::int AS darat,
				COALESCE(v."Laut", 0)::int AS laut,
				COALESCE(v."Udara", 0)::int AS udara,
				COALESCE(NULLIF(regexp_replace(COALESCE(t.harilibur::text, ''), '[^0-9-]', '', 'g'), '')::int, 0) AS hari_libur_manual,

				CASE 
					WHEN NULLIF(TRIM(COALESCE(v."Tanggal_Diterima"::text, '')), '') IS NOT NULL THEN
						CASE 
							WHEN v."Tanggal_Diterima"::text LIKE '%%/%%/%%' THEN TO_DATE(SPLIT_PART(v."Tanggal_Diterima"::text, ' ', 1), 'MM/DD/YYYY')
							ELSE SPLIT_PART(v."Tanggal_Diterima"::text, ' ', 1)::date
						END
					ELSE NULL
				END AS dt_diterima,
				COALESCE(NULLIF(TRIM(v."Nama_Penerima"), ''), '') AS nama_penerima,
				CASE 
					WHEN UPPER(COALESCE(v."Keterangan", '')) LIKE '%%GAGAL%%' THEN 1
					ELSE 0
				END AS is_gagal,

				COALESCE(v."Keterangan", '') AS keterangan,
				CASE 
					WHEN NULLIF(SPLIT_PART(COALESCE(v."TB_TglKembali"::text, ''), ' ', 1), '') IS NOT NULL 
					THEN SPLIT_PART(v."TB_TglKembali"::text, ' ', 1)::date
					ELSE NULL
				END AS dt_dokumen_kembali,
				COALESCE(v.spb_id, '') AS spb_id,
				COALESCE(v.artih_nokw, '') AS artih_nokw,
				COALESCE(NULLIF(TRIM(c.nomobil), ''), '') AS no_mobil,

				COALESCE(SPLIT_PART(COALESCE(v."Tanggal_berangkat"::text, ''), ' ', 1), '') AS tanggal_berangkat,
				COALESCE(to_jsonb(t)->>'tanggal_kendaraan_tiba', c.jammobiltiba, '') AS tanggal_kendaraan_tiba,
				COALESCE(to_jsonb(t)->>'tanggal_pi_approve', v.artih_tglacc, '') AS artih_tgl_acc,
				COALESCE(to_jsonb(t)->>'inv', 'N') AS inv,
				COALESCE(to_jsonb(t)->>'grn', 'N') AS grn,
				COALESCE(to_jsonb(t)->>'fp', 'N') AS fp,
				COALESCE(to_jsonb(t)->>'dll', 'N') AS dll,

				COALESCE(e.bttt_jmlunit, v."BTTT_JmlUnit", 1) AS bttt_jmlunit,
				COALESCE(e.bttt_beratvol, v."BTTT_BeratVol", 0) AS bttt_beratvol,
				COALESCE(NULLIF(e.bttt_ukuran::text, ''), v."BTTT_Ukuran", '0') AS bttt_ukuran,
				COALESCE(e.bttt_berat, v."BTTT_Berat", 1) AS bttt_berat,
				v."bttt_servid" AS bttt_servid
			FROM vw_transport_lspb v
			LEFT JOIN public.opr_t_transportlspb t
				ON t.dn_od_sj::text = v."DN_OD_SJ"::text
				AND COALESCE(t.nomorbtt::text, '') = COALESCE(v."nomorbtt"::text, '')
			LEFT JOIN public.mkt_t_econote e 
				ON e.bttt_id::text = v."nomorbtt"::text
			LEFT JOIN public.mkt_m_service s
				ON s.serv_id::text = e.bttt_servid::text
			LEFT JOIN public.mkt_m_customer m 
				ON m.cust_id::text = COALESCE(NULLIF(TRIM(t.customerid::text), ''), e.bttt_asalcustid::text)
			LEFT JOIN public.opr_t_transportlspbwithcarinoutanddatetime c 
				ON v."DN_OD_SJ"::text = c.nodo::text
			WHERE v."DN_OD_SJ" IS NOT NULL AND v."DN_OD_SJ" <> ''
			%s
			LIMIT 1000
		),
		calc_lspb AS (
			SELECT
				b.*,
				CASE 
					WHEN b.serv_name = 'LAUT' AND b.laut > 0 THEN b.laut
					WHEN b.serv_name = 'UDARA' AND b.udara > 0 THEN b.udara
					WHEN b.darat > 0 THEN b.darat
					ELSE GREATEST(b.darat, b.laut, b.udara, 1)
				END AS leadtime_aktif,
				CASE 
					WHEN b.hari_libur_manual > 0 THEN b.hari_libur_manual
					WHEN b.dt_handover IS NOT NULL THEN (
						SELECT COUNT(*)::int
						FROM generate_series(
							b.dt_handover + 1,
							b.dt_handover + GREATEST(b.darat, b.laut, b.udara, 1),
							'1 day'::interval
						) gs
						WHERE EXTRACT(ISODOW FROM gs) IN (6, 7)
					)
					ELSE 0
				END AS hari_libur_calc
			FROM base_lspb b
		)
		SELECT
			dn_od_sj,
			cust_name,
			customer_id,
			COALESCE(TO_CHAR(dt_handover, 'YYYY-MM-DD'), '') AS tanggal_handover,
			nomorbtt,
			pickup_from,
			bttt_tujuannama,
			bttt_tujuan_kota,
			serv_name,
			darat,
			laut,
			udara,
			hari_libur_calc AS hari_libur,
			COALESCE(TO_CHAR(dt_diterima, 'YYYY-MM-DD'), '') AS tanggal_diterima,
			COALESCE(TO_CHAR(dt_handover + (leadtime_aktif + hari_libur_calc), 'YYYY-MM-DD'), '') AS tanggal_sampai_harusnya,
			CASE 
				WHEN dt_diterima IS NOT NULL AND dt_handover IS NOT NULL THEN (dt_diterima - dt_handover)
				ELSE 0
			END AS realisasi_hari,
			CASE 
				WHEN is_gagal = 0 AND dt_diterima IS NOT NULL AND dt_handover IS NOT NULL 
				     AND dt_diterima <= (dt_handover + (leadtime_aktif + hari_libur_calc)) THEN 1
				ELSE 0
			END AS on_time,
			CASE 
				WHEN is_gagal = 0 AND dt_diterima IS NOT NULL AND dt_handover IS NOT NULL 
				     AND dt_diterima > (dt_handover + (leadtime_aktif + hari_libur_calc)) THEN 1
				ELSE 0
			END AS miss_lead_time,
			is_gagal AS gagal,
			CASE 
				WHEN is_gagal = 0 AND dt_diterima IS NULL THEN 1
				ELSE 0
			END AS dalam_pengiriman,
			nama_penerima,
			keterangan,
			COALESCE(TO_CHAR(dt_handover + ((2 * leadtime_aktif) + hari_libur_calc + 1), 'YYYY-MM-DD'), '') AS tgl_dokumen_wajib_kembali,
			COALESCE(TO_CHAR(dt_dokumen_kembali, 'YYYY-MM-DD'), '') AS tb_tglkembali,
			CASE 
				WHEN dt_diterima IS NOT NULL AND dt_dokumen_kembali IS NOT NULL AND dt_dokumen_kembali > dt_diterima THEN (
					SELECT COUNT(*)::int
					FROM generate_series(dt_diterima + 1, dt_dokumen_kembali, '1 day'::interval) gs
					WHERE EXTRACT(ISODOW FROM gs) IN (6, 7)
				)
				ELSE 0
			END AS hari_libur_do,
			spb_id,
			artih_nokw,
			no_mobil,
			tanggal_berangkat,
			tanggal_kendaraan_tiba,
			artih_tgl_acc,
			inv,
			grn,
			fp,
			dll,
			bttt_jmlunit,
			bttt_beratvol,
			bttt_ukuran,
			bttt_berat,
			'' AS noskb
		FROM calc_lspb
		WHERE 1=1
	`, baseWhereSQL)

	var postConditions []string
	if customer != "" {
		postConditions = append(postConditions, `UPPER(cust_name) LIKE UPPER(?)`)
		args = append(args, "%"+customer+"%")
	}
	if tujuan != "" {
		postConditions = append(postConditions, `UPPER(bttt_tujuan_kota) LIKE UPPER(?)`)
		args = append(args, "%"+tujuan+"%")
	}
	if status == "DELIVERED" {
		postConditions = append(postConditions, `dt_diterima IS NOT NULL`)
	} else if status == "SHIPPED" {
		postConditions = append(postConditions, `dt_diterima IS NULL`)
	}
	if kirim == "Y" {
		postConditions = append(postConditions, `dt_dokumen_kembali IS NOT NULL`)
	} else if kirim == "N" {
		postConditions = append(postConditions, `dt_dokumen_kembali IS NULL`)
	}
	if bayar == "Y" {
		postConditions = append(postConditions, `TRIM(COALESCE(spb_id, '')) <> ''`)
	} else if bayar == "N" {
		postConditions = append(postConditions, `TRIM(COALESCE(spb_id, '')) = ''`)
	}

	if len(postConditions) > 0 {
		queryStr += " AND " + strings.Join(postConditions, " AND ")
	}

	queryStr += ` ORDER BY dt_handover DESC NULLS LAST, dn_od_sj ASC LIMIT 500`

	var results []map[string]interface{}
	if err := database.Raw(queryStr, args...).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memuat laporan lspb: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
		"count":  len(results),
	})
}

// SAVE / UPDATE LSPB (Menyesuaikan logika p-opr_t_lspb_e.asp pada web lawas)
func SaveLSPB(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = c.Query("pt_id")
		if ptID == "" {
			ptID = "C"
		}
	}
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	userID, _ := c.Get("username")
	if userID == nil || fmt.Sprintf("%v", userID) == "" {
		userID = "SYSTEM"
	}

	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload tidak valid: " + err.Error()})
		return
	}

	getStr := func(key string) string {
		if v, ok := payload[key]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if s != "<nil>" {
				return s
			}
		}
		return ""
	}

	getBoolYN := func(key string) string {
		if v, ok := payload[key]; ok && v != nil {
			if b, isBool := v.(bool); isBool {
				if b {
					return "Y"
				}
				return "N"
			}
			s := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", v)))
			if s == "Y" || s == "TRUE" || s == "1" {
				return "Y"
			}
		}
		return "N"
	}

	parseDateOrNil := func(val string) interface{} {
		val = strings.TrimSpace(val)
		if val == "" {
			return nil
		}
		for _, layout := range []string{"2006-01-02", "01/02/2006", "02/01/2006", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, val); err == nil {
				return t.Format("2006-01-02")
			}
		}
		return val
	}

	isEdit := false
	if v, ok := payload["is_edit"].(bool); ok {
		isEdit = v
	}

	originalNoDO := getStr("original_nodo")
	originalNoBTT := getStr("original_nobtt")
	noDO := getStr("nodo")
	if originalNoDO == "" {
		originalNoDO = noDO
	}
	noBTT := getStr("noskb")
	if noBTT == "" {
		noBTT = getStr("nomorbtt")
	}
	if originalNoBTT == "" {
		originalNoBTT = noBTT
	}

	if noDO == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "DN / OD / SJ Number wajib diisi"})
		return
	}

	custName := getStr("cust_name")
	custID := getStr("customer_id")
	if custID == "" && custName != "" {
		database.Raw(`SELECT TRIM(cust_id) FROM public.mkt_m_customer WHERE UPPER(TRIM(cust_name)) = UPPER(TRIM(?)) LIMIT 1`, custName).Scan(&custID)
	}

	pickupFrom := strings.ToUpper(getStr("pickup_from"))
	keterangan := strings.ToUpper(getStr("keterangan"))
	namaPenerima := strings.ToUpper(getStr("nama_penerima"))

	tglHandover := parseDateOrNil(getStr("tanggal_handover"))
	tglBerangkat := parseDateOrNil(getStr("tanggal_berangkat"))
	tglKendTiba := parseDateOrNil(getStr("tanggal_kendaraan_tiba"))
	tglPIApprove := parseDateOrNil(getStr("tanggal_pi_disetujui"))
	tglDiterima := parseDateOrNil(getStr("tanggal_diterima"))

	invYN := getBoolYN("inv")
	grnYN := getBoolYN("grn")
	fpYN := getBoolYN("faktur_pajak")
	dllYN := getBoolYN("dokumen_lain")

	hariLibur := 0
	if hl, ok := payload["hari_libur"].(float64); ok {
		hariLibur = int(hl)
	}

	// Pastikan kolom pendukung di opr_t_transportlspb tersedia
	database.Exec(`ALTER TABLE public.opr_t_transportlspb ADD COLUMN IF NOT EXISTS tanggal_kendaraan_tiba DATE`)
	database.Exec(`ALTER TABLE public.opr_t_transportlspb ADD COLUMN IF NOT EXISTS tanggal_pi_approve DATE`)
	database.Exec(`ALTER TABLE public.opr_t_transportlspb ADD COLUMN IF NOT EXISTS inv VARCHAR(5) DEFAULT 'N'`)
	database.Exec(`ALTER TABLE public.opr_t_transportlspb ADD COLUMN IF NOT EXISTS grn VARCHAR(5) DEFAULT 'N'`)
	database.Exec(`ALTER TABLE public.opr_t_transportlspb ADD COLUMN IF NOT EXISTS fp VARCHAR(5) DEFAULT 'N'`)
	database.Exec(`ALTER TABLE public.opr_t_transportlspb ADD COLUMN IF NOT EXISTS dll VARCHAR(5) DEFAULT 'N'`)

	updates := map[string]interface{}{
		"dn_od_sj":               noDO,
		"nomorbtt":               noBTT,
		"pickupfrom":             pickupFrom,
		"keterangan":             keterangan,
		"tanggal_handover":       tglHandover,
		"tanggal_berangkat":      tglBerangkat,
		"tanggal_kendaraan_tiba": tglKendTiba,
		"tanggal_pi_approve":     tglPIApprove,
		"tanggal_diterima":       tglDiterima,
		"nama_penerima":          namaPenerima,
		"harilibur":              fmt.Sprintf("%d", hariLibur),
		"inv":                    invYN,
		"grn":                    grnYN,
		"fp":                     fpYN,
		"dll":                    dllYN,
	}
	if custID != "" {
		updates["customerid"] = custID
	}

	if isEdit {
		// 1. Update spesifik baris DO yang sedang dibuka (termasuk jika DN/OD/SJ-nya diubah)
		res := database.Table("public.opr_t_transportlspb").
			Where("UPPER(TRIM(dn_od_sj)) = UPPER(TRIM(?)) AND UPPER(TRIM(COALESCE(nomorbtt, ''))) = UPPER(TRIM(?))", originalNoDO, originalNoBTT).
			Updates(updates)

		if res.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal update LSPB: " + res.Error.Error()})
			return
		}
		if res.RowsAffected == 0 {
			database.Table("public.opr_t_transportlspb").
				Where("UPPER(TRIM(dn_od_sj)) = UPPER(TRIM(?))", originalNoDO).
				Updates(updates)
		}

		// 2. Update semua DO lain yang memiliki Nomor BTT sama (case-insensitive)
		targetBTT := noBTT
		if targetBTT == "" {
			targetBTT = originalNoBTT
		}

		totalDOUpdated := res.RowsAffected
		if targetBTT != "" {
			bulkUpdatesByBTT := make(map[string]interface{})
			for k, v := range updates {
				if k != "dn_od_sj" {
					bulkUpdatesByBTT[k] = v
				}
			}

			resBulk := database.Table("public.opr_t_transportlspb").
				Where("UPPER(TRIM(COALESCE(nomorbtt, ''))) = UPPER(TRIM(?)) AND UPPER(TRIM(dn_od_sj)) <> UPPER(TRIM(?))", targetBTT, noDO).
				Updates(bulkUpdatesByBTT)

			if resBulk.Error == nil {
				totalDOUpdated += resBulk.RowsAffected
			}

			// Sinkronkan juga ke opr_t_transportlspbwithcarinoutanddatetime untuk semua DO di BTT tersebut
			database.Exec(`
				UPDATE public.opr_t_transportlspbwithcarinoutanddatetime
				SET noskb = ?, updated_at = NOW()
				WHERE TRIM(nodo) IN (
					SELECT DISTINCT TRIM(dn_od_sj)
					FROM public.opr_t_transportlspb
					WHERE TRIM(COALESCE(nomorbtt, '')) = ?
				)
			`, targetBTT, targetBTT)
		}

		// Pastikan jika nomor DO utama berubah, tabel carinout ikut ter-update
		database.Exec(`
			UPDATE public.opr_t_transportlspbwithcarinoutanddatetime
			SET nodo = ?, noskb = ?, updated_at = NOW()
			WHERE TRIM(nodo) = ?
		`, noDO, targetBTT, originalNoDO)

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": fmt.Sprintf("Data LSPB berhasil diperbarui untuk %d DO pada BTT %s oleh %v!", totalDOUpdated, targetBTT, userID),
		})
		return
	}

	// Mode Tambah Baru
	if err := database.Table("public.opr_t_transportlspb").Create(&updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan LSPB baru: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Data LSPB baru %s berhasil disimpan!", noDO),
	})
}

// DELETE lspb KENDARAAN
func DeleteLSPB(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" {
		ptID = c.Query("pt_id")
		if ptID == "" {
			ptID = "C"
		}
	}
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.DB
	}

	nodo := strings.TrimSpace(c.Param("nodo"))
	if nodo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "No DO wajib disertakan"})
		return
	}

	database.Exec(`DELETE FROM public.opr_t_transportlspb WHERE TRIM(dn_od_sj) = ?`, nodo)
	database.Exec(`DELETE FROM public.opr_t_transportlspbwithcarinoutanddatetime WHERE TRIM(nodo) = ?`, nodo)

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data lspb berhasil dihapus"})
}
