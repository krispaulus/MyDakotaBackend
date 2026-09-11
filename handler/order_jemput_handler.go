package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
)

// DTO Response Order Jemput Item
type OrderJemputItem struct {
	OrderID       string  `gorm:"column:order_id" json:"order_id"`
	CabangNama    string  `gorm:"column:agen_nama" json:"agen_nama"`
	OrderDate     string  `gorm:"column:order_date" json:"order_date"`
	CustName      string  `gorm:"column:cust_name" json:"cust_name"`
	CustAlamat    string  `gorm:"column:cust_alamat" json:"cust_alamat"`
	CustTelp      string  `gorm:"column:cust_telp" json:"cust_telp"`
	OrderNote     string  `gorm:"column:order_note" json:"order_note"`
	OrderKoli     float64 `gorm:"column:order_koli" json:"order_koli"`
	OrderBerat    float64 `gorm:"column:order_berat" json:"order_berat"`
	OrderVolume   float64 `gorm:"column:order_volume" json:"order_volume"`
	OrderAmount   float64 `gorm:"column:order_amount" json:"order_amount"`
	OrderSupir    string  `gorm:"column:order_supir" json:"order_supir"`
	OrderNoPol    string  `gorm:"column:order_nopol" json:"order_nopol"`
	NoReceipt     string  `gorm:"column:no_receipt" json:"no_receipt"`
	OrderTJurHNo  string  `gorm:"column:order_tjurhno" json:"order_tjurhno"`
	OrderActiveYN string  `gorm:"column:order_activeyn" json:"order_activeyn"`
	OrderPaidYN   string  `gorm:"column:order_paidyn" json:"order_paidyn"`
}

// Request Payload Save Order Jemput
type OrderJemputRequest struct {
	OrderID     string  `json:"order_id"`
	OrderDate   string  `json:"order_date"`
	OrderCustID string  `json:"order_custid"`
	OrderName   string  `json:"order_name" binding:"required"`
	OrderNote   string  `json:"order_note"`
	OrderKoli   int     `json:"order_koli"`
	OrderBerat  float64 `json:"order_berat"`
	OrderVolume float64 `json:"order_volume"`
	OrderAmount float64 `json:"order_amount"`
	OrderSupir  string  `json:"order_supir"`
	OrderNoPol  string  `json:"order_nopol"`
	OrderPaidYN string  `json:"order_paidyn"`
}

// 1. GET /api/marketing/order-jemput/data
func GetOrderJemputList(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database resolver gagal"})
		return
	}

	useTanggal := c.Query("use_tanggal") == "true"
	tglAwal := strings.TrimSpace(c.Query("start_date"))
	tglAkhir := strings.TrimSpace(c.Query("end_date"))
	cabang := strings.TrimSpace(c.Query("cabang"))
	customer := strings.TrimSpace(c.Query("customer"))
	noOrder := strings.TrimSpace(c.Query("no_order"))
	noReceipt := strings.TrimSpace(c.Query("no_receipt"))
	aktif := strings.TrimSpace(c.Query("aktif"))

	whereOj := "WHERE 1=1"
	var args []interface{}

	if aktif != "" && !strings.EqualFold(aktif, "SEMUA") {
		whereOj += " AND COALESCE(NULLIF(TRIM(oj.order_activeyn), ''), 'Y') = ?"
		args = append(args, strings.ToUpper(strings.TrimSpace(aktif)))
	}

	if useTanggal && tglAwal != "" && tglAkhir != "" {
		whereOj += " AND oj.order_date BETWEEN ? AND ?"
		args = append(args, tglAwal+" 00:00:00", tglAkhir+" 23:59:59")
	}

	if noOrder != "" {
		whereOj += " AND oj.order_id ILIKE ?"
		args = append(args, "%"+noOrder+"%")
	}

	// 2. Gunakan CTE dan SUBSTRING skalar untuk mengekstrak angka ID agen secara aman
	query := fmt.Sprintf(`
		WITH target_orders AS (
			SELECT 
				oj.order_id,
				oj.order_date,
				oj.order_custid,
				oj.order_name,
				oj.order_note,
				oj.order_koli,
				oj.order_berat,
				oj.order_volume,
				oj.order_amount,
				oj.order_supir,
				oj.order_nopol,
				oj.order_tjurhno,
				oj.order_activeyn,
				oj.order_paidyn,
				COALESCE(NULLIF(SUBSTRING(oj.order_id FROM '^[^\d]*(\d{1,3})'), '')::integer, 0) AS urutan_kode,
				COALESCE(NULLIF(REGEXP_REPLACE(cust_ref.cust_agenid, '[^\d]', '', 'g'), '')::integer, 0) AS cust_urutan_kode
			FROM public.mkt_t_orderjemput oj
			LEFT JOIN public.mkt_m_customer cust_ref 
				ON TRIM(oj.order_custid) = TRIM(cust_ref.cust_id)
			%s
			ORDER BY oj.order_date DESC, oj.order_id DESC
			LIMIT 500
		),
		unique_agen AS (
			SELECT DISTINCT ON (agen_urutan) 
				agen_urutan, 
				agen_id, 
				agen_nama 
			FROM public.glb_m_agen 
			WHERE COALESCE(agen_aktifyn, 'Y') = 'Y'
			ORDER BY agen_urutan, agen_id ASC
		)
		SELECT 
			t.order_id,
			COALESCE(ag_order.agen_nama, ag_cust.agen_nama, '-') AS agen_nama,
			COALESCE(TO_CHAR(t.order_date, 'YYYY-MM-DD'), '') AS order_date,
			COALESCE(cust.cust_name, t.order_name, '-') AS cust_name,
			COALESCE(cust.cust_alamat1, '-') AS cust_alamat,
			COALESCE(cust.cust_telp1, '-') AS cust_telp,
			COALESCE(t.order_note, '-') AS order_note,
			COALESCE(t.order_koli, 1) AS order_koli,
			COALESCE(t.order_berat, 0) AS order_berat,
			COALESCE(t.order_volume, 0) AS order_volume,
			COALESCE(t.order_amount, 0) AS order_amount,
			COALESCE(t.order_supir, '-') AS order_supir,
			COALESCE(t.order_nopol, '-') AS order_nopol,
			COALESCE(rc.trectdd_trectdtrecthno, '-') AS no_receipt,
			COALESCE(t.order_tjurhno, '-') AS order_tjurhno,
			COALESCE(NULLIF(TRIM(t.order_activeyn), ''), 'Y') AS order_activeyn,
			COALESCE(NULLIF(TRIM(t.order_paidyn), ''), 'N') AS order_paidyn
		FROM target_orders t
		LEFT JOIN public.mkt_m_customer cust 
			ON TRIM(t.order_custid) = TRIM(cust.cust_id)
		LEFT JOIN unique_agen ag_order 
			ON ag_order.agen_urutan = t.urutan_kode AND t.urutan_kode > 0
		LEFT JOIN unique_agen ag_cust 
			ON ag_cust.agen_urutan = t.cust_urutan_kode AND t.cust_urutan_kode > 0
		LEFT JOIN (
			SELECT DISTINCT ON (TRIM(trectdd_nofpum)) 
				TRIM(trectdd_nofpum) AS nofpum, 
				trectdd_trectdtrecthno 
			FROM public.art_t_receiptdd
		) rc ON TRIM(t.order_id) = rc.nofpum
		WHERE 1=1
	`, whereOj)

	if cabang != "" && !strings.Contains(strings.ToUpper(cabang), "SEMUA") {
		query += " AND (ag.agen_nama ILIKE ? OR ag.agen_id = ?)"
		args = append(args, "%"+cabang+"%", cabang)
	}

	if customer != "" && !strings.Contains(strings.ToUpper(customer), "SEMUA") {
		query += " AND (cust.cust_name ILIKE ? OR t.order_name ILIKE ?)"
		args = append(args, "%"+customer+"%", "%"+customer+"%")
	}

	if noReceipt != "" {
		query += " AND rc.trectdd_trectdtrecthno ILIKE ?"
		args = append(args, "%"+noReceipt+"%")
	}

	query += " ORDER BY t.order_date DESC, t.order_id DESC"

	var results []OrderJemputItem
	if err := database.Raw(query, args...).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []OrderJemputItem{}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 2. POST /api/marketing/order-jemput/save
