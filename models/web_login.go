package models

type WebLogin struct {
	// Gunakan tag gorm:"column:NAMA_KOLOM_DI_SQL" supaya mappingnya presisi
	Username     string   `gorm:"column:username" json:"username"`
	RealName     string   `gorm:"column:realname" json:"realname"`
	KodeCabang   string   `gorm:"column:kode_cabang" json:"kode_cabang"`
	All_cabangYN string   `gorm:"column:all_cabangyn" json:"all_cabangyn"`
	LastLogin    string   `gorm:"column:lastlogin" json:"lastlogin"`
	User_aktifYN string   `gorm:"column:user_aktifyn" json:"user_aktifyn"`
	ProfileImage string   `gorm:"column:profileimage" json:"profileimage"`
	Gender       string   `gorm:"column:gender" json:"gender"`
	MobileNumber string   `json:"mobilenumber" gorm:"column:mobilenumber"`
	Email        string   `json:"email" gorm:"column:email"`
	Password     string   `json:"password" gorm:"column:password"`
	PasswordJWT  string   `json:"passwordjwt" gorm:"column:passwordjwt"`
	UserType     string   `gorm:"column:usertype" json:"usertype"`
	LastIPlogin  string   `gorm:"column:lastiplogin" json:"lastiplogin"`
	NickName     string   `gorm:"column:nickname" json:"nickname"`
	Cabangs      []string `gorm:"-" json:"cabangs"`
}
