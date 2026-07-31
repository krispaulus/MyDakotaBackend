package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

type MassCarterPayload struct {
	AgenIDAsal string   `json:"agenid_asal" binding:"required"`
	KendJenis  string   `json:"kend_jenis" binding:"required"`
	Propinsi   string   `json:"tujuan_propinsi" binding:"required"`
	Kabupaten  []string `json:"tujuan_kabupaten" binding:"required"` // Array banyak kota sekaligus bray
}

func AddMassCarter(c *gin.Context) {
	var input MassCarterPayload
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload input tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)

	// Lakukan looping insert database sesuai pakem kode .asp lawas bray
	for _, kota := range input.Kabupaten {
		if kota == "" {
			continue
		}

		// Cek dulu biar ga duplikat data rute yg sama bray
		var count int64
		database.Model(&models.MktMeHargaCharter{}).
			Where("agenid_asal = ? AND kend_jenis = ? AND tujuan_propinsi = ? AND tujuan_kabupaten = ?",
				input.AgenIDAsal, input.KendJenis, input.Propinsi, kota).
			Count(&count)

		if count > 0 {
			continue // Skip jika rute tersebut sudah nangkring di DB
		}

		newRow := models.MktMeHargaCharter{
			AgenidAsal:      input.AgenIDAsal,
			KendJenis:       input.KendJenis,
			TujuanPropinsi:  input.Propinsi,
			TujuanKabupaten: kota,
			HargaPokok:      0, // Default 0 sesuai query .asp asli bray!
			Keterangan:      "",
			UpdateID:        &userStr,
			UpdateTime:      time.Now(),
		}

		if err := database.Create(&newRow).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan baris rute masal"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Mass rute carter berhasil didaftarkan!"})
}

// 📱 1. API GET: Ambil List Halaman Utama (Index Master Carter) [cite: 48]
func GetTarifCarterIndex(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	ptStr := fmt.Sprintf("%v", ptID)

	database, ok := db.ResolveDB(ptStr)
	if !ok {
		database = db.DB
	}

	searchAgen := strings.TrimSpace(c.Query("agen_nama"))

	// 🟢 JIKA PT = DLI / MEMAKAI OPR_M_KENDJENIS
	if database.Migrator().HasTable("public.opr_m_kendjenis") && !database.Migrator().HasTable("public.mkt_m_eharga_charter") {
		type KendJenisDTO struct {
			JenisID        string  `json:"agen_id"`   // Digunakan sebagai ID utama
			JenisMerk      string  `json:"agen_nama"` // Merek Armada
			JenisModel     string  `json:"agen_kota"` // Model Armada
			JenisHargasewa float64 `json:"hargasewa"` // Harga Sewa DLI
			Jml            int64   `json:"jml"`       // Indicator Status
		}

		var listKend []KendJenisDTO
		query := database.Table("public.opr_m_kendjenis").
			Select("jenis_id, jenis_merk, jenis_model, jenis_hargasewa, 1 as jml").
			Where("jenis_aktifyn = 'Y'")

		if searchAgen != "" {
			query = query.Where("UPPER(jenis_merk) LIKE UPPER(?) OR UPPER(jenis_model) LIKE UPPER(?)", "%"+searchAgen+"%", "%"+searchAgen+"%")
		}

		if err := query.Order("jenis_id ASC").Scan(&listKend).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat tarif carter DLI"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "success", "data": listKend, "schema_type": "DLI"})
		return
	}

	// 🔵 JIKA PT = DBS / MEMAKAI MKT_M_EHARGA_CHARTER
	var result []models.CarterIndexDTO
	queryStr := `
		SELECT a.agen_id, a.agen_nama, a.agen_alamat, a.agen_kota, a.agen_phone1, 
		       COUNT(c.id) AS jml 
		FROM glb_m_agen a 
		LEFT JOIN mkt_m_eharga_charter c ON a.agen_id = c.agenid_asal 
		WHERE a.agen_aktifyn = 'Y'
	`
	if searchAgen != "" {
		queryStr += fmt.Sprintf(" AND a.agen_nama ILIKE '%%%s%%'", searchAgen)
	}
	queryStr += " GROUP BY a.agen_id, a.agen_nama, a.agen_alamat, a.agen_kota, a.agen_phone1 ORDER BY a.agen_nama ASC"

	err := database.Raw(queryStr).Scan(&result).Error
	if err != nil {
		// Fallback jika tabel mkt_m_eharga_charter belum dibuat di DB ini
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": []interface{}{}, "schema_type": "NONE"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": result, "schema_type": "DBS"})
}

// 🗺️ 2. API GET: Ambil Matrix Detail Berdasarkan Agen & Jenis Kendaraan untuk Halaman Edit
func GetTarifCarterDetail(c *gin.Context) {
	agenID := c.Query("agen_id")
	kendJenis := c.Query("kend_jenis")

	var listTarif []models.MktMeHargaCharter

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	// Bersihkan kata 'window.' agar murni dibaca kolom PostgreSQL bray!
	err := database.Where("agenid_asal = ? AND kend_jenis = ?", agenID, kendJenis).
		Order("tujuan_propinsi ASC, tujuan_kabupaten ASC").
		Find(&listTarif).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memuat detail harga"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": listTarif})
}

// ⚡ 3. API PUT: AJAX Inline Update Harga Pokok & Keterangan (Sembuh Total!) [cite: 30, 41]
func UpdateInlineCarter(c *gin.Context) {
	var input struct {
		ID         int64   `json:"id" binding:"required"`
		HargaPokok float64 `json:"hargapokok"`
		Keterangan string  `json:"keterangan"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload input tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)

	// Update data baris terarah menggunakan Single ID GORM [cite: 19]
	err := database.Model(&models.MktMeHargaCharter{}).Where("id = ?", input.ID).Updates(map[string]interface{}{
		"hargapokok":  input.HargaPokok,
		"keterangan":  input.Keterangan,
		"update_id":   userStr,
		"update_time": time.Now(),
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memperbarui item harga carter"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Baris tarif carter ter-update real-time bray!"})
}

// 🗑️ 4. API DELETE: Hapus Item Jalur Pengiriman Carter [cite: 19]
func DeleteTarifCarterItem(c *gin.Context) {
	id := c.Param("id")

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	if err := database.Where("id = ?", id).Delete(&models.MktMeHargaCharter{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menghapus baris rute carter"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Jalur carter resmi dihapus bray!"})
}

// 🔍 API GET: Ambil Semua Agen Aktif untuk Dropdown Pencarian di Frontend
func GetActiveAgenList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database tidak ditemukan"})
		return
	}

	type AgenDropdownDTO struct {
		AgenID   string `json:"agen_id" gorm:"column:agen_id"`
		AgenNama string `json:"agen_nama" gorm:"column:agen_nama"`
		AgenKota string `json:"agen_kota" gorm:"column:agen_kota"`
	}

	var listAgen []AgenDropdownDTO
	// Tarik data agen yang aktif saja bray agar sinkron 100%
	err := database.Table("glb_m_agen").
		Select("agen_id, agen_nama, agen_kota").
		Where("agen_aktifyn = 'Y'").
		Order("agen_nama ASC").
		Find(&listAgen).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memuat daftar master agen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": listAgen})
}
