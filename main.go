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

	http.HandleFunc("/api/v0/prices", pricesHandler)

	fmt.Println("Server is starting on port 8080...")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Printf("Server error: %v", err)
	}
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

func handlePost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

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

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(fileBytes), int64(len(fileBytes)))
	if err != nil {
		http.Error(w, "Failed to read zip archive", http.StatusBadRequest)
		return
	}

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

	csvReader := csv.NewReader(rc)
	records, err := csvReader.ReadAll()
	if err != nil {
		http.Error(w, "Failed to read CSV", http.StatusInternalServerError)
		return
	}

	startIndex := 0
	if len(records) > 0 && records[0][0] == "id" {
		startIndex = 1
	}

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	for i := startIndex; i < len(records); i++ {
		record := records[i]
		if len(record) != 5 {
			http.Error(w, "Invalid CSV format", http.StatusBadRequest)
			return
		}

		name := record[1]
		category := record[2]
		priceStr := record[3]
		createDate := record[4]

		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			http.Error(w, "Invalid price value", http.StatusBadRequest)
			return
		}

		_, err = tx.Exec(
			"INSERT INTO prices (name, category, price, create_date) VALUES ($1, $2, $3, $4)",
			name, category, price, createDate,
		)
		if err != nil {
			http.Error(w, "Failed to insert record", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	var totalItems int
	var totalCategories int
	var totalPrice float64

	totalItems = len(records) - startIndex

	err = db.QueryRow(`
    SELECT
        COUNT(DISTINCT category),
        COALESCE(SUM(price), 0)
    FROM prices
`).Scan(&totalCategories, &totalPrice)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = fmt.Fprintf(w, `{"total_items":%d,"total_categories":%d,"total_price":%.0f}`,
		totalItems, totalCategories, totalPrice)
	if err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET request from %s", r.RemoteAddr)

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
	if err := rows.Err(); err != nil {
		http.Error(w, "Row iteration error", http.StatusInternalServerError)
		return
	}

	csvWriter.Flush()

	if err := csvWriter.Error(); err != nil {
		http.Error(w, "Failed to create CSV", http.StatusInternalServerError)
		return
	}

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

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=data.zip")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(zipBuffer.Bytes()); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}
