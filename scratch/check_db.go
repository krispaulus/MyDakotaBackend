package main

import (
	"fmt"
	"log"

	"dakotagroup/business-insight-be/db"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		if err = godotenv.Load(".env"); err != nil {
			log.Println("No .env file found, using system environment variables")
		}
	}

	if err := db.ConnectAll(); err != nil {
		log.Fatalf("Gagal connect ke database: %v", err)
	}

	database := db.GetDB()

	// Query glb_m_agen
	fmt.Println("--- SELECT FROM glb_m_agen ---")
	var agens []map[string]interface{}
	err := database.Table("public.glb_m_agen").Where("agen_nama ILIKE ?", "%gorontalo%").Find(&agens).Error
	if err != nil {
		log.Fatalf("Gagal query glb_m_agen: %v", err)
	}

	for _, a := range agens {
		fmt.Printf("ID: %v, Kode: %v, Nama: %v, CabangID: %v\n", a["agen_id"], a["agen_kode"], a["agen_nama"], a["agen_cabangid"])
	}
}
