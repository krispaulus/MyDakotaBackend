package handler

import (
	"dakotagroup/business-insight-be/db"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func GetProfile(c *gin.Context) {
	claimsVal, exists := c.Get("user_data")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Sesi tidak ditemukan"})
		return
	}

	claims, ok := claimsVal.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error: Gagal membaca data sesi"})
		return
	}

	username := claims["username"].(string)

	ptID, ok := claims["pt_id"].(string)
	if !ok || ptID == "" {
		ptID = "A" // fallback
	}

	activeDB, ok := db.ResolveDB(ptID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database gagal"})
		return
	}

	// 🟩 DITARIK BERSIH MENGGUNAKAN STRUCT AGAR KEBAL TYPO DAN PERBEDAAN DRIVER POSTGRES
	var dbUser WebLogin
	if err := activeDB.Table("weblogin").Where("LOWER(username) = LOWER(?)", username).First(&dbUser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan di database corporate"})
		return
	}

	// 2. AMBIL DAFTAR KODE CABANG MENGGUNAKAN VALUE DARI STRUCT dbUser
	var cabangs []string
	if strings.ToUpper(dbUser.All_cabangYN) == "Y" {
		// Jika Super Admin (Y), ambil semua kode cabang aktif dari glb_m_agen
		err := activeDB.Table("glb_m_agen").
			Where("agen_aktifyn = ?", "Y").
			Where("agen_nama NOT LIKE ?", "%XXX%").
			Pluck("agen_kode", &cabangs).Error
		if err != nil {
			cabangs = []string{}
		}
	} else {
		// Jika User Biasa (N), ambil daftar kode cabang dari weblogin_cabang
		err := activeDB.Table("weblogin_cabang").
			Where("LOWER(username) = LOWER(?)", username).
			Pluck("kode_cabang", &cabangs).Error
		if err != nil {
			cabangs = []string{}
		}
	}

	// 💥 RITUAL DUA SISI MENGGUNAKAN DATA STRUCT GORM YANG PASTI TERISI UNTUK MENYUAP FRONTEND REACT LU!
	c.JSON(http.StatusOK, gin.H{
		"username":      dbUser.Username,
		"realname":      dbUser.RealName,
		"real_name":     dbUser.RealName,
		"RealName":      dbUser.RealName, // SANGAT PENTING UNTUK HALAMAN /account REACT LU
		"mobilenumber":  dbUser.MobileNumber,
		"mobile_number": dbUser.MobileNumber,
		"MobileNumber":  dbUser.MobileNumber, // SANGAT PENTING UNTUK HALAMAN /account REACT LU
		"nickname":      dbUser.NickName,
		"nick_name":     dbUser.NickName,
		"NickName":      dbUser.NickName, // SANGAT PENTING UNTUK HALAMAN /account REACT LU
		"email":         dbUser.Email,
		"Email":         dbUser.Email,
		"profileimage":  dbUser.ProfileImage,
		"profile_image": dbUser.ProfileImage,
		"ProfileImage":  dbUser.ProfileImage, // SANGAT PENTING UNTUK HALAMAN /account REACT LU
		"cabangs":       cabangs,
		"usertype":      dbUser.UserType, // Menyediakan flag hak akses cadangan
		"role_akses":    dbUser.UserType, // KUNCI EMAS: Kembalikan ke dbUser.UserType agar menu sidebar lu jebol keluar lagi bray!
		"division":      dbUser.KodeCabang,

		// 🟩 BUNGKUS JUGA DI DALAM KEY "data" PASCALCASE BIAR MATCH 100% SAMA STATE FRONTEND LU BRAY!
		"data": gin.H{
			"Username":     dbUser.Username,
			"RealName":     dbUser.RealName,
			"NickName":     dbUser.NickName,
			"MobileNumber": dbUser.MobileNumber,
			"Email":        dbUser.Email,
			"ProfileImage": dbUser.ProfileImage,
			"gender":       dbUser.Gender,
			"kode_cabang":  dbUser.KodeCabang,
			"usertype":     dbUser.UserType,
			"cabangs":      cabangs,
		},
	})
}

