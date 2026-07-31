package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// LSPBV2DTO Struct khusus penampung Laporan LSBP V2
type LSPBV2DTO struct {
	DNODSJ            string `json:"dn_od_sj" gorm:"column:DN_OD_SJ"`
	CustName          string `json:"cust_name" gorm:"column:Cust_Name"`
	TanggalHandOver   string `json:"tanggal_handover" gorm:"column:Tanggal_HandOver"`
	NomorBTT          string `json:"nomorbtt" gorm:"column:nomorbtt"`
	PickupFrom        string `json:"pickup_from" gorm:"column:PickupFrom"`
	BTTTTujuanNama    string `json:"bttt_tujuan_nama" gorm:"column:BTTT_TujuanNama"`
	BTTTTujuanKota    string `json:"bttt_tujuan_kota" gorm:"column:BTTT_TujuanKota"`
	ServName          string `json:"serv_name" gorm:"column:Serv_Name"`
	TanggalBerangkat  string `json:"tanggal_berangkat" gorm:"column:Tanggal_berangkat"`
	Darat             int    `json:"darat" gorm:"column:Darat"`
	Laut              int    `json:"laut" gorm:"column:Laut"`
	Udara             int    `json:"udara" gorm:"column:Udara"`
	TanggalDiterima   string `json:"tanggal_diterima" gorm:"column:Tanggal_Diterima"`
	NamaPenerima      string `json:"nama_penerima" gorm:"column:Nama_Penerima"`
	Keterangan        string `json:"keterangan" gorm:"column:Keterangan"`
	SPBID             string `json:"spb_id" gorm:"column:spb_id"`
	ArtihNoKw         string `json:"artih_nokw" gorm:"column:artih_nokw"`
	NoMobil           string `json:"no_mobil" gorm:"column:NoMobil"`
	JamMobilMasuk     string `json:"jam_mobil_masuk" gorm:"column:JamMobilMasuk"`
	JamMobilKeluar    string `json:"jam_mobil_keluar" gorm:"column:JamMobilKeluar"`
	JamMobilTiba      string `json:"jam_mobil_tiba" gorm:"column:JamMobilTiba"`
	NoSKB             string `json:"no_skb" gorm:"column:NoSKB"`
	ArtihTglAcc       string `json:"artih_tgl_acc" gorm:"column:artih_tglacc"`
	BTTTJmlUnit       string `json:"bttt_jml_unit" gorm:"column:BTTT_JmlUnit"`
	BTTTBeratVol      string `json:"bttt_berat_vol" gorm:"column:BTTT_BeratVol"`
	BTTTUkuran        string `json:"bttt_ukuran" gorm:"column:BTTT_Ukuran"`
	BTTTBerat         string `json:"bttt_berat" gorm:"column:BTTT_Berat"`
	TglDokumenKembali string `json:"tgl_dokumen_kembali" gorm:"column:TB_TglKembali"`
}

// GET LIST LAPORAN LSBP V2
func GetLSPBV2ReportList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	tglAwal := strings.TrimSpace(c.Query("tgla"))
	tglAkhir := strings.TrimSpace(c.Query("tgle"))
	customer := strings.TrimSpace(c.Query("customer"))
	nobtt := strings.TrimSpace(c.Query("nobtt"))
	nosj := strings.TrimSpace(c.Query("nosj"))
	nosmu := strings.TrimSpace(c.Query("nosmu"))
	service := strings.TrimSpace(c.Query("service"))
	status := strings.TrimSpace(c.Query("status"))

	queryStr := `
		SELECT * 
		FROM public.vw_transport_lspb_with_car_in_out_and_datetime 
		WHERE "DN_OD_SJ" IS NOT NULL AND "DN_OD_SJ" <> ''
	`

	var conditions []string
	var args []interface{}

	if tglAwal != "" && tglAkhir != "" {
		conditions = append(conditions, `"Tanggal_HandOver" BETWEEN ? AND ?`)
		args = append(args, tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}

	if customer != "" {
		conditions = append(conditions, `UPPER("Cust_Name") LIKE UPPER(?)`)
		args = append(args, "%"+customer+"%")
	}

	if nobtt != "" {
		conditions = append(conditions, `"nomorbtt" LIKE ?`)
		args = append(args, "%"+nobtt+"%")
	}

	if nosj != "" {
		conditions = append(conditions, `"DN_OD_SJ" = ?`)
		args = append(args, nosj)
	}

	if nosmu != "" {
		conditions = append(conditions, `"bttt_smuno" LIKE ?`)
		args = append(args, "%"+nosmu+"%")
	}

	if service != "" {
		conditions = append(conditions, `"bttt_servid" = ?`)
		args = append(args, service)
	}

	if status == "DELIVERED" {
		conditions = append(conditions, `"Nama_Penerima" IS NOT NULL AND "Nama_Penerima" <> ''`)
	} else if status == "SHIPPED" {
		conditions = append(conditions, `("Nama_Penerima" IS NULL OR "Nama_Penerima" = '')`)
	}

	if len(conditions) > 0 {
		queryStr += " AND " + strings.Join(conditions, " AND ")
	}

	queryStr += ` ORDER BY "nomorbtt", "DN_OD_SJ", "Tanggal_HandOver" DESC LIMIT 500`

	var rawResults []LSPBV2DTO
	err := database.Raw(queryStr, args...).Scan(&rawResults).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal memuat Laporan LSBP V2: " + err.Error(),
		})
		return
	}

	// 💡 LOGIKA GROUPING KOLI CARGO SESUAI V2 LEGACY ASP (opr_t_lspb_v2.asp)
	// Jika No. BTT sama dengan baris sebelumnya, info Koli dikosongkan agar tidak double-count
	var previousNoBtt string = ""
	for i := range rawResults {
		if rawResults[i].NomorBTT != "" && rawResults[i].NomorBTT == previousNoBtt {
			rawResults[i].BTTTJmlUnit = ""
			rawResults[i].BTTTBeratVol = ""
			rawResults[i].BTTTUkuran = ""
			rawResults[i].BTTTBerat = ""
		} else {
			previousNoBtt = rawResults[i].NomorBTT
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   rawResults,
		"count":  len(rawResults),
	})
}
