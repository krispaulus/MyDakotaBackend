package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DriverProfileDTO struct {
	Username  string `json:"username"`
	SupirNIP  string `json:"supir_nip"`
	SupirNama string `json:"supir_nama"`
	CabangID  string `json:"cabang_id"`
	IsAdmin   bool   `json:"is_admin"`
}

// Helper: Resolusi identitas sopir berdasarkan akun yang login (atau parameter ?supir_nip= jika admin melakukan simulasi)
func resolveDriverIdentity(c *gin.Context, database *gorm.DB) DriverProfileDTO {
	rawUser, _ := c.Get("username")
	username := strings.TrimSpace(fmt.Sprintf("%v", rawUser))
	overrideNIP := strings.TrimSpace(c.Query("supir_nip"))

	var webUser struct {
		Username     string `gorm:"column:username"`
		RealName     string `gorm:"column:realname"`
		MobileNumber string `gorm:"column:mobilenumber"`
		UserType     string `gorm:"column:usertype"`
		KodeCbg      string `gorm:"column:kode_cabang"`
	}
	database.Raw(`
		SELECT 
			TRIM(COALESCE(username, '')) AS username,
			TRIM(COALESCE(realname, '')) AS realname,
			TRIM(COALESCE(mobilenumber, '')) AS mobilenumber,
			TRIM(COALESCE(usertype, '')) AS usertype,
			TRIM(COALESCE(kode_cabang, '')) AS kode_cabang
		FROM public.weblogin
		WHERE LOWER(TRIM(username)) = LOWER(TRIM(?))
		LIMIT 1
	`, username).Scan(&webUser)

	isAdmin := strings.EqualFold(username, "superdli") ||
		strings.Contains(strings.ToUpper(webUser.UserType), "ADMIN") ||
		strings.Contains(strings.ToUpper(webUser.UserType), "SUPER")

	// Jika Admin sedang mode simulasi memilih sopir dari dropdown
	if isAdmin && overrideNIP != "" {
		var sim struct {
			NIP  string `gorm:"column:nip"`
			Nama string `gorm:"column:nama"`
		}
		database.Raw(`
			SELECT TRIM(k.kry_nip) AS nip, COALESCE(NULLIF(TRIM(s.supir_nama), ''), TRIM(k.kry_nama)) AS nama
			FROM public.hrd_m_karyawan k
			LEFT JOIN public.opr_m_supir s ON TRIM(s.supir_id) = TRIM(k.kry_nip)
			WHERE TRIM(k.kry_nip) = TRIM(?)
			LIMIT 1
		`, overrideNIP).Scan(&sim)

		return DriverProfileDTO{
			Username:  username,
			SupirNIP:  sim.NIP,
			SupirNama: strings.ToUpper(sim.Nama),
			CabangID:  webUser.KodeCbg,
			IsAdmin:   true,
		}
	}

	var kry struct {
		NIP  string `gorm:"column:nip"`
		Nama string `gorm:"column:nama"`
	}

	// 1. Validasi Utama: Cocokkan NIP (username == kry_nip)
	database.Raw(`
		SELECT TRIM(k.kry_nip) AS nip, COALESCE(NULLIF(TRIM(s.supir_nama), ''), TRIM(k.kry_nama)) AS nama
		FROM public.hrd_m_karyawan k
		LEFT JOIN public.opr_m_supir s ON TRIM(s.supir_id) = TRIM(k.kry_nip)
		WHERE TRIM(k.kry_nip) = TRIM(?)
		LIMIT 1
	`, username).Scan(&kry)

	// 2. Validasi Kedua: Cocokkan Nomor HP (weblogin.mobilenumber == kry_telp1 / kry_telp2)
	if kry.NIP == "" && webUser.MobileNumber != "" {
		cleanHP := strings.TrimLeft(strings.ReplaceAll(webUser.MobileNumber, " ", ""), "+620")
		if len(cleanHP) >= 8 {
			database.Raw(`
				SELECT TRIM(k.kry_nip) AS nip, COALESCE(NULLIF(TRIM(s.supir_nama), ''), TRIM(k.kry_nama)) AS nama
				FROM public.hrd_m_karyawan k
				LEFT JOIN public.opr_m_supir s ON TRIM(s.supir_id) = TRIM(k.kry_nip)
				WHERE RIGHT(REGEXP_REPLACE(COALESCE(k.kry_telp1, ''), '\D', '', 'g'), 9) = RIGHT(?, 9)
				   OR RIGHT(REGEXP_REPLACE(COALESCE(k.kry_telp2, ''), '\D', '', 'g'), 9) = RIGHT(?, 9)
				LIMIT 1
			`, cleanHP, cleanHP).Scan(&kry)
		}
	}

	// 3. Validasi Ketiga (Fallback): Cocokkan Nama Lengkap persis pada karyawan jabatan sopir
	if kry.NIP == "" && webUser.RealName != "" {
		database.Raw(`
			SELECT TRIM(k.kry_nip) AS nip, COALESCE(NULLIF(TRIM(s.supir_nama), ''), TRIM(k.kry_nama)) AS nama
			FROM public.hrd_m_karyawan k
			LEFT JOIN public.opr_m_supir s ON TRIM(s.supir_id) = TRIM(k.kry_nip)
			WHERE (TRIM(k.kry_jabcode) IN ('30', '31') OR s.supir_id IS NOT NULL)
			  AND UPPER(TRIM(k.kry_nama)) = UPPER(TRIM(?))
			LIMIT 1
		`, webUser.RealName).Scan(&kry)
	}

	return DriverProfileDTO{
		Username:  username,
		SupirNIP:  kry.NIP,
		SupirNama: strings.ToUpper(kry.Nama),
		CabangID:  webUser.KodeCbg,
		IsAdmin:   isAdmin,
	}
}

