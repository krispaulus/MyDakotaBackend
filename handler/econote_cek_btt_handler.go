package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// Response struct ringkas untuk validasi BTT
type CekBTTResponse struct {
	ConoteNomor       string  `json:"conote_nomor"`
	ConoteNobttManual string  `json:"conote_nobttmanual"`
	ConoteTgl         string  `json:"conote_tgl"`
	ConoteAsal        string  `json:"conote_asal"`
	ConoteTujuan      string  `json:"conote_tujuan"`
	PengirimNama      string  `json:"pengirim_nama"`
	PenerimaNama      string  `json:"penerima_nama"`
	ConoteKoli        int     `json:"conote_koli"`
	ConoteBerat       float64 `json:"conote_berat"`
	ConoteTotalTarif  float64 `json:"conote_total_tarif"`
	ConoteLayanan     string  `json:"conote_layanan"`
	ConoteStatus      string  `json:"conote_status"`
}

// GET /api/mkt/econote/cek-btt?no=...
func CekBTTManualAtauBarcode(c *gin.Context) {
	noInput := strings.TrimSpace(c.Query("no"))
	if noInput == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor BTT wajib diisi"})
		return
	}

	// Normalisasi auto padding 12 digit jika input murni angka dan kurang dari 12 digit
	paddedNo := noInput
	if len(noInput) < 12 && !strings.ContainsAny(noInput, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		paddedNo = fmt.Sprintf("%012s", noInput)
	}

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak ditemukan"})
		return
	}

	var result CekBTTResponse

	// Query pencarian fleksibel: mencocokkan ke kolom nomor manual, nomor conote barcode, atau versi padded-nya
	queryRaw := `
		SELECT 
			COALESCE(c.conote_nomor::text, '') AS conote_nomor,
			COALESCE(c.conote_nobttmanual::text, '') AS conote_nobttmanual,
			COALESCE(TO_CHAR(c.conote_tgl, 'YYYY-MM-DD HH24:MI'), '') AS conote_tgl,
			COALESCE(c.conote_asal::text, '') AS conote_asal,
			COALESCE(c.conote_tujuan::text, '') AS conote_tujuan,
			COALESCE(c.conote_pengirim_nama::text, c.conote_pengirim::text, '') AS pengirim_nama,
			COALESCE(c.conote_penerima_nama::text, c.conote_penerima::text, '') AS penerima_nama,
			COALESCE(c.conote_koli, 1) AS conote_koli,
			COALESCE(c.conote_berat, 0.0) AS conote_berat,
			COALESCE(c.conote_totalbiaya, c.conote_totaltarif, 0.0) AS conote_total_tarif,
			COALESCE(c.conote_layanan::text, 'REGULER') AS conote_layanan,
			COALESCE(c.conote_status::text, 'TERDATA') AS conote_status
		FROM public.mkt_t_econote c
		WHERE 
			c.conote_nobttmanual::text = ? 
			OR c.conote_nobttmanual::text = ? 
			OR c.conote_nomor::text = ? 
			OR c.conote_nomor::text = ?
		LIMIT 1
	`

	if err := database.Raw(queryRaw, noInput, paddedNo, noInput, paddedNo).Scan(&result).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal validasi BTT: " + err.Error()})
		return
	}

	if result.ConoteNomor == "" && result.ConoteNobttManual == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "not_found",
			"message": fmt.Sprintf("Nomor BTT [%s] belum terdaftar atau belum diinput oleh cabang pengirim.", paddedNo),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data fisik BTT ditemukan dalam sistem e-Conote",
		"data":    result,
	})
}
