package models

// Item BTT Deadline Outstanding
type LoperDeadlineDTO struct {
	LoperID      string `json:"loper_id"`
	BttID        string `json:"btt_id"`
	LoperTanggal string `json:"loper_tanggal"`
	NipSopir     string `json:"nip_sopir"`
	NamaSopir    string `json:"nama_sopir"`
	NipKerani    string `json:"nip_kerani"`
	NamaKerani   string `json:"nama_kerani"`
	NoMobil      string `json:"no_mobil"`
}

// Request Payload untuk Action Approval (Terima / Tolak)
type LoperDeadlineActionReq struct {
	LoperID string `json:"loper_id" binding:"required"`
	BttID   string `json:"btt_id" binding:"required"`
	Status  string `json:"status" binding:"required"` // 'Y' untuk Terima, 'N' untuk Tolak
}
