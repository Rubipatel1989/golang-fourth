package main

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

var db *sql.DB

func main() {
	var err error
	// 🔐 Update your DB credentials here
	dsn := "root:new_password@tcp(localhost:3306)/gin_demo"
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("DB not reachable:", err)
	}

	r := gin.Default()

	r.POST("/signup", signup)
	r.POST("/login", login)
	auth := r.Group("/")
	auth.Use(AuthMiddleware())
	auth.GET("/users", getUsers)
	auth.GET("/users/:id", getSingleUser)
	auth.PUT("/users/:id", updateUser)
	auth.DELETE("/users/:id", deleteUser)

	r.Run(":8080")
}

func successResponse(message string, data any) gin.H {
	return gin.H{
		"status":  true,
		"message": message,
		"data":    data,
	}
}

func errorResponse(message string) gin.H {
	return gin.H{
		"status":  false,
		"message": message,
	}
}

func signup(c *gin.Context) {
	var u struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Age      int    `json:"age"`
		Password string `json:"password"`
	}

	// Bind JSON
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("Invalid request data: "+err.Error()))
		return
	}

	// Check if email already exists
	var existingID int
	err := db.QueryRow("SELECT id FROM users WHERE email = ?", u.Email).Scan(&existingID)
	if err == nil {
		c.JSON(http.StatusConflict, errorResponse("Email already exists"))
		return
	} else if err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, errorResponse("Database error: "+err.Error()))
		return
	}

	// Hash password
	hashedPassword, err := HashPassword(u.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Failed to hash password"))
		return
	}

	// Insert into DB
	result, err := db.Exec("INSERT INTO users(name, email, age, password) VALUES (?, ?, ?, ?)",
		u.Name, u.Email, u.Age, hashedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Failed to create user: "+err.Error()))
		return
	}

	// Return inserted user details
	id, _ := result.LastInsertId()
	c.JSON(http.StatusCreated, successResponse("Signup successful", gin.H{
		"id":    id,
		"name":  u.Name,
		"email": u.Email,
		"age":   u.Age,
	}))
}

func login(c *gin.Context) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user User
	var storedHash string
	err := db.QueryRow("SELECT id, name, email, age, password FROM users WHERE email = ?", input.Email).
		Scan(&user.ID, &user.Name, &user.Email, &user.Age, &storedHash)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	if !CheckPasswordHash(input.Password, storedHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	token, err := GenerateToken(user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token generation failed"})
		return
	}

	c.JSON(http.StatusOK, successResponse("Login successful", gin.H{
		"token": token,
		"user":  user,
	}))

}

func getUsers(c *gin.Context) {
	rows, err := db.Query("SELECT id, name, email, age FROM users")
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Failed to fetch users: "+err.Error()))
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Age); err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse("Failed to scan user: "+err.Error()))
			return
		}
		users = append(users, u)
	}

	c.JSON(http.StatusOK, successResponse("Users fetched successfully", users))
}

func getSingleUser(c *gin.Context) {
	id := c.Param("id")

	var u User
	err := db.QueryRow("SELECT id, name, email, age FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Name, &u.Email, &u.Age)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, errorResponse("User not found"))
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Database error: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, successResponse("User fetched successfully", u))
}

func createUser(c *gin.Context) {
	var u User
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := db.Exec("INSERT INTO users(name, email, age) VALUES (?, ?, ?)", u.Name, u.Email, u.Age)
	// Check for errors
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(err.Error()))
		return
	}

	id, _ := result.LastInsertId()
	u.ID = int(id)

	c.JSON(http.StatusOK, successResponse("Users fetched successfully", u))

}

func updateUser(c *gin.Context) {
	id := c.Param("id")
	var u User
	if err := c.ShouldBindJSON(&u); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := db.Exec("UPDATE users SET name = ?, email = ?, age = ? WHERE id = ?", u.Name, u.Email, u.Age, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(err.Error()))
		return
	}

	u.ID, _ = strconv.Atoi(id)
	c.JSON(http.StatusOK, successResponse("Users updated successfully", u))

}

func deleteUser(c *gin.Context) {
	id := c.Param("id")

	// Check if user exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", id).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Database error: "+err.Error()))
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, errorResponse("User not found"))
		return
	}

	// Proceed to delete
	_, err = db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("Failed to delete user: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, successResponse("User deleted successfully", nil))
}
