package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dakotagroup/business-insight-be/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func resolveSPDB(c *gin.Context) *gorm.DB {
	ptID, _ := c.Get("pt_id")
	database, ok := db.ResolveDB(fmt.Sprintf("%v", ptID))
	if !ok {
		return db.DB
	}
	return database
}

// 1. GET LIST HISTORI SURAT PENGIRIMAN (Sesuai opr_t_sp_terima.asp)
func GetSuratPengantarFetch(c *gin.Context) {
	database := resolveSPDB(c)

	tglDari := strings.TrimSpace(c.Query("tanggal_awal"))
	tglSampai := strings.TrimSpace(c.Query("tanggal_akhir"))
	agenID := strings.TrimSpace(c.Query("agen_id"))
	tujuan := strings.TrimSpace(c.Query("tujuan"))
	noSP := strings.TrimSpace(c.Query("no_sp"))
	noBTT := strings.TrimSpace(c.Query("no_btt"))
	transit := strings.TrimSpace(c.Query("transit"))
	noMobil := strings.TrimSpace(c.Query("no_mobil"))

	filterClause := ""
	var args []interface{}

	if tglDari != "" && tglSampai != "" {
		filterClause += " AND CAST(s.spt_tanggal AS DATE) BETWEEN ? AND ? "
		args = append(args, tglDari, tglSampai)
	}
	if noSP != "" {
		filterClause += " AND UPPER(TRIM(s.spt_eid)) LIKE UPPER(?) "
		args = append(args, "%"+noSP+"%")
	}
	if noMobil != "" {
		filterClause += " AND UPPER(TRIM(s.spt_nomobil)) LIKE UPPER(?) "
		args = append(args, "%"+noMobil+"%")
	}
	if agenID != "" && !strings.Contains(strings.ToUpper(agenID), "PUSAT") && !strings.Contains(strings.ToUpper(agenID), "HOLDING") && strings.ToUpper(agenID) != "ALL" {
		filterClause += " AND (UPPER(a1.agen_nama) LIKE UPPER(?) OR TRIM(CAST(s.spt_asalagenid AS VARCHAR)) = ?) "
		args = append(args, "%"+agenID+"%", agenID)
	}
	if tujuan != "" {
		filterClause += " AND (UPPER(a2.agen_nama) LIKE UPPER(?) OR UPPER(TRIM(CAST(s.spt_tujuanagenid AS VARCHAR))) LIKE UPPER(?)) "
		args = append(args, "%"+tujuan+"%", "%"+tujuan+"%")
	}
	if transit != "" {
		filterClause += " AND UPPER(TRIM(s.spt_transityn)) = UPPER(?) "
		args = append(args, transit)
	}
	if noBTT != "" {
		filterClause += " AND EXISTS (SELECT 1 FROM public.opr_t_esp_terimadetil d WHERE TRIM(d.sptd_esptid) = TRIM(s.spt_eid) AND UPPER(TRIM(d.sptd_bttid)) LIKE UPPER(?)) "
		args = append(args, "%"+noBTT+"%")
	}

	queryStr := fmt.Sprintf(`
		SELECT 
			TRIM(COALESCE(s.spt_eid, '-')) AS no_sp,
			TO_CHAR(s.spt_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS tanggal,
			CASE 
				WHEN UPPER(TRIM(a1.agen_nama)) = 'PUSAT DAKOTA' OR TRIM(CAST(s.spt_asalagenid AS VARCHAR)) IN ('1', '001', 'PST001') THEN 'DLI PUSAT'
				ELSE COALESCE(TRIM(a1.agen_nama), TRIM(CAST(s.spt_asalagenid AS VARCHAR)), '-')
			END AS asal,
			CASE 
				WHEN UPPER(TRIM(a2.agen_nama)) = 'PUSAT DAKOTA' OR TRIM(CAST(s.spt_tujuanagenid AS VARCHAR)) IN ('1', '001', 'PST001') THEN 'DLI PUSAT'
				ELSE COALESCE(TRIM(a2.agen_nama), TRIM(CAST(s.spt_tujuanagenid AS VARCHAR)), '-')
			END AS tujuan,
			COALESCE(TRIM(s.spt_transityn), 'N') AS spt_transityn,
			COALESCE(TRIM(s.spt_nomobil), '-') AS no_mobil,
			COALESCE(TRIM(s.spt_namasopir), '-') AS sopir,
			COALESCE(TRIM(s.spt_surattugas), '-') AS surat_tugas,
			COALESCE(TRIM(s.spt_boronganyn), 'N') AS borongan_yn,
			COALESCE(NULLIF(TRIM(CAST(s.spt_service AS VARCHAR)), ''), '1') AS service,
			COALESCE(TRIM(s.spt_aktifyn), 'Y') AS aktif,
			COUNT(d.sptd_bttid)::int AS jmlbtt,
			COALESCE(SUM(c.bttt_tagihtujuan), 0) AS jmlcod
		FROM public.opr_t_esp_terima s
		LEFT JOIN public.opr_t_esp_terimadetil d ON TRIM(s.spt_eid) = TRIM(d.sptd_esptid)
		LEFT JOIN public.mkt_t_econote c ON TRIM(d.sptd_bttid) = TRIM(c.bttt_id)
		LEFT JOIN public.glb_m_agen a1 ON TRIM(CAST(s.spt_asalagenid AS VARCHAR)) = TRIM(CAST(a1.agen_id AS VARCHAR))
		LEFT JOIN public.glb_m_agen a2 ON TRIM(CAST(s.spt_tujuanagenid AS VARCHAR)) = TRIM(CAST(a2.agen_id AS VARCHAR))
		WHERE s.spt_eid IS NOT NULL %s
		GROUP BY s.spt_eid, s.spt_tanggal, a1.agen_nama, s.spt_asalagenid, a2.agen_nama, s.spt_tujuanagenid, s.spt_transityn, s.spt_nomobil, s.spt_namasopir, s.spt_surattugas, s.spt_boronganyn, s.spt_service, s.spt_aktifyn
		ORDER BY s.spt_tanggal DESC, s.spt_eid DESC
	`, filterClause)

	results := make([]map[string]interface{}, 0)
	if err := database.Raw(queryStr, args...).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal memuat data SP: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": results})
}

