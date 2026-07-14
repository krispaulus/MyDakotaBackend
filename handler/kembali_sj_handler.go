package handler

import (
	"net/http"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 👑 FIX 1: Hapus total struct KembaliSJHandler & NewKembaliSJHandler karena rute main.go lu
// memanggil handler.GetKembaliSJList secara polos/langsung!

func GetKembaliSJList(c *gin.Context) {
	tglAwal := c.Query("tgl_awal")       //
	tglAkhir := c.Query("tgl_akhir")     //
	custName := c.Query("customer_name") //
	docID := c.Query("document_id")      //
	noSJ := c.Query("no_sj")             //

	// 👑 FIX 2: Ubah dari models.ReturnSJResponse menjadi models.KembaliSJResponse
	// agar sesuai 100% dengan isi file model kembali_sj.go lu bray!
	var results []models.KembaliSJResponse

	// 👑 Menggunakan db.DB (Variabel GORM global bawaan Dakota backend lu)
	query := db.DB.Table("mkt_t_kembalisj_h h").
		Select("h.mkt_t_kembalisj_id, h.mkt_t_kembalisj_tanggal, h.mkt_t_kembalisj_custid, c.cust_name, h.mkt_t_kembalisj_keterangan, h.mkt_t_kembalisj_diterima, h.mkt_t_kembalisj_aktifyn").
		Joins("LEFT JOIN mkt_m_customer c ON h.mkt_t_kembalisj_custid = c.cust_id"). //
		Where("h.mkt_t_kembalisj_aktifyn = ?", "Y")                                  //

	if tglAwal != "" && tglAkhir != "" { //
		query = query.Where("h.mkt_t_kembalisj_tanggal BETWEEN ? AND ?", tglAwal+" 00:00:00", tglAkhir+" 23:59:59") //
	} //
	if custName != "" { //
		query = query.Where("c.cust_name ILIKE ?", "%"+custName+"%") //
	} //
	if docID != "" { //
		query = query.Where("h.mkt_t_kembalisj_id ILIKE ?", "%"+docID+"%") //
	} //
	if noSJ != "" { //
		query = query.Where("h.mkt_t_kembalisj_id IN (SELECT mkt_t_kembalisj_id_h FROM mkt_t_kembalisj_d WHERE mkt_t_kembalisj_nosj ILIKE ?)", "%"+noSJ+"%") //
	} //

	if err := query.Order("h.mkt_t_kembalisj_tanggal DESC").Scan(&results).Error; err != nil { //
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()}) //
		return                                                                                   //
	} //

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results}) //
}
