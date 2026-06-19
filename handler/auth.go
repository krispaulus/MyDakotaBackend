package handler

import (
	"crypto/md5"
	"dakotagroup/business-insight-be/db"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	PTID     string `json:"pt_id"`
}

type LoginResponse struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
	PTID  string   `json:"pt_id"`
}

type UserInfo struct {
	Username     string   `json:"username"`
	RealName     string   `json:"realname"`
	Email        string   `json:"email"`
	PTID         string   `json:"pt_id"`
	UserType     string   `json:"usertype"` // Ini penampung S, A, V, atau U dari DB
	All_cabangYN string   `json:"all_cabangyn"`
	Cabangs      []string `json:"cabangs"`
	ProfileImage string   `json:"profileimage"`
}

type OTPRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	OTPCode  string `json:"otp_code"`
	PTID     string `json:"pt_id"`
}

type WebLogin struct {
	Username     string `gorm:"column:username;primaryKey" json:"username"`
	RealName     string `gorm:"column:realname" json:"realname"`
	NickName     string `gorm:"column:nickname" json:"nickname"`
	MobileNumber string `gorm:"column:mobilenumber" json:"mobilenumber"`
	Gender       int    `gorm:"column:gender" json:"gender"`
	KodeCabang   string `gorm:"column:kode_cabang" json:"kode_cabang"`
	All_cabangYN string `gorm:"column:all_cabangyn" json:"all_cabangyn"`
	LastLogin    string `gorm:"column:lastlogin" json:"lastlogin"`
	LastIPlogin  string `gorm:"column:lastiplogin" json:"lastiplogin"`
	ProfileImage string `gorm:"column:profileimage" json:"profileimage"`

	Email string `gorm:"column:email" json:"email"`
}

func sendOTPMail(targetEmail string, code string) error {
	from := "admin.dakota@gmail.com" // Ganti dengan email perusahaan
	password := "your-app-password"  // Gunakan App Password Google

	to := []string{targetEmail}
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	message := []byte("Subject: OTP Verifikasi Email Dakota\r\n" +
		"\r\n" +
		"Kode OTP Anda adalah: " + code + "\r\n")

	auth := smtp.PlainAuth("", from, password, smtpHost)
	return smtp.SendMail(smtpHost+":"+smtpPort, auth, from, to, message)
}

func RequestOTPHandler(c *gin.Context) {
	var req OTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body"})
		return
	}

	conn, ok := resolveDB(req.PTID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid pt_id"})
		return
	}

	var exists int
	checkQuery := "SELECT COUNT(*) FROM weblogin WHERE username = ?"
	// checkQuery := "SELECT COUNT(*) FROM webLogin WHERE LTRIM(RTRIM(username)) = @user"
	// err := conn.QueryRow(checkQuery, sql.Named("user", req.Username)).Scan(&exists)
	err := conn.QueryRow(checkQuery, req.Username).Scan(&exists)

	if err != nil {
		log.Printf("DB Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal kirim email"})
		return
	}

	if exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "Username tidak ditemukan"})
		return
	}

	// 2. Generate Kode OTP (6 digit)
	otpCode := fmt.Sprintf("%06d", rand.Intn(1000000))
	if err := sendOTPMail(req.Email, otpCode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal kirim email"})
		return
	}

	// updateQuery := "UPDATE webLogin SET userToken = @otp WHERE username = @user"
	updateQuery := "UPDATE weblogin SET usertoken = ? WHERE username = ?"
	_, errUpdate := conn.Exec(updateQuery, sql.Named("otp", otpCode), sql.Named("user", req.Username))
	if errUpdate != nil {
		log.Printf("Gagal simpan OTP ke DB: %v", errUpdate)
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP terkirim ke " + req.Email})
}

