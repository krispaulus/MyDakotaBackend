package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type FormHRDRow struct {
	FormID         int64     `json:"form_id" gorm:"column:form_id"`
	FormNama       string    `json:"form_nama" gorm:"column:form_nama"`
	FormKategori   string    `json:"form_kategori" gorm:"column:form_kategori"`
	FormFilename   string    `json:"form_filename" gorm:"column:form_filename"`
	FormFilesize   int64     `json:"form_filesize" gorm:"column:form_filesize"`
	FormKeterangan string    `json:"form_keterangan" gorm:"column:form_keterangan"`
	FormAktifYN    string    `json:"form_aktifyn" gorm:"column:form_aktifyn"`
	FormCreateID   string    `json:"form_createid" gorm:"column:form_createid"`
	FormCreateTime time.Time `json:"form_createtime" gorm:"column:form_createtime"`
}

// GET /hrd/form/list
func GetListFormHRD(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	search := strings.TrimSpace(c.Query("search"))
	kategori := strings.TrimSpace(c.Query("kategori"))

	query := database.Table("public.hrd_m_form").
		Select("form_id, form_nama, form_kategori, form_filename, form_filesize, form_keterangan, form_aktifyn, form_createid, form_createtime").
		Where("COALESCE(form_aktifyn, 'Y') = 'Y'")

	if search != "" {
		s := "%" + search + "%"
		query = query.Where("(form_nama ILIKE ? OR form_filename ILIKE ? OR form_keterangan ILIKE ?)", s, s, s)
	}

	if kategori != "" && kategori != "ALL" {
		query = query.Where("form_kategori = ?", kategori)
	}

	var results []FormHRDRow
	if err := query.Order("form_nama ASC").Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if results == nil {
		results = []FormHRDRow{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// POST /hrd/form/upload (Admin / Super Admin)
func UploadFormHRD(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	username, _ := c.Get("username")
	formNama := strings.TrimSpace(c.PostForm("form_nama"))
	formKategori := strings.TrimSpace(c.PostForm("form_kategori"))
	formKeterangan := strings.TrimSpace(c.PostForm("form_keterangan"))

	if formNama == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nama form wajib diisi"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Berkas file harus diunggah"})
		return
	}

	// Direktori penyimpanan file
	uploadDir := "./uploads/form_hrd"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal membuat direktori upload"})
		return
	}

	// Buat nama file unik
	ext := filepath.Ext(file.Filename)
	uniqueFileName := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), strings.ReplaceAll(filepath.Base(file.Filename[:len(file.Filename)-len(ext)]), " ", "_"), ext)
	dstPath := filepath.Join(uploadDir, uniqueFileName)

	if err := c.SaveUploadedFile(file, dstPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan file ke server"})
		return
	}

	if formKategori == "" {
		formKategori = "UMUM"
	}

	newRecord := map[string]interface{}{
		"form_nama":       formNama,
		"form_kategori":   formKategori,
		"form_filename":   file.Filename,
		"form_filesize":   file.Size,
		"form_filepath":   dstPath,
		"form_keterangan": formKeterangan,
		"form_aktifyn":    "Y",
		"form_createid":   fmt.Sprintf("%v", username),
		"form_createtime": time.Now(),
		"form_updateid":   fmt.Sprintf("%v", username),
		"form_updatetime": time.Now(),
	}

	if err := database.Table("public.hrd_m_form").Create(&newRecord).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan ke database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Form HRD berhasil diunggah"})
}

// GET /hrd/form/download/:id (Semua User)
func DownloadFormHRD(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	id := c.Param("id")
	var row struct {
		FormFilename string `gorm:"column:form_filename"`
		FormFilepath string `gorm:"column:form_filepath"`
	}

	if err := database.Table("public.hrd_m_form").Select("form_filename, form_filepath").Where("form_id = ? AND form_aktifyn = 'Y'", id).Take(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Form tidak ditemukan"})
		return
	}

	if _, err := os.Stat(row.FormFilepath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "File fisik tidak ada di server"})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", row.FormFilename))
	c.File(row.FormFilepath)
}

// DELETE /hrd/form/delete/:id (Admin / Super Admin)
func DeleteFormHRD(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	id := c.Param("id")
	username, _ := c.Get("username")

	if err := database.Table("public.hrd_m_form").
		Where("form_id = ?", id).
		Updates(map[string]interface{}{
			"form_aktifyn":    "N",
			"form_updateid":   fmt.Sprintf("%v", username),
			"form_updatetime": time.Now(),
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus form"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Form berhasil dihapus"})
}

// GET /hrd/form/view/:id (Semua Role - Preview Dokumen)
func ViewFormHRD(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	id := c.Param("id")
	var row struct {
		FormFilename string `gorm:"column:form_filename"`
		FormFilepath string `gorm:"column:form_filepath"`
	}

	if err := database.Table("public.hrd_m_form").Select("form_filename, form_filepath").Where("form_id = ? AND form_aktifyn = 'Y'", id).Take(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Form tidak ditemukan"})
		return
	}

	if _, err := os.Stat(row.FormFilepath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "File fisik tidak ada di server"})
		return
	}

	ext := strings.ToLower(filepath.Ext(row.FormFilename))
	switch ext {
	case ".pdf":
		c.Header("Content-Type", "application/pdf")
	case ".jpg", ".jpeg":
		c.Header("Content-Type", "image/jpeg")
	case ".png":
		c.Header("Content-Type", "image/png")
	default:
		c.Header("Content-Type", "application/octet-stream")
	}

	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", row.FormFilename))
	c.File(row.FormFilepath)
}
