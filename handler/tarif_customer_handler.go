package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CargoTarifItem struct {
	ID                 uint    `json:"id" gorm:"column:id"`
	TujuanPropinsi     string  `json:"tujuan_propinsi" gorm:"column:tujuan_propinsi"`
	TujuanKabupaten    string  `json:"tujuan_kabupaten" gorm:"column:tujuan_kabupaten"`
	TujuanKecamatan    string  `json:"tujuan_kecamatan" gorm:"column:tujuan_kecamatan"`
	MinimalKg          float64 `json:"minimalkg" gorm:"column:minimalkg"`
	HargaPokok         float64 `json:"hargapokok" gorm:"column:hargapokok"`
	BypassKg           float64 `json:"bypasskg" gorm:"column:bypasskg"`
	HargaKgSelanjutnya float64 `json:"hargakgselanjutnya" gorm:"column:hargakgselanjutnya"`
	BiayaTambahan      float64 `json:"biayatambahan" gorm:"column:biayatambahan"`
	Keterangan         string  `json:"keterangan" gorm:"column:keterangan"`
	EstimasiHari       string  `json:"estimasihari" gorm:"column:estimasihari"`
}

type MktMEHargaPelanggan struct {
	ID                 uint    `gorm:"column:id;primaryKey" json:"id"`
	AgenIDAsal         string  `gorm:"column:agenid_asal" json:"agenid_asal"`
	CustomerID         string  `gorm:"column:customerid" json:"customerid"`
	ServID             string  `gorm:"column:servid" json:"servid"`
	TujuanPropinsi     string  `gorm:"column:tujuan_propinsi" json:"tujuan_propinsi"`
	TujuanKabupaten    string  `gorm:"column:tujuan_kabupaten" json:"tujuan_kabupaten"`
	TujuanKecamatan    string  `gorm:"column:tujuan_kecamatan" json:"tujuan_kecamatan"`
	MinimalKg          float64 `gorm:"column:minimalkg" json:"minimalkg"`
	BypassKg           float64 `gorm:"column:bypasskg" json:"bypasskg"`
	HargaPokok         float64 `gorm:"column:hargapokok" json:"hargapokok"`
	HargaKgSelanjutnya float64 `gorm:"column:hargakgselanjutnya" json:"hargakgselanjutnya"`
	EstimasiHari       string  `gorm:"column:estimasihari" json:"estimasihari"`
	Keterangan         string  `gorm:"column:keterangan" json:"keterangan"`
	JnHarga            string  `gorm:"column:jnharga" json:"jnharga"`
}

func (MktMEHargaPelanggan) TableName() string {
	return "public.mkt_m_eharga_pelanggan"
}

// Helper internal pengambil koneksi DB dinamis
func getTarifCustomerDB(c *gin.Context) *gorm.DB {
	ptID, exists := c.Get("pt_id")
	if !exists || fmt.Sprintf("%v", ptID) == "" || fmt.Sprintf("%v", ptID) == "<nil>" {
		ptID = "A"
	}

	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		if db.DB != nil {
			return db.DB
		}
		return db.DB
	}
	return database
}

