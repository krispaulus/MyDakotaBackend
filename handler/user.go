package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func GetAllWebLogins(c *gin.Context) {
	var users []models.WebLogin
	err := db.DB.Table("weblogin").Select(`
        username, 
        realname, 
        user_aktifyn, 
        gender, 
        all_cabangyn, 
        usertype, 
        email, 
        lastlogin, 
        profileimage, 
        kode_cabang, 
        mobilenumber, 
        nickname, 
        lastiplogin
    `).Find(&users).Error

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// 2. Looping untuk memperkaya data (Data Enrichment)
	for i := 0; i < len(users); i++ {
		if users[i].All_cabangYN == "Y" {
			// Jika ALL, kita set teks keterangannya
			users[i].KodeCabang = "PUSAT DAKOTA"
		} else {
			// Jika tidak ALL, kita ambil daftar cabang dari tabel relasi
			var cabangList []string
			db.DB.Table("weblogin_cabang").
				Where("username = ?", users[i].Username).
				Pluck("kode_cabang", &cabangList) // Ambil kolom kode_cabang saja jadi array string

			// Gabungkan jadi string "JKT, BDG, SUB" untuk tampilan tabel
			users[i].KodeCabang = strings.Join(cabangList, ", ")
		}
	}

	c.JSON(200, users)
}

func UpdateWebLogin(c *gin.Context) {
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"message": "Format data tidak valid"})
		return
	}

	// 1. Resolve DB & Username
	ptid, _ := input["pt_id"].(string)
	username, _ := input["Username"].(string)
	if username == "" {
		username, _ = input["username"].(string)
	}
	allCabangYN, _ := input["all_cabangyn"].(string)

	gormDB, ok := resolveGormDB(ptid)
	if !ok {
		c.JSON(400, gin.H{"message": "Koneksi database tidak ditemukan"})
		return
	}

	// 2. Siapkan penampung untuk string cabang (biar tabel utama rapi)
	var cabangTeks string
	if rawCabang, ok := input["kode_cabang"].([]interface{}); ok {
		var temp []string
		for _, v := range rawCabang {
			if s, ok := v.(string); ok {
				temp = append(temp, s)
			}
		}
		cabangTeks = strings.Join(temp, ", ") // Hasilnya: "CAB1, CAB2, CAB3"
	}

	tx := gormDB.Begin()

	updateData := map[string]interface{}{
		"all_cabangyn": allCabangYN,
		"kode_cabang":  cabangTeks, // <--- MASUKKAN INI supaya di pgAdmin muncul namanya
	}

	// Mapping field standar lainnya (realname, email, dll)
	if val, ok := input["real_name"].(string); ok {
		updateData["realname"] = val
	}
	if val, ok := input["mobilenumber"].(string); ok {
		updateData["mobilenumber"] = val
	}
	if val, ok := input["email"].(string); ok {
		updateData["email"] = val
	}
	if val, ok := input["Gender"].(string); ok {
		updateData["gender"] = val
	}

	if val, ok := input["gender"]; ok {
		updateData["gender"] = val
	}

	if val, ok := input["usertype"].(string); ok {
		updateData["usertype"] = val
	}
	// Atau jaga-jaga kalau payload pakai huruf besar (UserType)
	if val, ok := input["UserType"].(string); ok {
		updateData["usertype"] = val
	}

	if val, ok := input["User_aktifYN"].(string); ok {
		updateData["user_aktifyn"] = val
	}

	// 3. Eksekusi Update Tabel Utama
	if err := tx.Table("weblogin").Where("LOWER(username) = LOWER(?)", username).Updates(updateData).Error; err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"message": "Gagal update weblogin: " + err.Error()})
		return
	}

	// 4. LOGIKA CABANG KAMU (Optimasi)
	// Selalu hapus data lama di weblogin_cabang untuk username ini supaya bersih
	if err := tx.Table("weblogin_cabang").Where("LOWER(username) = LOWER(?)", username).Delete(map[string]interface{}{}).Error; err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"message": "Gagal membersihkan data cabang lama"})
		return
	}

	// Jika All Cabang = 'N' (Tidak), baru kita simpan list cabang pilihannya
	if allCabangYN == "N" {
		if rawCabang, ok := input["kode_cabang"].([]interface{}); ok {
			for _, v := range rawCabang {
				if kdBranche, ok := v.(string); ok && kdBranche != "" {
					relasi := map[string]interface{}{
						"username":    username,
						"kode_cabang": kdBranche,
					}
					if err := tx.Table("weblogin_cabang").Create(relasi).Error; err != nil {
						tx.Rollback()
						c.JSON(500, gin.H{"message": "Gagal simpan detail cabang"})
						return
					}
				}
			}
		}
	}

	// 5. COMMIT JIKA SEMUA BERHASIL
	tx.Commit()
	c.JSON(200, gin.H{"status": "success", "message": "Update berhasil dengan logika optimasi cabang"})

}

