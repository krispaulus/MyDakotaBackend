package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type CustomerApprovalItem struct {
	CustID            string  `json:"cust_id"`
	CustName          string  `json:"cust_name"`
	CustAlamat1       string  `json:"cust_alamat1"`
	CustAlamat2       string  `json:"cust_alamat2"`
	CustKotaID        string  `json:"cust_kotaid"`
	KotaName          string  `json:"kota_name"`
	CustTelp1         string  `json:"cust_telp1"`
	CustTelp2         string  `json:"cust_telp2"`
	CustFax1          string  `json:"cust_fax1"`
	CustFax2          string  `json:"cust_fax2"`
	CustContactPerson string  `json:"cust_contact_person"`
	CustAgenID        string  `json:"cust_agenid"`
	AgenNama          string  `json:"agen_nama"`
	CustApproveYN     string  `json:"cust_approve_yn"`
	CustAktifYN       string  `json:"cust_aktif_yn"`
	CustVirtualAcc    string  `json:"cust_virtual_acc"`
	CustKreditYN      string  `json:"cust_kredit_yn"`
	CustKreditLimit   float64 `json:"cust_kredit_limit"`
	CustKreditAktif   float64 `json:"cust_kredit_aktif"`
	CabangID          string  `json:"cabang_id"`
	CounterID         string  `json:"counter_id"`
}

type SaveApprovalCustomerReq struct {
	CustID          string  `json:"cust_id" binding:"required"`
	CustApproveYN   string  `json:"cust_approve_yn" binding:"required"`
	CustKreditYN    string  `json:"cust_kredit_yn"`
	CustKreditLimit float64 `json:"cust_kredit_limit"`
	CustKreditAktif float64 `json:"cust_kredit_aktif"`
	CabangID        string  `json:"cabang_id"`
	CounterID       string  `json:"counter_id"`
	CustVirtualAcc  string  `json:"cust_virtual_acc"`
}

// 1. GET /api/gl/customers (Dropdown Opsi Customer)
func GetCustomerOptionsHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Koneksi database tidak tersedia"})
		return
	}

	type CustOption struct {
		CustID   string `json:"cust_id" gorm:"column:cust_id"`
		CustName string `json:"cust_name" gorm:"column:cust_name"`
	}

	var list []CustOption
	if err := database.Table("public.mkt_m_customer").
		Select("cust_id, cust_name").
		Where("COALESCE(cust_aktifyn, 'Y') = 'Y'").
		Order("cust_name ASC").
		Limit(1000).
		Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// 2. GET /api/piutang/approval-customer (List Antrean Approval)
func GetApprovalCustomerListHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	agenNama := strings.TrimSpace(c.Query("agen_nama"))
	aktifYN := strings.TrimSpace(c.Query("aktif_yn"))
	custName := strings.TrimSpace(c.Query("cust_name"))
	approveFilter := strings.TrimSpace(c.Query("approve_yn"))

	query := database.Table("public.mkt_m_customer c").
		Select(`
			c.cust_id,
			c.cust_name,
			COALESCE(c.cust_telp1, '') AS cust_telp1,
			COALESCE(c.cust_telp2, '') AS cust_telp2,
			COALESCE(c.cust_fax1, '') AS cust_fax1,
			COALESCE(c.cust_fax2, '') AS cust_fax2,
			COALESCE(c.cust_contactperson, '') AS cust_contact_person,
			COALESCE(c.cust_aktifyn, 'Y') AS cust_aktif_yn,
			COALESCE(c.cust_approveyn, 'N') AS cust_approve_yn,
			COALESCE(a.agen_nama, 'DLI PUSAT') AS agen_nama,
			COALESCE(k.kota_nama, c.cust_kotaid, '-') AS kota_name
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON (TRIM(a.agen_id::varchar) = TRIM(c.cust_agenid::varchar) OR a.agen_id::varchar = LPAD(LEFT(c.cust_id, 3), 3, '0'))").
		Joins("LEFT JOIN public.glb_m_kota k ON TRIM(k.kota_id::varchar) = TRIM(c.cust_kotaid::varchar)").
		Where("c.cust_id IS NOT NULL")

	// Filter Status Aktif
	if aktifYN != "" {
		query = query.Where("COALESCE(c.cust_aktifyn, 'Y') = ?", aktifYN)
	}

	// Filter Status Approve (N = antrean yang belum approve)
	if approveFilter == "Y" {
		query = query.Where("c.cust_approveyn = 'Y'")
	} else if approveFilter == "N" {
		query = query.Where("(c.cust_approveyn = 'N' OR c.cust_approveyn IS NULL OR c.cust_approveyn = '')")
	}

	// Filter Cabang
	if agenNama != "" && agenNama != "ALL" && !strings.Contains(strings.ToUpper(agenNama), "SEMUA") {
		if strings.Contains(strings.ToUpper(agenNama), "PUSAT") || strings.Contains(strings.ToUpper(agenNama), "HOLDING") {
			query = query.Where("(a.agen_nama ILIKE '%PUSAT%' OR a.agen_nama ILIKE '%HOLDING%' OR a.agen_id IN ('1', '001') OR LEFT(c.cust_id, 3) = '001')")
		} else {
			query = query.Where("(a.agen_nama ILIKE ? OR a.agen_id::varchar = ?)", "%"+agenNama+"%", agenNama)
		}
	}

	if custName != "" {
		query = query.Where("c.cust_name ILIKE ?", "%"+custName+"%")
	}

	var list []CustomerApprovalItem
	if err := query.Order("c.cust_id ASC").Limit(500).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}

// 3. GET /api/piutang/approval-customer/detail/:id
func GetApprovalCustomerDetailHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	custID := strings.TrimSpace(c.Param("id"))

	var item CustomerApprovalItem
	err := database.Table("public.mkt_m_customer c").
		Select(`
			c.cust_id,
			c.cust_name,
			COALESCE(c.cust_alamat1, '') AS cust_alamat1,
			COALESCE(c.cust_alamat2, '') AS cust_alamat2,
			COALESCE(c.cust_kotaid, '') AS cust_kotaid,
			COALESCE(c.cust_telp1, '') AS cust_telp1,
			COALESCE(c.cust_telp2, '') AS cust_telp2,
			COALESCE(c.cust_fax1, '') AS cust_fax1,
			COALESCE(c.cust_fax2, '') AS cust_fax2,
			COALESCE(c.cust_contactperson, '') AS cust_contact_person,
			COALESCE(c.cust_agenid, '') AS cust_agenid,
			COALESCE(c.cust_approveyn, 'N') AS cust_approve_yn,
			COALESCE(c.cust_aktifyn, 'Y') AS cust_aktif_yn,
			COALESCE(c.cust_virtualacc, '') AS cust_virtual_acc,
			COALESCE(c.cust_kredityn, 'N') AS cust_kredit_yn,
			COALESCE(c.cust_kreditlimit, 0) AS cust_kredit_limit,
			COALESCE(c.cust_kreditaktif, 0) AS cust_kredit_aktif,
			COALESCE(k.kota_nama, c.cust_kotaid, '-') AS kota_name,
			COALESCE(a.agen_nama, 'DLI PUSAT') AS agen_nama
		`).
		Joins("LEFT JOIN public.glb_m_agen a ON (TRIM(a.agen_id::varchar) = TRIM(c.cust_agenid::varchar) OR a.agen_id::varchar = LPAD(LEFT(c.cust_id, 3), 3, '0'))").
		Joins("LEFT JOIN public.glb_m_kota k ON TRIM(k.kota_id::varchar) = TRIM(c.cust_kotaid::varchar)").
		Where("TRIM(c.cust_id) = TRIM(?)", custID).
		Take(&item).Error

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Data customer tidak ditemukan: " + err.Error()})
		return
	}

	va := strings.TrimSpace(item.CustVirtualAcc)
	if len(va) >= 15 {
		item.CabangID = va[5:8]
		item.CounterID = va[8:11]
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   item,
	})
}

// 4. POST /api/piutang/approval-customer/save
func SaveApprovalCustomerHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	userID, _ := c.Get("username")

	var req SaveApprovalCustomerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	custID := strings.TrimSpace(req.CustID)

	// Hitung Virtual Account BCA: 01058 + 3 digit Cabang + 3 digit Counter + 4 digit Ekor CustID
	kdv := "01058"
	cbg3Digit := fmt.Sprintf("%03s", strings.TrimSpace(req.CabangID))
	if len(cbg3Digit) > 3 {
		cbg3Digit = cbg3Digit[len(cbg3Digit)-3:]
	}
	ctr3Digit := fmt.Sprintf("%03s", strings.TrimSpace(req.CounterID))
	if len(ctr3Digit) > 3 {
		ctr3Digit = ctr3Digit[len(ctr3Digit)-3:]
	}
	ekorCust := custID
	if len(ekorCust) > 4 {
		ekorCust = ekorCust[len(ekorCust)-4:]
	} else {
		ekorCust = fmt.Sprintf("%04s", ekorCust)
	}

	virtualAccFinal := fmt.Sprintf("%s%s%s%s", kdv, cbg3Digit, ctr3Digit, ekorCust)
	if strings.TrimSpace(req.CustVirtualAcc) != "" {
		virtualAccFinal = strings.TrimSpace(req.CustVirtualAcc)
	}

	updateData := map[string]interface{}{
		"cust_approveyn":   req.CustApproveYN,
		"cust_kredityn":    req.CustKreditYN,
		"cust_kreditlimit": req.CustKreditLimit,
		"cust_kreditaktif": req.CustKreditAktif,
		"cust_virtualacc":  virtualAccFinal,
		"cust_updateid":    fmt.Sprintf("%v", userID),
		"cust_updatetime":  time.Now(),
	}

	if err := database.Table("public.mkt_m_customer").Where("TRIM(cust_id) = TRIM(?)", custID).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan approval customer: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"message":     fmt.Sprintf("Customer %s berhasil diperbarui / diapprove!", custID),
		"virtual_acc": virtualAccFinal,
	})
}

// 5. DELETE /api/piutang/approval-customer/:id (Hapus / Nonaktifkan Customer)
func DeleteApprovalCustomerHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}
	custID := strings.TrimSpace(c.Param("id"))
	userID, _ := c.Get("username")

	updateData := map[string]interface{}{
		"cust_aktifyn":    "N",
		"cust_updateid":   fmt.Sprintf("%v", userID),
		"cust_updatetime": time.Now(),
	}

	if err := database.Table("public.mkt_m_customer").
		Where("TRIM(cust_id) = TRIM(?)", custID).
		Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Gagal menghapus customer: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Customer %s berhasil dihapus / dinonaktifkan.", custID),
	})
}

// GET /api/marketing/cities
func GetKotaOptionsHandler(c *gin.Context) {
	database := getJurnalDB(c)
	if database == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database corporate tidak terhubung"})
		return
	}

	type KotaOption struct {
		KotaID   string `json:"kota_id" gorm:"column:kota_id"`
		KotaName string `json:"kota_name" gorm:"column:kota_name"`
	}

	var list []KotaOption
	if err := database.Table("public.glb_m_kota").
		Select("kota_id, COALESCE(kota_nama, kota_id) AS kota_name").
		Where("COALESCE(kota_aktifyn, 'Y') = 'Y'").
		Order("kota_name ASC").
		Limit(1000).
		Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}
