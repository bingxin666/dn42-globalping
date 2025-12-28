package main

import (
	"log"
	"net/http"

	"github.com/bingxin666/dn42-globalping/internal/handler"
	"github.com/bingxin666/dn42-globalping/internal/hub"
	"github.com/gin-gonic/gin"
)

func main() {
	// Create hub for managing connections
	h := hub.NewHub()

	// Create handler
	hdl := handler.NewHandler(h)

	// Setup Gin router
	r := gin.Default()

	// Serve static files for frontend
	r.Static("/assets", "./web/dist/assets")
	r.StaticFile("/vite.svg", "./web/dist/vite.svg")

	// API routes
	api := r.Group("/api")
	{
		api.GET("/probes", hdl.GetProbes)
	}

	// WebSocket routes
	r.GET("/ws/probe", hdl.HandleProbeWS)
	r.GET("/ws/client", hdl.HandleClientWS)

	// Serve index.html for all other routes (SPA)
	r.NoRoute(func(c *gin.Context) {
		c.File("./web/dist/index.html")
	})

	// Add CORS middleware for development
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	log.Println("Server starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
