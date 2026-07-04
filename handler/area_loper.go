package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetAreaLopers(c *gin.Context) {
	search := c.Query("search")
	var listRekap []map[string]interface{}

	// 🚀 QUERY AGREGASI SAKTI: Menggabungkan profil agen dengan pencacah jumlah wilayah operasionalnya
	queryRaw := `
		SELECT 
			a.agen_id,
			TRIM(a.agen_kode) AS agen_kode, 
			a.agen_nama, 
			a.agen_alamat, 
			a.agen_kota, 
			a.agen_phone1 AS agen_phone,
			COUNT(w.id) AS jumlah_wilayah
		FROM public.glb_m_agen a
		LEFT JOIN public.opr_m_earea w ON TRIM(w.area_agenid::text) = TRIM(a.agen_kode)
	`

	// Saringan jika user mengetik di kotak pencarian frontend
	if search != "" {
		queryRaw += ` WHERE a.agen_nama ILIKE '%` + search + `%' OR a.agen_kode ILIKE '%` + search + `%' OR a.agen_kota ILIKE '%` + search + `%' `
	}

	queryRaw += `
		GROUP BY a.agen_id, a.agen_kode, a.agen_nama, a.agen_alamat, a.agen_kota, a.agen_phone1
		ORDER BY a.agen_id ASC
	`

	if err := db.DB.Raw(queryRaw).Scan(&listRekap).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat rekap data agen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   listRekap,
	})
}

// CreateAreaLoper untuk fitur TAMBAH
func CreateAreaLoper(c *gin.Context) {
	var input models.AreaLoper
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}
	if err := db.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal tambah data wilayah"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Wilayah operasional berhasil didaftarkan"})
}

// UpdateAreaLoper untuk fitur EDIT
func UpdateAreaLoper(c *gin.Context) {
	id := c.Param("id")
	var area models.AreaLoper
	if err := db.DB.First(&area, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data tidak ditemukan"})
		return
	}
	if err := c.ShouldBindJSON(&area); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}
	db.DB.Save(&area)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data wilayah berhasil diperbarui"})
}

// GetWilayahBelumTerdaftar melakukan komparasi NOT EXISTS dengan kolom glb_m_kodepos yang valid
func GetWilayahBelumTerdaftar(c *gin.Context) {
	var hasil []models.WilayahBelumTerdaftar

	// 🚀 TRICK SAKTI SINKRONISASI MATANG: Nama tabel & nama kolom disesuaikan 100% dengan pgAdmin!
	queryRaw := `
		SELECT 
			k.kecamatandistrik AS kecamatan, 
			k.desakelurahan AS kelurahan, 
			k.kotakabupaten AS kabupaten, 
			k.propinsi AS propinsi
		FROM public.glb_m_kodepos k
		WHERE NOT EXISTS (
			SELECT 1 
			FROM public.opr_m_earea a 
			WHERE UPPER(TRIM(a.tujuan_kelurahan)) = UPPER(TRIM(k.desakelurahan)) 
			  AND UPPER(TRIM(a.tujuan_kecamatan)) = UPPER(TRIM(k.kecamatandistrik))
		)
		ORDER BY k.kecamatandistrik ASC 
		LIMIT 200`

	if err := db.DB.Raw(queryRaw).Scan(&hasil).Error; err != nil {
		log.Printf("❌ ERROR SQL NOT EXISTS LAPIS BAJA: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal sinkronisasi data saringan kode pos fisik database",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": hasil})
}

// GetAgenDetailByID menarik detail profil agen dengan taktik bypass map murni (Anti-Scan Error Struct)
func GetAgenDetailByID(c *gin.Context) {
	kode := c.Param("id") // Menerima kode agen dari frontend

	// 🚀 TRICK DEWA LAPIS BAJA: Menampung via map murni agar tidak terikat tipe data struct models.Agen yang crash
	var hasilMap map[string]interface{}

	// Ambil kolom-kolom penting yang dibutuhkan saja dari database fisik
	err := db.DB.Table("public.glb_m_agen").
		Select("agen_kode, agen_nama, agen_alamat, agen_kota, agen_phone1").
		Where("TRIM(agen_kode) = ?", kode).
		Limit(1).
		Scan(&hasilMap).Error

	// Fallback jaring pengaman jika trimmer varchar database ada padding kaku
	if err != nil || len(hasilMap) == 0 {
		err = db.DB.Table("public.glb_m_agen").
			Select("agen_kode, agen_nama, agen_alamat, agen_kota, agen_phone1").
			Where("agen_kode LIKE ?", "%"+kode+"%").
			Limit(1).
			Scan(&hasilMap).Error
	}

	if err != nil || len(hasilMap) == 0 {
		log.Printf("❌ PROFILE NOT FOUND IN DB FOR KODE [%s]", kode)
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Profil Agen tidak ditemukan"})
		return
	}

	// Pastikan nilai dikonversi aman ke format string json balik ke frontend
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"agen_kode":   hasilMap["agen_kode"],
			"agen_nama":   hasilMap["agen_nama"],
			"agen_alamat": hasilMap["agen_alamat"],
			"agen_kota":   hasilMap["agen_kota"],
			"agen_phone":  hasilMap["agen_phone1"],
		},
	})
}

