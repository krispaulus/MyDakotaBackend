package handler

import (
	"fmt"
	"net/http"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 🔍 1. LIST DATA PENGAMBILAN RETUR (LENGKAP DAN TANPA ERR COLUMN ID)
func GetAmbilReturList(c *gin.Context) {
	var resultList []map[string]interface{}
	searchID := c.Query("ambilretur_id")
	searchBtt := c.Query("btt_id")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	// ✅ PERBAIKAN: Hapus a.id karena primary key tabel adalah ambilretur_id
	rawSQL := `
		SELECT 
			a.ambilretur_id,
			TO_CHAR(a.ambilretur_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS ambilretur_tanggal,
			a.ambilretur_bttid,
			a.ambilretur_chekernip AS ambilretur_checkernip,
			a.ambilretur_custnama,
			a.ambilretur_custid,
			a.ambilretur_custalamat,
			a.ambilretur_custtelp,
			a.ambilretur_custjnsid,
			a.ambilretur_skyn,
			a.ambilretur_foldersk,
			a.ambilretur_aktifyn,
			COALESCE(k.kry_nama, '-') AS kry_nama
		FROM public.opr_t_eambilretur a
		LEFT OUTER JOIN public.hrd_m_karyawan k ON a.ambilretur_chekernip = k.kry_nip
		WHERE 1=1
	`
	var args []interface{}

	if searchID != "" {
		rawSQL += " AND a.ambilretur_id ILIKE ?"
		args = append(args, "%"+searchID+"%")
	}
	if searchBtt != "" {
		rawSQL += " AND a.ambilretur_bttid ILIKE ?"
		args = append(args, "%"+searchBtt+"%")
	}

	rawSQL += " ORDER BY a.ambilretur_tanggal DESC, a.ambilretur_id DESC"

	err := database.Raw(rawSQL, args...).Scan(&resultList).Error
	if err != nil {
		fmt.Println("❌ [CRASH RAW BYPASS SQL]:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resultList,
	})
}

// 💾 2. CREATE PENGAMBILAN RETUR
func CreateAmbilRetur(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTEambilRetur
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid: " + err.Error()})
		return
	}

	now := time.Now()
	req.AmbilReturTanggal = now.Format("2006-01-02")
	req.AmbilReturJam = now.Format("15")
	req.AmbilReturMenit = now.Format("04")
	req.AmbilReturUpdateTime = now.Format("2006-01-02 15:04:05")
	req.AmbilReturAktifYN = "Y"

	if req.AmbilReturSKYN == "" {
		req.AmbilReturSKYN = "N"
	}

	if err := database.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memproses penyerahan barang retur: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Transaksi pengambilan barang retur berhasil direkam!",
		"data":    req,
	})
}

// ✏️ 3. UPDATE PENGAMBILAN RETUR
func UpdateAmbilRetur(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTEambilRetur
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	updateSQL := `
		UPDATE public.opr_t_eambilretur 
		SET ambilretur_bttid = ?, 
		    ambilretur_chekernip = ?, 
		    ambilretur_custnama = ?, 
		    ambilretur_custalamat = ?, 
		    ambilretur_custtelp = ?, 
		    ambilretur_custjnsid = ?, 
		    ambilretur_custid = ?, 
		    ambilretur_skyn = ?, 
		    ambilretur_foldersk = ?, 
		    ambilretur_updateid = ?, 
		    ambilretur_updatetime = ?
		WHERE ambilretur_id = ?
	`

	if err := database.Exec(
		updateSQL,
		req.AmbilReturBTTID,
		req.AmbilReturCheckerNIP,
		req.AmbilReturCustNama,
		req.AmbilReturCustAlamat,
		req.AmbilReturCustTelp,
		req.AmbilReturCustJnsID,
		req.AmbilReturCustID,
		req.AmbilReturSKYN,
		req.AmbilReturFolderSK,
		fmt.Sprintf("%v", username),
		now,
		req.AmbilReturID,
	).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate data retur: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data transaksi pengambilan retur berhasil diperbarui!",
	})
}

// 🗑️ 4. DELETE PENGAMBILAN RETUR
func DeleteAmbilRetur(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	returID := c.Query("ambilretur_id")
	if returID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter ambilretur_id wajib diisi"})
		return
	}

	deleteSQL := `DELETE FROM public.opr_t_eambilretur WHERE ambilretur_id = ?`
	if err := database.Exec(deleteSQL, returID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus data retur: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data transaksi pengambilan retur berhasil dihapus!",
	})
}
