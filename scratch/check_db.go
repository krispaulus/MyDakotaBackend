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

	// Update record yang bttt_aktifyn nya masih NULL atau kosong
	fmt.Println("--- UPDATING NULL/EMPTY bttt_aktifyn to 'Y' ---")
	result := database.Exec("UPDATE public.mkt_t_econote SET bttt_aktifyn = 'Y' WHERE bttt_aktifyn IS NULL OR bttt_aktifyn = ''")
	if result.Error != nil {
		fmt.Printf("Gagal update database: %v\n", result.Error)
	} else {
		fmt.Printf("Berhasil update %d baris data!\n", result.RowsAffected)
	}

	// Verifikasi record AGOR001062600009
	fmt.Println("\n--- VERIFIKASI RECORD AGOR001062600009 ---")
	var econoteRow map[string]interface{}
	err := database.Raw("SELECT bttt_id, bttt_aktifyn, bttt_spyn, bttt_asalagenid FROM public.mkt_t_econote WHERE bttt_id = ?", "AGOR001062600009").Scan(&econoteRow).Error
	if err != nil {
		fmt.Printf("Gagal fetch record: %v\n", err)
	} else {
		for k, v := range econoteRow {
			fmt.Printf("%s: %v (%T)\n", k, v, v)
		}
	}
}
