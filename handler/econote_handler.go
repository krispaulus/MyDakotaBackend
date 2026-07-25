package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Helper Cek Hak Akses WebRights
func checkRights(c *gin.Context, username string, serverID string, appRights string) bool {
	var count int64

	// 1. Coba query ke public.webrights terlebih dahulu
	err := db.DB.WithContext(c.Request.Context()).
		Table("public.webrights").
		Where("LOWER(username) = LOWER(?) AND (serverid = ? OR serverid = '1' OR serverid = '') AND appidrights = ?", username, serverID, appRights).
		Count(&count).Error

	// 2. Fallback query jika skema tanpa "public."
	if err != nil {
		_ = db.DB.WithContext(c.Request.Context()).
			Table("webrights").
			Where("LOWER(username) = LOWER(?) AND (serverid = ? OR serverid = '1' OR serverid = '') AND appidrights = ?", username, serverID, appRights).
			Count(&count)
	}

	return count > 0
}

// GET /api/master/econote/list
func GetEconoteList(c *gin.Context) {
	var filter models.EconoteFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Pagination default
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Limit <= 0 {
		filter.Limit = 15
	}
	offset := (filter.Page - 1) * filter.Limit

	// Ambil Server/Cabang ID & Username dari Session JWT
	serverID := c.GetString("server_id")
	if serverID == "" {
		serverID = c.GetString("kode_cabang")
	}
	if len(serverID) < 3 && serverID != "" {
		serverID = fmt.Sprintf("%03s", serverID)
	}
	username := c.GetString("username")

	// 1. Cek Hak Akses User
	rights := models.UserMenuRights{
		CanAdd:    checkRights(c, username, serverID, "E2a"),
		CanEdit:   checkRights(c, username, serverID, "E2b"),
		CanDelete: checkRights(c, username, serverID, "E2c"),
	}

	// 2. Query Utama BTT Pengiriman (mkt_t_econote)
	query := db.DB.WithContext(c.Request.Context()).
		Table("mkt_t_econote").
		Select(`
			mkt_t_econote.bttt_id, 
			mkt_t_econote.bttt_tanggal, 
			mkt_t_econote.bttt_servid, 
			mkt_t_econote.bttt_asalcustid, 
			mkt_t_econote.bttt_asalname, 
			mkt_t_econote.bttt_tujuannama, 
			mkt_t_econote.bttt_pembayaran, 
			mkt_t_econote.bttt_namabarang, 
			mkt_t_econote.bttt_nosuratjalan, 
			mkt_t_econote.bttt_jmlunit, 
			mkt_t_econote.bttt_berat, 
			mkt_t_econote.bttt_ukuran, 
			mkt_t_econote.bttt_aktifyn, 
			mkt_t_econote.bttt_tagihtujuan,
			COALESCE(mkt_m_customer.cust_name, '') as cust_name
		`).
		Joins("LEFT JOIN mkt_m_customer ON mkt_t_econote.bttt_asalcustid = mkt_m_customer.cust_id")

	// Filter berdasarkan Cabang / Agen ID jika ada
	if serverID != "" && serverID != "000" && serverID != "1" {
		query = query.Where("SUBSTRING(mkt_t_econote.bttt_id, 1, 3) = ?", serverID)
	}

	// Dynamic Multi-Filtering
	if filter.TanggalStart != "" && filter.TanggalEnd != "" && filter.TanggalStart != filter.TanggalEnd {
		query = query.Where("mkt_t_econote.bttt_tanggal BETWEEN ? AND ?",
			filter.TanggalStart+" 00:00:00", filter.TanggalEnd+" 23:59:59")
	}

	if filter.Service != "" {
		query = query.Where("mkt_t_econote.bttt_servid = ?", filter.Service)
	}

	if filter.Customer != "" {
		query = query.Where("LOWER(mkt_m_customer.cust_name) LIKE LOWER(?) OR LOWER(mkt_t_econote.bttt_asalname) LIKE LOWER(?)",
			"%"+filter.Customer+"%", "%"+filter.Customer+"%")
	}

	if filter.Tujuan != "" {
		query = query.Where("LOWER(mkt_t_econote.bttt_tujuankota) LIKE LOWER(?)", "%"+filter.Tujuan+"%")
	}

	if filter.TujPulau != "" {
		query = query.Where("LOWER(mkt_t_econote.bttt_tujuanpulau) LIKE LOWER(?)", "%"+filter.TujPulau+"%")
	}

	if filter.Posting != "" {
		vPosting := "N"
		if filter.Posting == "Ya" || filter.Posting == "Y" {
			vPosting = "Y"
		}
		query = query.Where("mkt_t_econote.bttt_postingyn = ?", vPosting)
	}

	if filter.Kirim != "" {
		vKirim := "N"
		if filter.Kirim == "Ya" || filter.Kirim == "Y" {
			vKirim = "Y"
		}
		query = query.Where("mkt_t_econote.bttt_kirimyn = ?", vKirim)
	}

	if filter.NoBTT != "" {
		query = query.Where("mkt_t_econote.bttt_id LIKE ?", "%"+filter.NoBTT+"%")
	}

	if filter.NoSMU != "" {
		query = query.Where("mkt_t_econote.bttt_smuno LIKE ?", "%"+filter.NoSMU+"%")
	}

	if filter.Agen != "" {
		vAgen := "N"
		if filter.Agen == "Ya" || filter.Agen == "Y" {
			vAgen = "Y"
		}
		query = query.Where("mkt_t_econote.bttt_agenyn = ?", vAgen)
	}

	if filter.Bayar != "" {
		vBayar := "N"
		if filter.Bayar == "Ya" || filter.Bayar == "Y" {
			vBayar = "Y"
		}
		query = query.Where("mkt_t_econote.bttt_bayaryn = ?", vBayar)
	}

	if filter.Pembayaran != "" {
		query = query.Where("mkt_t_econote.bttt_pembayaran = ?", filter.Pembayaran)
	}

	if filter.NoSJ != "" {
		query = query.Where("mkt_t_econote.bttt_nosuratjalan = ?", filter.NoSJ)
	}

	// Hitung Total Data untuk Pagination
	var totalRecords int64
	query.Count(&totalRecords)

	// Fetch Data Result dengan Ordering & Limit
	var rawList []models.Econote
	err := query.Order("mkt_t_econote.bttt_tanggal DESC, mkt_t_econote.bttt_id DESC").
		Limit(filter.Limit).
		Offset(offset).
		Find(&rawList).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data BTT pengiriman: " + err.Error(),
		})
		return
	}

	// Format Field Alias (JnBayar, ServisJD, AktifJD)
	for i := range rawList {
		switch rawList[i].BtttPembayaran {
		case 1:
			rawList[i].JnBayar = "Tunai"
		case 2:
			rawList[i].JnBayar = "Kredit"
		default:
			rawList[i].JnBayar = "Tagih"
		}

		switch rawList[i].BtttServID {
		case 1:
			rawList[i].ServisJD = "Darat"
		case 2:
			rawList[i].ServisJD = "Laut"
		default:
			rawList[i].ServisJD = "Udara"
		}

		if rawList[i].BtttAktifYN == "Y" {
			rawList[i].AktifJD = "Ya"
		} else {
			rawList[i].AktifJD = "Tidak"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          rawList,
		"total_records": totalRecords,
		"page":          filter.Page,
		"limit":         filter.Limit,
		"rights":        rights,
	})
}

