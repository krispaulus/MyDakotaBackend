package handler

import (
	"fmt"
	"net/http"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 🔍 1. PENCARIAN & HISTORY LIST DATA PENGAMBILAN
func GetAmbilList(c *gin.Context) {
	var ambilList []models.OprTEambil
	searchID := c.Query("ambil_id")
	searchBtt := c.Query("btt_id")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	// ✅ PERBAIKAN: ON opr_t_eambil.ambil_chekernip = hrd_m_karyawan.kry_nip (tanpa 'c')
	query := database.Table("opr_t_eambil").
		Select("opr_t_eambil.*, hrd_m_karyawan.kry_nama").
		Joins("LEFT OUTER JOIN hrd_m_karyawan ON opr_t_eambil.ambil_chekernip = hrd_m_karyawan.kry_nip")

	// Filter pencarian
	if searchID != "" {
		query = query.Where("opr_t_eambil.ambil_id ILIKE ?", "%"+searchID+"%")
	}
	if searchBtt != "" {
		query = query.Where("opr_t_eambil.ambil_bttid ILIKE ?", "%"+searchBtt+"%")
	}

	// Order berdasarkan tanggal & ID
	err := query.Order("opr_t_eambil.ambil_tanggal DESC, opr_t_eambil.ambil_id DESC").Find(&ambilList).Error
	if err != nil {
		fmt.Println("❌ [CRASH QUERY AMBIL BARANG]:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data pengambilan barang: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   ambilList,
	})
}

// 💾 2. ENTRI / PEMBUATAN TRANSAKSI PENGAMBILAN BARANG BARU
func CreateAmbil(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTEambil
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid: " + err.Error()})
		return
	}

	now := time.Now()
	req.AmbilTanggal = now.Format("2006-01-02")
	req.AmbilJam = now.Format("15")
	req.AmbilMenit = now.Format("04")
	req.AmbilUpdateTime = now.Format("2006-01-02 15:04:05")
	req.AmbilAktifYN = "Y"
	if req.AmbilSKYN == "" {
		req.AmbilSKYN = "N"
	}

	if err := database.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memproses penyerahan barang: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Transaksi penyerahan barang berhasil direkam!",
		"data":    req,
	})
}

// ✏️ 3. UPDATE TRANSAKSI PENGAMBILAN BARANG (SOLUSI UNDEFINED)
func UpdateAmbil(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTEambil
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	updateSQL := `
		UPDATE public.opr_t_eambil 
		SET ambil_bttid = ?, 
		    ambil_checkernip = ?, 
		    ambil_custnama = ?, 
		    ambil_custalamat = ?, 
		    ambil_custtelp = ?, 
		    ambil_custjnsid = ?, 
		    ambil_custid = ?, 
		    ambil_skyn = ?, 
		    ambil_foldersk = ?, 
		    ambil_updateid = ?, 
		    ambil_updatetime = ?
		WHERE ambil_id = ?
	`

	if err := database.Exec(
		updateSQL,
		req.AmbilBTTID,
		req.AmbilCheckerNIP,
		req.AmbilCustNama,
		req.AmbilCustAlamat,
		req.AmbilCustTelp,
		req.AmbilCustJnsID,
		req.AmbilCustID,
		req.AmbilSKYN,
		req.AmbilFolderSK,
		fmt.Sprintf("%v", username),
		now,
		req.AmbilID,
	).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate data pengambilan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data transaksi pengambilan berhasil diperbarui!",
	})
}

// 🗑️ 4. DELETE TRANSAKSI PENGAMBILAN BARANG (SOLUSI UNDEFINED)
func DeleteAmbil(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	ambilID := c.Query("ambil_id")
	if ambilID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter ambil_id wajib diisi"})
		return
	}

	deleteSQL := `DELETE FROM public.opr_t_eambil WHERE ambil_id = ?`
	if err := database.Exec(deleteSQL, ambilID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus data pengambilan: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data transaksi pengambilan berhasil dihapus!",
	})
}