// GetAreaLoperTerpilihByAgen menarik daftar kelurahan aktif milik agen spesifik (Gambar 2)
func GetAreaLoperTerpilihByAgen(c *gin.Context) {
	kodeAgen := c.Param("kode")
	var listArea []models.AreaLoper

	// Menghindari scan error struct dengan select kolom esensial murni tabel lama
	err := db.DB.Where("TRIM(area_agenid::text) = ?", kodeAgen).
		Order("tujuan_propinsi ASC, tujuan_kabupaten ASC, tujuan_kecamatan ASC").
		Find(&listArea).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   listArea,
	})
}

// SearchMasterWilayah melayani live autocomplete suggest dari tabel master kodepos se-Indonesia
func SearchMasterWilayah(c *gin.Context) {
	propinsi := c.Query("propinsi")
	kabupaten := c.Query("kabupaten")
	kecamatan := c.Query("kecamatan")
	kelurahan := c.Query("kelurahan")

	var listLokasi []map[string]interface{}
	query := db.DB.Table("public.glb_m_kodepos").
		Select("TRIM(propinsi) AS propinsi, TRIM(kotakabupaten) AS kabupaten, TRIM(kecamatandistrik) AS kecamatan, TRIM(desakelurahan) AS kelurahan").
		Limit(15) // Batasi 15 row saja agar loading kilat seperskian detik

	// Logic Cascading Suggest Bertingkat
	if propinsi != "" {
		query = query.Where("propinsi ILIKE ?", "%"+propinsi+"%")
	}
	if kabupaten != "" {
		query = query.Where("kotakabupaten ILIKE ?", "%"+kabupaten+"%")
	}
	if kecamatan != "" {
		query = query.Where("kecamatandistrik ILIKE ?", "%"+kecamatan+"%")
	}
	if kelurahan != "" {
		query = query.Where("desakelurahan ILIKE ?", "%"+kelurahan+"%")
	}

	// Kelompokkan agar tidak memunculkan duplikasi nama daerah kembar
	query.Group("propinsi, kotakabupaten, kecamatandistrik, desakelurahan").Scan(&listLokasi)

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": listLokasi})
}

