package main

import (
	"dakotagroup/business-insight-be/db"
	"dakotagroup/business-insight-be/handler"
	"dakotagroup/business-insight-be/middleware"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func infoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "running", "message": "Dakota Business Insight API is Live"}`)
}

func main() {
	handler.InitGlobalLogger()

	// 1. Load Environment Variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, menggunakan system env")
	}

	// 2. Inisialisasi Database
	if err := db.ConnectAll(); err != nil {
		log.Fatalf("❌ Gagal connect ke database: %v", err)
	}
	fmt.Println("✅ Database Dakota Group (DBS, DLB, DLI) Berhasil Inisialisasi!")

	// 3. Setup Server Gin
	r := gin.Default()
	r.Use(CorsMiddleware())
	r.Use(middleware.ActivityLogger())

	// 5. Routing API
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "running",
			"message": "Dakota Business Insight API is Live",
		})
	})

	r.Static("/uploads", "./uploads")

	// Grouping API
	api := r.Group("/api")
	{
		api.POST("/login", handler.LoginHandler)
		api.POST("/request-otp", handler.RequestOTPHandler)
		api.POST("/verify-otp", handler.VerifyAndSaveEmailHandler)
		api.GET("/health", handler.HealthHandler)
		api.POST("/users/add", handler.CreateUser)
		api.GET("/users/check/:username", handler.CheckUsername)
		api.GET("/btt/generate-custid-umum", handler.GenerateCustIDUmumHandler)
		api.GET("/sp/list", handler.GetSuratPengantarFetch)
		api.POST("/sp/add", handler.CreateSuratPengantar)

		// Group ini butuh token JWT
		authorized := api.Group("/")
		authorized.Use(middleware.AuthMiddleware())
		{
			authorized.POST("/logout", handler.Logout)
			authorized.GET("/users", handler.GetAllWebLogins)
			authorized.POST("/users/update-access", handler.HandleUpdateAccess)
			authorized.GET("/users/access/:username", handler.GetUserAccess)
			authorized.POST("/user-dli/create", handler.CreateUserDLI)
			authorized.GET("/user-dli/list", handler.GetUserDLIList)

			authorized.GET("/agens", handler.GetAgens)
			authorized.GET("/agens/detail/:id", handler.GetAgenDetailByID)
			authorized.POST("/agens", handler.CreateAgen)
			authorized.PUT("/agens/:id", handler.UpdateAgen)
			authorized.DELETE("/agens/:id", handler.DeleteAgen)
			authorized.GET("/agens/detail-name/:nama", handler.GetDetailAgenByName)

			authorized.GET("/area-loper", handler.GetAreaLopers)
			authorized.POST("/area-loper", handler.CreateAreaLoper)
			authorized.PUT("/area-loper/:id", handler.UpdateAreaLoper)
			authorized.GET("/area-loper/unregistered", handler.GetWilayahBelumTerdaftar)
			authorized.GET("/area-loper/terpilih/:kode", handler.GetAreaLoperTerpilihByAgen)
			authorized.GET("/area-loper/suggest-wilayah", handler.SearchMasterWilayah)
			authorized.POST("/area-loper/assign-wilayah", handler.AssignWilayahKeAgen)
			authorized.POST("/area-loper/assign-wilayah-massal", handler.AssignWilayahMassalKeAgen)
			authorized.POST("/area-loper/batch-update", handler.ProcessBatchAreaLoper)
			authorized.DELETE("/area-loper/remove-wilayah-massal", handler.RemoveWilayahMassalDariAgen)
			authorized.PUT("/area-loper/update-single-atribut", handler.UpdateSingleAreaLoperAtribut)

			authorized.GET("/master/kodepos", handler.GetKodePos)
			authorized.POST("/master/kodepos", handler.CreateKodePos)
			authorized.PUT("/master/kodepos/:id", handler.UpdateKodePos)
			authorized.DELETE("/master/kodepos/:id", handler.DeleteKodePos)

			authorized.GET("/profile", handler.GetProfile)
			authorized.PUT("/profile/update", handler.UpdateProfile)
			authorized.POST("/profile/change-password", handler.ChangePassword)
			authorized.PUT("/users/update", handler.UpdateWebLogin)
			authorized.DELETE("/users/:username", handler.DeleteUser)
			authorized.POST("/settings/web-user-access", handler.SaveWebUserAccess)
			authorized.GET("/tarif/reguler", handler.GetTarifReguler)
			authorized.GET("/tarif/ekonomis", handler.GetTarifEkonomis)
			authorized.GET("/tarif/unit", handler.GetTarifUnit)

			authorized.POST("/customer/create", handler.CreateCustomerHandler)
			authorized.POST("/customer/update", handler.UpdateCustomerHandler)
			authorized.POST("/customer/delete", handler.DeleteCustomerHandler)
			authorized.GET("/customer/search-kota", handler.SearchKotaHandler)
			authorized.GET("/customer", handler.GetMasterCustomerList)

			authorized.POST("/customer/workdays/update", handler.UpdateCustomerWorkDaysHandler)

			authorized.GET("/btt/search-customer", handler.SearchCustomerHandler)
			authorized.POST("/btt/add", handler.CreateBTT)
			authorized.GET("/closing-agen/list", handler.GetClosingAgenList)
			authorized.POST("/closing-agen/process", handler.ProcessTambahClosingHarian)

			// --- GRUP ENDPOINT OPERASIONAL BTT DAKOTA ---
			authorized.GET("/marketing/btt", handler.GetBTT)
			authorized.GET("/btt/kecamatan", handler.GetKecamatanByKota)
			authorized.GET("/btt/check-lock", handler.CheckLockBTT)
			authorized.GET("/btt/search-area", handler.SearchAreaByKecamatan)

			authorized.GET("/btt/search-geo", handler.SearchMasterGeoBtt)

			authorized.GET("/btt/get-kelurahan", handler.GetKelurahanByKecamatan)

			authorized.GET("/btt/generate-custid", handler.GenerateCustIDHandler)

			// Step 1: Otak hitung tarif volume vs berat asli
			authorized.POST("/btt/calculate-tarif", handler.CalculateTarifHandler)

			// Step 2: Benteng proteksi server (Nomor telp, batas COD, limit kredit plafon)
			authorized.POST("/btt/validate", handler.ValidateBTTHandler)
			authorized.GET("/btt/check-closing-gate", handler.CheckStatusClosingKemarin)

			// =========================================================================
			authorized.GET("/operasional/fleet-drivers", handler.GetFleetAndDrivers)
			authorized.GET("/operasional/pool-btt", handler.GetPoolBTT)
			authorized.POST("/operasional/sp-naik", handler.CreateSPNaikHandler)

			// =========================================================================
			authorized.GET("/operasional/sp-turun/preview", handler.GetDetailSPNaik)
			authorized.POST("/operasional/sp-turun/initial", handler.SaveInitialSPTurun)
			authorized.POST("/operasional/sp-turun/autosave", handler.AutoSaveRowSPTurun)
			authorized.GET("/operasional/sp-turun/history", handler.GetHistorySPTurun)

			// 📄 OPERASIONAL: LOPER
			authorized.GET("/operasional/loper-list", handler.GetLoperList)
			authorized.POST("/operasional/loper-create", handler.CreateLoper)
			authorized.GET("/operasional/loper/history", handler.GetHistoryLoper)

			// 📄 OPERASIONAL: LOPER DEADLINE
			authorized.GET("/operasional/loper-deadline-list", handler.GetLoperDeadlineList)
			authorized.POST("/operasional/loper-deadline-action", handler.ProcessLoperDeadlineAction)
			authorized.POST("/operasional/loper-deadline-create", handler.CreateLoperDeadline)
			authorized.PUT("/operasional/loper-deadline-update", handler.UpdateLoperDeadline)
			authorized.DELETE("/operasional/loper-deadline-delete", handler.DeleteLoperDeadline)

			// 📄 OPERASIONAL: PEMBONGKARAN BARANG (UNLOADING)
			authorized.GET("/operasional/unload-barang", handler.GetListUnloadBarang)
			authorized.POST("/operasional/unload-barang-create", handler.CreateUnloadBarang)
			authorized.PUT("/operasional/unload-barang-update", handler.UpdateUnloadBarang)
			authorized.DELETE("/operasional/unload-barang-delete", handler.DeleteUnloadBarang)

			// 📄 OPERASIONAL: INVENTORY BARANG CUSTOMER
			authorized.GET("/operasional/inventory-customer", handler.GetListCustomerInventory)
			authorized.POST("/operasional/inventory-customer-create", handler.CreateCustomerInventory)
			authorized.PUT("/operasional/inventory-customer-update", handler.UpdateCustomerInventory)
			authorized.DELETE("/operasional/inventory-customer-delete", handler.DeleteCustomerInventory)

			authorized.GET("/operasional/sp-terima/print-detail/:id", handler.GetPrintSPDetail)

			// 📄 OPERASIONAL: PENGEMBALIAN BTT (CLEANED ROUTE)
			authorized.GET("/operasional/kembali-btt/history", handler.GetListKembaliBTT)
			authorized.GET("/operasional/kembali-btt-list", handler.GetListKembaliBTT)
			authorized.POST("/operasional/kembali-btt-create", handler.CreateKembaliBTT)
			authorized.PUT("/operasional/kembali-btt-update", handler.UpdateKembaliBTT)
			authorized.DELETE("/operasional/kembali-btt-delete", handler.DeleteKembaliBTT)
			//authorized.GET("/operasional/kembali-btt/monitor-belum-kembali", handler.GetBTTBelumKembali)
			//authorized.GET("/operasional/kembali-btt/monitor-outstanding-bdb", handler.GetReturOutstandingBDB)
			authorized.GET("/operasional/kembali-btt/monitor-belum-kembali", handler.GetMonitorBelumKembali)
			authorized.GET("/operasional/kembali-btt/monitor-outstanding-bdb", handler.GetMonitorOutstandingBDB)

			// 📄 OPERASIONAL: HASIL LOPER (POD)
			authorized.GET("/operasional/hasil-loper-list", handler.GetHasilLoperList)
			authorized.GET("/operasional/reason-list", handler.GetReasonList)
			authorized.POST("/operasional/hasil-loper-create", handler.CreateHasilLoper)
			authorized.PUT("/operasional/hasil-loper-update", handler.UpdateHasilLoper)
			authorized.DELETE("/operasional/hasil-loper-delete", handler.DeleteHasilLoper)

			// 📄 OPERASIONAL: RETUR BTT / BARANG BERMASALAH
			authorized.GET("/operasional/retur-btt-list", handler.GetListReturBTT)
			authorized.POST("/operasional/retur-btt-create", handler.CreateReturBTT)
			authorized.PUT("/operasional/retur-btt-update", handler.UpdateReturBTT)
			authorized.DELETE("/operasional/retur-btt-delete", handler.DeleteReturBTT)

			// 📑 OPERASIONAL: LAPORAN LSBP (INFORMASI LSPB)
			authorized.GET("/operasional/laporan-lspb/list", handler.GetLSPBReportList)
			authorized.GET("/operasional/laporan-lsbp", handler.GetLSPBReportList)
			authorized.POST("/operasional/laporan-lspb/save", handler.SaveLSPB)
			authorized.DELETE("/operasional/laporan-lspb/delete/:nodo", handler.DeleteLSPB)

			authorized.GET("/operasional/laporan-lspb-v2/list", handler.GetLSPBV2ReportList)
			authorized.GET("/operasional/laporan-barang-turun/detail", handler.GetLaporanBarangTurunDetail)
			authorized.GET("/operasional/laporan-barang-turun/rekap", handler.GetLaporanBarangTurunRekap)
			authorized.GET("/operasional/laporan-btt-belum-kembali/detail", handler.GetBTTBelumKembaliDetail)
			authorized.GET("/operasional/laporan-btt-belum-kembali/rekap", handler.GetBTTBelumKembaliRekap)
			authorized.GET("/operasional/laporan-data-penerima-customer", handler.GetLaporanDataPenerimaCustomer)
			authorized.GET("/operasional/laporan-pendapatan-operasional", handler.GetLaporanPendapatanOperasional)
			authorized.GET("/operasional/loading-barang", handler.GetListLoadingBarang)

			// 📄 OPERASIONAL: PENGAMBILAN BARANG RETUR
			authorized.GET("/operasional/ambilretur-list", handler.GetAmbilReturList)
			authorized.POST("/operasional/ambilretur-create", handler.CreateAmbilRetur)
			authorized.PUT("/operasional/ambilretur-update", handler.UpdateAmbilRetur)
			authorized.DELETE("/operasional/ambilretur-delete", handler.DeleteAmbilRetur)

			// 📄 OPERASIONAL: PENGAMBILAN BARANG SENDIRI
			authorized.GET("/operasional/ambil-list", handler.GetAmbilList)
			authorized.POST("/operasional/ambil-create", handler.CreateAmbil)
			authorized.PUT("/operasional/ambil-update", handler.UpdateAmbil)
			authorized.DELETE("/operasional/ambil-delete", handler.DeleteAmbil)

			authorized.GET("/marketing/bdb/list", handler.GetBDBListHandler)
			authorized.GET("/marketing/monitoring-btt", handler.GetMonitoringBTT)
			authorized.GET("/marketing/kembali-sj", handler.GetKembaliSJList)
			authorized.GET("/marketing/proses-packing", handler.GetProsesPackingList)
			authorized.POST("/marketing/proses-packing/add", handler.SimpanProsesPacking)
			authorized.GET("/marketing/btt-outstanding-packing", handler.GetBttOutstandingPacking)

			authorized.GET("/marketing/uncovered-areas", handler.GetUncoveredAreas)
			authorized.POST("/marketing/uncovered-areas/process", handler.ProcessUncoveredArea)

			authorized.GET("/hrd/device-karyawan", handler.GetDeviceKaryawanList)
			authorized.PUT("/hrd/device-karyawan/:nip", handler.UpdateDeviceKaryawan)
			authorized.POST("/hrd/device-karyawan/whatsapp-import", handler.ImportDeviceViaWhatsApp)
			authorized.GET("/hrd/raw-absensi", handler.GetRawAbsensiList)

			authorized.GET("/master/kendaraan", handler.GetMasterKendaraanList)
			authorized.GET("/master/kendaraan/expired-service", handler.GetExpiredServiceKendaraan)

			authorized.POST("/master/kendaraan", handler.CreateKendaraan)
			authorized.PUT("/master/kendaraan/:id", handler.UpdateKendaraan)
			authorized.POST("/master/kendaraan/masuk-service", handler.RegisterTrukService)

			authorized.GET("/master/sewa-kendaraan", handler.GetSewaKendaraanList)
			authorized.POST("/master/sewa-kendaraan", handler.CreateSewaKendaraan)
			authorized.PUT("/master/sewa-kendaraan/:id", handler.UpdateSewaKendaraan)
			authorized.DELETE("/master/sewa-kendaraan/:id", handler.DeleteSewaKendaraan)

			authorized.GET("/master/korwil", handler.GetKorwilList)
			authorized.POST("/master/korwil", handler.CreateKorwil)
			authorized.PUT("/master/korwil/:id", handler.UpdateKorwil)
			authorized.GET("/master/korwil/detail/:id", handler.GetKorwilDetail)
			authorized.POST("/master/korwil/detail", handler.AddAgenToKorwil)
			authorized.DELETE("/master/korwil/detail", handler.RemoveAgenFromKorwil)

			authorized.GET("/master/sopir", handler.GetSupirList)
			authorized.GET("/sopir-list", handler.GetSupirList)
			authorized.GET("/master/assignment", handler.GetAssignmentList)
			authorized.POST("/master/sopir", handler.CreateSupir)
			authorized.PUT("/master/sopir/:id", handler.UpdateSupir)
			authorized.DELETE("/master/sopir/:id", handler.DeleteSupir)

			authorized.GET("/master/trayek", handler.GetTrayekList)
			authorized.POST("/master/trayek", handler.CreateTrayek)
			authorized.PUT("/master/trayek/:id", handler.UpdateTrayek)
			authorized.GET("/master/trayek/detail/rute/:id", handler.GetTrayekDetailRute)
			authorized.GET("/master/trayek/detail/bplk/:id", handler.GetTrayekDetailBplk)
			authorized.GET("/master/trayek/detail/voucher/:id", handler.GetTrayekDetailVoucher)
			authorized.POST("/master/trayek/save-full", handler.SaveTrayekFull)

			authorized.GET("/master/tarif-carter", handler.GetTarifCarterIndex)
			authorized.GET("/master/tarif-carter/detail", handler.GetTarifCarterDetail)
			authorized.PUT("/master/tarif-carter/inline-update", handler.UpdateInlineCarter)
			authorized.DELETE("/master/tarif-carter/item/:id", handler.DeleteTarifCarterItem)
			authorized.POST("/master/tarif-carter/mass-add", handler.AddMassCarter)
			authorized.GET("/master/active-agen-list", handler.GetActiveAgenList)

			authorized.GET("/master/econote/list", handler.GetEconoteList)
			authorized.GET("/master/econote/detail/:id", handler.GetEconoteDetail)
			authorized.PUT("/master/econote/update", handler.UpdateEconote)
			authorized.DELETE("/master/econote/delete/:id", handler.DeleteEconote)

			configGroup := authorized.Group("/config")
			{
				configGroup.GET("/params", handler.GetParams)
				configGroup.POST("/params", handler.CreateParam)
				configGroup.PUT("/params", handler.UpdateParam)
				configGroup.DELETE("/params/:id", handler.DeleteParam)
			}

			// 🚀 MASTER TARIF TRANSIT (HARGA PERWILAYAH)
			authorized.GET("/master/tarif-transit/list", handler.GetTarifTransitList)
			authorized.GET("/master/tarif-transit/provinsi", handler.GetProvinsiOptions)
			authorized.GET("/master/tarif-transit/kota-by-provinsi", handler.GetKotaByProvinsi)
			authorized.POST("/master/tarif-transit/add", handler.CreateTarifTransit)
			authorized.PUT("/master/tarif-transit/update/:id", handler.UpdateTarifTransit)
			authorized.DELETE("/master/tarif-transit/delete/:id", handler.DeleteTarifTransit)

			// 🚀 HRD - MONITORING LOKASI KARYAWAN
			authorized.GET("/hrd/monitoring-lokasi/list", handler.GetMonitoringLokasiList)
			authorized.GET("/hrd/monitoring-lokasi/history-gps", handler.GetHistoryGPSKaryawan)
			authorized.POST("/hrd/monitoring-lokasi/update", handler.PostUpdateLokasiKaryawan)

			// 🚚 MASTER DAFTAR KENDARAAN
			authorized.GET("/master/perawatan-kendaraan/list", handler.GetDaftarKendaraanList)
			authorized.POST("/master/perawatan-kendaraan/add", handler.CreateKendaraanBaru)
			authorized.PUT("/master/perawatan-kendaraan/update/:id", handler.UpdateKendaraanBaru)
			authorized.DELETE("/master/perawatan-kendaraan/delete/:id", handler.DeleteKendaraanBaru)

			leadTimeGroup := authorized.Group("/master/leadtime-customer")
			{
				leadTimeGroup.GET("/list", handler.GetListLeadTime)
				leadTimeGroup.POST("/add", handler.CreateLeadTime)
				leadTimeGroup.PUT("/update", handler.UpdateLeadTime)
				leadTimeGroup.DELETE("/delete", handler.DeleteLeadTime)
			}

			tarifCustGroup := authorized.Group("/master/tarif-customer")
			{
				tarifCustGroup.GET("/list", handler.GetTarifCustomerList)
				tarifCustGroup.GET("/detail/:customer_id", handler.GetDetailTarifByCustomer)
				tarifCustGroup.POST("/save", handler.SaveTarifCustomer)
				tarifCustGroup.POST("/delete", handler.DeleteTarifCustomer)
			}

			tarifHandlingGroup := authorized.Group("/master/tarif-handling-propinsi")
			{
				tarifHandlingGroup.GET("/list", handler.GetTarifHandlingPropinsiList)
				tarifHandlingGroup.POST("/save", handler.SaveTarifHandlingPropinsi)
				tarifHandlingGroup.POST("/delete", handler.DeleteTarifHandlingPropinsi)
			}

			tarifPaketGroup := authorized.Group("/master/tarif-paket")
			{
				tarifPaketGroup.GET("/list", handler.GetTarifPaketSummaryList)
				tarifPaketGroup.GET("/detail/:agen_id", handler.GetDetailTarifPaketByAgen)
				tarifPaketGroup.POST("/save", handler.SaveTarifPaketArea)
				tarifPaketGroup.POST("/delete", handler.DeleteTarifPaketArea)
			}

			authorized.GET("/master/jenis-kendaraan-carter/list", handler.GetJenisKendaraanCarterList)
			authorized.POST("/master/jenis-kendaraan-carter/save", handler.SaveJenisKendaraanCarter)
			authorized.DELETE("/master/jenis-kendaraan-carter/delete/:id", handler.DeleteJenisKendaraanCarter)

			// 🏬 MASTER VENDOR (MKT_M_VENDOR)
			authorized.GET("/master/vendor/list", handler.GetVendorList)
			authorized.POST("/master/vendor/save", handler.SaveVendor)
			authorized.DELETE("/master/vendor/delete/:id", handler.DeleteVendor)

			// 📄 OPERASIONAL: PENGELUARAN INVENTORY BARANG CUSTOMER
			authorized.GET("/operasional/inventory-customer-out", handler.GetListCustomerInventoryOut)
			authorized.POST("/operasional/inventory-customer-out-create", handler.CreateCustomerInventoryOut)
			authorized.PUT("/operasional/inventory-customer-out-update", handler.UpdateCustomerInventoryOut)
			authorized.DELETE("/operasional/inventory-customer-out-delete", handler.DeleteCustomerInventoryOut)

			// 📄 OPERASIONAL: PENGISIAN BBM
			authorized.GET("/operasional/isibbm-list", handler.GetIsiBBMList)
			authorized.POST("/operasional/isibbm-create", handler.CreateIsiBBM)
			authorized.PUT("/operasional/isibbm-update", handler.UpdateIsiBBM)
			authorized.DELETE("/operasional/isibbm-delete", handler.DeleteIsiBBM)

			// 📄 OPERASIONAL: SURAT TUGAS SUPIR / SURAT JALAN
			authorized.GET("/operasional/surattugas-list", handler.GetSuratTugasList)
			authorized.POST("/operasional/surattugas-create", handler.CreateSuratTugas)
			authorized.PUT("/operasional/surattugas-update", handler.UpdateSuratTugas)
			authorized.DELETE("/operasional/surattugas-delete", handler.DeleteSuratTugas)

			// 📄 OPERASIONAL: SURAT MUATAN UDARA (SMU)
			authorized.GET("/operasional/smu-list", handler.GetSMUList)
			authorized.POST("/operasional/smu-create", handler.CreateSMU)
			authorized.PUT("/operasional/smu-update", handler.UpdateSMU)
			authorized.DELETE("/operasional/smu-delete", handler.DeleteSMU)

			authorized.GET("/operasional/sp-terima-print", handler.GetPrintSuratPengiriman)
			authorized.GET("/operasional/sp-pad", handler.GetListSPPAD)
			authorized.GET("/master/vendor", handler.GetVendorList)

			// 📄 OPERASIONAL: STOK BARANG GUDANG
			authorized.GET("/operasional/stok-gudang", handler.GetStokBarangGudang)
			authorized.GET("/operasional/voucher-bbm", handler.GetVoucherBBM)

		}
	}

	// 6. Jalankan Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 Server Dakota Business Insight running on %s\n", port)
	fmt.Printf("🚀 Server Golang Dakota Cargo Menyala di Port: %s\n", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("❌ Server gagal jalan: %v", err)
	}
}

func ProfileHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "success",
		"data": gin.H{
			"username":  "KRIS",
			"real_name": "Kriswanto Priyo",
		},
	})
}

func CorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
