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

// GET API: Laporan LSBP / Informasi LSPB
func GetLSPBReportList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	// Catch Filter Query Parameters
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

	// Build Raw SQL dengan Left Join ke Logistik Mobil In/Out
	queryStr := `
		SELECT 
			v.*,
			c.nomobil AS no_mobil,
			c.jammobilmasuk AS jam_mobil_masuk,
			c.jammobilkeluar AS jam_mobil_keluar
		FROM vw_transport_lspb v
		LEFT JOIN public.opr_t_transportlspbwithcarinoutanddatetime c 
			ON v."DN_OD_SJ" = c.nodo
		WHERE v."DN_OD_SJ" IS NOT NULL AND v."DN_OD_SJ" <> ''
	`

	var conditions []string
	var args []interface{}

	// Filter Tanggal Handover
	if tglAwal != "" && tglAkhir != "" {
		conditions = append(conditions, `v."Tanggal_HandOver" BETWEEN ? AND ?`)
		args = append(args, tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}

	// Filter Customer
	if customer != "" {
		conditions = append(conditions, `UPPER(v."Cust_Name") LIKE UPPER(?)`)
		args = append(args, "%"+customer+"%")
	}

	// Filter No. BTT
	if nobtt != "" {
		conditions = append(conditions, `v."nomorbtt" LIKE ?`)
		args = append(args, "%"+nobtt+"%")
	}

	// Filter SPB
	if spb != "" {
		conditions = append(conditions, `v."spb_id" LIKE ?`)
		args = append(args, "%"+spb+"%")
	}

	// Filter No. DN / OD / SJ
	if nosj != "" {
		conditions = append(conditions, `v."DN_OD_SJ" = ?`)
		args = append(args, nosj)
	}

	// Filter Tujuan Kota
	if tujuan != "" {
		conditions = append(conditions, `UPPER(v."BTTT_TujuanKota") LIKE UPPER(?)`)
		args = append(args, "%"+tujuan+"%")
	}

	// Filter Service (1 = Darat, 2 = Laut, 3 = Udara)
	if service != "" {
		conditions = append(conditions, `v."bttt_servid" = ?`)
		args = append(args, service)
	}

	// Filter Status Kiriman
	if status == "DELIVERED" {
		conditions = append(conditions, `v."Nama_Penerima" IS NOT NULL AND v."Nama_Penerima" <> ''`)
	} else if status == "SHIPPED" {
		conditions = append(conditions, `(v."Nama_Penerima" IS NULL OR v."Nama_Penerima" = '')`)
	}

	// Filter BTT Sudah Kembali (kirim Y/N)
	if kirim == "Y" {
		conditions = append(conditions, `v."TB_TglKembali" IS NOT NULL AND v."TB_TglKembali" <> ''`)
	} else if kirim == "N" {
		conditions = append(conditions, `(v."TB_TglKembali" IS NULL OR v."TB_TglKembali" = '')`)
	}

	// Filter Sudah SPB (bayar Y/N)
	if bayar == "Y" {
		conditions = append(conditions, `v."spb_id" IS NOT NULL AND v."spb_id" <> ''`)
	} else if bayar == "N" {
		conditions = append(conditions, `(v."spb_id" IS NULL OR v."spb_id" = '')`)
	}

	// Gabungkan kondisi WHERE
	if len(conditions) > 0 {
		queryStr += " AND " + strings.Join(conditions, " AND ")
	}

	queryStr += ` ORDER BY v."Tanggal_HandOver" DESC LIMIT 500`

	var results []models.LSPBReportDTO
	err := database.Raw(queryStr, args...).Scan(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memuat laporan LSBP: " + err.Error(),
		})
		return
	}

	// Kalkulasi Status Ontime / Dalam Pengiriman
	for i := range results {
		if results[i].TanggalDiterima != "" {
			results[i].OnTime = 1
			results[i].MissLeadTime = 0
			results[i].DalamPengiriman = 0
		} else {
			results[i].OnTime = 0
			results[i].MissLeadTime = 0
			results[i].DalamPengiriman = 1
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
		"count":  len(results),
	})
}

// SAVE / UPDATE LSBP KENDARAAN (OPR_T_TRANSPORTLSPBWITHCARINOUTANDDATETIME)
func SaveLSPB(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

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

	// Jika ID > 0 berarti Edit/Update
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

	// Create Baru
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
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

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