// =========================================================================
// 🚚 1. GET SURAT TUGAS KHUSUS SOPIR LOGIN
// GET /api/driver/surat-tugas
// =========================================================================
func GetDriverSuratTugasHandler(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.GetDB()
	}

	driver := resolveDriverIdentity(c, database)
	statusFilter := strings.ToUpper(strings.TrimSpace(c.Query("status"))) // BERJALAN / SELESAI / ALL

	// Penjaga di awal: jika NIP sopir kosong (misal admin belum memilih sopir), langsung return kosong
	if strings.TrimSpace(driver.SupirNIP) == "" {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"driver": driver,
			"data":   []interface{}{},
		})
		return
	}

	whereStatus := ""
	if statusFilter == "BERJALAN" {
		whereStatus = " AND COALESCE(s.sjh_completeyn, 'N') <> 'Y' "
	} else if statusFilter == "SELESAI" {
		whereStatus = " AND COALESCE(s.sjh_completeyn, 'N') = 'Y' "
	}

	query := fmt.Sprintf(`
		SELECT 
			TRIM(s.sjh_id) AS no_surat_tugas,
			TO_CHAR(s.sjh_tanggal, 'YYYY-MM-DD HH24:MI') AS tgl_berangkat,
			COALESCE(TO_CHAR(s.sjh_tanggalkembali, 'YYYY-MM-DD HH24:MI'), '-') AS tgl_kembali,
			COALESCE(TRIM(s.sjh_kendid), '-') AS no_polisi,
			COALESCE(TRIM(k.kend_merk), '-') AS merk_kendaraan,
			COALESCE(TRIM(s.sjh_sopir1_nip), '-') AS sopir1_nip,
			COALESCE(TRIM(sp1.kry_nama), TRIM(s.sjh_sopir1_nip), '-') AS sopir1_nama,
			COALESCE(TRIM(s.sjh_sopir2_nip), '-') AS sopir2_nip,
			COALESCE(TRIM(sp2.kry_nama), '-') AS sopir2_nama,
			CASE 
				WHEN UPPER(TRIM(cStart.agen_nama)) = 'PUSAT DAKOTA' OR TRIM(s.sjh_startagenid::varchar) IN ('1', '001', 'PST001') THEN 'DLI PUSAT'
				ELSE COALESCE(TRIM(cStart.agen_nama), TRIM(s.sjh_startagenid::varchar), '-')
			END AS asal_cabang,
			CASE 
				WHEN UPPER(TRIM(cEnd.agen_nama)) = 'PUSAT DAKOTA' OR TRIM(s.sjh_endagenid::varchar) IN ('1', '001', 'PST001') THEN 'DLI PUSAT'
				ELSE COALESCE(TRIM(cEnd.agen_nama), TRIM(s.sjh_endagenid::varchar), '-')
			END AS tujuan_tugas,
			COALESCE(TRIM(trh.trh_name), '') AS nama_trayek,
			COALESCE(TRIM(a.ass_nama), 'PENGANTARAN') AS jenis_tugas,
			COALESCE(TRIM(s.sjh_keterangan), '-') AS keterangan,
			COALESCE(TRIM(s.sjh_loadhid), '') AS load_id,
			COALESCE(TRIM(s.sjh_completeyn), 'N') AS complete_yn,
			(
				SELECT COUNT(DISTINCT SPLIT_PART(TRIM(ld.loadd_bttkoliid), '-', 1))
				FROM public.opr_t_loadingd ld
				WHERE TRIM(ld.loadd_loadhid) = TRIM(s.sjh_loadhid)
				  AND COALESCE(TRIM(s.sjh_loadhid), '') <> ''
			)::int AS total_resi_muat
		FROM public.opr_t_esuratjalan s
		LEFT JOIN public.glb_m_kendaraan k ON TRIM(s.sjh_kendid) = TRIM(k.kend_id)
		LEFT JOIN public.hrd_m_karyawan sp1 ON TRIM(s.sjh_sopir1_nip) = TRIM(sp1.kry_nip)
		LEFT JOIN public.hrd_m_karyawan sp2 ON TRIM(s.sjh_sopir2_nip) = TRIM(sp2.kry_nip)
		LEFT JOIN public.glb_m_agen cStart ON TRIM(s.sjh_startagenid::varchar) = TRIM(cStart.agen_id::varchar)
		LEFT JOIN public.glb_m_agen cEnd ON TRIM(s.sjh_endagenid::varchar) = TRIM(cEnd.agen_id::varchar)
		LEFT JOIN public.opr_m_assignment a ON TRIM(s.sjh_assid::varchar) = TRIM(a.ass_id::varchar)
		LEFT JOIN public.opr_m_trayekh trh ON TRIM(s.sjh_trayekid::varchar) = TRIM(trh.trh_id::varchar)
		WHERE COALESCE(s.sjh_aktifyn, 'Y') = 'Y'
		  AND (
		      TRIM(s.sjh_sopir1_nip) = TRIM(?)
		      OR TRIM(s.sjh_sopir2_nip) = TRIM(?)
		  )
		  %s
		ORDER BY s.sjh_tanggal DESC, s.sjh_id DESC
		LIMIT 50
	`, whereStatus)

	list := make([]map[string]interface{}, 0)
	if err := database.Raw(query, driver.SupirNIP, driver.SupirNIP).Scan(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat surat tugas sopir: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"driver": driver,
		"data":   list,
	})
}

