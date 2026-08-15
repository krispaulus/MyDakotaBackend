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

// GET /api/v1/users - SAFE FOR DBS & DLI (ORDER BY USERNAME)
func GetAllWebLogins(c *gin.Context) {
	ptID, exists := c.Get("pt_id")
	ptStr := fmt.Sprintf("%v", ptID)
	if !exists || ptStr == "" || ptStr == "<nil>" {
		ptStr = "A" // Default fallback ke DBS
	}

	gormDB, ok := resolveGormDB(ptStr)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database tenant tidak terhubung"})
		return
	}

	var users []models.WebLogin

	// 🌟 KUNCI PENJINAK: Ganti "id ASC" jadi "username ASC" bray!
	err := gormDB.Table("public.weblogin").
		Where("username <> ?", "administrator").
		Order("username ASC").
		Find(&users).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal muat user: " + err.Error()})
		return
	}

	for i := 0; i < len(users); i++ {
		if strings.ToUpper(users[i].All_cabangYN) == "Y" {
			users[i].KodeCabang = "ALL CABANG"
		} else {
			var cabangList []string
			gormDB.Table("public.weblogin_cabang").
				Where("LOWER(username) = LOWER(?)", users[i].Username).
				Pluck("kode_cabang", &cabangList)

			if len(cabangList) > 0 {
				users[i].KodeCabang = strings.Join(cabangList, ", ")
			}
		}
	}

	c.JSON(http.StatusOK, users)
}