// 2. GET DETAIL SP & RINCIAN BTT UNTUK MODAL EDIT / SCAN BARCODE (opr_t_sp_terima_e.asp)
func GetDetailSPByID(c *gin.Context) {
	database := resolveSPDB(c)
	spID := strings.TrimSpace(c.Query("sp_id"))
	if spID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter sp_id wajib diisi"})
		return
	}

	var header map[string]interface{}
	database.Raw(`
		SELECT 
			TRIM(s.spt_eid) AS no_sp,
			TO_CHAR(s.spt_tanggal, 'YYYY-MM-DD HH24:MI:SS') AS tanggal,
			TRIM(COALESCE(s.spt_asalagenid::varchar, '')) AS asal_id,
			COALESCE(TRIM(a1.agen_nama), '-') AS asal_nama,
			TRIM(COALESCE(s.spt_tujuanagenid::varchar, '')) AS tujuan_id,
			COALESCE(TRIM(a2.agen_nama), '-') AS tujuan_nama,
			COALESCE(TRIM(s.spt_transityn), 'N') AS transit_yn,
			COALESCE(TRIM(s.spt_nomobil), '') AS no_mobil,
			COALESCE(TRIM(s.spt_namasopir), '') AS nama_sopir,
			COALESCE(TRIM(s.spt_surattugas), '') AS surat_tugas,
			COALESCE(TRIM(s.spt_boronganyn), 'N') AS borongan_yn,
			COALESCE(NULLIF(TRIM(CAST(s.spt_service AS VARCHAR)), ''), '1') AS service,
			COALESCE(TRIM(s.spt_aktifyn), 'Y') AS aktif_yn
		FROM public.opr_t_esp_terima s
		LEFT JOIN public.glb_m_agen a1 ON TRIM(s.spt_asalagenid::varchar) = TRIM(a1.agen_id::varchar)
		LEFT JOIN public.glb_m_agen a2 ON TRIM(s.spt_tujuanagenid::varchar) = TRIM(a2.agen_id::varchar)
		WHERE TRIM(s.spt_eid) = TRIM(?)
		LIMIT 1
	`, spID).Scan(&header)

	details := make([]map[string]interface{}, 0)
	database.Raw(`
		SELECT 
			TRIM(d.sptd_bttid) AS btt_id,
			COALESCE(TRIM(e.bttt_nosuratjalan), '-') AS no_sj,
			COALESCE(TO_CHAR(e.bttt_tanggal, 'DD/MM/YYYY'), '-') AS tgl_btt,
			COALESCE(TRIM(e.bttt_asalname), '-') AS pengirim,
			COALESCE(TRIM(e.bttt_tujuannama), '-') AS penerima,
			COALESCE(TRIM(e.bttt_tujuankota), '-') AS kota,
			COALESCE(TRIM(e.bttt_tujuantelp), '-') AS telp,
			COALESCE(d.sptd_jml, e.bttt_jmlunit, 1) AS jumlah,
			GREATEST(COALESCE(e.bttt_berat, 0), COALESCE(e.bttt_beratvol, 0)) AS berat,
			COALESCE(e.bttt_tagihtujuan, 0) AS cod,
			EXISTS(SELECT 1 FROM public.opr_t_esp_turun t WHERE TRIM(t.sp_eid) = TRIM(d.sptd_esptid) AND TRIM(t.sp_bttid) = TRIM(d.sptd_bttid)) AS sudah_turun
		FROM public.opr_t_esp_terimadetil d
		LEFT JOIN public.mkt_t_econote e ON TRIM(d.sptd_bttid) = TRIM(e.bttt_id)
		WHERE TRIM(d.sptd_esptid) = TRIM(?)
		ORDER BY d.sptd_bttid ASC
	`, spID).Scan(&details)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"header":  header,
		"details": details,
	})
}

