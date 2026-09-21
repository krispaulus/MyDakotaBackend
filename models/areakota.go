package models

import "time"

// AreaKota merepresentasikan tabel mkt_m_areakota
type AreaKota struct {
	AreaKotaID    int       `json:"areakota_id" db:"areakota_id"`
	AgenID        int       `json:"agen_id" db:"agen_id"`
	AgenNama      string    `json:"agen_nama,omitempty" db:"agen_nama"`
	CustID        *string   `json:"cust_id" db:"cust_id"`
	CustName      *string   `json:"cust_name,omitempty" db:"cust_name"`
	KotaKabupaten string    `json:"kotakabupaten" db:"kotakabupaten"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// Request payload saat simpan batch area kota
type CreateAreaKotaBatchRequest struct {
	AgenID        int      `json:"agen_id" binding:"required"`
	CustID        *string  `json:"cust_id"` // Boleh null/kosong (Global)
	KotaKabupaten []string `json:"pilihan_kota" binding:"required,min=1"`
}
