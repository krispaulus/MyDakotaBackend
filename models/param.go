package models

import "time"

type ConfigParam struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SetCabID       string    `gorm:"column:set_cabid" json:"set_cabid"`
	SetVarName     string    `gorm:"column:set_varname" json:"set_varname"`
	SetDescription string    `gorm:"column:set_description" json:"set_description"`
	SetVarValue    string    `gorm:"column:set_varvalue" json:"set_varvalue"`
	SetVarType     string    `gorm:"column:set_vartype" json:"set_vartype"`
	SetUpdateID    string    `gorm:"column:set_updateid" json:"set_updateid"`
	SetUpdateTime  time.Time `gorm:"column:set_updatetime;autoUpdateTime" json:"set_updatetime"`
}

func (ConfigParam) TableName() string {
	return "glb_m_param"
}

type UpdateParamRequest struct {
	ID             int64  `json:"id"`
	SetCabID       string `json:"set_cabid"`
	SetVarName     string `json:"set_varname"`
	SetDescription string `json:"set_description"`
	SetVarValue    string `json:"set_varvalue"`
	SetVarType     string `json:"set_vartype"`
}
