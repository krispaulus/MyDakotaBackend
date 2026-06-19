package handler

import (
	"io"
	"log"
	"os"

	"github.com/natefinch/lumberjack"
)

func InitGlobalLogger() {
	// Setup Lumberjack sebagai logger
	lumberjackLog := &lumberjack.Logger{
		Filename:   "./logs/activity.log", // Nama file log
		MaxSize:    10,                    // Megabytes sebelum di-rotate (opsional)
		MaxBackups: 12,                    // Simpan history 12 bulan terakhir
		MaxAge:     30,                    // Simpan selama 30 hari (1 bulan)
		Compress:   true,                  // Kompres file lama jadi .gz biar hemat disk
	}

	// Supaya log muncul di terminal DAN masuk ke file
	multiWriter := io.MultiWriter(os.Stdout, lumberjackLog)

	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}
