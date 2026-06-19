package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	Username string `json:"username"`
	PTID     string `json:"pt_id"`
	UserType string `json:"user_type"`
	jwt.RegisteredClaims
}

// ResponseWriter wrapper untuk capture response body
type ResponseWriterWrapper struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *ResponseWriterWrapper) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *ResponseWriterWrapper) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// Extract username dari JWT token
func extractUsernameFromJWT(authHeader string) string {
	if authHeader == "" {
		return ""
	}

	tokenString := strings.TrimSpace(strings.Replace(authHeader, "Bearer ", "", 1))
	if tokenString == "" {
		return ""
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil {
		return ""
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if username, exists := claims["username"].(string); exists {
			return username
		}
	}

	return ""
}

func ActivityLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Ambil data request
		path := c.Request.URL.Path
		method := c.Request.Method

		// 2. Capture REQUEST BODY
		var requestBody map[string]interface{}
		if c.Request.Body != nil {
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			if len(bodyBytes) > 0 {
				json.Unmarshal(bodyBytes, &requestBody)
			}
		}

		// 3. Wrap response writer
		responseBody := &bytes.Buffer{}
		writer := &ResponseWriterWrapper{
			ResponseWriter: c.Writer,
			body:           responseBody,
		}
		c.Writer = writer

		// 4. Jalankan Handler
		c.Next()

		// 5. Logika penentuan UserIdentifier
		userIdentifier := "Guest"
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			if jwtUser := extractUsernameFromJWT(authHeader); jwtUser != "" {
				userIdentifier = jwtUser
			}
		}

		if path == "/api/login" && requestBody != nil {
			if email, ok := requestBody["email"].(string); ok && email != "" {
				userIdentifier = email
			} else if username, ok := requestBody["username"].(string); ok && username != "" {
				userIdentifier = username
			}
		}

		// 6. Ambil status dan data response
		status := c.Writer.Status()
		var responseData map[string]interface{}
		if responseBody.Len() > 0 {
			json.Unmarshal(responseBody.Bytes(), &responseData)
		}

		// 7. SETUP WAKTU JAKARTA
		loc, _ := time.LoadLocation("Asia/Jakarta")
		currentTime := time.Now().In(loc)
		timestamp := currentTime.Format("2006/01/02 15:04:05")

		// 8. FILTER SENSITIVE DATA
		safeRequest := requestBody
		if safeRequest != nil {
			if _, exists := safeRequest["password"]; exists {
				// Copy map agar tidak mengubah data asli jika diperlukan di tempat lain
				newSafeReq := make(map[string]interface{})
				for k, v := range safeRequest {
					newSafeReq[k] = v
				}
				newSafeReq["password"] = "***REDACTED***"
				safeRequest = newSafeReq
			}
		}

		// 9. FILTER RESPONSE (Hanya simpan data jika path penting)
		var logResponse map[string]interface{} // Deklarasi cukup SATU KALI di sini
		importantPaths := []string{
			"/api/login",
			"/api/logout",
			"/api/request-otp",
			"/api/verify-otp",
			"/api/users/add",
			"/api/users/update",
			"/api/profile",
		}

		for _, p := range importantPaths {
			if strings.Contains(path, p) {
				logResponse = responseData
				break
			}
		}

		// 10. CETAK KE LOG
		// if method != "GET" || logResponse != nil {
		// 	log.Printf(
		// 		"[%s] [ACTIVITY] User: %s | %s %s | Status: %d | Request: %+v | Response: %+v",
		// 		timestamp, userIdentifier, method, path, status, safeRequest, logResponse)
		// }

		// 10. CETAK KE LOG (DENGAN LIMIT KARAKTER AGAR TERMINAL BERSIH)
		if method != "GET" || logResponse != nil {
			// Ubah request & response jadi string dulu
			reqStr := fmt.Sprintf("%+v", safeRequest)
			resStr := fmt.Sprintf("%+v", logResponse)

			// Batasi maksimal 500 karakter saja yang tampil di terminal
			if len(reqStr) > 500 {
				reqStr = reqStr[:500] + "... [Request Data Too Long]"
			}
			if len(resStr) > 500 {
				resStr = resStr[:500] + "... [Response Data Too Long]"
			}

			log.Printf(
				"[%s] [ACTIVITY] User: %s | %s %s | Status: %d | Request: %s | Response: %s",
				timestamp, userIdentifier, method, path, status, reqStr, resStr)
		}
	}
}
