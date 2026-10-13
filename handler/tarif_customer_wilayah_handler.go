package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 1. Ambil Daftar Propinsi
func GetWilayahPropinsi(c *gin.Context) {
	db := getTarifCustomerDB(c)
	var propinsi []string

	// Coba ambil dari glb_m_ekodepos, fallback ke data rute terdaftar jika tabel belum ada
	err := db.Table("public.glb_m_ekodepos").
		Select("DISTINCT propinsi").
		Where("propinsi IS NOT NULL AND propinsi <> ''").
		Order("propinsi ASC").
		Pluck("propinsi", &propinsi).Error

	if err != nil || len(propinsi) == 0 {
		db.Table("public.mkt_m_eharga_pelanggan").
			Select("DISTINCT tujuan_propinsi").
			Where("tujuan_propinsi IS NOT NULL AND tujuan_propinsi <> ''").
			Order("tujuan_propinsi ASC").
			Pluck("tujuan_propinsi", &propinsi)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": propinsi})
}

// 2. Ambil Daftar Kota Berdasarkan Propinsi
func GetWilayahKota(c *gin.Context) {
	db := getTarifCustomerDB(c)
	propinsi := strings.TrimSpace(c.Query("propinsi"))

	var kota []string
	err := db.Table("public.glb_m_ekodepos").
		Select("DISTINCT kotakabupaten").
		Where("propinsi = ? AND kotakabupaten IS NOT NULL AND kotakabupaten <> ''", propinsi).
		Order("kotakabupaten ASC").
		Pluck("kotakabupaten", &kota).Error

	if err != nil || len(kota) == 0 {
		db.Table("public.mkt_m_eharga_pelanggan").
			Select("DISTINCT tujuan_kabupaten").
			Where("tujuan_propinsi = ?", propinsi).
			Order("tujuan_kabupaten ASC").
			Pluck("tujuan_kabupaten", &kota)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": kota})
}

// 3. Ambil Daftar Kecamatan Berdasarkan Kota & Propinsi
func GetWilayahKecamatan(c *gin.Context) {
	db := getTarifCustomerDB(c)
	propinsi := strings.TrimSpace(c.Query("propinsi"))
	kota := strings.TrimSpace(c.Query("kota"))

	var kec []string
	err := db.Table("public.glb_m_ekodepos").
		Select("DISTINCT kecamatandistrik").
		Where("propinsi = ? AND kotakabupaten = ? AND kecamatandistrik IS NOT NULL AND kecamatandistrik <> ''", propinsi, kota).
		Order("kecamatandistrik ASC").
		Pluck("kecamatandistrik", &kec).Error

	if err != nil || len(kec) == 0 {
		db.Table("public.mkt_m_eharga_pelanggan").
			Select("DISTINCT tujuan_kecamatan").
			Where("tujuan_propinsi = ? AND tujuan_kabupaten = ?", propinsi, kota).
			Order("tujuan_kecamatan ASC").
			Pluck("tujuan_kecamatan", &kec)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": kec})
}

// 4. Simpan Tarif Baru (Per Kota atau Per Kecamatan)
type SimpanTarifPayload struct {
	CustomerID    string  `json:"customer_id" binding:"required"`
	ServID        string  `json:"serv_id" binding:"required"`     // 1, 2, 3
	JnHarga       string  `json:"jenis_harga" binding:"required"` // 0, 1, 2
	Mode          string  `json:"mode" binding:"required"`        // "kt" atau "kc"
	Propinsi      string  `json:"propinsi" binding:"required"`
	Kota          string  `json:"kota" binding:"required"`
	Kecamatan     string  `json:"kecamatan"`
	MinimalKg     float64 `json:"minimalkg"`
	HargaPokok    float64 `json:"hargapokok"`
	BypassKg      float64 `json:"bypasskg"`
	Selanjutnya   float64 `json:"hargakgselanjutnya"`
	BiayaTambahan float64 `json:"biayatambahan"`
	EstimasiHari  string  `json:"estimasihari"`
	Keterangan    string  `json:"keterangan"`
}

func SimpanTarifRute(c *gin.Context) {
	db := getTarifCustomerDB(c)
	var req SimpanTarifPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	tx := db.Begin()

	if req.Mode == "kc" {
		// Hapus eksisting untuk 1 kecamatan spesifik
		tx.Exec(`DELETE FROM public.mkt_m_eharga_pelanggan 
		         WHERE customerid = ? AND servid = ? AND jnharga = ? 
		           AND tujuan_propinsi = ? AND tujuan_kabupaten = ? AND tujuan_kecamatan = ?`,
			req.CustomerID, req.ServID, req.JnHarga, req.Propinsi, req.Kota, req.Kecamatan)

		// Insert rute kecamatan
		err := tx.Exec(`INSERT INTO public.mkt_m_eharga_pelanggan 
		        (customerid, agenid_asal, servid, jnharga, tujuan_propinsi, tujuan_kabupaten, tujuan_kecamatan,
		         minimalkg, hargapokok, bypasskg, hargakgselanjutnya, biayatambahan, estimasihari, keterangan)
		        VALUES (?, '1', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			req.CustomerID, req.ServID, req.JnHarga, req.Propinsi, req.Kota, req.Kecamatan,
			req.MinimalKg, req.HargaPokok, req.BypassKg, req.Selanjutnya, req.BiayaTambahan, req.EstimasiHari, req.Keterangan).Error

		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
	} else {
		// PER KOTA: Ambil semua kecamatan dalam kota terpilih
		var daftarKec []string
		tx.Table("public.glb_m_ekodepos").
			Select("DISTINCT kecamatandistrik").
			Where("propinsi = ? AND kotakabupaten = ? AND kecamatandistrik <> ''", req.Propinsi, req.Kota).
			Pluck("kecamatandistrik", &daftarKec)

		if len(daftarKec) == 0 {
			tx.Table("public.mkt_m_eharga_pelanggan").
				Select("DISTINCT tujuan_kecamatan").
				Where("tujuan_propinsi = ? AND tujuan_kabupaten = ?", req.Propinsi, req.Kota).
				Pluck("tujuan_kecamatan", &daftarKec)
		}

		// Hapus rute lama se-kota
		tx.Exec(`DELETE FROM public.mkt_m_eharga_pelanggan 
		         WHERE customerid = ? AND servid = ? AND jnharga = ? 
		           AND tujuan_propinsi = ? AND tujuan_kabupaten = ?`,
			req.CustomerID, req.ServID, req.JnHarga, req.Propinsi, req.Kota)

		// Simpan tarif seragam untuk seluruh kecamatan di kota tersebut
		for _, kecName := range daftarKec {
			tx.Exec(`INSERT INTO public.mkt_m_eharga_pelanggan 
			        (customerid, agenid_asal, servid, jnharga, tujuan_propinsi, tujuan_kabupaten, tujuan_kecamatan,
			         minimalkg, hargapokok, bypasskg, hargakgselanjutnya, biayatambahan, estimasihari, keterangan)
			        VALUES (?, '1', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				req.CustomerID, req.ServID, req.JnHarga, req.Propinsi, req.Kota, kecName,
				req.MinimalKg, req.HargaPokok, req.BypassKg, req.Selanjutnya, req.BiayaTambahan, req.EstimasiHari, req.Keterangan)
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Tarif berhasil disimpan"})
}

// 6. Penyesuaian Tarif (%) Per Kota atau Per Kecamatan
type PenyesuaianWilayahPayload struct {
	CustomerID string  `json:"customer_id" binding:"required"`
	ServID     string  `json:"serv_id" binding:"required"`     // 1, 2, 3
	JnHarga    string  `json:"jenis_harga" binding:"required"` // 0, 1, 2
	Mode       string  `json:"mode" binding:"required"`        // "kt" (Kota) atau "kc" (Kecamatan)
	Propinsi   string  `json:"propinsi" binding:"required"`
	Kota       string  `json:"kota" binding:"required"`
	Kecamatan  string  `json:"kecamatan"`                     // wajib jika mode = "kc"
	TipeAksi   string  `json:"tipe_aksi" binding:"required"`  // "+" (Naik) atau "-" (Turun)
	Persentase float64 `json:"persentase" binding:"required"` // contoh: 5, 10
}

func PenyesuaianTarifWilayah(c *gin.Context) {
	db := getTarifCustomerDB(c)
	var req PenyesuaianWilayahPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter tidak lengkap"})
		return
	}

	if req.TipeAksi != "+" && req.TipeAksi != "-" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Tipe aksi harus '+' atau '-'"})
		return
	}

	faktor := req.Persentase / 100.0

	// Rumus SQL persis dari sistem lawas Dakota:
	// hargakgselanjutnya = ROUND(hargakgselanjutnya +/- (faktor * hargakgselanjutnya), -2)
	// hargapokok = minimalkg * ROUND(hargakgselanjutnya +/- (faktor * hargakgselanjutnya), -2)
	query := fmt.Sprintf(`
		UPDATE public.mkt_m_eharga_pelanggan
		SET 
			hargakgselanjutnya = ROUND(hargakgselanjutnya %s (? * hargakgselanjutnya), -2),
			hargapokok = minimalkg * ROUND(hargakgselanjutnya %s (? * hargakgselanjutnya), -2)
		WHERE customerid = ? AND servid = ? AND jnharga = ?
		  AND tujuan_propinsi = ? AND tujuan_kabupaten = ?
	`, req.TipeAksi, req.TipeAksi)

	args := []interface{}{faktor, faktor, req.CustomerID, req.ServID, req.JnHarga, req.Propinsi, req.Kota}

	if req.Mode == "kc" {
		query += " AND tujuan_kecamatan = ?"
		args = append(args, req.Kecamatan)
	}

	res := db.Exec(query, args...)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": res.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Berhasil menyesuaikan tarif pada %d rute", res.RowsAffected),
	})
}

