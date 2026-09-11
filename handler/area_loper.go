package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GET /api/area-loper (Rekap Agen & Jumlah Wilayah Loper)
// GET /api/area-loper (Rekap Agen & Jumlah Wilayah Loper)
func GetAreaLopers(c *gin.Context) {
	search := strings.TrimSpace(c.Query("search"))
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak ditemukan"})
		return
	}

	var listRekap []map[string]interface{}
	queryRaw := `
		SELECT 
			a.agen_id::varchar AS agen_id,
			COALESCE(NULLIF(TRIM(a.agen_kode), ''), a.agen_id::varchar, '') AS agen_kode,
			COALESCE(a.agen_nama, '') AS agen_nama,
			COALESCE(a.agen_alamat, '') AS agen_alamat,
			COALESCE(a.agen_kota, '') AS agen_kota,
			COALESCE(NULLIF(TRIM(a.agen_telp), ''), a.agen_hp, '') AS agen_phone,
			COUNT(w.area_agenid) AS jumlah_wilayah
		FROM public.glb_m_agen a
		LEFT JOIN public.opr_m_earea w ON (
			TRIM(w.area_agenid::text) = TRIM(a.agen_id::text) OR 
			(NULLIF(TRIM(a.agen_kode), '') IS NOT NULL AND TRIM(w.area_agenid::text) = TRIM(a.agen_kode::text))
		)
		WHERE a.agen_aktifyn = 'Y'
	`

	if search != "" {
		s := "%" + search + "%"
		queryRaw += fmt.Sprintf(` AND (a.agen_nama ILIKE '%s' OR a.agen_kode ILIKE '%s' OR a.agen_id::varchar ILIKE '%s' OR a.agen_kota ILIKE '%s') `, s, s, s, s)
	}

	queryRaw += ` GROUP BY a.agen_id, a.agen_kode, a.agen_nama, a.agen_alamat, a.agen_kota, a.agen_telp, a.agen_hp ORDER BY a.agen_nama ASC`

	if err := database.Raw(queryRaw).Scan(&listRekap).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat rekap data agen: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": listRekap})
}

// POST /api/area-lopers
func CreateAreaLoper(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung bray"})
		return
	}

	var input models.AreaLoper
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if err := database.Table("public.opr_m_earea").Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal tambah data wilayah"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Wilayah operasional berhasil didaftarkan"})
}

// PUT /api/area-lopers/:id
func UpdateAreaLoper(c *gin.Context) {
	id := c.Param("id")
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terdeteksi"})
		return
	}

	var area models.AreaLoper
	if err := database.Table("public.opr_m_earea").First(&area, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data tidak ditemukan"})
		return
	}

	if err := c.ShouldBindJSON(&area); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	database.Table("public.opr_m_earea").Where("id = ?", id).Save(&area)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data wilayah berhasil diperbarui"})
}