func VerifyAndSaveEmailHandler(c *gin.Context) {
	var req OTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	conn, ok := resolveDB(req.PTID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid pt_id"})
		return
	}

	// 1. Cek OTP di Database
	var dbOTP string
	query := "SELECT usertoken FROM weblogin WHERE username = ?" // Gunakan usertoken (huruf kecil)
	err := conn.QueryRow(query, req.Username).Scan(&dbOTP)

	if err != nil || req.OTPCode != dbOTP {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Kode OTP Salah atau Kadaluarsa!"})
		return
	}

	// 2. OTP COCOK -> Simpan Email Permanen & Hapus Token (Set NULL)
	// Query ini harus mengupdate email dan mengosongkan token
	updateQuery := "UPDATE weblogin SET email = ?, usertoken = NULL WHERE username = ?"
	_, errUpdate := conn.Exec(updateQuery, req.Email, req.Username)

	if errUpdate != nil {
		log.Printf("Gagal update email ke DB: %v", errUpdate)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal update database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email berhasil didaftarkan!"})
}

func md5Hash(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

func resolveDB(ptID string) (*sql.DB, bool) {
	switch ptID {
	case "A":
		return db.DBS, true
	case "B":
		return db.DLB, true
	case "C":
		return db.DLI, true
	default:
		return nil, false
	}
}

func LoginHandler(c *gin.Context) {
	fmt.Println("\n==================================")
	fmt.Println("🚀 REQUEST LOGIN MASUK KE HANDLER")
	fmt.Println("==================================")

	// 1. AMBIL INPUT DARI JSON
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Format data tidak valid"})
		return
	}

	// Koneksi DB sesuai PTID
	conn, ok := resolveDB(req.PTID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "PT ID tidak valid"})
		return
	}

	// 2. QUERY DATABASE (Deklarasikan variabel yang tadi undefined)
	var dbPassword, aktif string
	var user UserInfo // Ini variabel 'user' yang tadi undefined

	queryUser := `SELECT 
                    username, 
                    password, 
                    realname, 
                    user_aktifyn, 
                    COALESCE(email, '') as email, 
                    COALESCE(usertype, 'U') as usertype, 
					COALESCE(all_cabangyn, 'N') as all_cabangyn,
					COALESCE(kode_cabang, 'PUSAT DAKOTA') as kode_cabang
                  FROM weblogin 
                  WHERE username = $1 OR email = $2`

	var defaultKodeCabang string

	err := conn.QueryRow(queryUser, req.Email, req.Email).Scan(
		&user.Username,
		&dbPassword,
		&user.RealName,
		&aktif,
		&user.Email,
		&user.UserType,
		&user.All_cabangYN,
		&defaultKodeCabang,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "User tidak ditemukan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database error"})
		return
	}

	// 3. CEK PASSWORD MENGGUNAKAN BCRYPT
	err = bcrypt.CompareHashAndPassword([]byte(dbPassword), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Password yang kamu masukkan tidak sesuai",
		})
		return
	}

	// 4. CEK STATUS AKTIF
	if aktif != "Y" {
		c.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Akun tidak aktif"})
		return
	}

	// ==========================================
	// LOGIKA OPTIMASI CABANG
	// ==========================================
	var userCabangs []string

	if user.All_cabangYN == "Y" {
		// Jika 'Y', ambil semua dari master agen
		rows, _ := conn.Query("SELECT agen_kode FROM glb_m_agen")
		defer rows.Close()
		for rows.Next() {
			var code string
			rows.Scan(&code)
			userCabangs = append(userCabangs, code)
		}
	} else {
		// Jika 'N', ambil dari tabel relasi weblogin_cabang
		rows, _ := conn.Query("SELECT kode_cabang FROM weblogin_cabang WHERE username = $1", user.Username)
		defer rows.Close()
		for rows.Next() {
			var code string
			rows.Scan(&code)
			userCabangs = append(userCabangs, code)
		}
	}

	// Masukkan ke field penampung (Cabangs)
	user.Cabangs = userCabangs

	// 5. JWT PROCESS
	secret := os.Getenv("JWT_SECRET")
	// if secret == "" {
	// 	secret = os.Getenv("JWT_SECRET")
	// }

	expirationTime := time.Now().Add(24 * time.Hour).Unix()

	claims := jwt.MapClaims{
		"username":   user.Username,
		"cabangs":    user.Cabangs,
		"pt_id":      req.PTID,
		"user_type":  user.UserType,
		"agent_code": defaultKodeCabang,
		"exp":        expirationTime, // Gunakan variabel expirationTime di sini
		"iat":        time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat token"})
		return
	}

	// 6. RESPONSE FINAL
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"token":  tokenString,
		"pt_id":  req.PTID,
		"user": gin.H{
			"username":     user.Username,
			"real_name":    user.RealName,
			"email":        user.Email,
			"user_type":    user.UserType,
			"all_cabangyn": user.All_cabangYN,
			"profileimage": user.ProfileImage,
			"agent_code":   defaultKodeCabang, // Dioper ke frontend bro!
		},
	})
}

// Fungsi pembantu untuk membersihkan spasi jika diperlukan saat pengecekan string
func LTRIM_RTRIM(s string) string {
	return fmt.Sprintf("%s", s) // Sesuaikan jika database kamu mengembalikan trailing spaces
}

func UpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Hanya izinkan method PUT atau POST
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req WebLogin
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "Format data tidak valid"})
		return
	}

	// Ambil PTID (asumsi dikirim di header atau body,
	// karena struct WebLogin lo tidak ada PTID, kita ambil manual atau tambahkan di payload)
	// Untuk sementara kita pakai default PT A, atau sesuaikan dengan logika PT lo:
	ptID := r.URL.Query().Get("pt_id")
	conn, ok := resolveDB(ptID)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"message": "pt_id tidak valid"})
		return
	}

	// Query Update menggunakan tag 'gorm' column yang ada di struct lo
	// updateQuery := `UPDATE webLogin SET
	//                 realName = @realName,
	//                 mobileNumber = @mobile,
	//                 email = @email,
	//                 gender = @gender,
	//                 kode_cabang = @cabang
	//                 WHERE LTRIM(RTRIM(username)) = @user`

	// _, err := conn.Exec(updateQuery,
	// 	sql.Named("realName", req.RealName),
	// 	sql.Named("mobile", req.MobileNumber),
	// 	sql.Named("email", req.Email),
	// 	sql.Named("gender", req.Gender),
	// 	sql.Named("cabang", req.KodeCabang),
	// 	sql.Named("user", req.Username))

	updateQuery := `UPDATE weblogin SET 
                    realname = ?, 
                    mobilenumber = ?, 
                    email = ?,
                    gender = ?,
                    kode_cabang = ?
                WHERE username = ?`

	_, err := conn.Exec(updateQuery,
		req.RealName,
		req.MobileNumber,
		req.Email,
		req.Gender,
		req.KodeCabang,
		req.Username,
	)

	if err != nil {
		log.Printf("Error Update DB: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Gagal update data ke database"})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Data berhasil diperbarui!"})
}

func UpdateProfileImage(c *gin.Context) {
	val, exists := c.Get("username")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User tidak teridentifikasi"})
		return
	}
	userNameStr := fmt.Sprintf("%v", val)

	// 1. Cek File ada di Request?
	file, err := c.FormFile("profileImage")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal menerima file"})
		return
	}

	// 2. Buat Folder Uploads
	if _, err := os.Stat("uploads"); os.IsNotExist(err) {
		os.Mkdir("uploads", os.ModePerm)
	}

	// 3. Rename File (Username_Timestamp.jpg)
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%v_%d%v", userNameStr, time.Now().Unix(), ext)
	dst := filepath.Join("uploads", filename)

	// 4. Save File ke Folder
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan foto"})
		return
	}

	// 5. Ambil PTID dari Query
	ptID := c.Query("pt_id")
	if ptID == "" {
		ptID = "A" // Default
	}

	conn, ok := resolveDB(ptID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "pt_id tidak valid"})
		return
	}

	// 6. Update Database
	// Kita set juga link lengkapnya biar bisa langsung dipakai di frontend
	imageURL := fmt.Sprintf("http://localhost:8080/uploads/%s", filename)

	// 1. Pastikan username di-string dan dibersihkan dari spasi kiri-kanan di Go
	cleanUsername := strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", userNameStr)))

	updateQuery := `UPDATE webLogin SET 
                    profileimage = ?
                    WHERE LOWER(LTRIM(RTRIM(username))) = ?`

	// _, err = conn.Exec(updateQuery, imageURL, userNameStr)
	// 3. Eksekusi ke database
	result, err := conn.Exec(updateQuery, imageURL, cleanUsername)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update foto di database: " + err.Error()})
		return
	}

	// 4. VALIDASI SAKTI: Cek apakah ada baris yang benar-benar berubah!
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Println("Gagal mengambil rows affected:", err)
	}

	if rowsAffected == 0 {
		// Jika 0, berarti username dari JWT kagak cocok dengan yang ada di tabel weblogin!
		log.Printf("⚠️ Peringatan: Foto tersimpan di server, tapi 0 baris diupdate di DB untuk username: %s", cleanUsername)
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "warning",
			"message": "Foto sukses diunggah, namun data user tidak ditemukan di database. Pastikan Username sesuai.",
		})
		return
	}

	if err != nil {
		log.Printf("DB Error: %v", err) // Tambahkan log buat debug
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Foto berhasil diupload!",
		"url":     imageURL,
	})
}

func Logout(c *gin.Context) {
	fmt.Println(">>> REQUEST LOGOUT DITERIMA OLEH SERVER <<<")
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}