// =========================================================================
// 📦 2. GET DAFTAR RESI LOPER & LOADING BARANG UNTUK TITIK PENGIRIMAN SOPIR
// GET /api/driver/loper-resi
// =========================================================================
func GetDriverLoperResiHandler(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.GetDB()
	}

	driver := resolveDriverIdentity(c, database)
	noST := strings.TrimSpace(c.Query("no_st"))

	// Penjaga di awal: jika NIP sopir kosong, langsung return kosong
	if strings.TrimSpace(driver.SupirNIP) == "" {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"driver": driver,
			"data":   []interface{}{},
		})
		return
	}

	whereST := ""
	var args []interface{}
	args = append(args, driver.SupirNIP, driver.SupirNIP, driver.SupirNIP)

	if noST != "" {
		whereST = " AND TRIM(x.no_surat_tugas) = TRIM(?) "
		args = append(args, noST)
	}

	query := fmt.Sprintf(`
		WITH driver_resi AS (
			-- Sumber 1: Resmi masuk e-Loper (opr_t_eloper + opr_t_eloperdetail)
			SELECT 
				TRIM(l.loper_eid) AS no_loper,
				COALESCE(TRIM(s.sjh_id), '-') AS no_surat_tugas,
				COALESCE(TRIM(l.loper_nomobil), TRIM(s.sjh_kendid), '-') AS no_polisi,
				TO_CHAR(l.loper_tanggal, 'YYYY-MM-DD HH24:MI') AS tgl_muat,
				TRIM(ld.loperd_bttid) AS no_btt,
				COALESCE(TRIM(ld.loperd_histid), '') AS hist_id,
				'LOPER' AS sumber_dokumen
			FROM public.opr_t_eloper l
			INNER JOIN public.opr_t_eloperdetail ld ON TRIM(l.loper_eid) = TRIM(ld.loperd_eloperid)
			LEFT JOIN public.opr_t_esuratjalan s ON TRIM(s.sjh_loadhid) = TRIM(l.loper_loadhid) AND COALESCE(TRIM(l.loper_loadhid), '') <> ''
			WHERE COALESCE(l.loper_aktifyn, 'Y') = 'Y'
			  AND TRIM(l.loper_nipsopir) = TRIM(?)

			UNION

			-- Sumber 2: Otomatis dari Loading Barang (opr_t_loadingd) pada Surat Tugas milik NIP sopir
			SELECT 
				COALESCE(TRIM(el.loper_eid), TRIM(s.sjh_loadhid)) AS no_loper,
				TRIM(s.sjh_id) AS no_surat_tugas,
				COALESCE(TRIM(s.sjh_kendid), '-') AS no_polisi,
				TO_CHAR(COALESCE(lh.loadh_tanggal, s.sjh_tanggal), 'YYYY-MM-DD HH24:MI') AS tgl_muat,
				SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1) AS no_btt,
				COALESCE(TRIM(eld.loperd_histid), '') AS hist_id,
				'LOADING' AS sumber_dokumen
			FROM public.opr_t_esuratjalan s
			INNER JOIN public.opr_t_loadingh lh ON TRIM(lh.loadh_id) = TRIM(s.sjh_loadhid)
			INNER JOIN public.opr_t_loadingd d ON TRIM(d.loadd_loadhid) = TRIM(lh.loadh_id)
			LEFT JOIN public.opr_t_eloper el ON TRIM(el.loper_loadhid) = TRIM(lh.loadh_id)
			LEFT JOIN public.opr_t_eloperdetail eld ON TRIM(eld.loperd_eloperid) = TRIM(el.loper_eid) AND TRIM(eld.loperd_bttid) = SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)
			WHERE COALESCE(s.sjh_aktifyn, 'Y') = 'Y'
			  AND COALESCE(TRIM(s.sjh_loadhid), '') <> ''
			  AND (
			      TRIM(s.sjh_sopir1_nip) = TRIM(?)
			      OR TRIM(s.sjh_sopir2_nip) = TRIM(?)
			  )
		)
		SELECT DISTINCT ON (x.no_btt)
			x.no_loper,
			x.no_surat_tugas,
			x.no_polisi,
			x.tgl_muat,
			x.no_btt,
			x.sumber_dokumen,
			COALESCE(TRIM(e.bttt_nosuratjalan), '-') AS no_sj_customer,
			COALESCE(TRIM(e.bttt_asalname), '-') AS pengirim,
			COALESCE(TRIM(e.bttt_tujuannama), '-') AS penerima,
			COALESCE(TRIM(e.bttt_tujuanalamat), '-') AS alamat_tujuan,
			COALESCE(TRIM(e.bttt_tujuankota), '-') AS kota_tujuan,
			COALESCE(TRIM(e.bttt_tujuantelp), '-') AS telp_penerima,
			COALESCE(NULLIF(e.bttt_jmlunit, 0), 1)::int AS koli,
			GREATEST(COALESCE(e.bttt_berat, 0), COALESCE(e.bttt_beratvol, 0)) AS berat,
			COALESCE(e.bttt_tagihtujuan, 0) AS nominal_cod,
			CASE 
				WHEN x.hist_id <> '' THEN 'TERKIRIM'
				ELSE 'DALAM PENGANTARAN'
			END AS status_antar
		FROM driver_resi x
		LEFT JOIN public.mkt_t_econote e ON TRIM(e.bttt_id) = TRIM(x.no_btt)
		WHERE COALESCE(TRIM(x.no_btt), '') <> ''
		  %s
		ORDER BY x.no_btt ASC, x.tgl_muat DESC
	`, whereST)

	rows := make([]map[string]interface{}, 0)
	if err := database.Raw(query, args...).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat daftar resi loper: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"driver": driver,
		"data":   rows,
	})
}