func CheckUsername(c *gin.Context) {
	username := c.Param("username")
	ptid := c.Query("pt_id")

	if ptid == "" {
		ptid = "A" // Default fallback
	}

	gormDB, ok := resolveGormDB(ptid)
	if !ok {
		c.JSON(400, gin.H{"error": "Database tidak ditemukan"})
		return
	}

	var count int64

	// Kita hitung apakah ada username yang sama di tabel webLogin
	err := gormDB.Table("weblogin").Where("LOWER(username) = LOWER(?)", username).Count(&count).Error

	if err != nil {
		c.JSON(500, gin.H{"error": "Gagal cek database"})
		return
	}

	// Jika count > 0, berarti exists = true
	c.JSON(200, gin.H{
		"exists": count > 0,
	})
}

func CreateUser(c *gin.Context) {
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"message": "Format data salah"})
		return
	}

	// 1. Resolve Database berdasarkan PT ID
	ptid, _ := input["pt_id"].(string)
	gormDB, ok := resolveGormDB(ptid)
	if !ok {
		c.JSON(400, gin.H{"message": "Database tidak ditemukan"})
		return
	}

	// 2. Mapping User Type
	userTypeInput, _ := input["UserType"].(string)
	var finalUserType string
	switch userTypeInput {
	case "Superadmin":
		finalUserType = "S"
	case "Admin":
		finalUserType = "A"
	case "Supervisor":
		finalUserType = "V"
	default:
		finalUserType = "U"
	}

	// 3. Prepare Multi-Cabang dari Frontend
	var cabangs []string
	if rawCabang, ok := input["kode_cabang"].([]interface{}); ok {
		for _, v := range rawCabang {
			if s, ok := v.(string); ok {
				cabangs = append(cabangs, s)
			}
		}
	}

	// 4. Ambil Cabang Utama untuk serverid
	mainCabang := ""
	if len(cabangs) > 0 {
		mainCabang = cabangs[0]
	}

	// 5. Cari ServerID (Agen_ID)
	var agenID string
	db.DB.Table("glb_m_agen").Select("agen_id").Where("agen_kode = ?", mainCabang).Limit(1).Scan(&agenID)
	if agenID == "" {
		agenID = "1"
	}

	// 6. Validasi & Hash Password
	rawPassword, _ := input["Passwordjwt"].(string)
	if rawPassword == "" {
		c.JSON(400, gin.H{"message": "Password wajib diisi"})
		return
	}
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	passStr := string(hashedPassword)

	// 7. Handle Gender (Smallint protection)
	var finalGender interface{}
	g, okG := input["Gender"].(string)
	if !okG {
		g, _ = input["gender"].(string)
	}
	if g != "" {
		finalGender = g
	} else {
		finalGender = nil
	}

	// 8. MULAI TRANSAKSI DATABASE (tx)
	tx := gormDB.Begin()

	// Ambil Username (pastikan tidak nil)
	usernameVal := input["Username"]
	if usernameVal == nil {
		usernameVal = input["username"]
	}

	dataBaru := map[string]interface{}{
		"username":     usernameVal,
		"realname":     input["real_name"],
		"password":     passStr,
		"passwordjwt":  passStr,
		"pt_id":        ptid,
		"mobilenumber": input["mobilenumber"],
		"email":        input["email"],
		"gender":       finalGender,
		"kode_cabang":  mainCabang,
		"user_aktifyn": input["User_aktifYN"],
		"usertype":     finalUserType,
		"serverid":     agenID,
	}

	// Simpan ke table weblogin
	if err := tx.Table("weblogin").Create(dataBaru).Error; err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"message": "Gagal simpan user: " + err.Error()})
		return
	}

	// Simpan ke table weblogin_cabang (Looping semua cabang yang dipilih)
	for _, kdBranche := range cabangs {
		relasiCabang := map[string]interface{}{
			"username":    usernameVal,
			"kode_cabang": kdBranche,
		}
		if err := tx.Table("weblogin_cabang").Create(relasiCabang).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"message": "Gagal simpan relasi cabang: " + err.Error()})
			return
		}
	}

	// COMMIT!
	tx.Commit()

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "User " + fmt.Sprintf("%v", usernameVal) + " berhasil ditambahkan dengan " + fmt.Sprintf("%d", len(cabangs)) + " akses cabang",
	})
}