func GetWilayahBelumTerdaftar(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var hasil []models.WilayahBelumTerdaftar

	// 1. Cek kolom yang tersedia di tabel opr_m_earea
	var colNames []string
	database.Raw(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_schema = 'public' AND table_name = 'opr_m_earea'
	`).Pluck("column_name", &colNames)

	hasCol := func(target string) bool {
		for _, col := range colNames {
			if strings.EqualFold(col, target) {
				return true
			}
		}
		return false
	}

	// 2. Susun kondisi pencocokan kelurahan & kecamatan sesuai kolom fisik yang ada
	var kelCol, kecCol string
	if hasCol("area_kelurahan") {
		kelCol = "a.area_kelurahan"
	} else if hasCol("tujuan_kelurahan") {
		kelCol = "a.tujuan_kelurahan"
	} else if hasCol("kelurahan") {
		kelCol = "a.kelurahan"
	} else if hasCol("earea_kelurahan") {
		kelCol = "a.earea_kelurahan"
	}

	if hasCol("area_kecamatan") {
		kecCol = "a.area_kecamatan"
	} else if hasCol("tujuan_kecamatan") {
		kecCol = "a.tujuan_kecamatan"
	} else if hasCol("kecamatan") {
		kecCol = "a.kecamatan"
	} else if hasCol("earea_kecamatan") {
		kecCol = "a.earea_kecamatan"
	}

	var queryRaw string
	if kelCol != "" && kecCol != "" {
		queryRaw = fmt.Sprintf(`
			SELECT 
				COALESCE(k.kecamatandistrik, '') AS kecamatan,
				COALESCE(k.desakelurahan, '') AS kelurahan,
				COALESCE(k.kotakabupaten, '') AS kabupaten,
				COALESCE(k.propinsi, '') AS propinsi
			FROM public.glb_m_kodepos k
			WHERE NOT EXISTS (
				SELECT 1 FROM public.opr_m_earea a 
				WHERE UPPER(TRIM(%s::text)) = UPPER(TRIM(k.desakelurahan))
				  AND UPPER(TRIM(%s::text)) = UPPER(TRIM(k.kecamatandistrik))
			)
			ORDER BY k.kecamatandistrik ASC LIMIT 200`, kelCol, kecCol)
	} else {
		// Fallback query jika belum ada mapping kolom kelurahan/kecamatan
		queryRaw = `
			SELECT 
				COALESCE(k.kecamatandistrik, '') AS kecamatan,
				COALESCE(k.desakelurahan, '') AS kelurahan,
				COALESCE(k.kotakabupaten, '') AS kabupaten,
				COALESCE(k.propinsi, '') AS propinsi
			FROM public.glb_m_kodepos k
			ORDER BY k.kecamatandistrik ASC LIMIT 200`
	}

	if err := database.Raw(queryRaw).Scan(&hasil).Error; err != nil {
		log.Printf("❌ ERROR SQL UNREGISTERED AREA: %v", err)
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": []models.WilayahBelumTerdaftar{}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": hasil})
}

// GET /api/area-lopers/detail/:id
func GetAgenDetailByID(c *gin.Context) {
	kode := c.Param("id")
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	var hasilMap map[string]interface{}
	err := database.Table("public.glb_m_agen").
		Select("agen_kode, agen_nama, agen_alamat, agen_kota, COALESCE(agen_telp, '') AS agen_phone").
		Where("TRIM(agen_kode::text) = TRIM(?)", kode).Limit(1).Scan(&hasilMap).Error

	if err != nil || len(hasilMap) == 0 {
		database.Table("public.glb_m_agen").
			Select("agen_kode, agen_nama, agen_alamat, agen_kota, COALESCE(agen_telp, '') AS agen_phone").
			Where("agen_kode ILIKE ?", "%"+kode+"%").Limit(1).Scan(&hasilMap)
	}

	if len(hasilMap) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Profil Agen tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"agen_kode":   hasilMap["agen_kode"],
			"agen_nama":   hasilMap["agen_nama"],
			"agen_alamat": hasilMap["agen_alamat"],
			"agen_kota":   hasilMap["agen_kota"],
			"agen_phone":  hasilMap["agen_phone"],
		},
	})
}

// GET /api/area-lopers/terpilih/:kode
// GET /api/area-lopers/terpilih/:kode
func GetAreaLoperTerpilihByAgen(c *gin.Context) {
	kodeAgen := strings.TrimSpace(c.Param("kode"))
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak ditemukan"})
		return
	}

	// 1. Ambil agen_id dan agen_kode yang valid dari glb_m_agen
	var agen struct {
		AgenID   string `gorm:"column:agen_id"`
		AgenKode string `gorm:"column:agen_kode"`
	}
	database.Table("public.glb_m_agen").
		Select("agen_id::varchar, COALESCE(agen_kode, '') as agen_kode").
		Where("agen_id::varchar = ? OR agen_kode = ?", kodeAgen, kodeAgen).
		Take(&agen)

	targetID := kodeAgen
	if agen.AgenID != "" {
		targetID = agen.AgenID
	}
	targetKode := agen.AgenKode

	// 2. Query list area terpilih dengan fallback ID & Kode
	var listArea []map[string]interface{}
	err := database.Table("public.opr_m_earea").
		Where("TRIM(area_agenid::text) = ? OR (NULLIF(?, '') IS NOT NULL AND TRIM(area_agenid::text) = ?)", targetID, targetKode, targetKode).
		Order("tujuan_propinsi ASC, tujuan_kabupaten ASC, tujuan_kecamatan ASC, tujuan_kelurahan ASC").
		Scan(&listArea).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": listArea})
}

// GET /api/area-lopers/search-master
func SearchMasterWilayah(c *gin.Context) {
	propinsi := c.Query("propinsi")
	kabupaten := c.Query("kabupaten")
	kecamatan := c.Query("kecamatan")
	kelurahan := c.Query("kelurahan")

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak ditemukan"})
		return
	}

	var listLokasi []map[string]interface{}
	query := database.Table("public.glb_m_kodepos").
		Select("TRIM(propinsi) AS propinsi, TRIM(kotakabupaten) AS kabupaten, TRIM(kecamatandistrik) AS kecamatan, TRIM(desakelurahan) AS kelurahan").
		Limit(25)

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

	query.Group("propinsi, kotakabupaten, kecamatandistrik, desakelurahan").Scan(&listLokasi)
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": listLokasi})
}

// POST /api/area-lopers/assign-massal
func AssignWilayahMassalKeAgen(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak ditemukan"})
		return
	}

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

	tx := database.Begin()
	for _, w := range input.Wilayahs {
		deleteQuery := `
			DELETE FROM public.opr_m_earea 
			WHERE (
				UPPER(TRIM(COALESCE(area_propinsi, tujuan_propinsi, ''))) = UPPER(TRIM(?)) 
				AND UPPER(TRIM(COALESCE(area_kota, tujuan_kabupaten, ''))) = UPPER(TRIM(?)) 
				AND UPPER(TRIM(COALESCE(area_kecamatan, tujuan_kecamatan, ''))) = UPPER(TRIM(?)) 
				AND UPPER(TRIM(COALESCE(area_kelurahan, tujuan_kelurahan, ''))) = UPPER(TRIM(?))
			)`
		tx.Exec(deleteQuery, w.Propinsi, w.Kabupaten, w.Kecamatan, w.Kelurahan)

		insertQuery := `
			INSERT INTO public.opr_m_earea (
				area_agenid, area_propinsi, area_kota, area_kecamatan, area_kelurahan,
				tujuan_propinsi, tujuan_kabupaten, tujuan_kecamatan, tujuan_kelurahan,
				penerusyn, kgmin, hrgpenerus, leadtime, prosentasebykirimyn
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'N')`
		if err := tx.Exec(insertQuery, input.AgenKode, w.Propinsi, w.Kabupaten, w.Kecamatan, w.Kelurahan, w.Propinsi, w.Kabupaten, w.Kecamatan, w.Kelurahan, w.PenerusYN, w.KgMin, w.HrgPenerus, w.LeadTime).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan kluster wilayah massal: " + err.Error()})
			return
		}
	}
	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Seluruh wilayah terpilih berhasil didaftarkan!"})
}

// PUT /api/area-lopers/batch-update
func ProcessBatchAreaLoper(c *gin.Context) {
	var input struct {
		Level       string `json:"level"`
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

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak ditemukan"})
		return
	}

	query := database.Table("public.opr_m_earea").
		Where("(UPPER(TRIM(COALESCE(area_propinsi, tujuan_propinsi, ''))) = UPPER(TRIM(?)))", input.Propinsi)

	if input.Level == "kabupaten" || input.Level == "kecamatan" || input.Level == "kelurahan" {
		query = query.Where("(UPPER(TRIM(COALESCE(area_kota, tujuan_kabupaten, ''))) = UPPER(TRIM(?)))", input.Kabupaten)
		if input.Level == "kecamatan" || input.Level == "kelurahan" {
			query = query.Where("(UPPER(TRIM(COALESCE(area_kecamatan, tujuan_kecamatan, ''))) = UPPER(TRIM(?)))", input.Kecamatan)
			if input.Level == "kelurahan" {
				query = query.Where("(UPPER(TRIM(COALESCE(area_kelurahan, tujuan_kelurahan, ''))) = UPPER(TRIM(?)))", input.Kelurahan)
			}
		}
	}

	if err := query.Update("area_agenid", input.NewAgenKode).Error; err != nil {
		log.Printf("❌ GAGAL BATCH UPDATE: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui data batch area"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Batch update area loper berhasil diperbarui!"})
}

// DELETE /api/area-lopers/remove-massal
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

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak ditemukan"})
		return
	}

	if err := database.Table("public.opr_m_earea").Where("id IN ?", input.IDs).Delete(nil).Error; err != nil {
		log.Printf("❌ GAGAL DELETE MASSAL AREA: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus ikatan wilayah operasional"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Sukses melepas ikatan wilayah dari agen!"})
}

// PUT /api/area-lopers/update-single-attribute
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

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak ditemukan"})
		return
	}

	queryRaw := `
		UPDATE public.opr_m_earea 
		SET penerusyn = ?, kgmin = ?, hrgpenerus = ?, leadtime = ?, area_agenid = ? 
		WHERE id = ?`

	if err := database.Exec(queryRaw, input.PenerusYN, input.KgMin, input.HrgPenerus, input.LeadTime, input.NewAgenKode, input.ID).Error; err != nil {
		log.Printf("❌ GAGAL UPDATE ATRIBUT AREA: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui parameter area loper"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data area loper sukses diperbarui!"})
}

// POST /api/area-lopers/assign - DYNAMIC MULTI-TENANT
func AssignWilayahKeAgen(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak ditemukan"})
		return
	}

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

	tx := database.Begin()
	deleteQuery := `
		DELETE FROM public.opr_m_earea 
		WHERE UPPER(TRIM(tujuan_propinsi)) = UPPER(TRIM(?)) 
		  AND UPPER(TRIM(tujuan_kabupaten)) = UPPER(TRIM(?)) 
		  AND UPPER(TRIM(tujuan_kecamatan)) = UPPER(TRIM(?)) 
		  AND UPPER(TRIM(tujuan_kelurahan)) = UPPER(TRIM(?))`

	if err := tx.Exec(deleteQuery, input.Propinsi, input.Kabupaten, input.Kecamatan, input.Kelurahan).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal sinkronisasi data wilayah lama"})
		return
	}

	insertQuery := `
		INSERT INTO public.opr_m_earea (area_agenid, tujuan_propinsi, tujuan_kabupaten, tujuan_kecamatan, tujuan_kelurahan, penerusyn, kgmin, hrgpenerus, leadtime, prosentasebykirimyn)
		VALUES (?, ?, ?, ?, ?, 'N', 0, 0, 1, 'N')`

	if err := tx.Exec(insertQuery, input.AgenKode, input.Propinsi, input.Kabupaten, input.Kecamatan, input.Kelurahan).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mendaftarkan wilayah operasional"})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Wilayah berhasil ditambahkan!"})
}
