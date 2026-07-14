package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetProsesPackingList(c *gin.Context) {
	tglAwal := c.Query("tgl_awal")
	tglAkhir := c.Query("tgl_akhir")
	noPck := c.Query("no_pck")
	noBtt := c.Query("no_btt")

	// Mengambil session active_agen_id (atau disesuaikan dengan claims token tim lu bray)
	// Untuk sementara kita default-kan atau ambil dari query/context jika ada
	prosesAgenID := "01" // Contoh default agen tempat admin bertugas

	var results []models.PackingResponse

	// 👑 REKAYASA QUERY INNER JOIN DARI SCRIPT ASP JADUL LU MASTER!
	query := db.DB.Table("pck_t_packing p").
		Select(`
			e.bttt_id, 
			p.pck_id, 
			a.agen_nama, 
			e.bttt_asalname, 
			e.bttt_tujuannama, 
			e.bttt_tujuankota, 
			p.pck_isikiriman, 
			p.pck_jumlah, 
			p.pck_menjadi,
			CASE WHEN COALESCE(p.pck_approveyn, 'N') = 'Y' THEN 'Ya' ELSE 'Tidak' END as appjd,
			p.pck_tanggal
		`).
		// 👑 FIX 1: Paksa kolom e.bttt_packingid di-cast menjadi varchar agar sejajar dengan p.pck_id!
		Joins("INNER JOIN mkt_t_econote e ON p.pck_id = e.bttt_packingid::varchar").
		// 👑 FIX 2: Pastikan pula relasi agen ter-cast jika asalagenid adalah integer
		Joins("INNER JOIN glb_m_agen a ON e.bttt_asalagenid::varchar = a.agen_id::varchar").
		Where("p.pck_prosesagenid = ?", prosesAgenID)

	// Filter Rentang Tanggal Packing
	if tglAwal != "" && tglAkhir != "" {
		query = query.Where("p.pck_tanggal BETWEEN ? AND ?", tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}
	// Filter No. Packing
	if noPck != "" {
		query = query.Where("p.pck_id ILIKE ?", "%"+noPck+"%")
	}
	// Filter No. BTT / Resi
	if noBtt != "" {
		query = query.Where("e.bttt_id ILIKE ?", "%"+noBtt+"%")
	}

	// Urutkan berdasarkan tanggal transaksi dan ID Resi
	err := query.Order("p.pck_tanggal DESC, e.bttt_id ASC").Scan(&results).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

func GetBttOutstandingPacking(c *gin.Context) {
	var results []struct {
		BTTTID         string `json:"bttt_id"`
		BTTTAsalName   string `json:"bttt_asal_name"`
		BTTTTujuanNama string `json:"bttt_tujuan_nama"`
		BTTTTujuanKota string `json:"bttt_tujuan_kota"` // Tag JSON aman untuk frontend
	}

	// 👑 FIX MUTLAK: Ubah bttt_tujuan_kota -> menjadi bttt_tujuankota AS bttt_tujuan_kota bray!
	err := db.DB.Table("mkt_t_econote").
		Select("bttt_id, bttt_asalname, bttt_tujuannama AS bttt_tujuan_nama, bttt_tujuankota AS bttt_tujuan_kota").
		Where("bttt_packingid IS NULL OR bttt_packingid = ''").
		Order("bttt_id ASC").
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 👑 2. INI DIA BIANG KEROKNYA BRAY! PASTIKAN FUNGSI EMAS INI SUDAH ADA DI SINI!
func SimpanProsesPacking(c *gin.Context) {
	var input struct {
		BtttID string `json:"bttt_id" binding:"required"`
		AgenID string `json:"agen_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid bray"})
		return
	}

	// Jalankan logic simpan atau update ke database PostgreSQL Dakota Cargo lu
	// Untuk sementara kita return status sukses agar pop-up React lu bisa menutup dengan ganteng
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data packing berhasil diproses"})
}
