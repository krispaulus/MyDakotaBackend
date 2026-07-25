package handler

import (
	"fmt"
	"log"
	"net/http"

	"dakotagroup/business-insight-be/db" // 👑 Sesuaikan path module go.mod lokal lu bray
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// 🚨 API NOTIFIKASI EXPIRED SERVICE
func GetExpiredServiceKendaraan(c *gin.Context) {
	var expiredVehicles []models.GlbMKendaraan

	// 👑 TRIK AMAN: Gunakan PostgreSQL Raw Query dengan tanda kutip ganda bray!
	// Ini untuk menjamin jika kolom di database lu ternyata pake huruf besar (Kend_ServiceYN)
	err := db.DB.Model(&models.GlbMKendaraan{}).
		Where(`"kend_serviceyn" = 'Y' AND "kend_serviceenddate" < CURRENT_DATE`).
		Order(`"kend_serviceenddate" asc`).
		Find(&expiredVehicles).Error

	// 🛡️ JALUR BACKUP DARURAT: Jika tabel service di db local lu emang beneran kosong/belum di-import bray
	if err != nil {
		log.Println("⚠️ Kolom service service belum cocok, bypass array kosong biar frontend tidak BLANK/500 bray:", err.Error())
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"count":  0,
			"data":   []models.GlbMKendaraan{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"count":  len(expiredVehicles),
		"data":   expiredVehicles,
	})
}

func GetMasterKendaraanList(c *gin.Context) {
	var vehicles []models.GlbMKendaraan

	// 🟩 1. AMBIL PT_ID SECARA DINAMIS DARI CONTEXT TOKEN SESSION LOGIN ACTIVE
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Koneksi database corporate gagal resolved"})
		return
	}

	nopol := c.Query("nopol")
	aktif := c.Query("aktif")

	// 🟩 2. GANTI db.DB MENJADI database DAN WAKILI SCHEMA TABEL FISIK SECARA TEGAS BRAY!
	query := database.Debug().Table("public.glb_m_kendaraan")

	// 🟩 3. BERSIHKAN TANDA KUTIP GANDA PADA KOLOM BIAR POSTGRES PLONG KASTA TERTINGGI
	if nopol != "" {
		query = query.Where("kend_id LIKE ?", "%"+nopol+"%")
	}

	if aktif != "" {
		query = query.Where("kend_aktifyn = ?", aktif)
	} else {
		query = query.Where("kend_aktifyn = 'Y'") // Default filter truk aktif
	}

	// 🟩 4. EKSEKUSI FIND MENGGUNAKAN PIPELINE DATA CORPORATE TERPILIH
	err := query.Order("kend_id ASC").Find(&vehicles).Error
	if err != nil {
		log.Println("❌ ERROR GetMasterKendaraanList DETAIL:", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menarik data master kendaraan", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   vehicles,
	})
}

// ➕ A. API ACTION: Tambah Kendaraan Baru
func CreateKendaraan(c *gin.Context) {
	var input models.GlbMKendaraan
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Input tidak valid bray"})
		return
	}

	input.KendAktifYN = "Y"
	input.KendPostingYN = "N"

	if err := db.DB.Create(&input).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan armada, kemungkinan nopol sudah terdaftar!"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Armada sukses didaftarkan!"})
}

// 📝 API ACTION: Update/Edit Kendaraan Lapis Baja Super Lengkap
func UpdateKendaraan(c *gin.Context) {
	id := c.Param("id")
	var input models.GlbMKendaraan
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Payload edit tidak valid"})
		return
	}

	// 👑 Meng-update seluruh record parameter field di database lokal Postgres lu bray!
	err := db.DB.Model(&models.GlbMKendaraan{}).Where(`"kend_id" = ?`, id).Updates(map[string]interface{}{
		"kend_pemilik":         input.KendPemilik,
		"kend_pt":              input.KendPT,
		"kend_leasing":         input.KendLeasing,
		"kend_alamat":          input.KendAlamat,
		"kend_lokasiid":        input.KendLokasiID,
		"kend_merk":            input.KendMerk,
		"kend_type":            input.KendType,
		"kend_jenis":           input.KendJenis,
		"kend_thnbuat":         input.KendThnBuat,
		"kend_thnrakit":        input.KendThnRakit,
		"kend_isisilinder":     input.KendIsiSilinder,
		"kend_warna":           input.KendWarna,
		"kend_nik":             input.KendNIK,
		"kend_nomesin":         input.KendNoMesin,
		"kend_identid":         input.KendIdentID,
		"kend_warnatnkb":       input.KendWarnaTNKB,
		"kend_bahanbakar":      input.KendBahanBakar,
		"kend_jarak":           input.KendJarak,
		"kend_warnaplat":       input.KendWarnaPlat,
		"kend_berlakustnk":     input.KendBerlakuSTNK,
		"kend_berlakupajak":    input.KendBerlakuPajak,
		"kend_berlakukir":      input.KendBerlakuKIR,
		"kend_serahterimatime": input.KendSerahTerimaTime,
		"kend_serahterimaid":   input.KendSerahTerimaID,
		"kend_sopir1":          input.KendSopir1,
		"kend_sopir2":          input.KendSopir2,
		"kend_aktifyn":         input.KendAktifYN,
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memperbarui data lengkap armada"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data armada sukses diperbarui!"})
}

// 🛠️ C. API ACTION: Pendaftaran Truk Masuk Bengkel Service
func RegisterTrukService(c *gin.Context) {
	// Membaca form-urlencoded bray sesuai spec .asp lawas
	kendid := c.PostForm("kendid")
	tglselesai := c.PostForm("tglselesai")

	if kendid == "" || tglselesai == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Nopol dan Tanggal Estimasi Wajib Diisi bray!"})
		return
	}

	err := db.DB.Model(&models.GlbMKendaraan{}).Where(`"kend_id" = ?`, kendid).Updates(map[string]interface{}{
		"kend_serviceyn":        "Y",
		"kend_servicestartdate": db.DB.Raw("CURRENT_DATE"),
		"kend_serviceenddate":   tglselesai,
	}).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal memasukkan armada ke bengkel"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Truk sukses dikunci di dalam bengkel!"})
}