// AssignWilayahKeAgen mendaftarkan wilayah baru dengan taktik transaksi Delete-then-Insert (Anti-Conflict Crash)
func AssignWilayahKeAgen(c *gin.Context) {
	var input struct {
		AgenKode  string `json:"agen_kode"`
		Propinsi  string `json:"propinsi"`
		Kabupaten string `json:"kabupaten"`
		Kecamatan string `json:"kecamatan"`
		Kelurahan string `json:"kelurahan"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// 🚀 TRICK DEWA LOGISTIK: Mulai transaksi database agar proses hapus dan tambah berjalan serempak & aman
	tx := db.DB.Begin()

	// Step 1: Bersihkan/Hapus dulu wilayah tersebut dari ikatan agen mana pun sebelumnya
	deleteQuery := `
		DELETE FROM public.opr_m_earea 
		WHERE UPPER(TRIM(tujuan_propinsi)) = UPPER(TRIM(?))
		  AND UPPER(TRIM(tujuan_kabupaten)) = UPPER(TRIM(?))
		  AND UPPER(TRIM(tujuan_kecamatan)) = UPPER(TRIM(?))
		  AND UPPER(TRIM(tujuan_kelurahan)) = UPPER(TRIM(?))`

	if err := tx.Exec(deleteQuery, input.Propinsi, input.Kabupaten, input.Kecamatan, input.Kelurahan).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ GAGAL CLEAR DATA LAMA: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal sinkronisasi data wilayah lama"})
		return
	}

	// Step 2: Suntikkan data wilayah baru ke agen terpilih saat ini
	insertQuery := `
		INSERT INTO public.opr_m_earea (
			area_agenid, tujuan_propinsi, tujuan_kabupaten, tujuan_kecamatan, tujuan_kelurahan, 
			penerusyn, kgmin, hrgpenerus, leadtime, prosentasebykirimyn
		)
		VALUES (?, ?, ?, ?, ?, 'N', 0, 0, 1, 'N')`

	if err := tx.Exec(insertQuery, input.AgenKode, input.Propinsi, input.Kabupaten, input.Kecamatan, input.Kelurahan).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ GAGAL SUNTIK AREA BARU: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mendaftarkan wilayah operasional"})
		return
	}

	// Commit transaksi jika dua langkah di atas sukses tanpa hambatan
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Wilayah berhasil ditambahkan!"})
}

// AssignWilayahMassalKeAgen memproses pendaftaran banyak wilayah sekaligus lengkap dengan parameter operasional hasil input user
func AssignWilayahMassalKeAgen(c *gin.Context) {
	var input struct {
		AgenKode string `json:"agen_kode"`
		Wilayahs []struct {
			Propinsi   string  `json:"propinsi"`
			Kabupaten  string  `json:"kabupaten"`
			Kecamatan  string  `json:"kecamatan"`
			Kelurahan  string  `json:"kelurahan"`
			PenerusYN  string  `json:"penerusyn"`
			KgMin      float64 `json:"kgmin"`
			HrgPenerus float64 `json:"hrgpenerus"`
			LeadTime   int     `json:"leadtime"`
		} `json:"wilayahs"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	tx := db.DB.Begin()

	for _, w := range input.Wilayahs {
		// Bersihkan ikatan data wilayah lama
		deleteQuery := `
			DELETE FROM public.opr_m_earea 
			WHERE UPPER(TRIM(tujuan_propinsi)) = UPPER(TRIM(?))
			  AND UPPER(TRIM(tujuan_kabupaten)) = UPPER(TRIM(?))
			  AND UPPER(TRIM(tujuan_kecamatan)) = UPPER(TRIM(?))
			  AND UPPER(TRIM(tujuan_kelurahan)) = UPPER(TRIM(?))`

		tx.Exec(deleteQuery, w.Propinsi, w.Kabupaten, w.Kecamatan, w.Kelurahan)

		// 🚀 SUNTIKAN MASSAL: Masukkan data baru lengkap beserta parameter operasional yang diinput user!
		insertQuery := `
			INSERT INTO public.opr_m_earea (
				area_agenid, tujuan_propinsi, tujuan_kabupaten, tujuan_kecamatan, tujuan_kelurahan, 
				penerusyn, kgmin, hrgpenerus, leadtime, prosentasebykirimyn
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'N')`

		if err := tx.Exec(insertQuery, input.AgenKode, w.Propinsi, w.Kabupaten, w.Kecamatan, w.Kelurahan, w.PenerusYN, w.KgMin, w.HrgPenerus, w.LeadTime).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan kluster wilayah massal"})
			return
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Seluruh wilayah terpilih berhasil didaftarkan!"})
}

// ProcessBatchAreaLoper melayani update agen massal berdasarkan radius tingkat wilayah yang dipilih
func ProcessBatchAreaLoper(c *gin.Context) {
	var input struct {
		Level       string `json:"level"` // "propinsi", "kabupaten", "kecamatan", "kelurahan"
		Propinsi    string `json:"propinsi"`
		Kabupaten   string `json:"kabupaten"`
		Kecamatan   string `json:"kecamatan"`
		Kelurahan   string `json:"kelurahan"`
		NewAgenKode string `json:"new_agen_kode"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Buat query dasar
	query := db.DB.Table("public.opr_m_earea").Where("UPPER(TRIM(tujuan_propinsi)) = UPPER(TRIM(?))", input.Propinsi)

	// Filter radius bertingkat berdasarkan pilihan radio button (Gambar 2)
	if input.Level == "kabupaten" || input.Level == "kecamatan" || input.Level == "kelurahan" {
		query = query.Where("UPPER(TRIM(tujuan_kabupaten)) = UPPER(TRIM(?))", input.Kabupaten)
	}
	if input.Level == "kecamatan" || input.Level == "kelurahan" {
		query = query.Where("UPPER(TRIM(tujuan_kecamatan)) = UPPER(TRIM(?))", input.Kecamatan)
	}
	if input.Level == "kelurahan" {
		query = query.Where("UPPER(TRIM(tujuan_kelurahan)) = UPPER(TRIM(?))", input.Kelurahan)
	}

	// Eksekusi update massal kode agen petugas loper baru
	if err := query.Update("area_agenid", input.NewAgenKode).Error; err != nil {
		log.Printf("❌ GAGAL BATCH UPDATE: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui data batch area"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Batch update area loper berhasil diperbarui!"})
}

// RemoveWilayahMassalDariAgen menghapus massal ikatan wilayah operasional dari tabel opr_m_earea
func RemoveWilayahMassalDariAgen(c *gin.Context) {
	var input struct {
		IDs []int `json:"ids"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if len(input.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Tidak ada wilayah yang dipilih untuk dihapus"})
		return
	}

	// Eksekusi DELETE massal menggunakan query IN clause berdasarkan array ID yang dicentang
	if err := db.DB.Table("public.opr_m_earea").Where("id IN ?", input.IDs).Delete(nil).Error; err != nil {
		log.Printf("❌ GAGAL DELETE MASSAL AREA: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus ikatan wilayah operasional"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Sukses melepas ikatan wilayah dari agen!"})
}

// UpdateSingleAreaLoperAtribut memproses pembaruan parameter operasional dan perpindahan agen loper per baris
func UpdateSingleAreaLoperAtribut(c *gin.Context) {
	var input struct {
		ID          int     `json:"id"`
		PenerusYN   string  `json:"penerusyn"`
		KgMin       float64 `json:"kgmin"`
		HrgPenerus  float64 `json:"hrgpenerus"`
		LeadTime    int     `json:"leadtime"`
		NewAgenKode string  `json:"new_agen_kode"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Jalankan perintah UPDATE tunggal terarah
	queryRaw := `
		UPDATE public.opr_m_earea 
		SET penerusyn = ?, kgmin = ?, hrgpenerus = ?, leadtime = ?, area_agenid = ? 
		WHERE id = ?`

	if err := db.DB.Exec(queryRaw, input.PenerusYN, input.KgMin, input.HrgPenerus, input.LeadTime, input.NewAgenKode, input.ID).Error; err != nil {
		log.Printf("❌ GAGAL UPDATE ATRIBUT AREA: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui parameter area loper"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data area loper sukses diperbarui!"})
}