// GET /api/master/econote/detail/:id (GET SINGLE BTT)
func GetEconoteDetail(c *gin.Context) {
	bttID := c.Param("id")

	var econote models.Econote
	err := db.DB.WithContext(c.Request.Context()).
		Table("mkt_t_econote").
		Where("bttt_id = ?", bttID).
		First(&econote).Error

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data BTT tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": econote})
}

// PUT /api/master/econote/update (UPDATE BTT)
func UpdateEconote(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	bttID, ok := req["bttt_id"].(string)
	if !ok || bttID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor BTT wajib diisi"})
		return
	}

	username := c.GetString("username")
	if username == "" {
		username = "system"
	}

	query := `
		UPDATE mkt_t_econote 
		SET bttt_asalname = $1, bttt_tujuannama = $2, bttt_namabarang = $3, 
		    bttt_nosuratjalan = $4, bttt_jmlunit = $5, bttt_berat = $6, 
		    bttt_tagihtujuan = $7, bttt_updateid = $8, bttt_updatetime = NOW()
		WHERE bttt_id = $9
	`

	err := db.DB.WithContext(c.Request.Context()).Exec(query,
		req["bttt_asalname"],
		req["bttt_tujuannama"],
		req["bttt_namabarang"],
		req["bttt_nosuratjalan"],
		req["bttt_jmlunit"],
		req["bttt_berat"],
		req["bttt_tagihtujuan"],
		username,
		bttID,
	).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate BTT: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data BTT berhasil diperbarui"})
}

// DELETE /api/master/econote/delete/:id (SOFT DELETE BTT -> BTTT_AktifYN = 'N')
func DeleteEconote(c *gin.Context) {
	bttID := c.Param("id")
	username := c.GetString("username")
	if username == "" {
		username = "system"
	}

	query := `UPDATE mkt_t_econote SET bttt_aktifyn = 'N', bttt_updateid = $1, bttt_updatetime = NOW() WHERE bttt_id = $2`

	err := db.DB.WithContext(c.Request.Context()).Exec(query, username, bttID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus BTT: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "BTT berhasil dinonaktifkan"})
}
