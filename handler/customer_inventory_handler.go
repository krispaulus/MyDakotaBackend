package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"

	"github.com/gin-gonic/gin"
)

// GetListCustomerInventory Handler untuk Menarik Daftar Inventory Barang Customer
func GetListCustomerInventory(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	custID := strings.TrimSpace(c.Query("cust_id"))
	itemID := strings.TrimSpace(c.Query("item_id"))
	chkTgl := strings.TrimSpace(c.Query("chktgl"))

	filterClause := ""

	if (chkTgl == "true" || chkTgl == "on") && tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(inv_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	if custID != "" {
		filterClause += fmt.Sprintf(` AND (UPPER(inv_custid) LIKE UPPER('%%%s%%') OR UPPER(inv_custname) LIKE UPPER('%%%s%%')) `, custID, custID)
	}

	if itemID != "" {
		filterClause += fmt.Sprintf(` AND (UPPER(inv_itemid) LIKE UPPER('%%%s%%') OR UPPER(inv_itemname) LIKE UPPER('%%%s%%')) `, itemID, itemID)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			inv_id,
			TO_CHAR(inv_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS inv_tanggal,
			COALESCE(inv_custid, '-') AS inv_custid,
			COALESCE(inv_custname, '-') AS inv_custname,
			COALESCE(inv_itemid, '-') AS inv_itemid,
			COALESCE(inv_itemname, '-') AS inv_itemname,
			COALESCE(inv_qty, 0) AS inv_qty,
			COALESCE(inv_supid, '-') AS inv_supid,
			COALESCE(inv_supname, '-') AS inv_supname,
			COALESCE(inv_suratjalan, '-') AS inv_suratjalan,
			COALESCE(inv_updateid, '-') AS inv_updateid
		FROM public.opr_t_customer_inventory
		WHERE 1=1 %s
		ORDER BY inv_tanggal DESC, inv_id DESC
	`, filterClause)

	type RawResult struct {
		InvID         int64  `gorm:"column:inv_id"`
		InvTanggal    string `gorm:"column:inv_tanggal"`
		InvCustID     string `gorm:"column:inv_custid"`
		InvCustName   string `gorm:"column:inv_custname"`
		InvItemID     string `gorm:"column:inv_itemid"`
		InvItemName   string `gorm:"column:inv_itemname"`
		InvQty        int    `gorm:"column:inv_qty"`
		InvSupID      string `gorm:"column:inv_supid"`
		InvSupName    string `gorm:"column:inv_supname"`
		InvSuratJalan string `gorm:"column:inv_suratjalan"`
		InvUpdateID   string `gorm:"column:inv_updateid"`
	}

	var rawList []RawResult
	if err := database.Raw(queryStr).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query inventory customer: " + err.Error()})
		return
	}

	var resultList []models.CustomerInventoryDTO
	for _, r := range rawList {
		resultList = append(resultList, models.CustomerInventoryDTO{
			InvID:         r.InvID,
			InvTanggal:    r.InvTanggal,
			InvCustID:     r.InvCustID,
			InvCustName:   r.InvCustName,
			InvItemID:     r.InvItemID,
			InvItemName:   r.InvItemName,
			InvQty:        r.InvQty,
			InvSupID:      r.InvSupID,
			InvSupName:    r.InvSupName,
			InvSuratJalan: r.InvSuratJalan,
			InvUpdateID:   r.InvUpdateID,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resultList,
		"count":  len(resultList),
	})
}

// CreateCustomerInventory Handler Tambah Inventory Barang Customer
func CreateCustomerInventory(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.CustomerInventoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	insertSQL := `
		INSERT INTO public.opr_t_customer_inventory (
			inv_tanggal, inv_custid, inv_custname, inv_itemid, inv_itemname, inv_qty, inv_supid, inv_supname, inv_suratjalan, inv_updateid
		) VALUES (NOW(), ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if err := database.Exec(insertSQL, req.InvCustID, req.InvCustName, req.InvItemID, req.InvItemName, req.InvQty, req.InvSupID, req.InvSupName, req.InvSuratJalan, fmt.Sprintf("%v", username)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan inventory customer: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data Inventory Barang Customer berhasil ditambahkan"})
}

// UpdateCustomerInventory Handler Update Inventory Barang Customer
func UpdateCustomerInventory(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.CustomerInventoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	updateSQL := `
		UPDATE public.opr_t_customer_inventory 
		SET inv_custid = ?, inv_custname = ?, inv_itemid = ?, inv_itemname = ?, inv_qty = ?, inv_supid = ?, inv_supname = ?, inv_suratjalan = ?, inv_updateid = ?, inv_updatetime = NOW() 
		WHERE inv_id = ?
	`
	if err := database.Exec(updateSQL, req.InvCustID, req.InvCustName, req.InvItemID, req.InvItemName, req.InvQty, req.InvSupID, req.InvSupName, req.InvSuratJalan, fmt.Sprintf("%v", username), req.InvID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui inventory customer: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data Inventory Barang Customer berhasil diperbarui"})
}

// DeleteCustomerInventory Handler Hapus Record Inventory Customer
func DeleteCustomerInventory(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	invIDStr := c.Query("inv_id")
	invID, err := strconv.ParseInt(invIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID Inventory tidak valid"})
		return
	}

	deleteSQL := `DELETE FROM public.opr_t_customer_inventory WHERE inv_id = ?`
	if err := database.Exec(deleteSQL, invID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus record inventory: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Record Inventory Barang Customer berhasil dihapus"})
}
