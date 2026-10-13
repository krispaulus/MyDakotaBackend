package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Gunakan func(c *gin.Context) langsung:
func AdjustTarifCustomerHandler(c *gin.Context) {
	database := getTarifCustomerDB(c)

	type AdjustTarifRequest struct {
		CustomerID string  `json:"customer_id"`
		TipeAksi   string  `json:"tipe_aksi"`  // "+" atau "-"
		Persentase float64 `json:"persentase"` // contoh: 5
	}

	var req AdjustTarifRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Data tidak valid"})
		return
	}

	faktor := req.Persentase / 100.0
	operator := "+"
	if req.TipeAksi == "-" {
		operator = "-"
	}

	queryCargo := fmt.Sprintf(`
		UPDATE public.mkt_m_eharga_pelanggan
		SET 
			hargakgselanjutnya = ROUND(hargakgselanjutnya %s (hargakgselanjutnya * ?), -2),
			hargapokok = minimalkg * ROUND(hargakgselanjutnya %s (hargakgselanjutnya * ?), -2)
		WHERE customerid = ?
	`, operator, operator)

	if err := database.Exec(queryCargo, faktor, faktor, req.CustomerID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	queryCarter := fmt.Sprintf(`
		UPDATE public.mkt_m_eharga_pelanggan_carter
		SET harga = ROUND(harga %s (harga * ?), -2)
		WHERE customerid = ?
	`, operator)
	database.Exec(queryCarter, faktor, req.CustomerID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Berhasil menyesuaikan tarif customer sebesar %s%.1f%%!", operator, req.Persentase),
	})
}