func DeleteUser(c *gin.Context) {
	username := c.Param("username") // Ambil parameter username dari URL

	if username == "" {
		c.JSON(400, gin.H{"message": "Username kosong"})
		return
	}

	// Eksekusi Delete
	err := db.DB.Table("weblogin").Where("username = ?", username).Delete(nil).Error

	if err != nil {
		c.JSON(500, gin.H{"message": "Gagal hapus: " + err.Error()})
		return
	}

	fmt.Printf("\n🔥 [DELETE USER] Berhasil menghapus user dari database\n")
	fmt.Printf("Username yang dihapus: %s\n", username)
	fmt.Println("--------------------------------")

	c.JSON(200, gin.H{"status": "success", "message": "Data " + username + " terhapus"})
}

// resolveGormDB mengembalikan GORM instance berdasarkan PT_ID
func resolveGormDB(ptID string) (*gorm.DB, bool) {
	switch ptID {
	case "A":
		return db.DB, true
	case "B":
		return db.DLBDB, true
	case "C":
		return db.DLIDB, true
	default:
		return nil, false
	}
}

func SaveWebUserAccess(c *gin.Context) {
	var req models.AccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 1. Mulai Transaksi menggunakan db.DB
	tx := db.DB.Begin()

	// Dekfer fungsi untuk jaga-jaga jika terjadi panic, otomatis rollback
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for menuID, privs := range req.Permissions {
		v := 0
		if privs["view"] {
			v = 1
		}
		cr := 0
		if privs["create"] {
			cr = 1
		}
		ed := 0
		if privs["edit"] {
			ed = 1
		}
		del := 0
		if privs["delete"] {
			del = 1
		}

		query := `
			INSERT INTO webuser_access (username, menu_id, can_view, can_create, can_edit, can_delete, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, NOW())
			ON CONFLICT (username, menu_id) 
			DO UPDATE SET 
				can_view = EXCLUDED.can_view,
				can_create = EXCLUDED.can_create,
				can_edit = EXCLUDED.can_edit,
				can_delete = EXCLUDED.can_delete,
				updated_at = NOW();`

		// Eksekusi menggunakan tx (bukan db.DB langsung)
		err := tx.Exec(query,
			req.Username, menuID, // Parameter untuk SELECT source
			v, cr, ed, del, // Parameter untuk UPDATE
			v, cr, ed, del, // Parameter untuk INSERT
		).Error

		if err != nil {
			tx.Rollback() // 2. Rollback jika ada satu saja yang gagal
			c.JSON(500, gin.H{"message": "Gagal simpan akses: " + err.Error()})
			return
		}
	}

	// 3. Commit jika semua loop berhasil
	if err := tx.Commit().Error; err != nil {
		c.JSON(500, gin.H{"message": "Gagal commit ke database: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Role Access berhasil disimpan!"})
}
