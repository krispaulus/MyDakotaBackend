package handler

import (
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

type TarifTransitView struct {
	ID             uint    `json:"id" gorm:"column:id"`
	ProvinsiAsal   string  `json:"provinsi_asal" gorm:"column:provinsi_asal"`
	TrKotaAsal     string  `json:"tr_kotaasal" gorm:"column:tr_kotaasal"`
	ProvinsiTujuan string  `json:"provinsi_tujuan" gorm:"column:provinsi_tujuan"`
	TrKotaTujuan   string  `json:"tr_kotatujuan" gorm:"column:tr_kotatujuan"`
	TrKategori     float64 `json:"tr_kategori" gorm:"column:tr_kategori"`
	TrServiceType  float64 `json:"tr_servicetype" gorm:"column:tr_servicetype"`
	TrNominal      float64 `json:"tr_nominal" gorm:"column:tr_nominal"`
}

func GetTarifTransitList(c *gin.Context) {
	// Arahkan langsung ke koneksi DLI
	database := db.DLIDB

	var currentDB string
	database.Raw("SELECT current_database()").Scan(&currentDB)

	kotaAsal := strings.TrimSpace(c.Query("search_kotaAsal"))
	kotaTuj := strings.TrimSpace(c.Query("search_kotaTujuan"))
	provAsal := strings.TrimSpace(c.Query("search_provinsiAsal"))
	provTuj := strings.TrimSpace(c.Query("search_provinsiTujuan"))
	kategoriStr := strings.TrimSpace(c.Query("search_kategori"))
	serviceStr := strings.TrimSpace(c.Query("search_service"))

	sqlQuery := `
		SELECT 
			t.id,
			COALESCE(k1.propinsi, '-') AS provinsi_asal,
			t.tr_kotaasal,
			COALESCE(k2.propinsi, '-') AS provinsi_tujuan,
			t.tr_kotatujuan,
			COALESCE(t.tr_kategori, 0) AS tr_kategori,
			COALESCE(t.tr_servicetype, 1) AS tr_servicetype,
			COALESCE(t.tr_nominal, 0) AS tr_nominal
		FROM public.opr_m_tariftransit t
		LEFT JOIN (
			SELECT DISTINCT UPPER(kotakabupaten) AS kota, propinsi 
			FROM public.glb_m_ekodepos
		) k1 ON UPPER(t.tr_kotaasal) = k1.kota
		LEFT JOIN (
			SELECT DISTINCT UPPER(kotakabupaten) AS kota, propinsi 
			FROM public.glb_m_ekodepos
		) k2 ON UPPER(t.tr_kotatujuan) = k2.kota
		WHERE 1=1
	`

	var args []interface{}

	if kotaAsal != "" {
		sqlQuery += " AND t.tr_kotaasal ILIKE ?"
		args = append(args, "%"+kotaAsal+"%")
	}
	if kotaTuj != "" {
		sqlQuery += " AND t.tr_kotatujuan ILIKE ?"
		args = append(args, "%"+kotaTuj+"%")
	}
	if provAsal != "" {
		sqlQuery += " AND k1.propinsi ILIKE ?"
		args = append(args, "%"+provAsal+"%")
	}
	if provTuj != "" {
		sqlQuery += " AND k2.propinsi ILIKE ?"
		args = append(args, "%"+provTuj+"%")
	}
	if kategoriStr != "" {
		kats := strings.Split(kategoriStr, ",")
		sqlQuery += " AND t.tr_kategori IN (?)"
		args = append(args, kats)
	}
	if serviceStr != "" {
		svcs := strings.Split(serviceStr, ",")
		sqlQuery += " AND t.tr_servicetype IN (?)"
		args = append(args, svcs)
	}

	sqlQuery += " ORDER BY t.id DESC LIMIT 2000"

	var listData []TarifTransitView
	if err := database.Raw(sqlQuery, args...).Scan(&listData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":   "error",
			"message":  err.Error(),
			"database": currentDB,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"data":          listData,
		"total_records": len(listData),
		"database":      currentDB,
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

	// Cek Duplikasi Rute (konversi req.Kategori dan req.Service ke float64)
	var count int64
	database.Model(&models.OprMTarifTransit{}).
		Where("UPPER(tr_kotaasal) = UPPER(?) AND UPPER(tr_kotatujuan) = UPPER(?) AND tr_kategori = ? AND tr_servicetype = ?",
			req.KotaAsal, req.KotaTujuan, float64(req.Kategori), float64(req.Service)).
		Count(&count)

	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Data tarif transit untuk rute, kategori, dan service ini sudah ada!",
		})
		return
	}

	// Casting eksplisit int ke float64
	newTarif := models.OprMTarifTransit{
		TrKotaAsal:    req.KotaAsal,
		TrKotaTujuan:  req.KotaTujuan,
		TrKategori:    float64(req.Kategori),
		TrServiceType: float64(req.Service),
		TrNominal:     req.Nominal,
	}

	if err := database.Create(&newTarif).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan tarif transit: " + err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	var existing models.OprMTarifTransit
	if err := database.First(&existing, idStr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data tidak ditemukan"})
		return
	}

	existing.TrKotaAsal = req.KotaAsal
	existing.TrKotaTujuan = req.KotaTujuan
	existing.TrKategori = float64(req.Kategori)   // 👈 casting ke float64
	existing.TrServiceType = float64(req.Service) // 👈 casting ke float64
	existing.TrNominal = req.Nominal

	if err := database.Save(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate tarif transit: " + err.Error()})
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
