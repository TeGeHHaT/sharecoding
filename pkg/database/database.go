package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

// SessionInfo содержит информацию о сессии
type SessionInfo struct {
	SessionID    string
	SessionDBURL string
}

// DBHandler определяет методы для работы с базой данных
type DBHandler interface {
	CreateSessionDatabase(sessionID string) (*sql.DB, error)
	GetSessionInfo(sessionID string) (*SessionInfo, error)
}

// PostgreSQLHandler реализует DBHandler для PostgreSQL
type PostgreSQLHandler struct {
	MainDB *sql.DB
}

// NewPostgreSQLHandler создает новый экземпляр PostgreSQLHandler
func NewPostgreSQLHandler(mainDB *sql.DB) *PostgreSQLHandler {
	return &PostgreSQLHandler{MainDB: mainDB}
}

// CreateSessionDatabase создает новую базу данных для сессии
func (h *PostgreSQLHandler) CreateSessionDatabase(sessionID string) (*sql.DB, error) {
	// Загрузка переменных окружения
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// Формирование строки подключения
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)

	// Подключение к основной БД
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Создание новой БД для сессии, если ее нет
	sessionDBName := strings.ToLower(fmt.Sprintf("session_%s", sessionID))
	rows, err := db.Query("SELECT 1 FROM pg_database WHERE datname=$1", sessionDBName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Если база данных существует, возвращаем подключение к ней
	if rows.Next() {
		connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, sessionDBName)
		sessionDB, err := sql.Open("postgres", connStr)
		if err != nil {
			return nil, err
		}
		return sessionDB, nil
	}

	// Если база данных не существует, создаем ее
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", sessionDBName))
	if err != nil {
		return nil, err
	}

	// Подключение к новой БД
	connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, sessionDBName)
	sessionDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Логирование успешного создания базы данных
	log.Printf("Successfully created session database: %s", sessionDBName)

	// Создание таблицы
	_, err = sessionDB.Exec(`
		CREATE TABLE IF NOT EXISTS shared_code (
			id SERIAL PRIMARY KEY,
			code TEXT NOT NULL
		)
	`)

	_, err = sessionDB.Exec(`
	CREATE TABLE IF NOT EXISTS sessions (
		id SERIAL PRIMARY KEY,
		session_id VARCHAR(50) UNIQUE NOT NULL,
		session_db_url VARCHAR(100) NOT NULL
	)
`)

	if err != nil {
		log.Printf("Failed to create table in session database: %s", sessionDBName)
		log.Println(err) // Добавим эту строку для вывода ошибки в лог
		return nil, err
	}

	log.Printf("Successfully created and connected to session database: %s", sessionDBName)

	return sessionDB, nil
}

// GetSessionInfo возвращает информацию о сессии по ее идентификатору
func (h *PostgreSQLHandler) GetSessionInfo(sessionID string) (*SessionInfo, error) {
	// Создаем подключение к сессионной базе данных
	sessionDB, err := h.CreateSessionDatabase(sessionID)
	if err != nil {
		return nil, err
	}
	defer sessionDB.Close()

	var sessionInfo SessionInfo
	err = sessionDB.QueryRow("SELECT session_id, session_db_url FROM sessions WHERE session_id = $1", sessionID).Scan(&sessionInfo.SessionID, &sessionInfo.SessionDBURL)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &sessionInfo, nil
}

// InitDB инициализирует соединение с основной базой данных PostgreSQL
func InitDB() (*sql.DB, error) {
	// Загрузка переменных окружения
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// Формирование строки подключения
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)

	// Открываем соединение с БД
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to open a DB connection:", err)
	}

	// Проверяем соединение
	err = db.Ping()
	if err != nil {
		log.Fatal("Failed to ping the DB:", err)
	}

	return db, nil
}