func SaveOrderJemput(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database resolver gagal"})
		return
	}

	username, _ := c.Get("username")
	var req OrderJemputRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.OrderDate == "" {
		req.OrderDate = time.Now().Format("2006-01-02")
	}
	if req.OrderPaidYN == "" {
		req.OrderPaidYN = "N"
	}

	// Insert Baru atau Update
	if strings.TrimSpace(req.OrderID) == "" {
		req.OrderID = fmt.Sprintf("OJ%s%d", fmt.Sprintf("%v", ptID), time.Now().Unix()%1000000)
		insertSQL := `
			INSERT INTO public.mkt_t_orderjemput (
				order_id, order_date, order_custid, order_name, order_note,
				order_koli, order_berat, order_volume, order_amount, order_supir,
				order_nopol, order_paidyn, order_activeyn, order_updateid, order_updatetime
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'Y', ?, NOW())
		`
		if err := database.Exec(insertSQL,
			req.OrderID, req.OrderDate, req.OrderCustID, req.OrderName, req.OrderNote,
			req.OrderKoli, req.OrderBerat, req.OrderVolume, req.OrderAmount, req.OrderSupir,
			req.OrderNoPol, req.OrderPaidYN, fmt.Sprintf("%v", username),
		).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan order jemput: " + err.Error()})
			return
		}
	} else {
		updateSQL := `
			UPDATE public.mkt_t_orderjemput SET
				order_date = ?, order_custid = ?, order_name = ?, order_note = ?,
				order_koli = ?, order_berat = ?, order_volume = ?, order_amount = ?,
				order_supir = ?, order_nopol = ?, order_paidyn = ?, order_updateid = ?, order_updatetime = NOW()
			WHERE order_id = ?
		`
		if err := database.Exec(updateSQL,
			req.OrderDate, req.OrderCustID, req.OrderName, req.OrderNote,
			req.OrderKoli, req.OrderBerat, req.OrderVolume, req.OrderAmount,
			req.OrderSupir, req.OrderNoPol, req.OrderPaidYN, fmt.Sprintf("%v", username), req.OrderID,
		).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update order jemput: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Order Jemput berhasil disimpan", "order_id": req.OrderID})
}

// 3. DELETE /api/marketing/order-jemput/:id
func DeleteOrderJemput(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database resolver gagal"})
		return
	}

	id := c.Param("id")
	username, _ := c.Get("username")

	updateSQL := `UPDATE public.mkt_t_orderjemput SET order_activeyn = 'N', order_updateid = ?, order_updatetime = NOW() WHERE order_id = ?`
	if err := database.Exec(updateSQL, fmt.Sprintf("%v", username), id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal nonaktifkan order jemput: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Order Jemput berhasil dinonaktifkan"})
}
