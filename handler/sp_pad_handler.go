package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func resolveSPPADDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		return db.GetDB()
	}
	return database
}

type SPPADHeaderDTO struct {
	NoSPPAD            string `json:"no_sp_pad" gorm:"column:no_sp_pad"`
	TanggalSP          string `json:"tanggal_sp" gorm:"column:tanggal_sp"`
	CabangAsal         string `json:"cabang_asal" gorm:"column:cabang_asal"`
	VendorExpedisiLain string `json:"vendor_expedisi_lain" gorm:"column:vendor_expedisi_lain"`
	NoSuratTugas       string `json:"no_surat_tugas" gorm:"column:no_surat_tugas"`
	NoMobil            string `json:"no_mobil" gorm:"column:no_mobil"`
	Sopir1             string `json:"sopir1" gorm:"column:sopir1"`
	Sopir2             string `json:"sopir2" gorm:"column:sopir2"`
	JumlahBTT          int    `json:"jumlah_btt" gorm:"column:jumlah_btt"`
	StatusAktif        string `json:"status_aktif" gorm:"column:status_aktif"`
}

// =========================================================================
// 1. GET LIST HISTORI SP PAD (Sesuai opr_t_sp_pad.asp)
// GET /api/operasional/sp-pad
// =========================================================================
func GetListSPPAD(c *gin.Context) {
	database := resolveSPPADDB(c)

	tglDari := strings.TrimSpace(c.Query("dariTanggal"))
	tglSampai := strings.TrimSpace(c.Query("sampaiTanggal"))
	noSP := strings.TrimSpace(c.Query("noSP"))
	noBTT := strings.TrimSpace(c.Query("noBTT"))
	ekspedisiLain := strings.TrimSpace(c.Query("ekspedisiLain"))
	agenID := strings.TrimSpace(c.Query("agenID"))

	filterSQL := ""
	var args []interface{}

	if tglDari != "" && tglSampai != "" {
		filterSQL += " AND CAST(p.spp_tanggal AS DATE) BETWEEN ? AND ? "
		args = append(args, tglDari, tglSampai)
	}
	if agenID != "" && !strings.Contains(strings.ToUpper(agenID), "PUSAT") && strings.ToUpper(agenID) != "ALL" {
		filterSQL += " AND (TRIM(CAST(p.spp_asalagenid AS VARCHAR)) = ? OR UPPER(a.agen_nama) LIKE UPPER(?)) "
		args = append(args, agenID, "%"+agenID+"%")
	}
	if noSP != "" {
		filterSQL += " AND UPPER(TRIM(p.spp_eid)) LIKE UPPER(?) "
		args = append(args, "%"+noSP+"%")
	}
	if noBTT != "" {
		filterSQL += ` AND EXISTS (
			SELECT 1 FROM public.opr_t_esp_paddetil pd 
			WHERE TRIM(pd.sppd_esptid) = TRIM(p.spp_eid) 
			  AND UPPER(TRIM(pd.sppd_bttid)) LIKE UPPER(?)
		) `
		args = append(args, "%"+noBTT+"%")
	}
	if ekspedisiLain != "" {
		filterSQL += " AND UPPER(TRIM(p.spp_expedisilain)) LIKE UPPER(?) "
		args = append(args, "%"+ekspedisiLain+"%")
	}

	query := fmt.Sprintf(`
		SELECT 
			TRIM(p.spp_eid) AS no_sp_pad,
			TO_CHAR(p.spp_tanggal, 'DD/MM/YYYY HH24:MI') AS tanggal_sp,
			COALESCE(TRIM(a.agen_nama), '-') AS cabang_asal,
			COALESCE(TRIM(p.spp_expedisilain), '-') AS vendor_expedisi_lain,
			COALESCE(TRIM(p.spp_surattugas), '-') AS no_surat_tugas,
			COALESCE(TRIM(sj.sjh_kendid), '-') AS no_mobil,
			COALESCE(TRIM(d1.kry_nama), TRIM(sj.sjh_sopir1_nip), '-') AS sopir1,
			COALESCE(TRIM(d2.kry_nama), TRIM(sj.sjh_sopir2_nip), '') AS sopir2,
			COUNT(pd.sppd_bttid)::int AS jumlah_btt,
			CASE WHEN COALESCE(TRIM(p.spp_aktifyn), 'Y') = 'Y' THEN 'Aktif' ELSE 'Non-Aktif' END AS status_aktif
		FROM public.opr_t_esp_pad p
		LEFT JOIN public.opr_t_esp_paddetil pd ON TRIM(pd.sppd_esptid) = TRIM(p.spp_eid)
		LEFT JOIN public.glb_m_agen a ON TRIM(CAST(p.spp_asalagenid AS VARCHAR)) = TRIM(CAST(a.agen_id AS VARCHAR))
		LEFT JOIN public.opr_t_esuratjalan sj ON TRIM(p.spp_surattugas) = TRIM(sj.sjh_id)
		LEFT JOIN public.hrd_m_karyawan d1 ON TRIM(sj.sjh_sopir1_nip) = TRIM(d1.kry_nip)
		LEFT JOIN public.hrd_m_karyawan d2 ON TRIM(sj.sjh_sopir2_nip) = TRIM(d2.kry_nip)
		WHERE p.spp_eid IS NOT NULL %s
		GROUP BY p.spp_eid, p.spp_tanggal, a.agen_nama, p.spp_expedisilain, p.spp_surattugas, sj.sjh_kendid, d1.kry_nama, sj.sjh_sopir1_nip, d2.kry_nama, sj.sjh_sopir2_nip, p.spp_aktifyn
		ORDER BY p.spp_tanggal DESC, p.spp_eid DESC
		LIMIT 200
	`, filterSQL)

	results := make([]SPPADHeaderDTO, 0)
	if err := database.Raw(query, args...).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data SP PAD: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   results,
	})
}

