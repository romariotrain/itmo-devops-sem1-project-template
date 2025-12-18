package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

var db *sql.DB

const (
	host     = "localhost"
	port     = 5432
	user     = "validator"
	password = "val1dat0r"
	dbname   = "project-sem-1"
)

func main() {
	// Подключение к базе данных
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	var err error
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Failed to close database connection: %v", closeErr)
		}
	}()

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Successfully connected to database!")

	// Регистрация обработчиков
	http.HandleFunc("/api/v0/prices", pricesHandler)

	// Запуск сервера
	fmt.Println("Server is starting on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func pricesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		handlePost(w, r)
	case http.MethodGet:
		handleGet(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// POST /api/v0/prices - загрузка zip-архива с данными
func handlePost(w http.ResponseWriter, r *http.Request) {
	// Парсинг multipart form data
	err := r.ParseMultipartForm(32 << 20) // 32 MB максимум
	if err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	// Получение файла из формы
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Printf("Failed to close file: %v", closeErr)
		}
	}()

	// Чтение файла в память
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}

	// Открытие zip-архива из памяти
	zipReader, err := zip.NewReader(bytes.NewReader(fileBytes), int64(len(fileBytes)))
	if err != nil {
		http.Error(w, "Failed to read zip archive", http.StatusBadRequest)
		return
	}

	// Поиск CSV файла в архиве
	var csvFile *zip.File
	for _, f := range zipReader.File {
		if strings.HasSuffix(f.Name, ".csv") {
			csvFile = f
			break
		}
	}

	if csvFile == nil {
		http.Error(w, "CSV file not found in archive", http.StatusBadRequest)
		return
	}

	// Открытие CSV файла
	rc, err := csvFile.Open()
	if err != nil {
		http.Error(w, "Failed to open CSV file", http.StatusInternalServerError)
		return
	}
	defer func() {
		if closeErr := rc.Close(); closeErr != nil {
			log.Printf("Failed to close CSV reader: %v", closeErr)
		}
	}()

	// Чтение CSV
	csvReader := csv.NewReader(rc)
	records, err := csvReader.ReadAll()
	if err != nil {
		http.Error(w, "Failed to read CSV", http.StatusInternalServerError)
		return
	}

	// Пропускаем заголовок, если он есть
	startIndex := 0
	if len(records) > 0 && records[0][0] == "id" {
		startIndex = 1
	}

	// Запись данных в базу
	for i := startIndex; i < len(records); i++ {
		record := records[i]
		if len(record) != 5 {
			continue
		}

		id := record[0]
		name := record[1]
		category := record[2]
		priceStr := record[3]
		createDate := record[4]

		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			continue
		}

		_, err = db.Exec(
			"INSERT INTO prices (id, name, category, price, create_date) VALUES ($1, $2, $3, $4, $5)",
			id, name, category, price, createDate,
		)
		if err != nil {
			log.Printf("Failed to insert record: %v", err)
			continue
		}
	}

	// Подсчёт статистики
	var totalItems int
	var totalCategories int
	var totalPrice float64

	err = db.QueryRow("SELECT COUNT(*) FROM prices").Scan(&totalItems)
	if err != nil {
		http.Error(w, "Failed to count items", http.StatusInternalServerError)
		return
	}

	err = db.QueryRow("SELECT COUNT(DISTINCT category) FROM prices").Scan(&totalCategories)
	if err != nil {
		http.Error(w, "Failed to count categories", http.StatusInternalServerError)
		return
	}

	err = db.QueryRow("SELECT COALESCE(SUM(price), 0) FROM prices").Scan(&totalPrice)
	if err != nil {
		http.Error(w, "Failed to sum prices", http.StatusInternalServerError)
		return
	}

	// Формирование ответа
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = fmt.Fprintf(w, `{"total_items":%d,"total_categories":%d,"total_price":%.0f}`,
		totalItems, totalCategories, totalPrice)
	if err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// GET /api/v0/prices - выгрузка данных в zip-архиве
func handleGet(w http.ResponseWriter, r *http.Request) {
	// Логирование запроса
	log.Printf("GET request from %s", r.RemoteAddr)

	// Получение всех данных из базы
	rows, err := db.Query("SELECT id, name, category, price, create_date FROM prices ORDER BY id")
	if err != nil {
		http.Error(w, "Failed to query database", http.StatusInternalServerError)
		return
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("Failed to close rows: %v", closeErr)
		}
	}()

	// Создание CSV в памяти
	var csvBuffer bytes.Buffer
	csvWriter := csv.NewWriter(&csvBuffer)

	for rows.Next() {
		var id, name, category, createDate string
		var price float64

		err := rows.Scan(&id, &name, &category, &price, &createDate)
		if err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}

		record := []string{
			id,
			name,
			category,
			fmt.Sprintf("%.0f", price),
			createDate,
		}

		if err := csvWriter.Write(record); err != nil {
			log.Printf("Failed to write CSV record: %v", err)
		}
	}

	csvWriter.Flush()

	if err := csvWriter.Error(); err != nil {
		http.Error(w, "Failed to create CSV", http.StatusInternalServerError)
		return
	}

	// Создание zip-архива в памяти
	var zipBuffer bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuffer)

	csvFile, err := zipWriter.Create("data.csv")
	if err != nil {
		http.Error(w, "Failed to create zip file", http.StatusInternalServerError)
		return
	}

	_, err = csvFile.Write(csvBuffer.Bytes())
	if err != nil {
		http.Error(w, "Failed to write to zip", http.StatusInternalServerError)
		return
	}

	err = zipWriter.Close()
	if err != nil {
		http.Error(w, "Failed to close zip writer", http.StatusInternalServerError)
		return
	}

	// Отправка zip-архива клиенту
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=data.zip")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(zipBuffer.Bytes()); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}
