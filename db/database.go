package db

import (
	"database/sql"
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DBS   *sql.DB
	DLB   *sql.DB
	DLI   *sql.DB
	DB    *gorm.DB
	DLBDB *gorm.DB
	DLIDB *gorm.DB
)

// ResolveDB memilih koneksi database berdasarkan PT ID
// huruf R besar supaya bisa dipanggil dari folder handler

func ResolveDB(ptID string) (*gorm.DB, bool) {
	switch ptID {
	case "A": // Contoh: PT DBS
		return DB, true
	case "B": // Contoh: PT DLB
		return DLBDB, true
	case "C": // Contoh: PT DLI
		return DLIDB, true
	default:
		return nil, false
	}
}

func ConnectAll() error {
	var err error

	// 1. Koneksi DBS
	dsnDBS := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta",
		os.Getenv("DB_HOST"), os.Getenv("DB_DBS_USER"), os.Getenv("DB_DBS_PASSWORD"), os.Getenv("DB_DBS_NAME"), os.Getenv("DB_PORT"))

	DB, err = gorm.Open(postgres.Open(dsnDBS), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("Gagal konek DBS: %w", err)
	}
	DBS, _ = DB.DB() // Mengambil sql.DB standar untuk keperluan lain jika butuh

	// 2. Koneksi DLB
	dsnDLB := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta",
		os.Getenv("DB_HOST"), os.Getenv("DB_DLB_USER"), os.Getenv("DB_DLB_PASSWORD"), os.Getenv("DB_DLB_NAME"), os.Getenv("DB_PORT"))

	DLBDB, err = gorm.Open(postgres.Open(dsnDLB), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("Gagal konek DLB: %w", err)
	}
	DLB, _ = DLBDB.DB()

	// 3. Koneksi DLI
	dsnDLI := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta",
		os.Getenv("DB_HOST"), os.Getenv("DB_DLI_USER"), os.Getenv("DB_DLI_PASSWORD"), os.Getenv("DB_DLI_NAME"), os.Getenv("DB_PORT"))

	DLIDB, err = gorm.Open(postgres.Open(dsnDLI), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("Gagal konek DLI: %w", err)
	}
	DLI, _ = DLIDB.DB()

	fmt.Println("✅ Berhasil: Semua database sudah beralih ke PostgreSQL!")
	return nil
}

func GetDBS() *sql.DB {
	return DBS
}

func GetDB() *gorm.DB {
	return DB
}