// PUT /api/profile/update - 100% CLEAN & ERROR-FREE EDITION
func UpdateProfile(c *gin.Context) {
	usernameVal, _ := c.Get("username")
	ptid, _ := c.Get("pt_id")

	// 🌟 Biarkan string username apa adanya sesuai yang dikirim session token JWT (misal: "superDBS")
	usernameString := strings.TrimSpace(fmt.Sprintf("%v", usernameVal))

	// 1. Ambil data dari Multipart Form
	realName := c.PostForm("realname")
	nickName := c.PostForm("nickname")
	mobileNumber := c.PostForm("mobilenumber")
	gender := c.PostForm("gender")
	kodeCabangRaw := c.PostForm("kode_cabang")

	ptIDStr := fmt.Sprintf("%v", ptid)
	activeDB, ok := db.ResolveDB(ptIDStr)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database tidak ditemukan"})
		return
	}

	// 🔍 TARIK DATA USER SAAT INI UNTUK CEK STATUS ALL_CABANGYN & USERTYPE SEBELUM UPDATE
	var currentUser struct {
		AllCabangYN string `gorm:"column:all_cabangyn"`
		UserType    string `gorm:"column:usertype"`
	}
	activeDB.Table("weblogin").Select("all_cabangyn, usertype").Where("username ILIKE ?", usernameString).First(&currentUser)

	// =================================================================
	// 🌟 STRATEGI ANTI-SAMPAH FORMULA KODE CABANG UTAMA
	// =================================================================
	var finalKodeCabangTabelUtama string
	if currentUser.AllCabangYN == "Y" || currentUser.UserType == "S" {
		// Jika Superadmin, paksa kolom tabel utama menjadi "ALL CABANG", buang teks ratusan koma!
		finalKodeCabangTabelUtama = "PUSAT DAKOTA"
	} else {
		// Jika user biasa (N), simpan apa adanya (atau ambil indeks pertama cabang utamanya)
		finalKodeCabangTabelUtama = kodeCabangRaw
	}

	var genderInt int = 1 // default laki-laki
	if gender != "" {
		if gender == "1" || gender == "Laki-Laki" || gender == "Laki-laki" {
			genderInt = 1
		} else if gender == "2" || gender == "Perempuan" {
			genderInt = 2
		} else {
			// fallback jika berupa string angka murni dari select option
			fmt.Sscanf(gender, "%d", &genderInt)
		}
	}

	updateFields := map[string]interface{}{
		"realname":     realName,
		"nickname":     nickName,
		"mobilenumber": mobileNumber,
		"gender":       genderInt, // 🟩 Gunakan genderInt yang sudah dikonversi
		"kode_cabang":  finalKodeCabangTabelUtama,
	}

	// 2. Logika Upload File
	file, err := c.FormFile("profileimage")
	if err == nil {
		if _, err := os.Stat("uploads"); os.IsNotExist(err) {
			os.Mkdir("uploads", os.ModePerm)
		}

		ext := filepath.Ext(file.Filename)
		newFileName := fmt.Sprintf("%v_%d%v", usernameString, time.Now().Unix(), ext)
		dst := filepath.Join("uploads", newFileName)

		if err := c.SaveUploadedFile(file, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan foto profil"})
			return
		}

		baseURL := "http://localhost:8080"
		updateFields["profileimage"] = fmt.Sprintf("%s/uploads/%s", baseURL, newFileName)
	}

	// ==============================================================
	// 🌟 INTEGRASI INTEGRITAS DATABASE AMAN JEDERRR
	// ==============================================================
	tx := activeDB.Begin()

	// Gunakan kueri GORM murni dengan ILIKE (Insenstive Like) yang sangat ramah terhadap PostgreSQL case-insensitive
	result := tx.Table("weblogin").Where("username ILIKE ?", usernameString).Updates(updateFields)

	if err := result.Error; err != nil {
		tx.Rollback()
		log.Printf("❌ ERROR DATABASE UTAMA: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update profile ke database"})
		return
	}

	log.Printf("📢 [DATABASE INFO] Baris weblogin terupdate: %d", result.RowsAffected)

	// 4. Update weblogin_cabang (Gunakan ILIKE agar klop dengan kueri atas)
	if errDel := tx.Table("weblogin_cabang").Where("username ILIKE ?", usernameString).Delete(&map[string]interface{}{}).Error; errDel != nil {
		tx.Rollback()
		log.Printf("❌ ERROR SAAT CLEARING CABANG LAMA: %v", errDel)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membersihkan cabang lama"})
		return
	}

	if currentUser.AllCabangYN == "N" && currentUser.UserType != "S" && kodeCabangRaw != "" {
		cabangArray := strings.Split(kodeCabangRaw, ",")
		for _, cabang := range cabangArray {
			cleanCabang := strings.TrimSpace(cabang)
			if cleanCabang != "" {
				newCabang := map[string]interface{}{
					"username":    usernameString,
					"kode_cabang": cleanCabang,
				}
				if errIns := tx.Table("weblogin_cabang").Create(newCabang).Error; errIns != nil {
					tx.Rollback()
					log.Printf("❌ ERROR SAAT INSERT DETAIL CABANG BARU: %v", errIns)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan detail akses cabang"})
					return
				}
			}
		}
		log.Printf("💡 [CABANG] Berhasil menyimpan detail akses cabang vertikal untuk user biasa.")
	} else {
		// Jika Superadmin, bypass total pengisian detail agar tabel weblogin_cabang bersih dari ribuan baris sampah!
		log.Printf("💡 [ANTI-SAMPAH SAKTI] User %s adalah Superadmin/AllCabang Y. Proses insert detail di-bypass total!", usernameString)
	}

	// Commit seluruh perubahan data ke database corporate
	if errCommit := tx.Commit().Error; errCommit != nil {
		log.Printf("❌ ERROR TRANSACTION COMMIT: %v", errCommit)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal commit transaksi data"})
		return
	}

	log.Printf("✨ [SUCCESS] Seluruh data profil user %s berhasil diupdate total!", usernameString)

	// 🟩 AMBIL DATA TERBARU SECARA BERSIH DENGAN STRUCT UNTUK FRONTEND
	var freshUser WebLogin
	if errFetch := activeDB.Table("weblogin").Where("username ILIKE ?", usernameString).First(&freshUser).Error; errFetch == nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "Profile updated!",
			"data": gin.H{
				"Username":      freshUser.Username,
				"RealName":      freshUser.RealName,
				"NickName":      freshUser.NickName,
				"MobileNumber":  freshUser.MobileNumber,
				"Email":         freshUser.Email,
				"profileimage":  freshUser.ProfileImage,
				"profile_image": freshUser.ProfileImage,
				"ProfileImage":  freshUser.ProfileImage,
				"gender":        freshUser.Gender,
				"kode_cabang":   freshUser.KodeCabang,
				"usertype":      freshUser.UserType,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully bray!"})
}

