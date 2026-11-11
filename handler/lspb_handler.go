package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// GET API: Laporan LSBP / Informasi LSPB (Mendukung 37 Kolom Penuh)
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

	// Tangkap semua parameter filter
	tglAwal := strings.TrimSpace(c.Query("tgla"))
	tglAkhir := strings.TrimSpace(c.Query("tgle"))
	customer := strings.TrimSpace(c.Query("customer"))
	nobtt := strings.TrimSpace(c.Query("nobtt"))
	spb := strings.TrimSpace(c.Query("spb"))
	nosj := strings.TrimSpace(c.Query("nosj"))
	tujuan := strings.TrimSpace(c.Query("tujuan"))
	service := strings.TrimSpace(c.Query("service"))
	status := strings.TrimSpace(c.Query("status")) // SHIPPED / DELIVERED
	kirim := strings.TrimSpace(c.Query("kirim"))   // Y / N
	bayar := strings.TrimSpace(c.Query("bayar"))   // Y / N

	queryStr := `
		SELECT 
			v."DN_OD_SJ" AS dn_od_sj,
			COALESCE(v."Cust_Name", '-') AS cust_name,
			COALESCE(v."Tanggal_HandOver", '') AS tanggal_handover,
			COALESCE(v."nomorbtt", '') AS nomorbtt,
			COALESCE(v."PickupFrom", 'WH MARUNDA') AS pickup_from,
			COALESCE(v."BTTT_TujuanNama", '-') AS bttt_tujuannama,
			COALESCE(v."BTTT_TujuanKota", '-') AS bttt_tujuan_kota,
			COALESCE(v."Serv_Name", 'DARAT') AS serv_name,
			COALESCE(v."Tanggal_berangkat", '') AS tanggal_berangkat,
			COALESCE(v."Darat", 1) AS darat,
			COALESCE(v."Laut", 0) AS laut,
			COALESCE(v."Udara", 0) AS udara,
			0 AS hari_libur,
			COALESCE(v."Tanggal_Diterima", '') AS tanggal_diterima,
			COALESCE(v."Tanggal_Diterima", v."Tanggal_HandOver") AS tanggal_sampai_harusnya,
			1 AS realisasi_hari,
			CASE WHEN COALESCE(v."Tanggal_Diterima", '') <> '' THEN 1 ELSE 0 END AS on_time,
			0 AS miss_lead_time,
			0 AS gagal,
			CASE WHEN COALESCE(v."Tanggal_Diterima", '') = '' THEN 1 ELSE 0 END AS dalam_pengiriman,
			COALESCE(v."Nama_Penerima", '-') AS nama_penerima,
			COALESCE(v."Keterangan", '-') AS keterangan,
			COALESCE(v."TB_TglKembali", '') AS tgl_dokumen_wajib_kembali,
			COALESCE(v."TB_TglKembali", '') AS tb_tglkembali,
			0 AS hari_libur_do,
			COALESCE(v.spb_id, '') AS spb_id,
			COALESCE(v.artih_nokw, '') AS artih_nokw,
			COALESCE(c.nomobil, '') AS no_mobil,
			COALESCE(c.jammobilmasuk, '') AS jam_mobil_masuk,
			COALESCE(c.jammobilkeluar, '') AS jam_mobil_keluar,
			COALESCE(c.jammobiltiba, '') AS tanggal_kend_tiba,
			COALESCE(v.artih_tglacc, '') AS artih_tgl_acc,
			COALESCE(v."BTTT_JmlUnit", 1) AS bttt_jmlunit,
			COALESCE(v."BTTT_BeratVol", 0) AS bttt_beratvol,
			COALESCE(v."BTTT_Ukuran", '-') AS bttt_ukuran,
			COALESCE(v."BTTT_Berat", 1) AS bttt_berat,
			COALESCE(v.nomorbtt, '') AS noskb
		FROM vw_transport_lspb v
		LEFT JOIN public.opr_t_transportlspbwithcarinoutanddatetime c 
			ON v."DN_OD_SJ" = c.nodo
		WHERE v."DN_OD_SJ" IS NOT NULL AND v."DN_OD_SJ" <> ''
	`

	var conditions []string
	var args []interface{}

	// 1. Filter Tanggal Handover
	if tglAwal != "" && tglAkhir != "" {
		conditions = append(conditions, `
			(
				CASE 
					WHEN v."Tanggal_HandOver"::text LIKE '%/%/%' 
						THEN TO_DATE(v."Tanggal_HandOver"::text, 'MM/DD/YYYY')
					ELSE v."Tanggal_HandOver"::date
				END BETWEEN ?::date AND ?::date
			)
		`)
		args = append(args, tglAwal, tglAkhir)
	}

	// 2. Filter Customer
	if customer != "" {
		conditions = append(conditions, `UPPER(v."Cust_Name") LIKE UPPER(?)`)
		args = append(args, "%"+customer+"%")
	}

	// 3. Filter No. BTT
	if nobtt != "" {
		conditions = append(conditions, `v."nomorbtt" LIKE ?`)
		args = append(args, "%"+nobtt+"%")
	}

	// 4. Filter SPB
	if spb != "" {
		conditions = append(conditions, `v."spb_id" LIKE ?`)
		args = append(args, "%"+spb+"%")
	}

	// 5. Filter No. DN / OD / SJ
	if nosj != "" {
		conditions = append(conditions, `v."DN_OD_SJ" = ?`)
		args = append(args, nosj)
	}

	// 6. Filter Tujuan Kota
	if tujuan != "" {
		conditions = append(conditions, `UPPER(v."BTTT_TujuanKota") LIKE UPPER(?)`)
		args = append(args, "%"+tujuan+"%")
	}

	// 7. Filter Service
	if service != "" {
		conditions = append(conditions, `v."bttt_servid" = ?`)
		args = append(args, service)
	}

	// 8. Filter Status Kiriman
	if status == "DELIVERED" {
		conditions = append(conditions, `v."Nama_Penerima" IS NOT NULL AND v."Nama_Penerima" <> ''`)
	} else if status == "SHIPPED" {
		conditions = append(conditions, `(v."Nama_Penerima" IS NULL OR v."Nama_Penerima" = '')`)
	}

	// 🌟 9. Filter BTT Sudah Kembali (kirim Y / N)
	if kirim == "Y" {
		conditions = append(conditions, `v."TB_TglKembali" IS NOT NULL AND v."TB_TglKembali" <> ''`)
	} else if kirim == "N" {
		conditions = append(conditions, `(v."TB_TglKembali" IS NULL OR v."TB_TglKembali" = '')`)
	}

	// 🌟 10. Filter Sudah SPB (bayar Y / N)
	if bayar == "Y" {
		conditions = append(conditions, `v.spb_id IS NOT NULL AND v.spb_id <> ''`)
	} else if bayar == "N" {
		conditions = append(conditions, `(v.spb_id IS NULL OR v.spb_id = '')`)
	}

	if len(conditions) > 0 {
		queryStr += " AND " + strings.Join(conditions, " AND ")
	}

	queryStr += ` ORDER BY v."Tanggal_HandOver" DESC LIMIT 500`

	var results []map[string]interface{}
	if err := database.Raw(queryStr, args...).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memuat laporan LSBP: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
		"count":  len(results),
	})
}

// SAVE / UPDATE LSBP KENDARAAN
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

	var input models.TransportLSPBWithCarInOut
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload tidak valid"})
		return
	}

	input.NoDO = strings.TrimSpace(input.NoDO)
	input.NoSKB = strings.TrimSpace(input.NoSKB)
	input.NoMobil = strings.ToUpper(strings.TrimSpace(input.NoMobil))

	if input.NoDO == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor DO / DN / SJ wajib diisi"})
		return
	}

	if input.ID > 0 {
		input.UpdatedAt = time.Now()
		err := database.Model(&models.TransportLSPBWithCarInOut{}).Where("id = ?", input.ID).Updates(input).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui data LSBP"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data LSBP berhasil diperbarui"})
		return
	}

	input.CreatedAt = time.Now()
	input.UpdatedAt = time.Now()
	if err := database.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan data LSBP baru"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data LSBP baru berhasil disimpan"})
}

// DELETE LSBP KENDARAAN
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

	nodo := c.Param("nodo")
	if nodo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "No DO wajib disertakan"})
		return
	}

	if err := database.Where("nodo = ?", nodo).Delete(&models.TransportLSPBWithCarInOut{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus data LSBP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data LSBP berhasil dihapus"})
}
