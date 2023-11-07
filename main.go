package main

import (
	"log"
	"os"

	"github.com/TeGeHHaT/sharecoding/pkg/database"
	"github.com/TeGeHHaT/sharecoding/pkg/server"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Загрузка переменных окружения из файла .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Определяем порт, на котором будет работать сервер
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Инициализация основной базы данных
	mainDB, err := database.InitDB()
	if err != nil {
		log.Println("Failed to initialize the main database: %", err)
		log.Fatal()
	}
	defer mainDB.Close()

	// Инициализация обработчика базы данных
	dbHandler := database.NewPostgreSQLHandler(mainDB)

	r := gin.Default()

	// Подключаем маршруты из пакета server
	server.SetupRoutes(r, dbHandler)

	// Запуск сервера
	err = r.Run(":" + port)
	if err != nil {
		log.Fatal("Failed to start the server:", err)
	}
}
