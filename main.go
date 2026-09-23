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

			// authorized.GET("/marketing/bdb/list", handler.GetBDBListHandler)
			// authorized.GET("/marketing/monitoring-btt", handler.GetMonitoringBTT)

			authorized.GET("/marketing/monitoring-btt/data", handler.GetMonitoringBtt)
			authorized.GET("/marketing/monitoring-btt/combo-customer", handler.GetComboCustomerMonitoring)

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
			authorized.DELETE("/master/korwil/:id", handler.DeleteKorwil)

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
			authorized.GET("/gl/jurnal-tidak-seimbang", handler.GetJurnalTidakSeimbang)
			authorized.GET("/gl/jurnal-tidak-seimbang/export-csv", handler.ExportJurnalTidakSeimbangCSVHandler)
			authorized.GET("/gl/search-coa", handler.SearchCoaHandler)
			authorized.GET("/gl/buku-besar-report", handler.GetBukuBesarReportHandler)
			authorized.GET("/gl/export-buku-besar-xls", handler.ExportBukuBesarXlsHandler)
			authorized.GET("/gl/neraca-saldo-report", handler.GetNeracaSaldoReportHandler)
			authorized.GET("/gl/export-neraca-saldo-xls", handler.ExportNeracaSaldoXlsHandler)
			authorized.GET("/gl/neraca-report", handler.GetNeracaReportHandler)
			authorized.GET("/gl/export-neraca-xls", handler.ExportNeracaXlsHandler)

			authorized.GET("/gl/rugilaba-report", handler.GetRugiLabaReportHandler)
			authorized.GET("/gl/export-rugilaba-xls", handler.ExportRugiLabaXlsHandler)

			authorized.GET("/gl/posisi-keuangan-report", handler.GetPosisiKeuanganReportHandler)
			authorized.GET("/gl/export-posisi-keuangan-xls", handler.ExportPosisiKeuanganXlsHandler)
			authorized.GET("/gl/labarugi-komprehensif-report", handler.GetLabaRugiKomprehensifHandler)

			authorized.GET("/gl/daftar-bank", handler.GetDaftarBankHandler)
			authorized.PUT("/gl/toggle-bank/:id", handler.ToggleStatusBankHandler)
			authorized.POST("/gl/bank", handler.CreateBankHandler)
			authorized.PUT("/gl/bank/:id", handler.UpdateBankHandler)
			authorized.DELETE("/gl/bank/:id", handler.DeleteBankHandler)
			authorized.GET("/gl/master-kota", handler.GetKotaListHandler)

			authorized.GET("/gl/pemasukan-pengeluaran", handler.GetDaftarItemHandler)
			authorized.GET("/gl/master-category-item", handler.GetCategoryItemListHandler)
			authorized.POST("/gl/pemasukan-pengeluaran", handler.CreateItemHandler)
			authorized.PUT("/gl/pemasukan-pengeluaran/:id", handler.UpdateItemHandler)
			authorized.DELETE("/gl/pemasukan-pengeluaran/:id", handler.DeleteItemHandler)

			authorized.GET("/gl/kelompok-perkiraan", handler.GetDaftarKelompokPerkiraanHandler)
			authorized.POST("/gl/kelompok-perkiraan", handler.CreateKelompokPerkiraanHandler)
			authorized.PUT("/gl/kelompok-perkiraan/:id", handler.UpdateKelompokPerkiraanHandler)
			authorized.DELETE("/gl/kelompok-perkiraan/:id", handler.DeleteKelompokPerkiraanHandler)

			authorized.GET("/gl/kode-perkiraan", handler.GetDaftarKodePerkiraanHandler)
			authorized.POST("/gl/kode-perkiraan", handler.CreateKodePerkiraanHandler)
			authorized.PUT("/gl/kode-perkiraan/:id", handler.UpdateKodePerkiraanHandler)
			authorized.DELETE("/gl/kode-perkiraan/:id", handler.DeleteKodePerkiraanHandler)

			authorized.GET("/gl/daftar-sgu", handler.GetDaftarSGUHandler)
			authorized.POST("/gl/daftar-sgu", handler.CreateSGUHandler)
			authorized.PUT("/gl/daftar-sgu/:id", handler.UpdateSGUHandler)
			authorized.DELETE("/gl/daftar-sgu/:id", handler.DeleteSGUHandler)

			authorized.GET("/gl/agen-ca", handler.GetDaftarAgenCAHandler)
			authorized.POST("/gl/agen-ca/update", handler.UpdateAgenCAMappingHandler)
			authorized.DELETE("/gl/agen-ca/:id", handler.DeleteAgenCAMappingHandler)

			authorized.GET("/gl/insentif-loper", handler.GetInsentifLoperHandler)
			authorized.GET("/gl/insentif-loper/driver-options", handler.GetDriverOptionsHandler)
			authorized.DELETE("/gl/insentif-loper/:id", handler.DeleteInsentifLoperHandler)

			authorized.GET("/gl/jurnal", handler.GetJurnalListHandler)
			authorized.DELETE("/gl/jurnal/:id", handler.DeleteJurnalHandler)
			authorized.GET("/gl/jurnal/detail/:id", handler.GetJurnalDetailHandler)
			authorized.GET("/gl/chart-accounts", handler.GetChartAccountsHandler)
			authorized.POST("/gl/jurnal/create", handler.CreateJurnalHandler)
			authorized.POST("/gl/jurnal/update", handler.UpdateJurnalHandler)

			api.GET("/gl/bank-data", handler.GetBankDataHandler)
			api.GET("/gl/etoll-data", handler.GetEtollDataHandler)

			authorized.GET("/gl/komisi-sopir", handler.GetKomisiSopirListHandler)
			authorized.GET("/gl/komisi-sopir/lookup-sp/:nosp", handler.LookupSPKomisiHandler)
			authorized.POST("/gl/komisi-sopir/create", handler.CreateKomisiSopirHandler)
			authorized.DELETE("/gl/komisi-sopir/:id", handler.DeleteKomisiSopirHandler)

			authorized.GET("/gl/cashbank", handler.GetCashBankListHandler)
			authorized.POST("/gl/cashbank/create", handler.CreateCashBankHandler)
			authorized.DELETE("/gl/cashbank/*id", handler.DeleteCashBankHandler)
			authorized.POST("/gl/cashbank/update", handler.UpdateCashBankHandler)
			authorized.POST("/gl/cashbank/post", handler.PostCashBankHandler)
			authorized.POST("/gl/cashbank/unpost", handler.UnpostCashBankHandler)
			authorized.GET("/gl/cashbank/items", handler.GetMasterItemBiayaHandler)

			authorized.GET("/gl/pembayaran-vendor", handler.GetPembayaranVendorListHandler)
			authorized.POST("/gl/pembayaran-vendor/create", handler.CreatePembayaranVendorHandler)
			authorized.DELETE("/gl/pembayaran-vendor/:id", handler.DeletePembayaranVendorHandler)
			authorized.GET("/gl/pembayaran-vendor/vendor-options", handler.GetVendorOptionsHandler)
			authorized.PUT("/gl/pembayaran-vendor/update", handler.UpdatePembayaranVendorHandler)

			authorized.GET("/gl/posting-jurnal/status", handler.GetPostingJurnalStatusHandler)
			authorized.POST("/gl/posting-jurnal/process", handler.ProcessPostingJurnalHandler)
			authorized.GET("/gl/posting-jurnal/year-options", handler.GetYearOptionsHandler)

			authorized.GET("/gl/setoran-cod", handler.GetSetoranCODListHandler)
			authorized.GET("/gl/setoran-cod/detail/:id", handler.GetSetoranCODDetailHandler)
			authorized.POST("/gl/setoran-cod/create", handler.CreateSetoranCODHandler)
			authorized.PUT("/gl/setoran-cod/update", handler.UpdateSetoranCODHandler)
			authorized.DELETE("/gl/setoran-cod/:id", handler.DeleteSetoranCODHandler)

			authorized.GET("/gl/setoran-cod/btt-options", handler.GetBTTCODOptionsHandler)

			authorized.GET("/piutang/aging", handler.GetAgingPiutangHandler)
			authorized.GET("/gl/customers", handler.GetCustomerOptionsHandler)

			authorized.GET("/piutang/approval-customer", handler.GetApprovalCustomerListHandler)
			authorized.GET("/piutang/approval-customer/detail/:id", handler.GetApprovalCustomerDetailHandler)
			authorized.POST("/piutang/approval-customer/save", handler.SaveApprovalCustomerHandler)
			authorized.DELETE("/piutang/approval-customer/:id", handler.DeleteApprovalCustomerHandler)
			authorized.GET("/marketing/cities", handler.GetKotaOptionsHandler)

			authorized.GET("/piutang/btt-tagih-tujuan", handler.GetBTTTagihTujuanListHandler)
			authorized.GET("/piutang/btt-tagih-tujuan/detail/:id", handler.GetBTTTagihTujuanDetailHandler)
			authorized.POST("/piutang/btt-tagih-tujuan/save", handler.SaveBTTTagihTujuanHandler)

			// Credit Note (Piutang)
			authorized.GET("/piutang/credit-note", handler.GetCreditNoteListHandler)
			authorized.GET("/piutang/credit-note/detail/:no", handler.GetCreditNoteDetailHandler)
			authorized.GET("/piutang/credit-note/invoices-outstanding", handler.GetInvoicesOutstandingHandler)
			authorized.POST("/piutang/credit-note/save", handler.SaveCreditNoteHandler)
			authorized.POST("/piutang/credit-note/posting", handler.PostingCreditNoteHandler)
			authorized.POST("/piutang/credit-note/unposting", handler.UnpostingCreditNoteHandler)
			authorized.DELETE("/piutang/credit-note/:no", handler.DeleteCreditNoteHandler)

			// Invoice (Piutang Penagihan)
			authorized.GET("/piutang/invoice", handler.GetInvoiceListHandler)
			authorized.GET("/piutang/invoice/detail", handler.GetInvoiceDetailHandler)
			authorized.GET("/piutang/invoice/detail/*id", handler.GetInvoiceDetailHandler)
			authorized.GET("/piutang/invoice/unbilled-btt", handler.GetUnbilledBTTHandler)
			authorized.POST("/piutang/invoice/save", handler.SaveInvoiceHandler)
			authorized.POST("/piutang/invoice/acc", handler.UpdateTglACCInvoiceHandler)
			authorized.DELETE("/piutang/invoice", handler.DeleteInvoiceHandler)
			authorized.DELETE("/piutang/invoice/*id", handler.DeleteInvoiceHandler)
			authorized.POST("/piutang/invoice/update", handler.UpdateInvoiceFullHandler)
			authorized.POST("/piutang/invoice/unposting", handler.UnpostingInvoiceHandler)
			authorized.POST("/piutang/invoice/posting", handler.PostingInvoiceHandler)
			//authorized.GET("/gl/jurnal/detail/:id", handler.GetJurnalDetailHandler)

			// Kondisi BTT dan Order Jemput
			authorized.GET("/piutang/kondisi-btt/options", handler.GetKondisiBTTOptionsHandler)
			authorized.GET("/piutang/kondisi-btt", handler.GetKondisiBTTListHandler)

			// Master Faktur Pajak Berjalan
			authorized.GET("/piutang/faktur-pajak", handler.GetFakturPajakHandler)
			authorized.POST("/piutang/faktur-pajak/update", handler.UpdateFakturPajakHandler)

			authorized.GET("/piutang/mutasi-piutang", handler.GetMutasiPiutangHandler)
			authorized.GET("/marketing/customers", handler.GetCustomerDropdownHandler)
			authorized.GET("/customers", handler.GetCustomerDropdownHandler)

			// Penagihan Invoice oleh Kolektor
			authorized.GET("/piutang/tagih-invoice", handler.GetTagihInvoiceListHandler)
			authorized.GET("/piutang/tagih-invoice/available-invoices", handler.GetAvailableInvoicesForTagih)
			authorized.GET("/piutang/tagih-invoice/detail", handler.GetTagihInvoiceDetailByID)
			authorized.POST("/piutang/tagih-invoice", handler.CreateTagihInvoiceHandler)
			authorized.PUT("/piutang/tagih-invoice/batal", handler.CancelTagihInvoiceHandler)
			authorized.GET("/karyawan/kolektor-dropdown", handler.GetKolektorDropdownHandler)

			// PENERIMAAN PEMBAYARAN KREDIT
			authorized.GET("/piutang/penerimaan-pembayaran-kredit", handler.GetPenerimaanPembayaranKreditListHandler)
			authorized.GET("/piutang/penerimaan-pembayaran-kredit/detail", handler.GetPenerimaanPembayaranKreditDetailHandler)
			authorized.POST("/piutang/penerimaan-pembayaran-kredit", handler.CreatePenerimaanPembayaranKreditHandler)
			authorized.PUT("/piutang/penerimaan-pembayaran-kredit/batal", handler.CancelPenerimaanPembayaranKreditHandler)
			authorized.GET("/akun/kas-bank-dropdown", handler.GetKasBankAccountsHandler)

			// Alias / Fallback Endpoint
			authorized.GET("/piutang/receipt", handler.GetPenerimaanPembayaranKreditListHandler)
			authorized.GET("/piutang/receipt/detail", handler.GetPenerimaanPembayaranKreditDetailHandler)
			authorized.POST("/piutang/receipt", handler.CreatePenerimaanPembayaranKreditHandler)
			authorized.PUT("/piutang/receipt/batal", handler.CancelPenerimaanPembayaranKreditHandler)

			// PENERIMAAN PENAGIHAN KOLEKTOR
			authorized.GET("/piutang/penerimaan-penagihan-kolektor", handler.GetPenerimaanPenagihanListHandler)
			authorized.GET("/piutang/penerimaan-penagihan-kolektor/detail", handler.GetPenerimaanPenagihanDetailHandler)
			authorized.POST("/piutang/penerimaan-penagihan-kolektor/konfirmasi", handler.ConfirmPenerimaanPenagihanHandler)

			// PENERIMAAN SETORAN AGEN
			authorized.GET("/piutang/penerimaan-setoran-agen", handler.GetPenerimaanSetoranAgenListHandler)
			authorized.GET("/piutang/penerimaan-setoran-agen/detail", handler.GetPenerimaanSetoranAgenDetailHandler)
			authorized.POST("/piutang/penerimaan-setoran-agen/proses", handler.ProcessSetoranAgenHandler)

			// PROFORMA INVOICE
			authorized.GET("/piutang/proforma-invoice", handler.GetProformaInvoiceListHandler)
			authorized.GET("/piutang/proforma-invoice/detail", handler.GetProformaInvoiceDetailHandler)
			authorized.POST("/piutang/proforma-invoice/create", handler.CreateProformaInvoiceHandler)
			authorized.POST("/piutang/proforma-invoice/cancel", handler.CancelProformaInvoiceHandler)
			authorized.POST("/piutang/proforma-invoice/update", handler.UpdateProformaInvoiceHandler)
			authorized.GET("/pelanggan", handler.GetPelangganDropdownHandler)

			// PROSES PIUTANG
			authorized.POST("/piutang/proses-piutang/eksekusi", handler.ExecuteProsesPiutangHandler)
			authorized.GET("/piutang/proses-piutang/histori", handler.GetHistoriSaldoPiutangHandler)
			authorized.GET("/piutang/proses-piutang/detail-customer", handler.GetDetailProsesPiutangCustomerHandler)
			authorized.GET("/piutang/proses-piutang/preview", handler.PreviewProsesPiutangHandler)

			// REVISI BTT APL (HARGA)
			authorized.GET("/piutang/revisi-btt/search", handler.SearchBTTForRevisiHandler)
			authorized.POST("/piutang/revisi-btt/submit", handler.SubmitRevisiBTTHargaHandler)
			authorized.GET("/piutang/revisi-btt/history", handler.GetHistoryRevisiBTTHargaHandler)

			// SET SALDO AWAL PIUTANG
			authorized.GET("/piutang/saldo-awal/list", handler.GetListSaldoAwalPiutangHandler)
			authorized.POST("/piutang/saldo-awal/save", handler.SaveSaldoAwalPiutangHandler)

			// TUKAR FAKTUR
			authorized.GET("/piutang/tukar-faktur/list", handler.GetListTukarFakturHandler)
			authorized.GET("/piutang/tukar-faktur/available-invoices", handler.GetAvailableInvoicesForTFHandler)
			authorized.POST("/piutang/tukar-faktur/save", handler.SaveTukarFakturHandler)
			authorized.DELETE("/piutang/tukar-faktur/delete", handler.DeleteTukarFakturHandler)

			// AGING HUTANG VENDOR
			authorized.GET("/hutang/aging-vendor/list", handler.GetAgingHutangVendorHandler)
			// INVOICE VENDOR (HUTANG)
			authorized.GET("/hutang/invoice-vendor/list", handler.GetListInvoiceVendorHandler)
			authorized.POST("/hutang/invoice-vendor/save", handler.SaveInvoiceVendorHandler)
			authorized.DELETE("/hutang/invoice-vendor/delete", handler.DeleteInvoiceVendorHandler)

			// 👥 MASTER KARYAWAN (HRD)
			authorized.GET("/hrd/karyawan", handler.GetKaryawanListHandler)
			authorized.GET("/hrd/karyawan/:nip", handler.GetKaryawanDetailHandler)
			authorized.POST("/hrd/karyawan/save", handler.SaveKaryawanHandler)
			authorized.DELETE("/hrd/karyawan/:nip", handler.DeleteKaryawanHandler)
			authorized.GET("/hrd/divisi-options", handler.GetDivisiOptionsHandler)
			authorized.GET("/hrd/jabatan-options", handler.GetJabatanOptionsHandler)

			// 📑 FORM HRD & DOKUMEN DOWNLOAD
			authorized.GET("/hrd/form/list", handler.GetListFormHRD)
			authorized.POST("/hrd/form/upload", handler.UploadFormHRD)
			authorized.GET("/hrd/form/download/:id", handler.DownloadFormHRD)
			authorized.DELETE("/hrd/form/delete/:id", handler.DeleteFormHRD)
			authorized.GET("/hrd/form/view/:id", handler.ViewFormHRD)

			// 📍 MASTER DALAM KOTA (MKT)
			authorized.GET("/mkt/dalam-kota/list", handler.GetMasterDalamKota)
			authorized.GET("/mkt/dalam-kota/suggest", handler.GetMasterDalamKotaSuggest)
			authorized.POST("/mkt/dalam-kota/save", handler.SaveMasterDalamKota)
			authorized.DELETE("/mkt/dalam-kota/delete/:id", handler.DeleteMasterDalamKota)

			authorized.GET("/econote/cek-btt", handler.CekBTTManualAtauBarcode)

			authorized.GET("/laporan/handling/kendaraan", handler.GetKendaraanComboHandling)
			authorized.GET("/laporan/handling/data", handler.GetLaporanHandlingBarang)

			authorized.GET("/handling/kendaraan", handler.GetKendaraanComboHandling)
			authorized.GET("/handling/data", handler.GetLaporanHandlingBarang)

			authorized.GET("/laporan/btt-counter/combo-cabang", handler.GetCabangComboCounter)
			authorized.GET("/laporan/btt-counter/combo-kota", handler.GetKotaComboCounter)
			authorized.GET("/laporan/btt-counter/data", handler.GetLaporanBTTCounter)

			// Modul Laporan Penjualan BTT Harian (LPH)
			authorized.GET("/laporan/penjualan-harian/data", handler.GetLaporanPenjualanHarian)
			authorized.GET("/laporan/penjualan-harian/detail/:id", handler.GetDetailLaporanPenjualanHarian)
			authorized.GET("/laporan/penjualan-harian/btt-tersedia", handler.GetBTTTersediaUntukLaporan)
			authorized.POST("/laporan/penjualan-harian/create", handler.CreateLaporanPenjualanHarian)
			authorized.POST("/laporan/penjualan-harian/toggle-posting", handler.TogglePostingPenjualanHarian)

			authorized.GET("/laporan/penjualan/data", handler.GetLaporanPenjualan)
			authorized.GET("/laporan/penjualan/combo-kota", handler.GetComboKotaPenjualan)

			authorized.GET("/laporan/btt-outstanding/data", handler.GetBttOutstanding)

			authorized.GET("/marketing/packing-list/data", handler.GetPackingListData)
			authorized.GET("/marketing/packing-list/detail/:id", handler.GetPackingListDetail)
			authorized.POST("/marketing/packing-list/process-btt", handler.ProcessPackingListToBTT)
			authorized.POST("/marketing/packing-list/upload-csv", handler.UploadPackingListCSV)

			authorized.GET("/marketing/asuransi/data", handler.GetAsuransiList)
			authorized.POST("/marketing/asuransi/save", handler.SaveAsuransi)
			authorized.DELETE("/marketing/asuransi/:id", handler.DeleteAsuransi)

			authorized.GET("/marketing/order-jemput/data", handler.GetOrderJemputList)
			authorized.POST("/marketing/order-jemput/save", handler.SaveOrderJemput)
			authorized.DELETE("/marketing/order-jemput/:id", handler.DeleteOrderJemput)

			authorized.GET("/terima-btt/data", handler.GetTerimaBTTList)
			authorized.POST("/terima-btt/save", handler.SaveTerimaBTT)
			authorized.DELETE("/terima-btt/:id", handler.DeleteTerimaBTT)
			authorized.GET("/terima-btt/combo-kota", handler.GetComboKota)

			authorized.GET("/dashboard/metrics", handler.GetDashboardMetrics)
			authorized.GET("/dashboard/btt-by-status", handler.GetBTTByStatus)
			authorized.POST("/ai/chat", handler.AskAI)

			authorized.GET("/marketing/terima-retur", handler.GetTerimaReturList)
			authorized.PUT("/marketing/terima-retur/deactivate/:btt_id", handler.DeactivateTerimaRetur)
			authorized.GET("/marketing/terima-retur/check/:btt_id", handler.GetInfoBTTForRetur)
			authorized.POST("/marketing/terima-retur", handler.CreateTerimaRetur)
			authorized.GET("/marketing/terima-retur/options", handler.GetFilterOptionsTerimaRetur)

			authorized.GET("/marketing/econote-bayar", handler.GetEconoteBayarList)
			authorized.PUT("/marketing/econote-bayar/deactivate/:eid", handler.DeactivateEconoteBayar)
			authorized.GET("/marketing/econote-bayar/check/:btt_id", handler.GetBTTForBayar)
			authorized.POST("/marketing/econote-bayar", handler.CreateEconoteBayar)

			authorized.GET("/marketing/kembali-sj/available", handler.GetAvailableSJ)
			authorized.POST("/marketing/kembali-sj", handler.CreateKembaliSJ)
			authorized.DELETE("/marketing/kembali-sj/:id", handler.DeleteKembaliSJ)

			authorized.GET("/marketing/outstanding-sj/:customer_id", handler.GetOutstandingSJ)
			authorized.POST("/marketing/kembali-sj/add", handler.CreateKembaliSJAdd)

			// 💵 MARKETING / KASIR: SETORAN PENJUALAN TUNAI
			authorized.GET("/marketing/setoran-tunai", handler.GetSetoranTunaiList)
			authorized.GET("/marketing/setoran-tunai/available-lph", handler.GetAvailableLPH)
			authorized.POST("/marketing/setoran-tunai", handler.CreateSetoranTunai)
			authorized.DELETE("/marketing/setoran-tunai/:id", handler.DeleteSetoranTunai)

			authorized.GET("/marketing/transport-planning/suggest-customer", handler.SuggestCustomerPlanning)
			authorized.POST("/marketing/transport-planning/parse-csv", handler.ParseTransportPlanningCSV)
			authorized.POST("/marketing/transport-planning/save-batch", handler.SaveTransportPlanningBatch)

			// 📦 MARKETING / CUSTOMER - UPLOAD CSV: CUSTOMER KHUSUS
			authorized.GET("/marketing/customer-khusus/suggest", handler.SuggestCustomerKhusus)
			authorized.POST("/marketing/customer-khusus/upload", handler.UploadCSVCustomerKhusus)
			authorized.GET("/marketing/customer-khusus/unprocessed", handler.GetUnprocessedBTT)
			authorized.GET("/marketing/customer-khusus/hold-list", handler.GetBTTHoldList)
			authorized.POST("/marketing/customer-khusus/toggle-hold", handler.ToggleHoldBTT)

			// 🚀 MARKETING / CUSTOMER - UPLOAD DATA UNTUK PEMBUATAN BTT (KIMIA FARMA / MERCK)
			authorized.GET("/marketing/upload-btt/suggest-customer", handler.SuggestCustomerBTT)
			authorized.POST("/marketing/upload-btt/parse-csv", handler.ParseBTTCSV)
			authorized.POST("/marketing/upload-btt/save-batch", handler.SaveBTTCSVBatch)

			// 📦 MARKETING / UPLOAD CSV (BTT UPLOAD V2)
			authorized.GET("/marketing/btt-upload-v2/suggest-customer", handler.SuggestCustomerBTTV2)
			authorized.POST("/marketing/btt-upload-v2/parse-csv", handler.ParseBTTV2CSV)
			authorized.POST("/marketing/btt-upload-v2/save-batch", handler.SaveBTTV2CSVBatch)

			// 📑 MARKETING / UPLOAD DN/OJ
			authorized.GET("/marketing/dn-upload/outstanding", handler.GetDNBelumMasukLSPB)
			authorized.POST("/marketing/dn-upload/upload", handler.UploadDNCSV)
			// 🚚 MARKETING / OPERASIONAL: UPLOAD CSV DATA LOPERAN SUPIR
			authorized.POST("/marketing/hasil-loper/parse-csv", handler.ParseHasilLoperCSV)
			authorized.POST("/marketing/hasil-loper/save-batch", handler.SaveHasilLoperBatch)
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
