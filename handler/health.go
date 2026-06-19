package handler

import (
	"dakotagroup/business-insight-be/db"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthResponse struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"`
}

func pingStatus(name string, conn interface{ Ping() error }) string {
	if err := conn.Ping(); err != nil {
		log.Printf("[health] %s ping failed: %v", name, err)
		return "down"
	}
	return "up"
}

func HealthHandler(c *gin.Context) {
	services := map[string]string{
		"database_dbs": pingStatus("DBS", db.DBS),
		"database_dlb": pingStatus("DLB", db.DLB),
		"database_dli": pingStatus("DLI", db.DLI),
	}

	overallStatus := "healthy"
	for _, status := range services {
		if status == "down" {
			overallStatus = "unhealthy"
			break
		}
	}

	resp := HealthResponse{
		Status:   overallStatus,
		Services: services,
	}

	if overallStatus == "unhealthy" {
		c.JSON(http.StatusServiceUnavailable, resp)
	} else {
		c.JSON(http.StatusOK, resp)
	}
}
