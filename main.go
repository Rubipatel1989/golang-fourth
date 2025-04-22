package main

import (
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default() // creates a default router with logger and recovery middleware

	// define a route
	r.GET("/hello", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello, World!",
		})
	})

	// run the server on port 8080
	r.Run(":8080")
}
