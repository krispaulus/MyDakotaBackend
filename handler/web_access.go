package handler

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/models" // Sesuaikan dengan nama module go.mod kamu
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Fungsi Utama untuk Update ke Database
func UpdateWebAccess(db *sql.DB, req models.AccessRequest) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Delete dulu biar gak duplikat
	_, err = tx.Exec("DELETE FROM web_access WHERE username = $1", req.Username)
	if err != nil {
		tx.Rollback()
		return err
	}

	for menuID, rights := range req.Permissions {
		fmt.Printf("DEBUG: Mencoba simpan menu [%s] untuk user [%s]\n", menuID, req.Username)
		v, c, e, d := rights["view"], rights["create"], rights["edit"], rights["delete"]

		if v || c || e || d {
			query := `INSERT INTO web_access (username, menu_id, can_view, can_create, can_edit, can_delete) 
					  VALUES ($1, $2, $3, $4, $5, $6)`
			_, err = tx.Exec(query, req.Username, menuID, v, c, e, d)
			if err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit()
}

// Handler API yang dipanggil dari main.go
func HandleUpdateAccess(c *gin.Context) {
	var req models.AccessRequest

	// 1. Bind JSON dari React
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Println("BIND ERROR DETAIL:", err)
		c.JSON(400, gin.H{"error": "Payload tidak valid"})
		return
	}
	fmt.Printf("DATA MASUK: %+v\n", req)

	// 2. Eksekusi ke Database (Ambil koneksi DB dari package db kamu)
	// Pastikan kamu punya akses ke db.DBS (atau database utama kamu)
	database := db.GetDBS()

	if database == nil {
		c.JSON(500, gin.H{"error": "Koneksi database DBS tidak ditemukan"})
		return
	}

	err := UpdateWebAccess(database, req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	adminUsername, _ := c.Get("username")
	if adminUsername == nil {
		adminUsername = "Unknown Admin"
	}

	logMsg := fmt.Sprintf("Admin [%v] mengubah hak akses untuk user [%s]", adminUsername, req.Username)
	fmt.Println("LOG ACTIVITY:", logMsg)

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": logMsg,
	})
}

func GetUserAccess(c *gin.Context) {
	username := c.Param("username")
	database := db.GetDBS()

	rows, err := database.Query("SELECT menu_id, can_view, can_create, can_edit, can_delete FROM web_access WHERE username = $1", username)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	// Format datanya biar disukai React: map[string]map[string]bool
	permissions := make(map[string]map[string]bool)

	for rows.Next() {
		var menuID string
		var v, c_reg, e, d bool
		if err := rows.Scan(&menuID, &v, &c_reg, &e, &d); err != nil {
			continue
		}
		permissions[menuID] = map[string]bool{
			"view":   v,
			"create": c_reg,
			"edit":   e,
			"delete": d,
		}
	}

	c.JSON(200, permissions)
}
