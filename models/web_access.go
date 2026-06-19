package models

type AccessRequest struct {
	Username    string                     `json:"username" binding:"required"`
	Permissions map[string]map[string]bool `json:"permissions" binding:"required"`
}
