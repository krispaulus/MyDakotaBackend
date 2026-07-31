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

// GetListCustomerInventoryOut Handler untuk Menarik Daftar Pengeluaran Inventory Customer
func GetListCustomerInventoryOut(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	tgla := strings.TrimSpace(c.Query("tgla"))
	tgle := strings.TrimSpace(c.Query("tgle"))
	custID := strings.TrimSpace(c.Query("cust_id"))
	itemID := strings.TrimSpace(c.Query("item_id"))
	noBtt := strings.TrimSpace(c.Query("no_btt"))
	chkTgl := strings.TrimSpace(c.Query("chktgl"))

	filterClause := ""

	if (chkTgl == "true" || chkTgl == "on") && tgla != "" && tgle != "" {
		filterClause += fmt.Sprintf(` AND CAST(out_tanggal AS DATE) BETWEEN '%s' AND '%s' `, tgla, tgle)
	}

	if custID != "" {
		filterClause += fmt.Sprintf(` AND (UPPER(out_custid) LIKE UPPER('%%%s%%') OR UPPER(out_custname) LIKE UPPER('%%%s%%')) `, custID, custID)
	}

	if itemID != "" {
		filterClause += fmt.Sprintf(` AND (UPPER(out_itemid) LIKE UPPER('%%%s%%') OR UPPER(out_itemname) LIKE UPPER('%%%s%%')) `, itemID, itemID)
	}

	if noBtt != "" {
		filterClause += fmt.Sprintf(` AND UPPER(out_nobtt) LIKE UPPER('%%%s%%') `, noBtt)
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			out_id,
			TO_CHAR(out_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS out_tanggal,
			COALESCE(out_custid, '-') AS out_custid,
			COALESCE(out_custname, '-') AS out_custname,
			COALESCE(out_itemid, '-') AS out_itemid,
			COALESCE(out_itemname, '-') AS out_itemname,
			COALESCE(out_qty, 0) AS out_qty,
			COALESCE(out_nobtt, '-') AS out_nobtt,
			COALESCE(out_updateid, '-') AS out_updateid
		FROM public.opr_t_customer_inventory_out
		WHERE 1=1 %s
		ORDER BY out_tanggal DESC, out_id DESC
	`, filterClause)

	type RawResult struct {
		OutID       int64  `gorm:"column:out_id"`
		OutTanggal  string `gorm:"column:out_tanggal"`
		OutCustID   string `gorm:"column:out_custid"`
		OutCustName string `gorm:"column:out_custname"`
		OutItemID   string `gorm:"column:out_itemid"`
		OutItemName string `gorm:"column:out_itemname"`
		OutQty      int    `gorm:"column:out_qty"`
		OutNoBTT    string `gorm:"column:out_nobtt"`
		OutUpdateID string `gorm:"column:out_updateid"`
	}

	var rawList []RawResult
	if err := database.Raw(queryStr).Scan(&rawList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query pengeluaran inventory customer: " + err.Error()})
		return
	}

	var resultList []models.CustomerInventoryOutDTO
	for _, r := range rawList {
		resultList = append(resultList, models.CustomerInventoryOutDTO{
			OutID:       r.OutID,
			OutTanggal:  r.OutTanggal,
			OutCustID:   r.OutCustID,
			OutCustName: r.OutCustName,
			OutItemID:   r.OutItemID,
			OutItemName: r.OutItemName,
			OutQty:      r.OutQty,
			OutNoBTT:    r.OutNoBTT,
			OutUpdateID: r.OutUpdateID,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resultList,
		"count":  len(resultList),
	})
}

// CreateCustomerInventoryOut Handler Tambah Pengeluaran Inventory Customer
func CreateCustomerInventoryOut(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.CustomerInventoryOutReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	insertSQL := `
		INSERT INTO public.opr_t_customer_inventory_out (
			out_tanggal, out_custid, out_custname, out_itemid, out_itemname, out_qty, out_nobtt, out_updateid
		) VALUES (NOW(), ?, ?, ?, ?, ?, ?, ?)
	`
	if err := database.Exec(insertSQL, req.OutCustID, req.OutCustName, req.OutItemID, req.OutItemName, req.OutQty, req.OutNoBTT, fmt.Sprintf("%v", username)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan pengeluaran inventory customer: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Pengeluaran Inventory Barang Customer berhasil ditambahkan"})
}

// UpdateCustomerInventoryOut Handler Update Pengeluaran Inventory Customer
func UpdateCustomerInventoryOut(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	username, _ := c.Get("username")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	var req models.CustomerInventoryOutReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid: " + err.Error()})
		return
	}

	updateSQL := `
		UPDATE public.opr_t_customer_inventory_out 
		SET out_custid = ?, out_custname = ?, out_itemid = ?, out_itemname = ?, out_qty = ?, out_nobtt = ?, out_updateid = ?, out_updatetime = NOW() 
		WHERE out_id = ?
	`
	if err := database.Exec(updateSQL, req.OutCustID, req.OutCustName, req.OutItemID, req.OutItemName, req.OutQty, req.OutNoBTT, fmt.Sprintf("%v", username), req.OutID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memperbarui pengeluaran inventory customer: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Pengeluaran Inventory Barang Customer berhasil diperbarui"})
}

// DeleteCustomerInventoryOut Handler Hapus Record Pengeluaran Inventory Customer
func DeleteCustomerInventoryOut(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, _ := db.ResolveDB(fmt.Sprintf("%v", ptID))

	outIDStr := c.Query("out_id")
	outID, err := strconv.ParseInt(outIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID Pengeluaran Inventory tidak valid"})
		return
	}

	deleteSQL := `DELETE FROM public.opr_t_customer_inventory_out WHERE out_id = ?`
	if err := database.Exec(deleteSQL, outID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghapus record pengeluaran inventory: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Record Pengeluaran Inventory Barang Customer berhasil dihapus"})
}