// 🔄 3. UPDATE WEB LOGIN - FULLY ADAPTIVE WITH BCRYPT PASSWORD UPDATE
func UpdateWebLogin(c *gin.Context) {
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Format data tidak valid"})
		return
	}

	ctxPTID, _ := c.Get("pt_id")
	ptid := fmt.Sprintf("%v", ctxPTID)

	username, _ := input["Username"].(string)
	if username == "" {
		username, _ = input["username"].(string)
	}
	allCabangYN, _ := input["all_cabangyn"].(string)

	gormDB, ok := resolveGormDB(ptid)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Koneksi database corporate tidak ditemukan bray!"})
		return
	}

	var cabangTeks string
	if rawCabang, ok := input["kode_cabang"].([]interface{}); ok {
		var temp []string
		for _, v := range rawCabang {
			if s, ok := v.(string); ok {
				temp = append(temp, s)
			}
		}
		cabangTeks = strings.Join(temp, ", ")
	}

	tx := gormDB.Begin()

	updateData := map[string]interface{}{
		"all_cabangyn": allCabangYN,
		"kode_cabang":  cabangTeks,
	}

	// Mapping field standar dengan aman
	if val, ok := input["real_name"].(string); ok && val != "" {
		updateData["realname"] = val
	} else if val, ok := input["RealName"].(string); ok && val != "" {
		updateData["realname"] = val
	}

	if val, ok := input["mobilenumber"].(string); ok {
		updateData["mobilenumber"] = val
	}
	if val, ok := input["email"].(string); ok {
		updateData["email"] = val
	}
	if val, ok := input["gender"]; ok {
		updateData["gender"] = val
	}
	if val, ok := input["usertype"].(string); ok {
		updateData["usertype"] = val
	}
	if val, ok := input["user_aktifyn"].(string); ok {
		updateData["user_aktifyn"] = val
	}

	// 🎯 KUNCI FIXING EDIT PASSWORD: Tangkap input password dan Hash Bcrypt!
	passInput := ""
	if p, ok := input["Passwordjwt"].(string); ok && strings.TrimSpace(p) != "" {
		passInput = strings.TrimSpace(p)
	} else if p, ok := input["Password"].(string); ok && strings.TrimSpace(p) != "" {
		passInput = strings.TrimSpace(p)
	} else if p, ok := input["password"].(string); ok && strings.TrimSpace(p) != "" {
		passInput = strings.TrimSpace(p)
	}

	if passInput != "" {
		hashedPassword, errHash := bcrypt.GenerateFromPassword([]byte(passInput), bcrypt.DefaultCost)
		if errHash == nil {
			passStr := string(hashedPassword)
			updateData["password"] = passStr
			updateData["passwordjwt"] = passStr
			fmt.Printf("🔐 [Bcrypt Update] Password user '%s' berhasil di-hash dan diupdate ke DB!\n", username)
		} else {
			fmt.Printf("❌ [Bcrypt Error]: %v\n", errHash)
		}
	}

	// Eksekusi Update Tabel Utama weblogin
	if err := tx.Table("weblogin").Where("LOWER(username) = LOWER(?)", username).Updates(updateData).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal update weblogin: " + err.Error()})
		return
	}

	// Bersihkan data cabang lama di weblogin_cabang untuk username ini
	if err := tx.Table("weblogin_cabang").Where("LOWER(username) = LOWER(?)", username).Delete(map[string]interface{}{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membersihkan data cabang lama"})
		return
	}

	if allCabangYN == "N" {
		if rawCabang, ok := input["kode_cabang"].([]interface{}); ok {
			seenCabang := make(map[string]bool)
			for _, v := range rawCabang {
				if kdBranche, ok := v.(string); ok && strings.TrimSpace(kdBranche) != "" {
					cleanBranch := strings.TrimSpace(kdBranche)
					if !seenCabang[cleanBranch] {
						seenCabang[cleanBranch] = true
						relasi := map[string]interface{}{
							"username":    strings.ToUpper(strings.TrimSpace(username)),
							"kode_cabang": cleanBranch,
						}
						if err := tx.Table("weblogin_cabang").Create(relasi).Error; err != nil {
							tx.Rollback()
							c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal simpan detail cabang: " + err.Error()})
							return
						}
					}
				}
			}
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data user " + username + " berhasil diperbarui!"})
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
		c.JSON(http.StatusBadRequest, gin.H{"message": "Format data tidak valid: " + err.Error()})
		return
	}

	// 1. Ambil PT_ID dengan Fallback bertingkat
	ptid, _ := input["pt_id"].(string)
	if ptid == "" || ptid == "<nil>" {
		if tokenPT, exists := c.Get("pt_id"); exists {
			ptid = fmt.Sprintf("%v", tokenPT)
		}
	}
	if ptid == "" || ptid == "<nil>" {
		ptid = "C" // Fallback default DLI
	}

	// 2. Resolve GORM DB sesuai PT_ID
	gormDB, ok := resolveGormDB(ptid)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Database corporate (" + ptid + ") tidak ditemukan bray!"})
		return
	}

	// 3. Mapping User Type
	userTypeInput, _ := input["UserType"].(string)
	if userTypeInput == "" {
		userTypeInput, _ = input["usertype"].(string)
	}

	var finalUserType string
	switch strings.ToUpper(userTypeInput) {
	case "SUPERADMIN", "S":
		finalUserType = "S"
	case "ADMIN", "A":
		finalUserType = "A"
	case "SUPERVISOR", "V":
		finalUserType = "V"
	default:
		finalUserType = "U"
	}

	// 4. Multi-Cabang + 🌟 DEDUPLIKASI ARRAY (PENJINAK DUPLIKAT KEY!)
	var cabangs []string
	seenCabang := make(map[string]bool)

	if rawCabang, ok := input["kode_cabang"].([]interface{}); ok {
		for _, v := range rawCabang {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				cleanCode := strings.TrimSpace(s)
				// Cek apakah kode cabang ini sudah pernah dimasukkan ke map
				if !seenCabang[cleanCode] {
					seenCabang[cleanCode] = true
					cabangs = append(cabangs, cleanCode)
				}
			}
		}
	}

	mainCabang := "PUSAT DAKOTA"
	if len(cabangs) > 0 {
		mainCabang = cabangs[0]
	}

	// 5. Cari ServerID
	var agenID string
	gormDB.Table("public.glb_m_agen").Select("agen_id").Where("agen_kode = ?", mainCabang).Limit(1).Scan(&agenID)
	if agenID == "" {
		agenID = "1"
	}

	// 6. Validasi & Hash Password
	rawPassword, _ := input["Passwordjwt"].(string)
	if rawPassword == "" {
		rawPassword, _ = input["Password"].(string)
	}
	if rawPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Password wajib diisi bray!"})
		return
	}
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	passStr := string(hashedPassword)

	// 7. Handle Gender & All Cabang YN
	genderVal := input["gender"]
	if genderVal == nil {
		genderVal = input["Gender"]
	}

	allCabangYN, _ := input["all_cabangyn"].(string)
	if allCabangYN == "" {
		allCabangYN = "N"
	}

	// 8. Username Normalization
	usernameVal := input["Username"]
	if usernameVal == nil || fmt.Sprintf("%v", usernameVal) == "" {
		usernameVal = input["username"]
	}
	cleanUsername := strings.TrimSpace(fmt.Sprintf("%v", usernameVal))

	// Realname
	realNameVal := input["RealName"]
	if realNameVal == nil || fmt.Sprintf("%v", realNameVal) == "" {
		realNameVal = input["real_name"]
	}

	// 9. MULAI TRANSAKSI DATABASE
	tx := gormDB.Begin()

	dataBaru := map[string]interface{}{
		"username":     cleanUsername,
		"realname":     realNameVal,
		"password":     passStr,
		"passwordjwt":  passStr,
		"pt_id":        ptid,
		"mobilenumber": input["mobilenumber"],
		"email":        input["email"],
		"gender":       genderVal,
		"kode_cabang":  mainCabang,
		"user_aktifyn": "Y",
		"usertype":     finalUserType,
		"all_cabangyn": allCabangYN,
		"serverid":     agenID,
	}

	// Insert ke public.weblogin
	if err := tx.Table("public.weblogin").Create(dataBaru).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal simpan user ke DB: " + err.Error()})
		return
	}

	// Insert ke public.weblogin_cabang jika bukan ALL CABANG (Gunakan array cabangs yang sudah BERSIH & UNIK!)
	if allCabangYN == "N" {
		for _, kdBranche := range cabangs {
			relasiCabang := map[string]interface{}{
				"username":    cleanUsername,
				"kode_cabang": kdBranche,
			}
			if err := tx.Table("public.weblogin_cabang").Create(relasiCabang).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal simpan relasi cabang: " + err.Error()})
				return
			}
		}
	}

	tx.Commit()

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": fmt.Sprintf("User %s berhasil ditambahkan ke PT %s!", cleanUsername, ptid),
	})
}

