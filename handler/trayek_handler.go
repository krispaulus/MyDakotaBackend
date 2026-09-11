package handler

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DTO Payload penampung bundling data dari React bray
type TrayekFullPayload struct {
	Header  models.OprMTrayekH         `json:"header"`
	Rute    []models.OprMTrayekD       `json:"rute"`
	Bplk    []models.OprMTrayekBplk    `json:"bplk"`
	Voucher []models.OprMTrayekVoucher `json:"voucher"`
}

// 👑 API UPSERT MULTIPLEXING: Simpan / Edit Trayek Lapis Tiga dalam Satu Transaksi DB!
func SaveTrayekFull(c *gin.Context) {
	var payload TrayekFullPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload bundling tidak valid bray!"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database tidak ditemukan"})
		return
	}

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)

	trhID := payload.Header.TrhID

	// Menggunakan DB Transaction GORM agar jika salah satu detail gagal, otomatis rollback total bray!
	err := database.Transaction(func(tx *gorm.DB) error {
		var isEdit bool
		var existing models.OprMTrayekH

		if trhID != "" {
			// Cek apakah mode edit atau tambah baru bray
			if err := tx.Where("trh_id = ?", trhID).First(&existing).Error; err == nil {
				isEdit = true
			}
		}

		if !isEdit {
			// GENERATE KODE TRAYEK BARU JIKA MODE TAMBAH
			trhID = fmt.Sprintf("TR%s%d", time.Now().Format("200601"), time.Now().Unix()%10000)
			payload.Header.TrhID = trhID
			payload.Header.TrhAktifYN = "Y"
		}

		payload.Header.TrhUpdateID = &userStr
		payload.Header.TrhUpdateTime = time.Now()

		// 1. Simpan Header Utama
		if isEdit {
			if err := tx.Model(&models.OprMTrayekH{}).Where("trh_id = ?", trhID).Updates(&payload.Header).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Create(&payload.Header).Error; err != nil {
				return err
			}
		}

		// 2. Bersihkan detail lama & Ganti baru (Pakem Logistik Logis)
		if isEdit {
			if err := tx.Where("trd_hid = ?", trhID).Delete(&models.OprMTrayekD{}).Error; err != nil {
				return err
			}
			if err := tx.Where("tri_hid = ?", trhID).Delete(&models.OprMTrayekBplk{}).Error; err != nil {
				return err
			}
			if err := tx.Where("trv_hid = ?", trhID).Delete(&models.OprMTrayekVoucher{}).Error; err != nil {
				return err
			}
		}

		// 3. Inject detail rute kota singgah
		for i := range payload.Rute {
			payload.Rute[i].ID = 0 // Reset serial bray
			payload.Rute[i].TrdHid = trhID
			if err := tx.Create(&payload.Rute[i]).Error; err != nil {
				return err
			}
		}

		// 4. Inject detail komponen biaya (BPLK)
		for i := range payload.Bplk {
			payload.Bplk[i].ID = 0
			payload.Bplk[i].TriHid = trhID
			if err := tx.Create(&payload.Bplk[i]).Error; err != nil {
				return err
			}
		}

		// 5. Inject detail pairing SPBU Mandiri
		for i := range payload.Voucher {
			payload.Voucher[i].ID = 0
			payload.Voucher[i].TrvHid = trhID
			if err := tx.Create(&payload.Voucher[i]).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mengunci bundling data trayek: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Bundling rute operasional nasional berhasil disimpan!", "trh_id": trhID})
}

// 📱 1. API GET: Ambil Semua Daftar Rute Utama (Header) dengan Filter Nama
func GetTrayekList(c *gin.Context) {
	var trayek []models.OprMTrayekH
	searchName := strings.TrimSpace(c.Query("nama"))

	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Database resolver gagal"})
		return
	}

	query := database.Table("public.opr_m_trayekh")

	// 1. Cek kolom status aktif yang benar-benar ada di tabel opr_m_trayekh
	var hasAktifCol bool
	database.Raw(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_schema = 'public' 
			  AND table_name = 'opr_m_trayekh' 
			  AND column_name = 'trh_aktifyn'
		)
	`).Scan(&hasAktifCol)

	if hasAktifCol {
		query = query.Where("trh_aktifyn = 'Y'")
	}

	// 2. Filter nama jika ada input pencarian
	if searchName != "" {
		query = query.Where("trh_name ILIKE ?", "%"+searchName+"%")
	}

	// 3. Eksekusi query
	err := query.Order("trh_id DESC").Find(&trayek).Error
	if err != nil {
		log.Println("❌ ERROR GetTrayekList:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memuat jalur rute: " + err.Error()})
		return
	}

	if trayek == nil {
		trayek = []models.OprMTrayekH{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": trayek})
}

// ➕ 2. API POST: Buat Jalur Rute Baru (Header)
func CreateTrayek(c *gin.Context) {
	var input models.OprMTrayekH
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload input tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	// Generate ID Otomatis berbasis Kode Logistik Dakota
	input.TrhID = fmt.Sprintf("TR%s%d", time.Now().Format("200601"), time.Now().Unix()%10000)
	input.TrhAktifYN = "Y"

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)
	input.TrhUpdateID = &userStr

	if err := database.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal mendaftarkan kode rute baru bray"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Jalur rute baru berhasil dikunci!", "trh_id": input.TrhID})
}

// 📝 3. API PUT: Perbarui Angka Konsumsi BBM & Tarif Dasar Rute
func UpdateTrayek(c *gin.Context) {
	id := c.Param("id")
	var input models.OprMTrayekH
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload update tidak valid"})
		return
	}

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)

	err := database.Model(&models.OprMTrayekH{}).Where("trh_id = ?", id).Updates(map[string]interface{}{
		"trh_name":        input.TrhName,
		"trh_jns_kend":    input.TrhJnsKend,
		"trh_tarif_b":     input.TrhTarifB,
		"trh_tarif_p":     input.TrhTarifP,
		"trh_bbm_ratio":   input.TrhBbmRatio,
		"trh_bbm_jatah":   input.TrhBbmJatah,
		"trh_total_km":    input.TrhTotalKm,
		"trh_update_id":   userStr,
		"trh_update_time": time.Now(),
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal update master rute"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Spesifikasi performa trayek berhasil disimpan!"})
}

// 🗺️ 4. API GET: Ambil Sub-Detail 1 - Rute Singgah Kota
func GetTrayekDetailRute(c *gin.Context) {
	id := c.Param("id")
	var result []models.OprMTrayekD

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	err := database.Table("opr_m_trayek_d AS d").
		Select("d.*, a.agen_nama").
		Joins("LEFT JOIN glb_m_agen a ON d.trd_agen_id = a.agen_id").
		Where("d.trd_hid = ?", id).
		Order("d.trd_status ASC, d.trd_urut ASC").
		Find(&result).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memuat detail kota singgah"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": result})
}

// 💰 5. API GET: Ambil Sub-Detail 2 - Jatah Biaya Operasional (BPLK)
func GetTrayekDetailBplk(c *gin.Context) {
	id := c.Param("id")
	var bplk []models.OprMTrayekBplk

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	database.Where("tri_hid = ?", id).Find(&bplk)
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": bplk})
}

// ⛽ 6. API GET: Ambil Sub-Detail 3 - Alokasi Voucher SPBU
func GetTrayekDetailVoucher(c *gin.Context) {
	id := c.Param("id")
	var voucher []models.OprMTrayekVoucher

	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	database.Where("trv_hid = ?", id).Find(&voucher)
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": voucher})
}