// 3. AMBIL DATA DARI PROSES LOADING BARANG (opr_t_sp_terima_f.asp / opr_t_sp_terima_a.asp)
func GetLoadingReadyForSP(c *gin.Context) {
	database := resolveSPDB(c)
	loadID := strings.TrimSpace(c.Query("load_id"))
	if loadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Nomor Loading (LoadH_ID) wajib diisi"})
		return
	}

	type LoadHeaderDTO struct {
		LoadID         string `gorm:"column:load_id" json:"load_id"`
		SuratTugas     string `gorm:"column:surat_tugas" json:"surat_tugas"`
		NoMobil        string `gorm:"column:no_mobil" json:"no_mobil"`
		Sopir1         string `gorm:"column:sopir1" json:"sopir1"`
		Sopir2         string `gorm:"column:sopir2" json:"sopir2"`
		TujuanAgenID   string `gorm:"column:tujuan_agen_id" json:"tujuan_agen_id"`
		TujuanAgenNama string `gorm:"column:tujuan_agen_nama" json:"tujuan_agen_nama"`
	}

	var loadHeader LoadHeaderDTO
	err := database.Raw(`
		SELECT 
			TRIM(l.loadh_id) AS load_id,
			TRIM(COALESCE(s.sjh_id, '')) AS surat_tugas,
			TRIM(COALESCE(NULLIF(TRIM(l.loadh_kendid), ''), TRIM(s.sjh_kendid), '')) AS no_mobil,
			COALESCE(TRIM(sp1.kry_nama), TRIM(s.sjh_sopir1_nip), '') AS sopir1,
			COALESCE(TRIM(sp2.kry_nama), TRIM(s.sjh_sopir2_nip), '') AS sopir2,
			TRIM(COALESCE(s.sjh_endagenid::varchar, '')) AS tujuan_agen_id,
			COALESCE(TRIM(a2.agen_nama), '') AS tujuan_agen_nama
		FROM public.opr_t_loadingh l
		LEFT JOIN public.opr_t_esuratjalan s ON TRIM(l.loadh_id) = TRIM(s.sjh_loadhid)
		LEFT JOIN public.hrd_m_karyawan sp1 ON TRIM(s.sjh_sopir1_nip) = TRIM(sp1.kry_nip)
		LEFT JOIN public.hrd_m_karyawan sp2 ON TRIM(s.sjh_sopir2_nip) = TRIM(sp2.kry_nip)
		LEFT JOIN public.glb_m_agen a2 ON TRIM(s.sjh_endagenid::varchar) = TRIM(a2.agen_id::varchar)
		WHERE UPPER(TRIM(l.loadh_id)) = UPPER(TRIM(?))
		   OR UPPER(TRIM(s.sjh_id)) = UPPER(TRIM(?))
		LIMIT 1
	`, loadID, loadID).Scan(&loadHeader).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal query loading: " + err.Error()})
		return
	}

	if loadHeader.LoadID == "" {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": fmt.Sprintf("Nomor Loading / Surat Tugas [%s] tidak ditemukan!", loadID)})
		return
	}

	type LoadBTTDTO struct {
		BttID    string  `gorm:"column:btt_id" json:"btt_id"`
		NoSJ     string  `gorm:"column:no_sj" json:"no_sj"`
		TglBTT   string  `gorm:"column:tgl_btt" json:"tgl_btt"`
		Pengirim string  `gorm:"column:pengirim" json:"pengirim"`
		Penerima string  `gorm:"column:penerima" json:"penerima"`
		Alamat   string  `gorm:"column:alamat" json:"alamat"`
		Kota     string  `gorm:"column:kota" json:"kota"`
		Telp     string  `gorm:"column:telp" json:"telp"`
		Koli     int     `gorm:"column:koli" json:"koli"`
		Berat    float64 `gorm:"column:berat" json:"berat"`
	}

	bttRows := make([]LoadBTTDTO, 0)
	database.Raw(`
    SELECT DISTINCT
        TRIM(e.bttt_id) AS btt_id,
        COALESCE(TRIM(e.bttt_nosuratjalan), '-') AS no_sj,
        COALESCE(TO_CHAR(e.bttt_tanggal, 'DD/MM/YYYY'), '-') AS tgl_btt,
        COALESCE(TRIM(e.bttt_asalname), '-') AS pengirim,
        COALESCE(TRIM(e.bttt_tujuannama), '-') AS penerima,
        COALESCE(TRIM(e.bttt_tujuanalamat), '-') AS alamat,
        COALESCE(TRIM(e.bttt_tujuankota), '-') AS kota,
        COALESCE(TRIM(e.bttt_tujuantelp), '-') AS telp,
        COALESCE(NULLIF(e.bttt_jmlunit, 0), 1)::int AS koli,
        GREATEST(COALESCE(e.bttt_berat, 0), COALESCE(e.bttt_beratvol, 0)) AS berat
    FROM public.opr_t_loadingd d
    INNER JOIN public.mkt_t_econote e ON TRIM(e.bttt_id) = SPLIT_PART(TRIM(d.loadd_bttkoliid), '-', 1)
                                      OR LEFT(TRIM(d.loadd_bttkoliid), LENGTH(TRIM(e.bttt_id))) = TRIM(e.bttt_id)
    WHERE TRIM(d.loadd_loadhid) = TRIM(?)
    ORDER BY btt_id ASC
	`, loadHeader.LoadID).Scan(&bttRows)

	// Jika tujuan_agen_nama kosong (karena loading dibuat langsung tanpa ST), ambil dari tujuan resi pertama
	if loadHeader.TujuanAgenNama == "" && len(bttRows) > 0 {
		database.Raw(`
			SELECT COALESCE(TRIM(a.agen_nama), '')
			FROM public.mkt_t_econote e
			LEFT JOIN public.glb_m_agen a ON TRIM(a.agen_id::varchar) = TRIM(e.bttt_tujuanagenid::varchar)
			WHERE TRIM(e.bttt_id) = TRIM(?)
			LIMIT 1
		`, bttRows[0].BttID).Scan(&loadHeader.TujuanAgenNama)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"header": loadHeader,
		"btts":   bttRows,
	})
}

