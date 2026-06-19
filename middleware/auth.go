package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware: Satpam level 1 (Khusus Gin Framework)
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Token tidak ditemukan"})
			c.Abort() // Menghentikan request di sini
			return
		}

		tokenString := strings.TrimSpace(strings.Replace(authHeader, "Bearer ", "", 1))
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Metode signing tidak valid")
			}

			// Ambil dari ENV
			secret := os.Getenv("JWT_SECRET")

			// Fallback HARUS SAMA dengan yang ada di LoginHandler
			if secret == "" {
				secret = "rahasia-ilahi-123"
			}

			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			fmt.Println("SECRET YANG DIPAKAI:", os.Getenv("JWT_SECRET"))
			fmt.Println("Error Token:", err.Error())
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Token tidak valid"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
			c.Abort()
			return
		}

		// DEBUG AUTH middleware
		if username, ok := claims["username"].(string); ok {
			fmt.Printf(" [AUTH] User %s is accessing %s\n", username, c.Request.URL.Path)
		} else {
			fmt.Printf(" [AUTH] Username not found in token. Raw claims: %v\n", claims)
		}

		// SANGAT PENTING: Set data ke context GIN
		// Ini agar di handler/profile.go bisa panggil c.Get("username")
		c.Set("user_data", claims)
		c.Set("username", claims["username"])
		c.Set("pt_id", claims["pt_id"])

		c.Next() // Lanjut ke handler berikutnya (misal: GetProfile)
	}
}

// RoleMiddleware: Satpam level 2 (Khusus Gin Framework)
func RoleMiddleware(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ambil data yang tadi di-set oleh AuthMiddleware
		val, exists := c.Get("user_data")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Data user tidak ditemukan"})
			c.Abort()
			return
		}

		userData := val.(jwt.MapClaims)
		userType, ok := userData["user_type"].(string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Role tidak valid"})
			c.Abort()
			return
		}

		authorized := false
		for _, role := range requiredRoles {
			if userType == role {
				authorized = true
				break
			}
		}

		if !authorized {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Kamu tidak punya akses"})
			c.Abort()
			return
		}

		c.Next()
	}
}
