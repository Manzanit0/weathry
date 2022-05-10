package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	var port string
	if port = os.Getenv("PORT"); port == "" {
		port = "8080"
	}

	r.Run(fmt.Sprintf(":%s", port))
}
