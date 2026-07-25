package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

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
	cabID := getCabID(c)
	var params []models.ConfigParam

	err := db.DB.WithContext(c.Request.Context()).
		Where("(set_cabid = ? OR set_cabid = '1') AND set_varname <> ?", cabID, "AGEN_ID").
		Order("set_description ASC, set_varname ASC").
		Find(&params).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengambil data konfigurasi: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   params,
	})
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

// 3. PUT /api/config/params (UPDATE PAKAI RAW SQL)
func UpdateParam(c *gin.Context) {
	var req models.UpdateParamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	username := c.GetString("username")
	if username == "" {
		username = "system"
	}
	cabID := getCabID(c)

	query := `
		UPDATE glb_m_param 
		SET set_description = $1, set_varvalue = $2, set_vartype = $3, set_updateid = $4, set_updatetime = $5
		WHERE set_varname = $6 AND (set_cabid = $7 OR set_cabid = '1')
	`

	err := db.DB.WithContext(c.Request.Context()).Exec(query,
		req.SetDescription,
		req.SetVarValue,
		req.SetVarType,
		username,
		time.Now(),
		req.SetVarName,
		cabID,
	).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal mengupdate database: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Parameter berhasil diperbarui",
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
