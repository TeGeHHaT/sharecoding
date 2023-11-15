package server

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/TeGeHHaT/sharecoding/pkg/database"
	"github.com/TeGeHHaT/sharecoding/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// LiveCodingSession хранит информацию о сессии для лайв-кодинга
type LiveCodingSession struct {
	sync.Mutex
	Code    string
	Clients map[*websocket.Conn]struct{}
}

// liveCodingSessions содержит активные сессии для лайв-кодинга
var liveCodingSessions sync.Map

// SetupRoutes устанавливает маршруты для сервера
func SetupRoutes(router *gin.Engine, dbHandler database.DBHandler) {
	// Маршрут для получения sessionID и редиректа
	router.GET("/", createSession(dbHandler))

	// Маршрут для присоединения к существующей сессии
	router.GET("/session/:id", joinSession(dbHandler))

	// Маршрут для обработки WebSocket соединения и лайв-кодинга
	router.GET("/live/:id", liveCoding(dbHandler))

	// Catch-all маршрут для возвращения index.html
	router.NoRoute(func(c *gin.Context) {
		filePath := filepath.Join("www", "index.html")
		log.Println("Serving file:", filePath)
		c.File(filePath)
	})
}

func createSession(dbHandler database.DBHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := utils.GenerateRandomString(10)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate session ID"})
			return
		}

		sessionDB, err := dbHandler.CreateSessionDatabase(sessionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session database"})
			return
		}
		defer sessionDB.Close()

		err = saveSessionInfo(sessionID, sessionDB)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session information"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"sessionID": sessionID})
	}
}

// joinSession подключается к существующей сессии
func joinSession(dbHandler database.DBHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("id")

		sessionInfo, err := dbHandler.GetSessionInfo(sessionID)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get session information"})
			return
		}

		sessionDB, err := sql.Open("postgres", sessionInfo.SessionDBURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to session database"})
			return
		}
		defer sessionDB.Close()

		c.JSON(http.StatusOK, gin.H{"message": "Joined session", "sessionInfo": sessionInfo})
	}
}

// liveCoding обрабатывает WebSocket-соединение для лайв-кодинга
func liveCoding(dbHandler database.DBHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("id")

		sessionInfo, err := dbHandler.GetSessionInfo(sessionID)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось получить информацию о сессии"})
			return
		}

		sessionDB, err := sql.Open("postgres", sessionInfo.SessionDBURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось подключиться к базе данных сессии"})
			return
		}
		defer sessionDB.Close()

		// Преобразование соединения в WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось установить соединение по WebSocket"})
			return
		}
		defer conn.Close()

		// Загрузка или создание сессии лайв-кодинга
		session, _ := liveCodingSessions.LoadOrStore(sessionID, &LiveCodingSession{Clients: make(map[*websocket.Conn]struct{})})
		liveSession := session.(*LiveCodingSession)
		liveSession.Lock()
		liveSession.Clients[conn] = struct{}{}
		liveSession.Unlock()

		// Отправка текущего кода подключенному клиенту
		err = conn.WriteJSON(gin.H{"code": liveSession.Code})
		if err != nil {
			return
		}

		// Обработка сообщений от клиента
		for {
			log.Println("test")
			message := map[string]string{}
			err := conn.ReadJSON(&message)
			if err != nil {
				break
			}

			// Обновление кода и распространение изменений другим подключенным клиентам
			liveSession.Lock()
			liveSession.Code = message["code"]

			// Сохранение кода в базе данных
			err = saveCodeToDatabase(sessionID, liveSession.Code, sessionDB)
			if err != nil {
				fmt.Println("Не удалось сохранить код в базе данных:", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить код в базе данных"})
				liveSession.Unlock()
				return
			}

			for client := range liveSession.Clients {
				err := client.WriteJSON(gin.H{"code": liveSession.Code})
				if err != nil {
					client.Close()
					delete(liveSession.Clients, client)
				}
			}
			liveSession.Unlock()
		}

		// Клиент отключился, удаляем его из списка клиентов
		liveSession.Lock()
		delete(liveSession.Clients, conn)
		liveSession.Unlock()
	}
}

func saveCodeToDatabase(sessionID, code string, db *sql.DB) error {
	// Пытаемся обновить код для текущей сессии
	result, err := db.Exec("UPDATE shared_code SET code = $1 WHERE session_id = $2", code, sessionID)

	if err != nil {
		return err
	}

	// Если обновления не произошло (то есть, записи с таким session_id нет), выполняем вставку
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		_, err = db.Exec("INSERT INTO shared_code (session_id, code) VALUES ($1, $2)", sessionID, code)
		if err != nil {
			return err
		}
	}

	return nil
}

func saveSessionInfo(sessionID string, sessionDB *sql.DB) error {
	_, err := sessionDB.Exec("INSERT INTO sessions (session_id, session_db_url) VALUES ($1, $2)", sessionID, sessionID)
	if err != nil {
		log.Println(err)
	}
	return err
}
