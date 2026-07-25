package handler

import (
	"net/http"
	"strconv"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 1. GET LIST DATA
// 1. GET LIST DATA TARIF TRANSIT (Murni Tanpa JOIN)
func GetTarifTransitList(c *gin.Context) {
	// Ambil koneksi DB operasional (atau pakai helper getCorporateDB(c))
	database := db.DB

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "15"))
	offset := (page - 1) * limit

	kotaAsal := strings.TrimSpace(c.Query("search_kotaAsal"))
	kotaTuj := strings.TrimSpace(c.Query("search_kotaTujuan"))
	kategoriStr := strings.TrimSpace(c.Query("search_kategori")) // Contoh: "0,1"
	serviceStr := strings.TrimSpace(c.Query("search_service"))   // Contoh: "1,2,3"

	// Query Murni ke Tabel Tarif Transit
	query := database.Table("opr_m_tariftransit t")

	if kotaAsal != "" {
		query = query.Where("UPPER(t.tr_kotaasal) LIKE UPPER(?)", "%"+kotaAsal+"%")
	}
	if kotaTuj != "" {
		query = query.Where("UPPER(t.tr_kotatujuan) LIKE UPPER(?)", "%"+kotaTuj+"%")
	}
	if kategoriStr != "" {
		kats := strings.Split(kategoriStr, ",")
		query = query.Where("t.tr_kategori IN ?", kats)
	}
	if serviceStr != "" {
		svcs := strings.Split(serviceStr, ",")
		query = query.Where("t.tr_servicetype IN ?", svcs)
	}

	// Hitung Total Data
	var totalRecords int64
	query.Count(&totalRecords)

	// Fetch Data dengan Order & Pagination
	var listData []models.OprMTarifTransit
	err := query.Order("t.id DESC").Offset(offset).Limit(limit).Find(&listData).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data tarif transit: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          listData,
		"total_records": totalRecords,
		"page":          page,
		"limit":         limit,
	})
}

// 2. AUTOCOMPLETE PROVINSI (Tembak db.DBDBS)
func GetProvinsiOptions(c *gin.Context) {
	database := db.DB // Panggil DB Master yang menyimpan ekodepos
	var provs []string
	database.Table("glb_m_ekodepos").
		Where("propinsi IS NOT NULL AND propinsi != ''").
		Distinct("propinsi").
		Order("propinsi ASC").
		Pluck("propinsi", &provs)

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": provs})
}

// 3. AUTOCOMPLETE KOTA (Tembak db.DBDBS)
func GetKotaByProvinsi(c *gin.Context) {
	database := db.DB // Panggil DB Master yang menyimpan ekodepos
	prov := c.Query("provinsi")
	q := strings.TrimSpace(c.Query("query"))

	var kotas []string
	query := database.Table("glb_m_ekodepos").
		Where("UPPER(propinsi) = UPPER(?)", prov).
		Where("kotakabupaten IS NOT NULL AND kotakabupaten != ''")

	if q != "" {
		query = query.Where("UPPER(kotakabupaten) LIKE UPPER(?)", "%"+q+"%")
	}

	query.Distinct("kotakabupaten").Order("kotakabupaten ASC").Limit(20).Pluck("kotakabupaten", &kotas)

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": kotas})
}

// 3. CREATE TARIF TRANSIT (Dengan Check Duplicate)
func CreateTarifTransit(c *gin.Context) {
	database := db.DB
	var req models.TarifTransitRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	// Cek Duplikasi Rute
	var count int64
	database.Model(&models.OprMTarifTransit{}).
		Where("UPPER(tr_kotaasal) = UPPER(?) AND UPPER(tr_kotatujuan) = UPPER(?) AND tr_kategori = ? AND tr_servicetype = ?",
			req.KotaAsal, req.KotaTujuan, req.Kategori, req.Service).
		Count(&count)

	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Data tarif transit untuk rute, kategori, dan service ini sudah ada!",
		})
		return
	}

	newTarif := models.OprMTarifTransit{
		TrKotaAsal:    req.KotaAsal,
		TrKotaTujuan:  req.KotaTujuan,
		TrKategori:    req.Kategori,
		TrServiceType: req.Service,
		TrNominal:     req.Nominal,
	}

	if err := database.Create(&newTarif).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan tarif transit"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success", "message": "Tarif transit berhasil disimpan", "data": newTarif})
}

// 4. UPDATE TARIF TRANSIT
func UpdateTarifTransit(c *gin.Context) {
	database := db.DB
	idStr := c.Param("id")

	var req models.TarifTransitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid"})
		return
	}

	var existing models.OprMTarifTransit
	if err := database.First(&existing, idStr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data tidak ditemukan"})
		return
	}

	existing.TrKotaAsal = req.KotaAsal
	existing.TrKotaTujuan = req.KotaTujuan
	existing.TrKategori = req.Kategori
	existing.TrServiceType = req.Service
	existing.TrNominal = req.Nominal

	if err := database.Save(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate tarif transit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Tarif transit berhasil diperbarui"})
}

// 5. DELETE TARIF TRANSIT
func DeleteTarifTransit(c *gin.Context) {
	database := db.DB
	idStr := c.Param("id")

	if err := database.Delete(&models.OprMTarifTransit{}, idStr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus tarif transit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Tarif transit berhasil dihapus"})
}