// =========================================================================
// 2. TARIK DATA LOADING UNTUK PEMBUATAN SP-PAD (opr_t_sp_pad_loading.asp & _f.asp)
// GET /api/operasional/sp-pad/from-loading?load_id=...
// =========================================================================
func GetLoadingForSPPAD(c *gin.Context) {
	database := resolveSPPADDB(c)
	loadID := strings.TrimSpace(c.Query("load_id"))
	if loadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Loading (LoadH_ID) wajib diisi"})
		return
	}

	type LoadingPADHeader struct {
		LoadID     string `gorm:"column:load_id" json:"load_id"`
		SuratTugas string `gorm:"column:surat_tugas" json:"surat_tugas"`
		SjhAktifYN string `gorm:"column:sjh_aktifyn" json:"sjh_aktifyn"`
		NoMobil    string `gorm:"column:no_mobil" json:"no_mobil"`
		Sopir1NIP  string `gorm:"column:sopir1_nip" json:"sopir1_nip"`
		Sopir1Nama string `gorm:"column:sopir1_nama" json:"sopir1_nama"`
		Sopir2NIP  string `gorm:"column:sopir2_nip" json:"sopir2_nip"`
		Sopir2Nama string `gorm:"column:sopir2_nama" json:"sopir2_nama"`
		ExistingSP string `gorm:"column:existing_sp" json:"existing_sp"`
	}

	var hdr LoadingPADHeader
	err := database.Raw(`
		SELECT 
			TRIM(l.loadh_id) AS load_id,
			TRIM(COALESCE(s.sjh_id, '')) AS surat_tugas,
			COALESCE(TRIM(s.sjh_aktifyn), 'Y') AS sjh_aktifyn,
			TRIM(COALESCE(NULLIF(TRIM(l.loadh_kendid), ''), TRIM(s.sjh_kendid), '')) AS no_mobil,
			TRIM(COALESCE(s.sjh_sopir1_nip, '')) AS sopir1_nip,
			COALESCE(TRIM(sp1.kry_nama), '') AS sopir1_nama,
			TRIM(COALESCE(s.sjh_sopir2_nip, '')) AS sopir2_nip,
			COALESCE(TRIM(sp2.kry_nama), '') AS sopir2_nama,
			COALESCE((
				SELECT TRIM(p.spp_eid) FROM public.opr_t_esp_pad p 
				WHERE TRIM(p.spp_loadhid) = TRIM(l.loadh_id) AND COALESCE(p.spp_aktifyn, 'Y') = 'Y' 
				LIMIT 1
			), '') AS existing_sp
		FROM public.opr_t_loadingh l
		LEFT JOIN public.opr_t_esuratjalan s ON TRIM(l.loadh_id) = TRIM(s.sjh_loadhid)
		LEFT JOIN public.hrd_m_karyawan sp1 ON TRIM(s.sjh_sopir1_nip) = TRIM(sp1.kry_nip) AND COALESCE(sp1.kry_aktifyn, 'Y') = 'Y'
		LEFT JOIN public.hrd_m_karyawan sp2 ON TRIM(s.sjh_sopir2_nip) = TRIM(sp2.kry_nip) AND COALESCE(sp2.kry_aktifyn, 'Y') = 'Y'
		WHERE UPPER(TRIM(l.loadh_id)) = UPPER(TRIM(?))
		   OR UPPER(TRIM(s.sjh_id)) = UPPER(TRIM(?))
		LIMIT 1
	`, loadID, loadID).Scan(&hdr).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query loading: " + err.Error()})
		return
	}
	if hdr.LoadID == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": fmt.Sprintf("Data Loading / Surat Tugas [%s] tidak ditemukan!", loadID)})
		return
	}
	if hdr.SuratTugas != "" && hdr.SjhAktifYN != "Y" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": fmt.Sprintf("Surat Tugas [%s] berstatus non-aktif!", hdr.SuratTugas)})
		return
	}

	type LoadingPADBTT struct {
		BttID       string  `gorm:"column:btt_id" json:"btt_id"`
		NoSJ        string  `gorm:"column:no_sj" json:"no_sj"`
		TglBTT      string  `gorm:"column:tgl_btt" json:"tgl_btt"`
		Pengirim    string  `gorm:"column:pengirim" json:"pengirim"`
		Penerima    string  `gorm:"column:penerima" json:"penerima"`
		Alamat      string  `gorm:"column:alamat" json:"alamat"`
		Kota        string  `gorm:"column:kota" json:"kota"`
		Telp        string  `gorm:"column:telp" json:"telp"`
		ServiceNama string  `gorm:"column:service_nama" json:"service_nama"`
		Koli        int     `gorm:"column:koli" json:"koli"`
		Berat       float64 `gorm:"column:berat" json:"berat"`
		Harga       float64 `gorm:"column:harga" json:"harga"`
	}

	btts := make([]LoadingPADBTT, 0)
	database.Raw(`
		SELECT 
			TRIM(e.bttt_id) AS btt_id,
			COALESCE(NULLIF(TRIM(e.bttt_nosuratjalan), ''), '-') AS no_sj,
			COALESCE(TO_CHAR(e.bttt_tanggal, 'DD/MM/YYYY'), '-') AS tgl_btt,
			COALESCE(TRIM(e.bttt_asalname), '-') AS pengirim,
			COALESCE(TRIM(e.bttt_tujuannama), '-') AS penerima,
			COALESCE(TRIM(e.bttt_tujuanalamat), '-') AS alamat,
			COALESCE(TRIM(e.bttt_tujuankota), '-') AS kota,
			COALESCE(TRIM(e.bttt_tujuantelp), '-') AS telp,
			CASE 
				WHEN TRIM(COALESCE(e.bttt_servid::varchar, '')) = '2' THEN 'LAUT'
				WHEN TRIM(COALESCE(e.bttt_servid::varchar, '')) = '3' THEN 'UDARA'
				ELSE 'DARAT'
			END AS service_nama,
			COUNT(d.loadd_id)::int AS koli,
			GREATEST(COALESCE(e.bttt_berat, 0), COALESCE(e.bttt_beratvol, 0)) AS berat,
			COALESCE(e.bttt_harga, 0) AS harga
		FROM public.opr_t_loadingd d
		INNER JOIN public.mkt_t_econote e ON LEFT(TRIM(d.loadd_bttkoliid), 16) = LEFT(TRIM(e.bttt_id), 16)
		WHERE TRIM(d.loadd_loadhid) = TRIM(?)
		GROUP BY e.bttt_id, e.bttt_nosuratjalan, e.bttt_tanggal, e.bttt_asalname, e.bttt_tujuannama, e.bttt_tujuanalamat, e.bttt_tujuankota, e.bttt_tujuantelp, e.bttt_servid, e.bttt_berat, e.bttt_beratvol, e.bttt_harga
		ORDER BY e.bttt_id ASC
	`, hdr.LoadID).Scan(&btts)

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"header": hdr,
		"btts":   btts,
	})
}