// 1. GET LIST CUSTOMER + JUMLAH TARIF DITERAPKAN (SESUAI CLASSIC ASP MKT_M_EHARGA_PELANGGAN.ASP)
func GetTarifCustomerList(c *gin.Context) {
	database := getTarifCustomerDB(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset := (page - 1) * limit

	customerName := strings.TrimSpace(c.Query("customer_name"))
	cabangID := strings.TrimSpace(c.Query("cabang_id")) // ID Agen dari dropdown cabang

	// Parameter Filter Tarif
	ckTarif := c.Query("ck_tarif") == "true"
	hasReguler := c.Query("has_reguler") == "true"
	hasCharter := c.Query("has_charter") == "true"
	hasMultidrop := c.Query("has_multidrop") == "true"

	query := database.Table("public.mkt_m_customer c").
		Select(`
			c.cust_id, 
			c.cust_name, 
			COALESCE(c.cust_alamat1, '') AS cust_alamat1, 
			COALESCE(c.cust_alamat2, '') AS cust_alamat2, 
			COALESCE(c.cust_telp1, '') AS cust_telp1, 
			COALESCE(c.cust_telp2, '') AS cust_telp2,
			COALESCE(c.cust_agenid, '') AS cust_agenid,
			COALESCE(a.agen_nama, '') AS agen_nama,
			(SELECT COUNT(*) FROM public.mkt_m_eharga_pelanggan WHERE customerid = c.cust_id) AS jml_reguler,
			(SELECT COUNT(*) FROM public.mkt_m_eharga_pelanggan_carter WHERE customerid = c.cust_id) AS jml_charter,
			(SELECT COUNT(*) FROM public.mkt_m_eharga_pelanggan_multidrop WHERE customerid = c.cust_id) AS jml_multidrop
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON TRIM(c.cust_agenid) = TRIM(a.agen_id)").
		Where("c.cust_aktifyn = ?", "Y")

	// Filter Nama / Kode Customer
	if customerName != "" {
		query = query.Where("(UPPER(c.cust_name) LIKE UPPER(?) OR c.cust_id LIKE ?)", "%"+customerName+"%", "%"+customerName+"%")
	}

	// Filter Cabang (jika dipilih)
	if cabangID != "" && cabangID != "ALL" {
		query = query.Where("TRIM(c.cust_agenid) = ?", cabangID)
	}

	// Filter Kombinasi Status Tarif (Sesuai Aturan Klasik ASP)
	if ckTarif {
		if hasReguler {
			query = query.Where("EXISTS (SELECT 1 FROM public.mkt_m_eharga_pelanggan WHERE customerid = c.cust_id)")
		} else {
			query = query.Where("NOT EXISTS (SELECT 1 FROM public.mkt_m_eharga_pelanggan WHERE customerid = c.cust_id)")
		}

		if hasCharter {
			query = query.Where("EXISTS (SELECT 1 FROM public.mkt_m_eharga_pelanggan_carter WHERE customerid = c.cust_id)")
		} else {
			query = query.Where("NOT EXISTS (SELECT 1 FROM public.mkt_m_eharga_pelanggan_carter WHERE customerid = c.cust_id)")
		}

		if hasMultidrop {
			query = query.Where("EXISTS (SELECT 1 FROM public.mkt_m_eharga_pelanggan_multidrop WHERE customerid = c.cust_id)")
		} else {
			query = query.Where("NOT EXISTS (SELECT 1 FROM public.mkt_m_eharga_pelanggan_multidrop WHERE customerid = c.cust_id)")
		}
	}

	type CustomerTarifSummary struct {
		CustID       string `json:"cust_id"`
		CustName     string `json:"cust_name"`
		CustAlamat1  string `json:"cust_alamat1"`
		CustAlamat2  string `json:"cust_alamat2"`
		CustTelp1    string `json:"cust_telp1"`
		CustTelp2    string `json:"cust_telp2"`
		CustAgenID   string `json:"cust_agenid"`
		AgenNama     string `json:"agen_nama"`
		JmlReguler   int64  `json:"jml_reguler"`
		JmlCharter   int64  `json:"jml_charter"`
		JmlMultidrop int64  `json:"jml_multidrop"`
	}

	var results []CustomerTarifSummary
	err := query.Order("c.cust_name ASC").
		Offset(offset).
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
		"page":   page,
		"limit":  limit,
	})
}

// 2. GET DETAIL RUTE TARIF MILIK 1 CUSTOMER SPESIFIK
func GetDetailTarifByCustomer(c *gin.Context) {
	database := getTarifCustomerDB(c)
	customerID := strings.TrimSpace(c.Param("customer_id"))

	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Customer ID wajib diisi"})
		return
	}

	results := make([]models.MktMEHargaPelanggan, 0)
	err := database.Where("customerid = ?", customerID).Order("tujuan_propinsi ASC, tujuan_kabupaten ASC, tujuan_kecamatan ASC").Find(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengambil detail tarif customer"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

// 3. TAMBAH / UPDATE TARIF KHUSUS CUSTOMER (SAVE & UPSERT)
func SaveTarifCustomer(c *gin.Context) {
	database := getTarifCustomerDB(c)

	var input models.MktMEHargaPelanggan
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input data tidak valid: " + err.Error()})
		return
	}

	input.CustomerID = strings.ToUpper(strings.TrimSpace(input.CustomerID))
	input.KotaAsal = strings.ToUpper(strings.TrimSpace(input.KotaAsal))
	input.KotaTujuan = strings.ToUpper(strings.TrimSpace(input.KotaTujuan))
	input.Kabupaten = strings.ToUpper(strings.TrimSpace(input.Kabupaten))

	// Jika ID ada, lakukan update
	if input.ID > 0 {
		err := database.Model(&models.MktMEHargaPelanggan{}).Where("id = ?", input.ID).Updates(input).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate tarif customer"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Tarif customer berhasil diperbarui"})
		return
	}

	// Jika ID tidak ada, lakukan INSERT baru
	if err := database.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan tarif customer baru: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Tarif khusus customer berhasil ditambahkan",
		"data":    input,
	})
}

// 4. DELETE TARIF KHUSUS CUSTOMER (SATUAN ATAU ALL PER CUSTOMER)
func DeleteTarifCustomer(c *gin.Context) {
	database := getTarifCustomerDB(c)

	type DeleteTarifInput struct {
		ID         uint   `json:"id"`
		CustomerID string `json:"customer_id"`
	}

	var input DeleteTarifInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload input ID tidak valid"})
		return
	}

	if input.ID > 0 {
		if err := database.Delete(&models.MktMEHargaPelanggan{}, input.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus rute tarif"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Rute tarif customer berhasil dihapus"})
		return
	}

	if input.CustomerID != "" {
		if err := database.Where("customerid = ?", input.CustomerID).Delete(&models.MktMEHargaPelanggan{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus seluruh tarif customer"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Seluruh tarif khusus customer berhasil dihapus"})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID atau Customer ID wajib diisi"})
}

func GetCabangTarifList(c *gin.Context) {
	database := getTarifCustomerDB(c)

	type CabangOption struct {
		AgenID   string `json:"agen_id"`
		AgenNama string `json:"agen_nama"`
	}

	var cabangs []CabangOption
	err := database.Table("public.glb_m_agen").
		Select("agen_id, agen_nama").
		Where("agen_aktifyn = ?", "Y").
		Order("agen_nama ASC").
		Scan(&cabangs).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": cabangs})
}

func GetCargoTarifDetail(c *gin.Context) {
	database := getTarifCustomerDB(c)

	customerID := strings.TrimSpace(c.Query("customer_id"))
	servID := strings.TrimSpace(c.DefaultQuery("serv_id", "1"))
	jnHarga := strings.TrimSpace(c.DefaultQuery("jenis_harga", "0"))

	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Customer ID wajib diisi"})
		return
	}

	var items []CargoTarifItem

	query := database.Table("public.mkt_m_eharga_pelanggan").
		Select(`id, 
		        COALESCE(tujuan_propinsi, '') AS tujuan_propinsi, 
		        COALESCE(tujuan_kabupaten, '') AS tujuan_kabupaten, 
		        COALESCE(tujuan_kecamatan, '') AS tujuan_kecamatan, 
		        COALESCE(minimalkg, 0) AS minimalkg, 
		        COALESCE(hargapokok, 0) AS hargapokok, 
		        COALESCE(bypasskg, 0) AS bypasskg, 
		        COALESCE(hargakgselanjutnya, 0) AS hargakgselanjutnya, 
		        COALESCE(biayatambahan, 0) AS biayatambahan, 
		        COALESCE(keterangan, '') AS keterangan, 
		        COALESCE(estimasihari, '0') AS estimasihari`).
		Where("TRIM(customerid) = ? AND TRIM(servid::text) = ?", customerID, servID)

	if jnHarga == "0" {
		query = query.Where("(TRIM(jnharga::text) = '0' OR UPPER(TRIM(jnharga::text)) = 'BERAT')")
	} else if jnHarga == "1" {
		query = query.Where("(TRIM(jnharga::text) = '1' OR UPPER(TRIM(jnharga::text)) = 'VOLUME')")
	} else if jnHarga == "2" {
		query = query.Where("(TRIM(jnharga::text) = '2' OR UPPER(TRIM(jnharga::text)) = 'KUBIKASI')")
	} else {
		query = query.Where("TRIM(jnharga::text) = ?", jnHarga)
	}

	err := query.Order("tujuan_propinsi ASC, tujuan_kabupaten ASC, tujuan_kecamatan ASC").
		Scan(&items).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"total":  len(items),
		"data":   items,
	})
}

// 2. INLINE AUTOSAVE PER FIELD (MENGGANTIKAN AJAX ASP LAMA)
func UpdateCargoTarifField(c *gin.Context) {
	database := getTarifCustomerDB(c)

	type UpdateFieldPayload struct {
		ID    uint        `json:"id"`
		Field string      `json:"field"`
		Value interface{} `json:"value"`
	}

	var req UpdateFieldPayload
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Payload tidak valid"})
		return
	}

	allowedFields := map[string]bool{
		"minimalkg":          true,
		"hargapokok":         true,
		"bypasskg":           true,
		"hargakgselanjutnya": true,
		"biayatambahan":      true,
		"keterangan":         true,
		"estimasihari":       true,
	}

	if !allowedFields[req.Field] {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Field tidak diizinkan"})
		return
	}

	query := fmt.Sprintf("UPDATE public.mkt_m_eharga_pelanggan SET %s = ? WHERE id = ?", req.Field)
	if err := database.Exec(query, req.Value, req.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Tersimpan otomatis"})
}

// 3. DELETE MASSAL UNTUK BARIS YANG DICENTANG HAPUS
func DeleteBulkCargoTarif(c *gin.Context) {
	database := getTarifCustomerDB(c)

	type BulkDeletePayload struct {
		IDs []uint `json:"ids"`
	}

	var req BulkDeletePayload
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Pilih minimal 1 rute yang akan dihapus"})
		return
	}

	if err := database.Table("public.mkt_m_eharga_pelanggan").Where("id IN ?", req.IDs).Delete(nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Berhasil menghapus %d rute tarif", len(req.IDs)),
	})
}
