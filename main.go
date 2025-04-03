package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

const (
	WebSocketEndpoint = "/ws"
	HTTPPort          = ":8080"
	DBConnection      = "root:0000@tcp(localhost:3306)/qairline?parseTime=true"
	JWTSecret         = "your-very-secret-key-12345!"
	TokenExpiration   = 24 * time.Hour
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var db *sql.DB

type (
	User struct {
		ID        int
		Username  string
		Password  string
		TeamID    sql.NullInt64
		AvatarURL string
	}

	Team struct {
		ID   int
		Name string
	}

	AuthRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	AuthResponse struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Token   string `json:"token,omitempty"`
	}

	UserData struct {
		ID        int    `json:"id"`
		Username  string `json:"username"`
		TeamName  string `json:"team_name"`
		AvatarURL string `json:"avatar_url"`
	}

	Analytics struct {
		TotalTests    int `json:"total_tests"`
		Passed        int `json:"passed"`
		Failed        int `json:"failed"`
		DevicesOnline int `json:"devices_online"`
	}

	Claims struct {
		UserID int `json:"user_id"`
		jwt.RegisteredClaims
	}

	WebSocketMessage struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
)

func main() {
	initDatabase()
	defer db.Close()

	// Регистрируем только WebSocket endpoint
	http.HandleFunc(WebSocketEndpoint, wsHandler)
	http.HandleFunc("/health", healthCheck)

	log.Printf("Server starting on %s", HTTPPort)
	log.Fatal(http.ListenAndServe(HTTPPort, nil))
}

func initDatabase() {
	var err error
	db, err = sql.Open("mysql", DBConnection)
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("Database ping failed:", err)
	}

	createTables()
}

func createTables() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS teams (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			password VARCHAR(100) NOT NULL,
			team_id INT,
			avatar_url VARCHAR(255)
		)`,
		`CREATE TABLE IF NOT EXISTS tests (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			status ENUM('passed', 'failed') NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			online BOOLEAN DEFAULT false
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			log.Fatalf("Failed to create table: %v", err)
		}
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	// Основной цикл обработки сообщений
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("Connection error: %v", err)
			}
			return
		}

		var msg WebSocketMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			sendError(conn, "invalid message format")
			continue
		}

		switch msg.Type {
		case "auth":
			handleAuth(conn, msg.Payload)
		case "request_analytics":
			handleAnalyticsRequest(conn)
		default:
			sendError(conn, "unknown message type")
		}

		// Эхо-ответ для тестирования
		if err := conn.WriteMessage(messageType, message); err != nil {
			log.Println("Write error:", err)
			return
		}
	}
}
func handleConnection(conn *websocket.Conn) {
	defer conn.Close()
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
					log.Printf("Read error: %v", err)
				}
				return
			}

			var msg WebSocketMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				sendError(conn, "invalid message format")
				continue
			}

			switch msg.Type {
			case "auth":
				handleAuth(conn, msg.Payload) // Передаем done в handleAuth
			case "request_analytics":
				handleAnalyticsRequest(conn)
			default:
				sendError(conn, "unknown message type")
			}
		}
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	select {
	case <-done:
		return
	case <-ticker.C:
		// Общая аналитика для всех подключений
		handleAnalyticsRequest(conn)
	}
}

func handleAuth(conn *websocket.Conn, payload json.RawMessage) {
	var req AuthRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		sendError(conn, "invalid auth request")
		return
	}

	user, err := getUser(req.Username)
	if err != nil {
		sendError(conn, "user not found")
		return
	}

	if !checkPassword(user.Password, req.Password) {
		sendError(conn, "invalid password")
		return
	}

	token, err := generateToken(user.ID)
	if err != nil {
		sendError(conn, "failed to generate token")
		return
	}

	// Отправка ответа
	response := AuthResponse{
		Status:  "success",
		Message: "authenticated",
		Token:   token,
	}
	sendMessage(conn, "auth_response", response)
}

func handleAnalyticsRequest(conn *websocket.Conn) {
	analytics := getGeneralAnalytics()
	sendMessage(conn, "analytics", analytics)
}

func sendAuthSuccess(conn *websocket.Conn, token string) {
	response := AuthResponse{
		Status:  "success",
		Message: "authenticated",
		Token:   token,
	}
	sendMessage(conn, "auth", response)
}

func sendUserData(conn *websocket.Conn, userID int) {
	userData, err := getUserData(userID)
	if err != nil {
		sendError(conn, "failed to get user data")
		return
	}
	sendMessage(conn, "user_data", userData)
}

func sendAnalyticsUpdates(conn *websocket.Conn, userID int, done <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			analytics := getUserAnalytics(userID)
			sendMessage(conn, "analytics", analytics)
		case <-done:
			return
		}
	}
}

// Database functions
func getUser(username string) (*User, error) {
	user := &User{}
	err := db.QueryRow(`
		SELECT id, username, password, team_id, avatar_url 
		FROM users 
		WHERE username = ?
	`, username).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.TeamID,
		&user.AvatarURL,
	)

	if err != nil {
		return nil, err
	}
	return user, nil
}

func getUserData(userID int) (*UserData, error) {
	data := &UserData{}
	err := db.QueryRow(`
		SELECT u.id, u.username, COALESCE(t.name, ''), u.avatar_url 
		FROM users u
		LEFT JOIN teams t ON u.team_id = t.id
		WHERE u.id = ?
	`, userID).Scan(&data.ID, &data.Username, &data.TeamName, &data.AvatarURL)
	return data, err
}

func getUserAnalytics(userID int) Analytics {
	var analytics Analytics
	db.QueryRow(`
		SELECT 
			COUNT(*) AS total_tests,
			SUM(status = 'passed') AS passed,
			SUM(status = 'failed') AS failed,
			(SELECT COUNT(*) FROM devices WHERE online = true) AS devices_online
		FROM tests
		WHERE user_id = ?
	`, userID).Scan(
		&analytics.TotalTests,
		&analytics.Passed,
		&analytics.Failed,
		&analytics.DevicesOnline,
	)
	return analytics
}

func getGeneralAnalytics() Analytics {
	var analytics Analytics
	db.QueryRow(`
		SELECT 
			COUNT(*) AS total_tests,
			SUM(status = 'passed') AS passed,
			SUM(status = 'failed') AS failed,
			(SELECT COUNT(*) FROM devices WHERE online = true) AS devices_online
		FROM tests
	`).Scan(
		&analytics.TotalTests,
		&analytics.Passed,
		&analytics.Failed,
		&analytics.DevicesOnline,
	)
	return analytics
}

// Security functions
func checkPassword(hashedPassword, password string) bool {
	return bcrypt.CompareHashAndPassword(
		[]byte(hashedPassword),
		[]byte(password),
	) == nil
}

func generateToken(userID int) (string, error) {
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenExpiration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(JWTSecret))
}

// Helper functions
func sendMessage(conn *websocket.Conn, msgType string, data interface{}) {
	msg := WebSocketMessage{
		Type:    msgType,
		Payload: json.RawMessage{},
	}

	payload, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling data: %v", err)
		return
	}
	msg.Payload = payload

	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func sendError(conn *websocket.Conn, message string) {
	sendMessage(conn, "error", map[string]string{
		"status":  "error",
		"message": message,
	})
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