// =========================================================================
// 3. TERBITKAN SURAT PENGANTAR PAD BARU (p-opr_t_sp_pad_loading_f.asp / _a_btt.asp)
// POST /api/operasional/sp-pad
// =========================================================================
func CreateSPPADHandler(c *gin.Context) {
	ptID, _ := c.Get("pt_id")
	ptCode := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", ptID)))
	if ptCode == "" || ptCode == "<NIL>" {
		ptCode = "C"
	}

	database := resolveSPPADDB(c)
	username, _ := c.Get("username")
	userStr := strings.TrimSpace(fmt.Sprintf("%v", username))
	if userStr == "" || userStr == "<NIL>" {
		userStr = "SYSTEM"
	}

	var req struct {
		AsalAgenNama   string   `json:"spp_asal_agen"`
		ExpedisiLain   string   `json:"spp_expedisilain" binding:"required"`
		NoResiVendor   string   `json:"spp_noresi_vendor"`
		SopirVendor    string   `json:"spp_sopir"`
		NoSuratTugas   string   `json:"spp_surattugas"`
		LoadHID        string   `json:"spp_loadhid"`
		TanggalBerangk string   `json:"spp_tanggal"`
		DaftarBTT      []string `json:"daftar_btt" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Ekspedisi Lain dan Daftar BTT wajib diisi!"})
		return
	}
	if len(req.DaftarBTT) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Minimal harus ada 1 BTT dalam daftar muatan SP PAD!"})
		return
	}

	// Resolusi Agen Asal dan Kode Cabang 3 Digit dari Master DB
	var asalAgenID, cabangCode3 string
	rawAsal := strings.TrimSpace(req.AsalAgenNama)
	database.Raw(`
		SELECT 
			TRIM(agen_id::varchar),
			COALESCE(
				NULLIF(RIGHT(REGEXP_REPLACE(TRIM(COALESCE(agen_kode, '')), '\D', '', 'g'), 3), ''),
				NULLIF(RIGHT(REGEXP_REPLACE(TRIM(COALESCE(agen_id::varchar, '')), '\D', '', 'g'), 3), ''),
				LEFT(UPPER(TRIM(agen_id::varchar)), 3)
			)
		FROM public.glb_m_agen
		WHERE UPPER(TRIM(agen_nama)) = UPPER(TRIM(?))
		   OR TRIM(agen_id::varchar) = TRIM(?)
		ORDER BY LENGTH(TRIM(agen_id::varchar)) ASC
		LIMIT 1
	`, rawAsal, rawAsal).Row().Scan(&asalAgenID, &cabangCode3)

	if len(cabangCode3) > 3 {
		cabangCode3 = cabangCode3[len(cabangCode3)-3:]
	} else if len(cabangCode3) < 3 && cabangCode3 != "" {
		cabangCode3 = fmt.Sprintf("%03s", cabangCode3)
	}

	now := time.Now()
	bulanTahun := now.Format("012006") // MMYYYY
	prefix := fmt.Sprintf("P%s%s%s", ptCode, cabangCode3, bulanTahun)

	var maxHeader, maxDetil int
	database.Raw(`
		SELECT COALESCE(MAX(CAST(RIGHT(TRIM(spp_eid), 4) AS INTEGER)), 0)
		FROM public.opr_t_esp_pad
		WHERE TRIM(spp_eid) LIKE ? AND RIGHT(TRIM(spp_eid), 4) ~ '^[0-9]{4}$'
	`, prefix+"%").Scan(&maxHeader)

	database.Raw(`
		SELECT COALESCE(MAX(CAST(RIGHT(TRIM(sppd_esptid), 4) AS INTEGER)), 0)
		FROM public.opr_t_esp_paddetil
		WHERE TRIM(sppd_esptid) LIKE ? AND RIGHT(TRIM(sppd_esptid), 4) ~ '^[0-9]{4}$'
	`, prefix+"%").Scan(&maxDetil)

	nextUrut := maxHeader + 1
	if maxDetil >= nextUrut {
		nextUrut = maxDetil + 1
	}
	finalSPPAD := fmt.Sprintf("%s%04d", prefix, nextUrut)

	// Gabungkan keterangan ekspedisi dengan No Resi Vendor / Kontak Sopir jika diisi
	ekspedisiFinal := strings.ToUpper(strings.TrimSpace(req.ExpedisiLain))
	if strings.TrimSpace(req.NoResiVendor) != "" {
		ekspedisiFinal += " (RESI: " + strings.ToUpper(strings.TrimSpace(req.NoResiVendor)) + ")"
	}
	if strings.TrimSpace(req.SopirVendor) != "" {
		ekspedisiFinal += " - " + strings.ToUpper(strings.TrimSpace(req.SopirVendor))
	}

	tx := database.Begin()

	if err := tx.Exec(`
		INSERT INTO public.opr_t_esp_pad (
			spp_eid, spp_asalagenid, spp_tanggal, spp_expedisilain,
			spp_surattugas, spp_loadhid, spp_aktifyn, spp_updateid, spp_updatetime
		) VALUES (?, ?, CURRENT_TIMESTAMP, ?, ?, ?, 'Y', ?, CURRENT_TIMESTAMP)
	`, finalSPPAD, asalAgenID, ekspedisiFinal, strings.ToUpper(strings.TrimSpace(req.NoSuratTugas)), strings.ToUpper(strings.TrimSpace(req.LoadHID)), userStr).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan header SP PAD: " + err.Error()})
		return
	}

	seenBTT := make(map[string]bool)
	for _, rawBTT := range req.DaftarBTT {
		cleanBTT := strings.ToUpper(strings.TrimSpace(rawBTT))
		if len(cleanBTT) > 16 {
			cleanBTT = cleanBTT[:16]
		}
		if cleanBTT == "" || seenBTT[cleanBTT] {
			continue
		}
		seenBTT[cleanBTT] = true

		tx.Exec(`DELETE FROM public.opr_t_esp_paddetil WHERE TRIM(sppd_bttid) = TRIM(?)`, cleanBTT)
		if err := tx.Exec(`
			INSERT INTO public.opr_t_esp_paddetil (sppd_esptid, sppd_bttid)
			VALUES (?, ?)
		`, finalSPPAD, cleanBTT).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menyimpan detail BTT SP PAD: " + err.Error()})
			return
		}
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("Surat Pengantar PAD %s berhasil diterbitkan!", finalSPPAD),
		"no_sp":   finalSPPAD,
	})
}

// =========================================================================
// 4. GET DETAIL SP PAD UNTUK MODAL EDIT & CETAK (opr_t_sp_pad_e.asp & _print.asp)
// GET /api/operasional/sp-pad/detail?sp_id=...
// =========================================================================
func GetDetailSPPAD(c *gin.Context) {
	database := resolveSPPADDB(c)
	spID := strings.TrimSpace(c.Query("sp_id"))
	if spID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter sp_id wajib diisi"})
		return
	}

	type SPPADDetailHeader struct {
		NoSPPAD      string `gorm:"column:no_sp_pad" json:"no_sp_pad"`
		TanggalSP    string `gorm:"column:tanggal_sp" json:"tanggal_sp"`
		AsalAgen     string `gorm:"column:asal_agen" json:"asal_agen"`
		AsalKota     string `gorm:"column:asal_kota" json:"asal_kota"`
		AsalAlamat   string `gorm:"column:asal_alamat" json:"asal_alamat"`
		AsalTelp     string `gorm:"column:asal_telp" json:"asal_telp"`
		ExpedisiLain string `gorm:"column:expedisi_lain" json:"expedisi_lain"`
		SuratTugas   string `gorm:"column:surat_tugas" json:"surat_tugas"`
		LoadHID      string `gorm:"column:loadh_id" json:"loadh_id"`
		NoMobil      string `gorm:"column:no_mobil" json:"no_mobil"`
		Sopir1       string `gorm:"column:sopir1" json:"sopir1"`
		Sopir2       string `gorm:"column:sopir2" json:"sopir2"`
		UpdateID     string `gorm:"column:update_id" json:"update_id"`
		AktifYN      string `gorm:"column:aktif_yn" json:"aktif_yn"`
	}

	var hdr SPPADDetailHeader
	if err := database.Raw(`
		SELECT 
			TRIM(h.spp_eid) AS no_sp_pad,
			TO_CHAR(h.spp_tanggal, 'DD/MM/YYYY HH24:MI:SS') AS tanggal_sp,
			COALESCE(TRIM(a1.agen_nama), '-') AS asal_agen,
			COALESCE(TRIM(a1.agen_kota), '-') AS asal_kota,
			COALESCE(TRIM(a1.agen_alamat), '-') AS asal_alamat,
			COALESCE(TRIM(a1.agen_phone1), '-') AS asal_telp,
			COALESCE(TRIM(h.spp_expedisilain), '-') AS expedisi_lain,
			COALESCE(TRIM(h.spp_surattugas), '-') AS surat_tugas,
			COALESCE(TRIM(h.spp_loadhid), '-') AS loadh_id,
			COALESCE(TRIM(sj.sjh_kendid), '-') AS no_mobil,
			COALESCE(TRIM(d1.kry_nama), TRIM(sj.sjh_sopir1_nip), '-') AS sopir1,
			COALESCE(TRIM(d2.kry_nama), TRIM(sj.sjh_sopir2_nip), '') AS sopir2,
			COALESCE(TRIM(h.spp_updateid), '-') AS update_id,
			COALESCE(TRIM(h.spp_aktifyn), 'Y') AS aktif_yn
		FROM public.opr_t_esp_pad h
		LEFT JOIN public.glb_m_agen a1 ON TRIM(h.spp_asalagenid::varchar) = TRIM(a1.agen_id::varchar)
		LEFT JOIN public.opr_t_esuratjalan sj ON TRIM(h.spp_surattugas) = TRIM(sj.sjh_id)
		LEFT JOIN public.hrd_m_karyawan d1 ON TRIM(sj.sjh_sopir1_nip) = TRIM(d1.kry_nip)
		LEFT JOIN public.hrd_m_karyawan d2 ON TRIM(sj.sjh_sopir2_nip) = TRIM(d2.kry_nip)
		WHERE TRIM(h.spp_eid) = TRIM(?)
		LIMIT 1
	`, spID).Scan(&hdr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat header SP PAD: " + err.Error()})
		return
	}

	type SPPADItemDTO struct {
		BttID      string  `gorm:"column:btt_id" json:"btt_id"`
		NoSJ       string  `gorm:"column:no_sj" json:"no_sj"`
		TglBTT     string  `gorm:"column:tgl_btt" json:"tgl_btt"`
		Pengirim   string  `gorm:"column:pengirim" json:"pengirim"`
		Penerima   string  `gorm:"column:penerima" json:"penerima"`
		Kota       string  `gorm:"column:kota" json:"kota"`
		Koli       int     `gorm:"column:koli" json:"koli"`
		BeratAsli  float64 `gorm:"column:berat_asli" json:"berat_asli"`
		BeratVol   float64 `gorm:"column:berat_vol" json:"berat_vol"`
		HargaKirim float64 `gorm:"column:harga_kirim" json:"harga_kirim"`
	}

	items := make([]SPPADItemDTO, 0)
	if err := database.Raw(`
		SELECT 
			TRIM(pd.sppd_bttid) AS btt_id,
			COALESCE(NULLIF(TRIM(e.bttt_nosuratjalan), ''), '-') AS no_sj,
			COALESCE(TO_CHAR(e.bttt_tanggal, 'DD/MM/YYYY'), '-') AS tgl_btt,
			COALESCE(TRIM(e.bttt_asalname), '-') AS pengirim,
			COALESCE(TRIM(e.bttt_tujuannama), '-') || ' - ' || COALESCE(TRIM(e.bttt_tujuankota), '') AS penerima,
			COALESCE(TRIM(e.bttt_tujuankota), '-') AS kota,
			COALESCE(NULLIF(e.bttt_jmlunit, 0), 1)::int AS koli,
			COALESCE(e.bttt_berat, 0) AS berat_asli,
			COALESCE(e.bttt_beratvol, 0) AS berat_vol,
			(COALESCE(e.bttt_harga, 0) + COALESCE(e.bttt_biayapenerus, 0)) AS harga_kirim
		FROM public.opr_t_esp_paddetil pd
		LEFT JOIN public.mkt_t_econote e ON LEFT(TRIM(pd.sppd_bttid), 16) = LEFT(TRIM(e.bttt_id), 16)
		WHERE TRIM(pd.sppd_esptid) = TRIM(?)
		ORDER BY pd.sppd_bttid ASC
	`, spID).Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat rincian BTT SP PAD: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"header": hdr,
		"items":  items,
	})
}

// =========================================================================
// 5. UPDATE / SCAN TAMBAH / HAPUS BTT PADA SP PAD EXISTING (opr_t_sp_pad_e.asp)
// PUT /api/operasional/sp-pad/update
// =========================================================================
func UpdateSPPADHandler(c *gin.Context) {
	database := resolveSPPADDB(c)
	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)

	var req struct {
		NoSPPAD      string   `json:"no_sp_pad" binding:"required"`
		ExpedisiLain string   `json:"expedisi_lain"`
		SuratTugas   string   `json:"surat_tugas"`
		AddBTT       string   `json:"add_btt"`
		RemoveBTTs   []string `json:"remove_btts"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid"})
		return
	}

	if strings.TrimSpace(req.AddBTT) != "" {
		cleanBTT := strings.ToUpper(strings.TrimSpace(req.AddBTT))
		if len(cleanBTT) > 16 {
			cleanBTT = cleanBTT[:16]
		}
		var exists int64
		database.Raw(`SELECT COUNT(*) FROM public.opr_t_esp_paddetil WHERE TRIM(sppd_esptid) = TRIM(?) AND TRIM(sppd_bttid) = ?`, req.NoSPPAD, cleanBTT).Scan(&exists)
		if exists > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": "warning", "message": fmt.Sprintf("BTT %s sudah terdaftar pada SP PAD ini!", cleanBTT)})
			return
		}
		database.Exec(`DELETE FROM public.opr_t_esp_paddetil WHERE TRIM(sppd_bttid) = TRIM(?)`, cleanBTT)
		database.Exec(`INSERT INTO public.opr_t_esp_paddetil (sppd_esptid, sppd_bttid) VALUES (?, ?)`, req.NoSPPAD, cleanBTT)
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("BTT %s berhasil ditambahkan ke SP PAD!", cleanBTT)})
		return
	}

	for _, btt := range req.RemoveBTTs {
		database.Exec(`DELETE FROM public.opr_t_esp_paddetil WHERE TRIM(sppd_esptid) = TRIM(?) AND TRIM(sppd_bttid) = TRIM(?)`, req.NoSPPAD, btt)
	}

	if strings.TrimSpace(req.ExpedisiLain) != "" {
		database.Exec(`
			UPDATE public.opr_t_esp_pad
			SET spp_expedisilain = UPPER(?),
			    spp_surattugas = UPPER(?),
			    spp_updateid = ?,
			    spp_updatetime = CURRENT_TIMESTAMP
			WHERE TRIM(spp_eid) = TRIM(?)
		`, req.ExpedisiLain, req.SuratTugas, userStr, req.NoSPPAD)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Perubahan Surat Pengantar PAD berhasil disimpan!"})
}

// =========================================================================
// 6. NON-AKTIFKAN SP PAD (p-opr_t_sp_pad_h.asp)
// DELETE /api/operasional/sp-pad/delete?sp_id=...
// =========================================================================
func DeleteSPPADHandler(c *gin.Context) {
	database := resolveSPPADDB(c)
	spID := strings.TrimSpace(c.Query("sp_id"))
	if spID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter sp_id wajib diisi"})
		return
	}

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)

	if err := database.Exec(`
		UPDATE public.opr_t_esp_pad
		SET spp_aktifyn = 'N', spp_updateid = ?, spp_updatetime = CURRENT_TIMESTAMP
		WHERE TRIM(spp_eid) = TRIM(?)
	`, userStr, spID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menonaktifkan SP PAD: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Surat Pengantar PAD berhasil dinonaktifkan!"})
}
