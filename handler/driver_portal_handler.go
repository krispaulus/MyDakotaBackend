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

	// 1. Cari user login di database utama (db.GetDB()) terlebih dahulu, lalu fallback ke database tenant
	authDB := db.GetDB()
	if authDB != nil {
		authDB.Raw(`
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
	}
	if webUser.Username == "" {
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
	}

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

	// 2. Validasi Lapis 1: Cocokkan NIP langsung (username == kry_nip atau NIP terselip di realname)
	database.Raw(`
		SELECT TRIM(k.kry_nip) AS nip, COALESCE(NULLIF(TRIM(s.supir_nama), ''), TRIM(k.kry_nama)) AS nama
		FROM public.hrd_m_karyawan k
		LEFT JOIN public.opr_m_supir s ON TRIM(s.supir_id) = TRIM(k.kry_nip)
		WHERE TRIM(k.kry_nip) = TRIM(?)
		   OR (? <> '' AND POSITION(TRIM(k.kry_nip) IN ?) > 0)
		LIMIT 1
	`, username, webUser.RealName, webUser.RealName).Scan(&kry)

	// 3. Validasi Lapis 2: Cocokkan Nomor HP (baik dari username maupun weblogin.mobilenumber ke kry_telp1 / kry_telp2)
	if kry.NIP == "" {
		hpCandidate := webUser.MobileNumber
		if hpCandidate == "" {
			hpCandidate = username
		}
		cleanHP := strings.TrimLeft(strings.ReplaceAll(hpCandidate, " ", ""), "+620")
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

	// 4. Validasi Lapis 3 (Fallback): Cocokkan Nama Lengkap pada karyawan jabatan sopir
	if kry.NIP == "" && webUser.RealName != "" {
		database.Raw(`
			SELECT TRIM(k.kry_nip) AS nip, COALESCE(NULLIF(TRIM(s.supir_nama), ''), TRIM(k.kry_nama)) AS nama
			FROM public.hrd_m_karyawan k
			LEFT JOIN public.opr_m_supir s ON TRIM(s.supir_id) = TRIM(k.kry_nip)
			WHERE (TRIM(k.kry_jabcode) IN ('30', '31') OR s.supir_id IS NOT NULL)
			  AND (
			      UPPER(TRIM(k.kry_nama)) = UPPER(TRIM(?))
			      OR UPPER(TRIM(?)) LIKE UPPER(TRIM(k.kry_nama)) || '%'
			  )
			LIMIT 1
		`, webUser.RealName, webUser.RealName).Scan(&kry)
	}

	supirNama := kry.Nama
	if supirNama == "" {
		supirNama = webUser.RealName
	}

	return DriverProfileDTO{
		Username:  username,
		SupirNIP:  kry.NIP,
		SupirNama: strings.ToUpper(supirNama),
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
	statusFilter := strings.ToUpper(strings.TrimSpace(c.Query("status")))
	if statusFilter == "" {
		statusFilter = "BERJALAN" // Default: tampilkan hanya yang AKTIF JALAN
	}

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
		whereStatus = " AND COALESCE(TRIM(s.sjh_completeyn), 'N') <> 'Y' "
	} else if statusFilter == "SELESAI" {
		whereStatus = " AND COALESCE(TRIM(s.sjh_completeyn), 'N') = 'Y' "
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
// 📦 2. GET DAFTAR RESI LOPER HARI INI (HANYA YANG BELUM DIANTAR & TANPA DUPLIKAT KOLI)
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
			-- Sumber 1: e-Loper pada Surat Tugas aktif (atau tanpa ST yang masih aktif)
			SELECT 
				TRIM(l.loper_eid) AS no_loper,
				COALESCE(TRIM(s.sjh_id), '-') AS no_surat_tugas,
				COALESCE(TRIM(l.loper_nomobil), TRIM(s.sjh_kendid), '-') AS no_polisi,
				TO_CHAR(l.loper_tanggal, 'YYYY-MM-DD HH24:MI') AS tgl_muat,
				LEFT(SPLIT_PART(TRIM(ld.loperd_bttid), '-', 1), 16) AS no_btt,
				COALESCE(TRIM(ld.loperd_histid), '') AS hist_id,
				'LOPER' AS sumber_dokumen
			FROM public.opr_t_eloper l
			INNER JOIN public.opr_t_eloperdetail ld ON TRIM(l.loper_eid) = TRIM(ld.loperd_eloperid)
			LEFT JOIN public.opr_t_esuratjalan s ON TRIM(s.sjh_loadhid) = TRIM(l.loper_loadhid) AND COALESCE(TRIM(l.loper_loadhid), '') <> ''
			WHERE COALESCE(l.loper_aktifyn, 'Y') = 'Y'
			  AND COALESCE(TRIM(s.sjh_completeyn), 'N') <> 'Y'
			  AND TRIM(l.loper_nipsopir) = TRIM(?)

			UNION

			-- Sumber 2: Loading Barang pada Surat Tugas AKTIF JALAN milik NIP sopir (16 digit No. Resi)
			SELECT 
				COALESCE(TRIM(el.loper_eid), TRIM(s.sjh_loadhid)) AS no_loper,
				TRIM(s.sjh_id) AS no_surat_tugas,
				COALESCE(TRIM(s.sjh_kendid), '-') AS no_polisi,
				TO_CHAR(COALESCE(lh.loadh_tanggal, s.sjh_tanggal), 'YYYY-MM-DD HH24:MI') AS tgl_muat,
				LEFT(SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1), 16) AS no_btt,
				COALESCE(TRIM(eld.loperd_histid), '') AS hist_id,
				'LOADING' AS sumber_dokumen
			FROM public.opr_t_esuratjalan s
			INNER JOIN public.opr_t_loadingh lh ON TRIM(lh.loadh_id) = TRIM(s.sjh_loadhid)
			INNER JOIN public.opr_t_loadingd d ON TRIM(d.loadd_loadhid) = TRIM(lh.loadh_id)
			LEFT JOIN public.opr_t_eloper el ON TRIM(el.loper_loadhid) = TRIM(lh.loadh_id)
			LEFT JOIN public.opr_t_eloperdetail eld ON TRIM(eld.loperd_eloperid) = TRIM(el.loper_eid)
			                                       AND LEFT(TRIM(eld.loperd_bttid), 16) = LEFT(SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1), 16)
			WHERE COALESCE(s.sjh_aktifyn, 'Y') = 'Y'
			  AND COALESCE(TRIM(s.sjh_completeyn), 'N') <> 'Y'
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
				WHEN UPPER(x.hist_id) IN ('TERKIRIM', '7') THEN 'TERKIRIM'
				WHEN x.hist_id <> '' THEN UPPER(x.hist_id)
				ELSE 'DALAM PENGANTARAN'
			END AS status_antar
		FROM driver_resi x
		LEFT JOIN public.mkt_t_econote e ON TRIM(e.bttt_id) = TRIM(x.no_btt)
		WHERE COALESCE(TRIM(x.no_btt), '') <> ''
		  AND UPPER(COALESCE(x.hist_id, '')) NOT IN ('TERKIRIM', '7')
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
// ✅ 3. UPDATE STATUS PENGIRIMAN & SINKRONISASI KE OPR_T_EBBL (STATUS, JAM, INSENTIF)
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
		NoBTT           string   `json:"no_btt" binding:"required"`
		NoLoper         string   `json:"no_loper"`
		NamaPenerima    string   `json:"nama_penerima" binding:"required"`
		StatusAntar     string   `json:"status_antar"` // TERKIRIM / DITOLAK_OLEH_PENERIMA / dll.
		ScannedKoliList []string `json:"scanned_koli"`
		KoordinatGPS    string   `json:"gps"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor BTT dan Nama Penerima wajib diisi!"})
		return
	}

	cleanBTT := LEFT16(strings.ToUpper(strings.TrimSpace(req.NoBTT)))
	statusKode := strings.ToUpper(strings.TrimSpace(req.StatusAntar))
	if statusKode == "" {
		statusKode = "TERKIRIM"
	}

	// Tentukan TerimaYN ('Y' jika TERKIRIM, 'N' jika kendala/gagal)
	terimaYN := "N"
	if statusKode == "TERKIRIM" {
		terimaYN = "Y"
	}

	// Cari No. Loper asli (loper_eid) & AgenID Loper jika req.NoLoper masih berupa ID Loading
	var loperInfo struct {
		LoperID string `gorm:"column:loper_eid"`
		AgenID  string `gorm:"column:loper_agenid"`
	}
	database.Raw(`
		SELECT TRIM(l.loper_eid) AS loper_eid, TRIM(COALESCE(l.loper_agenid, '')) AS loper_agenid
		FROM public.opr_t_eloper l
		INNER JOIN public.opr_t_eloperdetail ld ON TRIM(l.loper_eid) = TRIM(ld.loperd_eloperid)
		WHERE LEFT(TRIM(ld.loperd_bttid), 16) = TRIM(?)
		  AND (TRIM(l.loper_eid) = TRIM(?) OR TRIM(l.loper_loadhid) = TRIM(?) OR TRIM(?) = '')
		ORDER BY l.loper_tanggal DESC
		LIMIT 1
	`, cleanBTT, req.NoLoper, req.NoLoper, req.NoLoper).Scan(&loperInfo)

	realNoLoper := loperInfo.LoperID
	if realNoLoper == "" {
		realNoLoper = strings.TrimSpace(req.NoLoper)
	}
	agenID := loperInfo.AgenID
	if agenID == "" {
		agenID = strings.TrimSpace(driver.CabangID)
	}
	if agenID == "" {
		agenID = "1"
	}

	// Pastikan UpdateID berisi 10 digit angka NIP Sopir agar valid untuk kolom Diinput Oleh & Insentif
	updateID10 := strings.TrimSpace(driver.SupirNIP)
	if len(updateID10) != 10 {
		updateID10 = strings.TrimSpace(driver.Username)
	}

	// 1. Update opr_t_eloperdetail
	if realNoLoper != "" {
		res := database.Exec(`
			UPDATE public.opr_t_eloperdetail
			SET loperd_histid = ?,
			    loperd_gps = COALESCE(NULLIF(?, ''), loperd_gps),
			    loperd_updatetime = CURRENT_TIMESTAMP
			WHERE TRIM(loperd_eloperid) = TRIM(?) AND LEFT(TRIM(loperd_bttid), 16) = TRIM(?)
		`, statusKode, req.KoordinatGPS, realNoLoper, cleanBTT)

		if res.RowsAffected == 0 {
			database.Exec(`
				INSERT INTO public.opr_t_eloperdetail (loperd_eloperid, loperd_bttid, loperd_histid, loperd_gps, loperd_updatetime)
				VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			`, realNoLoper, cleanBTT, statusKode, req.KoordinatGPS)
		}

		// 2. INSERT / UPDATE ke public.opr_t_ebbl agar kolom STATUS, JAM, DIINPUT OLEH, dan INSENTIF terisi
		resBBL := database.Exec(`
			UPDATE public.opr_t_ebbl
			SET bbl_terimayn = ?,
			    bbl_tanggal = CURRENT_TIMESTAMP,
			    bbl_jamditerima = TO_CHAR(CURRENT_TIMESTAMP, 'HH24'),
			    bbl_menitditerima = TO_CHAR(CURRENT_TIMESTAMP, 'MI'),
			    bbl_penerima = UPPER(?),
			    bbl_keterangan = ?,
			    bbl_updateid = ?,
			    bbl_updatetime = CURRENT_TIMESTAMP,
			    bbl_aktifyn = 'Y'
			WHERE TRIM(bbl_noloper) = TRIM(?) AND LEFT(TRIM(bbl_bttid), 16) = TRIM(?)
		`, terimaYN, req.NamaPenerima, statusKode, updateID10, realNoLoper, cleanBTT)

		if resBBL.RowsAffected == 0 {
			// Generate bbl_eid unik (misal: BBL + tanggal + 4 digit akhir BTT)
			newBBLEID := fmt.Sprintf("BBL%s", cleanBTT)
			if len(newBBLEID) > 20 {
				newBBLEID = newBBLEID[:20]
			}
			database.Exec(`
				INSERT INTO public.opr_t_ebbl (
					bbl_eid, bbl_agenid, bbl_noloper, bbl_bttid, bbl_tanggal,
					bbl_jamditerima, bbl_menitditerima, bbl_terimayn, bbl_penerima,
					bbl_keterangan, bbl_updateid, bbl_updatetime, bbl_aktifyn
				)
				VALUES (
					?, ?, ?, ?, CURRENT_TIMESTAMP,
					TO_CHAR(CURRENT_TIMESTAMP, 'HH24'), TO_CHAR(CURRENT_TIMESTAMP, 'MI'), ?, UPPER(?),
					?, ?, CURRENT_TIMESTAMP, 'Y'
				)
			`, newBBLEID, agenID, realNoLoper, cleanBTT, terimaYN, req.NamaPenerima, statusKode, updateID10)
		}
	}

	// 3. Catat keterangan riwayat pada mkt_t_econote
	scanSummary := ""
	if len(req.ScannedKoliList) > 0 {
		scanSummary = fmt.Sprintf(" [Scan Turun: %d Koli (%s)]", len(req.ScannedKoliList), strings.Join(req.ScannedKoliList, ", "))
	}
	ketLog := fmt.Sprintf("[%s] Penerima: %s (Sopir: %s)%s", statusKode, strings.ToUpper(req.NamaPenerima), driver.SupirNama, scanSummary)
	database.Exec(`
		UPDATE public.mkt_t_econote
		SET bttt_keterangan = COALESCE(NULLIF(TRIM(bttt_keterangan), ''), '') || ' | ' || ?
		WHERE TRIM(bttt_id) = TRIM(?)
	`, ketLog, cleanBTT)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Status Resi %s (%s) berhasil diperbarui!", cleanBTT, statusKode),
	})
}

func LEFT16(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s
}
