package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func getConfigDB(c *gin.Context) *gorm.DB {
	database := getJurnalDB(c)
	if database != nil {
		return database
	}
	return db.DB
}

func getCabID(c *gin.Context) string {
	cabID := c.GetString("server_id")
	if cabID == "" {
		cabID = c.GetString("kode_cabang")
	}
	if cabID == "" {
		cabID = c.Query("server_id")
	}
	if cabID == "" {
		cabID = "1"
	}
	return cabID
}

// 1. GET /api/config/params
func GetParams(c *gin.Context) {
	database := getConfigDB(c)
	cabID := getCabID(c)

	type ConfigParamRow struct {
		SetCabID       string     `json:"set_cabid" gorm:"column:set_cabid"`
		SetVarName     string     `json:"set_varname" gorm:"column:set_varname"`
		SetDescription string     `json:"set_description" gorm:"column:set_description"`
		SetVarValue    string     `json:"set_varvalue" gorm:"column:set_varvalue"`
		SetVarType     string     `json:"set_vartype" gorm:"column:set_vartype"`
		SetUpdateID    string     `json:"set_updateid" gorm:"column:set_updateid"`
		SetUpdateTime  *time.Time `json:"set_updatetime" gorm:"column:set_updatetime"`
	}

	var params []ConfigParamRow
	search := strings.TrimSpace(c.Query("search"))
	varType := strings.TrimSpace(c.Query("vartype"))

	tx := database.Table("public.glb_m_param").
		Where("(set_cabid = ? OR set_cabid = '1') AND set_varname <> 'AGEN_ID'", cabID)

	if search != "" {
		tx = tx.Where("(set_varname ILIKE ? OR set_description ILIKE ?)", "%"+search+"%", "%"+search+"%")
	}
	if varType != "" {
		tx = tx.Where("set_vartype = ?", varType)
	}

	err := tx.Order("set_description ASC, set_varname ASC").Find(&params).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengambil data konfigurasi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": params})
}

// 2. POST /api/config/params (CREATE PAKAI RAW SQL - BANTING RETURNING ID)
func CreateParam(c *gin.Context) {
	var req models.ConfigParam
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if req.SetCabID == "" {
		req.SetCabID = getCabID(c)
	}
	username := c.GetString("username")
	if username == "" {
		username = "system"
	}

	query := `
		INSERT INTO glb_m_param (set_cabid, set_varname, set_description, set_varvalue, set_vartype, set_updateid, set_updatetime)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	err := db.DB.WithContext(c.Request.Context()).Exec(query,
		req.SetCabID,
		req.SetVarName,
		req.SetDescription,
		req.SetVarValue,
		req.SetVarType,
		username,
		time.Now(),
	).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menyimpan ke database: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Parameter berhasil ditambahkan",
	})
}

// 3. PUT /api/config/params (UPDATE FIX LANGSUNG KE NAMA VARIABEL)
func UpdateParam(c *gin.Context) {
	database := getConfigDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tidak terhubung"})
		return
	}

	var req struct {
		SetCabID       string `json:"set_cabid"`
		SetVarName     string `json:"set_varname" binding:"required"`
		SetDescription string `json:"set_description"`
		SetVarValue    string `json:"set_varvalue"`
		SetVarType     string `json:"set_vartype"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Data tidak valid: " + err.Error()})
		return
	}

	username := c.GetString("username")
	if username == "" {
		username = "superdli"
	}

	varName := strings.TrimSpace(req.SetVarName)
	varValue := strings.TrimSpace(req.SetVarValue)
	cabID := strings.TrimSpace(req.SetCabID)
	if cabID == "" {
		cabID = getCabID(c)
	}

	// Update langsung di target database
	query := `
		UPDATE public.glb_m_param 
		SET set_varvalue = $1, 
		    set_description = $2, 
		    set_vartype = $3, 
		    set_updateid = $4, 
		    set_updatetime = NOW()
		WHERE TRIM(LOWER(set_varname)) = TRIM(LOWER($5))
	`

	res := database.Exec(query,
		varValue,
		strings.TrimSpace(req.SetDescription),
		strings.TrimSpace(req.SetVarType),
		username,
		varName,
	)

	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal mengupdate database: " + res.Error.Error()})
		return
	}

	// Jika baris belum ada di database, lakukan insert
	if res.RowsAffected == 0 {
		insertQuery := `
			INSERT INTO public.glb_m_param (set_cabid, set_varname, set_description, set_varvalue, set_vartype, set_updateid, set_updatetime)
			VALUES ($1, $2, $3, $4, $5, $6, NOW())
		`
		if err := database.Exec(insertQuery,
			cabID,
			varName,
			strings.TrimSpace(req.SetDescription),
			varValue,
			strings.TrimSpace(req.SetVarType),
			username,
		).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal insert parameter baru: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"message":       fmt.Sprintf("Parameter %s berhasil diperbarui", varName),
		"rows_affected": res.RowsAffected,
		"var_value":     varValue,
	})
}

// 4. DELETE /api/config/params/:varname
func DeleteParam(c *gin.Context) {
	varName := c.Param("id")
	cabID := getCabID(c)

	query := `DELETE FROM glb_m_param WHERE set_varname = $1 AND (set_cabid = $2 OR set_cabid = '1')`

	err := db.DB.WithContext(c.Request.Context()).Exec(query, varName, cabID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghapus parameter: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Parameter berhasil dihapus",
	})
}