// POST /api/profile/change-password
func ChangePassword(c *gin.Context) {
	username, _ := c.Get("username")
	ptid, _ := c.Get("pt_id")

	ptIDStr := fmt.Sprintf("%v", ptid)
	activeDB, ok := db.ResolveDB(ptIDStr)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Koneksi database tidak ditemukan"})
		return
	}

	var input struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Format data tidak valid"})
		return
	}

	var currentPassword string
	// Ambil passwordjwt yang bertipe Bcrypt dari database
	err := activeDB.Table("weblogin").Select("LTRIM(RTRIM(passwordjwt))").Where("LOWER(username) = LOWER(?)", username).Row().Scan(&currentPassword)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan"})
		return
	}

	errCompare := bcrypt.CompareHashAndPassword([]byte(currentPassword), []byte(input.OldPassword))
	if errCompare != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password lama yang kamu masukkan salah"})
		return
	}

	hashedNewPassword, errHash := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if errHash != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memproses keamanan password baru"})
		return
	}
	passStr := string(hashedNewPassword)

	// Update kedua kolom password (password dan passwordjwt) agar sinkron abadi
	errUpdate := activeDB.Table("weblogin").Where("LOWER(username) = LOWER(?)", username).Updates(map[string]interface{}{
		"password":    passStr,
		"passwordjwt": passStr,
	}).Error

	if errUpdate != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui password baru ke database"})
		return
	}

	// var currentPassword string
	// // Ambil password dari database dan bersihkan spasi
	// err := activeDB.Table("weblogin").Select("LTRIM(RTRIM(password))").Where("LOWER(username) = LOWER(?)", username).Row().Scan(&currentPassword)
	// if err != nil {
	// 	c.JSON(http.StatusNotFound, gin.H{"error": "User tidak ditemukan"})
	// 	return
	// }

	// // Cek password lama (Database menyimpan MD5)
	// hashedOldPassword := fmt.Sprintf("%x", md5.Sum([]byte(input.OldPassword)))
	// if currentPassword != hashedOldPassword {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": "Password lama salah"})
	// 	return
	// }

	// // Update ke password baru (Jangan lupa di-hash ke MD5)
	// hashedNewPassword := fmt.Sprintf("%x", md5.Sum([]byte(input.NewPassword)))
	// if err := activeDB.Table("weblogin").Where("LOWER(username) = LOWER(?)", username).Update("password", hashedNewPassword).Error; err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui password"})
	// 	return
	// }

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}