// 1. Ambil Opsi Customer Sumber (Hanya yang punya tarif di serv_id & jenis_harga yang sama, dan satu cabang)
func GetSourceCustomerTarifOptions(c *gin.Context) {
	db := getTarifCustomerDB(c)

	targetCustID := strings.TrimSpace(c.Query("target_cust_id"))
	servID := strings.TrimSpace(c.DefaultQuery("serv_id", "1"))
	jnHarga := strings.TrimSpace(c.DefaultQuery("jenis_harga", "0"))

	// Ambil agenID_asal dari customer target
	var targetAgenID string
	err := db.Table("public.mkt_m_customer").
		Select("cust_agenid").
		Where("cust_id = ?", targetCustID).
		Pluck("cust_agenid", &targetAgenID).Error

	if err != nil || targetAgenID == "" {
		targetAgenID = "1" // Fallback jika default pusat
	}

	type SourceCustOption struct {
		CustomerID string `json:"customer_id"`
		CustName   string `json:"cust_name"`
		TotalRute  int64  `json:"total_rute"`
	}

	var results []SourceCustOption
	query := db.Table("public.mkt_m_eharga_pelanggan AS t").
		Select("t.customerid AS customer_id, COALESCE(c.cust_name, t.customerid) AS cust_name, COUNT(t.id) AS total_rute").
		Joins("LEFT JOIN public.mkt_m_customer AS c ON TRIM(t.customerid) = TRIM(c.cust_id)").
		Where("TRIM(t.customerid) <> ? AND TRIM(t.servid::text) = ? AND TRIM(t.jnharga::text) = ?", targetCustID, servID, jnHarga)

	// Filter cabang jika bukan holding/pusat
	if targetAgenID != "ALL" && targetAgenID != "" {
		query = query.Where("TRIM(t.agenid_asal::text) = ?", targetAgenID)
	}

	err = query.Group("t.customerid, c.cust_name").
		Order("c.cust_name ASC").
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 2. Eksekusi Salin Rute (Sesuai Logika p-mkt_m_eharga_pelanggan_e_c.asp)
func SalinTarifCustomerLain(c *gin.Context) {
	db := getTarifCustomerDB(c)

	type SalinReq struct {
		TargetCustID string `json:"target_cust_id" binding:"required"`
		SourceCustID string `json:"source_cust_id" binding:"required"`
		ServID       string `json:"serv_id" binding:"required"`
		JnHarga      string `json:"jenis_harga" binding:"required"`
	}

	var req SalinReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter tidak lengkap"})
		return
	}

	// Query INSERT ... SELECT ... WHERE NOT EXISTS
	query := `
		INSERT INTO public.mkt_m_eharga_pelanggan 
		(customerid, agenid_asal, servid, jnharga, tujuan_propinsi, tujuan_kabupaten, tujuan_kecamatan,
		 minimalkg, hargapokok, bypasskg, hargakgselanjutnya, biayatambahan, estimasihari, keterangan)
		SELECT 
			?, src.agenid_asal, src.servid, src.jnharga, src.tujuan_propinsi, src.tujuan_kabupaten, src.tujuan_kecamatan,
			src.minimalkg, src.hargapokok, src.bypasskg, src.hargakgselanjutnya, src.biayatambahan, src.estimasihari, COALESCE(src.keterangan, '')
		FROM public.mkt_m_eharga_pelanggan AS src
		WHERE src.customerid = ? AND TRIM(src.servid::text) = ? AND TRIM(src.jnharga::text) = ?
		  AND NOT EXISTS (
			  SELECT 1 FROM public.mkt_m_eharga_pelanggan AS dest
			  WHERE dest.customerid = ? 
			    AND TRIM(dest.servid::text) = TRIM(src.servid::text)
			    AND TRIM(dest.jnharga::text) = TRIM(src.jnharga::text)
			    AND TRIM(dest.tujuan_propinsi) = TRIM(src.tujuan_propinsi)
			    AND TRIM(dest.tujuan_kabupaten) = TRIM(src.tujuan_kabupaten)
			    AND TRIM(dest.tujuan_kecamatan) = TRIM(src.tujuan_kecamatan)
		  )
	`

	res := db.Exec(query, req.TargetCustID, req.SourceCustID, req.ServID, req.JnHarga, req.TargetCustID)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": res.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"inserted_rows": res.RowsAffected,
		"message":       fmt.Sprintf("Berhasil menyalin %d rute baru dari customer sumber", res.RowsAffected),
	})
}