// 🗑️ DELETE USER - UNIVERSAL (PARMA URL / BODY JSON)

func DeleteUser(c *gin.Context) {
	username := strings.TrimSpace(c.Param("username"))

	// Jika username tidak dikirim via URL Param, baca dari JSON Body
	if username == "" {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err == nil {
			if u, ok := body["username"].(string); ok {
				username = strings.TrimSpace(u)
			} else if u, ok := body["cust_id"].(string); ok {
				username = strings.TrimSpace(u)
			}
		}
	}

	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Username target hapus kosong bray!"})
		return
	}

	ctxPTID, _ := c.Get("pt_id")
	ptid := fmt.Sprintf("%v", ctxPTID)

	gormDB, ok := resolveGormDB(ptid)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Database tidak ditemukan bray!"})
		return
	}

	tx := gormDB.Begin()

	// 1. Hapus dari tabel utama
	if err := tx.Table("weblogin").Where("LOWER(username) = LOWER(?)", username).Delete(map[string]interface{}{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal hapus weblogin: " + err.Error()})
		return
	}

	// 2. Hapus dari tabel relasi cabang biar bersih tanpa ampas data
	if err := tx.Table("weblogin_cabang").Where("LOWER(username) = LOWER(?)", username).Delete(map[string]interface{}{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal hapus relasi cabang: " + err.Error()})
		return
	}

	tx.Commit()

	fmt.Printf("\n🔥 [DELETE USER TENANT %s] Sukses membersihkan user: %s\n", ptid, username)
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Data user " + username + " resmi dimusnahkan bray!"})
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
