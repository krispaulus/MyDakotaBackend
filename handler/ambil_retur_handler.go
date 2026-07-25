package handler

import (
	"fmt"
	"net/http"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

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

	// 🟩 TULIS SQL MURNI BIAR POSTGRES MEMUNTAHKAN LOWERCASE SNAKE_CASE SECARA PAKSA BRAY!
	rawSQL := `
		SELECT 
			a.id,
			a.ambilretur_id,
			a.ambilretur_tanggal,
			a.ambilretur_bttid,
			a.ambilretur_checkernip,
			a.ambilretur_custnama,
			a.ambilretur_custid,
			a.ambilretur_skyn,
			a.ambilretur_aktifyn,
			k.kry_nama
		FROM public.opr_t_eambilretur a
		LEFT OUTER JOIN public.hrd_m_karyawan k ON a.ambilretur_checkernip = k.kry_nip
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

	rawSQL += " ORDER BY a.id DESC"

	// Eksekusi scanning bypass structural GORM bray
	err := database.Raw(rawSQL, args...).Scan(&resultList).Error
	if err != nil {
		fmt.Println("❌ [CRASH RAW BYPASS SQL]:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resultList,
	})
}

// 💾 2. POST ENTRI TRANSAKSI PENGAMBILAN BARANG RETUR BARU
func CreateAmbilRetur(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req models.OprTEambilRetur
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Format data tidak valid"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memproses penyerahan barang retur"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Transaksi pengambilan barang retur berhasil direkam!",
		"data":    req,
	})
}
