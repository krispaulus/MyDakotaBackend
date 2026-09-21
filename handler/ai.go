package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type AIChatRequest struct {
	Prompt string `json:"prompt" binding:"required"`
}

func AskAI(c *gin.Context) {
	var req AIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Prompt wajib diisi"})
		return
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "GEMINI_API_KEY belum disetel di file environment",
		})
		return
	}

	// Konteks ringkas agar AI memahami domain Dakota Cargo
	systemInstruction := `Kamu adalah asisten eksekutif cerdas Dakota Cargo (PT Dakota Logistik Indonesia / DLI).
Profil Dakota Cargo: Didirikan pada tahun 1998, bergerak di bidang jasa ekspedisi dan pengiriman barang logistik (darat, laut, udara) di seluruh Indonesia.
Tugasmu: Menjawab pertanyaan manajemen, baik umum maupun seputar logistik operasional, dengan ramah, lugas, profesional, dan dalam bahasa Indonesia.`

	// Payload Gemini REST API
	requestBody, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": fmt.Sprintf("%s\n\nUser: %s", systemInstruction, req.Prompt)},
				},
			},
		},
	})

	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", apiKey)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Gagal menghubungi layanan AI"})
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil || len(geminiResp.Candidates) == 0 {
		c.JSON(http.StatusOK, gin.H{"reply": "Maaf, AI belum bisa merespons saat ini."})
		return
	}

	reply := geminiResp.Candidates[0].Content.Parts[0].Text
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"reply":  reply,
	})
}