// =========================================================================
// ✅ 3. UPDATE STATUS PENGIRIMAN TITIK LOPER (POD / TERKIRIM)
// POST /api/driver/loper-resi/deliver
// =========================================================================
func DeliverLoperResiHandler(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		database = db.GetDB()
	}

	driver := resolveDriverIdentity(c, database)

	var req struct {
		NoBTT        string `json:"no_btt" binding:"required"`
		NoLoper      string `json:"no_loper"`
		NamaPenerima string `json:"nama_penerima" binding:"required"`
		StatusAntar  string `json:"status_antar"` // TERKIRIM / KENDALA
		Catatan      string `json:"catatan"`
		KoordinatGPS string `json:"gps"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor BTT dan Nama Penerima wajib diisi!"})
		return
	}

	cleanBTT := strings.ToUpper(strings.TrimSpace(req.NoBTT))
	statusKode := "TERKIRIM"
	if strings.ToUpper(req.StatusAntar) == "KENDALA" {
		statusKode = "KENDALA"
	}

	if strings.TrimSpace(req.NoLoper) != "" {
		res := database.Exec(`
			UPDATE public.opr_t_eloperdetail
			SET loperd_histid = ?,
			    loperd_gps = COALESCE(NULLIF(?, ''), loperd_gps),
			    loperd_updatetime = CURRENT_TIMESTAMP
			WHERE TRIM(loperd_eloperid) = TRIM(?) AND TRIM(loperd_bttid) = TRIM(?)
		`, statusKode, req.KoordinatGPS, req.NoLoper, cleanBTT)

		if res.RowsAffected == 0 {
			database.Exec(`
				INSERT INTO public.opr_t_eloperdetail (loperd_eloperid, loperd_bttid, loperd_histid, loperd_gps, loperd_updatetime)
				VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			`, req.NoLoper, cleanBTT, statusKode, req.KoordinatGPS)
		}
	}

	ketLog := fmt.Sprintf("Diterima oleh: %s (Sopir: %s)", strings.ToUpper(req.NamaPenerima), driver.SupirNama)
	if strings.TrimSpace(req.Catatan) != "" {
		ketLog += " - " + req.Catatan
	}
	database.Exec(`
		UPDATE public.mkt_t_econote
		SET bttt_keterangan = COALESCE(NULLIF(TRIM(bttt_keterangan), ''), '') || ' | ' || ?
		WHERE TRIM(bttt_id) = TRIM(?)
	`, ketLog, cleanBTT)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Status Resi %s berhasil diperbarui!", cleanBTT),
	})
}
