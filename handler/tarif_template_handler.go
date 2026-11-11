package handler

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// DownloadTarifTemplateCSV mengunduh format template CSV sesuai jenis tarif yang dipilih
func DownloadTarifTemplateCSV(c *gin.Context) {
	jenis := c.Query("jenis") // 'cargo' | 'charter' | 'multidrop'
	customerID := c.Query("customer_id")

	buf := new(bytes.Buffer)
	writer := csv.NewWriter(buf)

	var filename string
	var header []string
	var sampleRow []string

	switch jenis {
	case "charter":
		filename = fmt.Sprintf("template_charter_%s.csv", customerID)
		header = []string{"customer_id", "jns_kend", "propinsi", "kota", "harga", "harga_dg", "kg_max", "harga_over_kg_max", "estimasihari"}
		sampleRow = []string{customerID, "K0001", "JAWA BARAT", "BEKASI", "1500000", "1800000", "5000", "2000", "1"}

	case "multidrop":
		filename = fmt.Sprintf("template_multidrop_%s.csv", customerID)
		header = []string{"customer_id", "propinsi", "kabupaten", "kecamatan", "harga_kg", "kg_min", "kg_max", "harga_over_kg_max", "harga_kg_dg"}
		sampleRow = []string{customerID, "JAWA BARAT", "BEKASI", "CIBITUNG", "2500", "10", "100", "2000", "3000"}

	default: // cargo / reguler
		filename = fmt.Sprintf("template_cargo_%s.csv", customerID)
		header = []string{"customer_id", "propinsi", "kota", "kecamatan", "berat_min", "harga_kg", "harga_kg_lebih", "harga_volume", "harga_kubikasi", "harga_dg"}
		sampleRow = []string{customerID, "JAWA TIMUR", "SURABAYA", "GUBENG", "10", "3000", "2500", "4000", "500000", "4500"}
	}

	_ = writer.Write(header)
	_ = writer.Write(sampleRow)
	writer.Flush()

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv")
	c.Data(http.StatusOK, "text/csv", buf.Bytes())
}