// 4. TAMBAH / HAPUS BTT PADA SP EXISTING & UPDATE HEADER SP (opr_t_sp_terima_e.asp)
func UpdateSPTerimaHandler(c *gin.Context) {
	database := resolveSPDB(c)
	var req struct {
		SpID           string   `json:"no_sp" binding:"required"`
		TujuanAgenNama string   `json:"tujuan_nama"`
		TransitYN      string   `json:"transit_yn"`
		NamaSopir      string   `json:"nama_sopir"`
		NoMobil        string   `json:"no_mobil"`
		SuratTugas     string   `json:"surat_tugas"`
		BoronganYN     string   `json:"borongan_yn"`
		AddBTT         string   `json:"add_btt"`
		RemoveBTTs     []string `json:"remove_btts"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Input tidak valid"})
		return
	}

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)

	// Jika ada penambahan 1 BTT (scan barcode di modal edit)
	if strings.TrimSpace(req.AddBTT) != "" {
		cleanBTT := strings.ToUpper(strings.TrimSpace(req.AddBTT))
		if len(cleanBTT) > 16 {
			cleanBTT = cleanBTT[:16]
		}
		var exists int64
		database.Raw(`SELECT COUNT(*) FROM public.opr_t_esp_terimadetil WHERE TRIM(sptd_esptid) = TRIM(?) AND TRIM(sptd_bttid) = ?`, req.SpID, cleanBTT).Scan(&exists)
		if exists > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": "warning", "message": fmt.Sprintf("BTT %s sudah terdaftar pada SP ini!", cleanBTT)})
			return
		}
		database.Exec(`INSERT INTO public.opr_t_esp_terimadetil (sptd_esptid, sptd_bttid) VALUES (?, ?)`, req.SpID, cleanBTT)
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": fmt.Sprintf("BTT %s berhasil ditambahkan ke SP!", cleanBTT)})
		return
	}

	// Hapus BTT yang dicentang (hanya jika belum diturunkan di opr_t_esp_turun)
	for _, btt := range req.RemoveBTTs {
		database.Exec(`
			DELETE FROM public.opr_t_esp_terimadetil
			WHERE TRIM(sptd_esptid) = TRIM(?) AND TRIM(sptd_bttid) = TRIM(?)
			  AND NOT EXISTS (SELECT 1 FROM public.opr_t_esp_turun t WHERE TRIM(t.sp_eid) = TRIM(?) AND TRIM(t.sp_bttid) = TRIM(?))
		`, req.SpID, btt, req.SpID, btt)
	}

	// Update header SP
	if req.NoMobil != "" {
		var tujuanID string
		database.Raw(`SELECT agen_id FROM public.glb_m_agen WHERE UPPER(agen_nama) LIKE UPPER(?) LIMIT 1`, "%"+req.TujuanAgenNama+"%").Scan(&tujuanID)
		database.Exec(`
			UPDATE public.opr_t_esp_terima
			SET spt_tujuanagenid = COALESCE(NULLIF(?, ''), spt_tujuanagenid),
			    spt_transityn = ?,
			    spt_namasopir = UPPER(?),
			    spt_nomobil = UPPER(?),
			    spt_surattugas = UPPER(?),
			    spt_boronganyn = ?,
			    spt_updateid = ?,
			    spt_updatetime = CURRENT_TIMESTAMP
			WHERE TRIM(spt_eid) = TRIM(?)
		`, tujuanID, req.TransitYN, req.NamaSopir, req.NoMobil, req.SuratTugas, req.BoronganYN, userStr, req.SpID)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Surat Pengantar berhasil diperbarui!"})
}

// 5. NON-AKTIFKAN / HAPUS SURAT PENGANTAR (p-opr_t_sp_terima_h.asp)
func DeleteSPTerimaHandler(c *gin.Context) {
	database := resolveSPDB(c)
	spID := strings.TrimSpace(c.Query("sp_id"))
	if spID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Parameter sp_id wajib diisi"})
		return
	}

	username, _ := c.Get("username")
	userStr := fmt.Sprintf("%v", username)

	if err := database.Exec(`
		UPDATE public.opr_t_esp_terima
		SET spt_aktifyn = 'N', spt_updateid = ?, spt_updatetime = CURRENT_TIMESTAMP
		WHERE TRIM(spt_eid) = TRIM(?)
	`, userStr, spID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menonaktifkan SP: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Surat Pengantar berhasil dinonaktifkan!"})
}
