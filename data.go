package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Step 1: Open the SQLite database (it will create the file if it doesn't exist)
	db, err := sql.Open("sqlite3", "./btcprice.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Step 2: Create the table if it doesn't exist
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS crypto_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		snapped_at TEXT,
		price REAL,
		market_cap INTEGER,
		total_volume INTEGER
	)`)
	if err != nil {
		log.Fatal(err)
	}

	// Step 3: Open the CSV file
	file, err := os.Open("data.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Step 4: Read the CSV file
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	// Step 5: Loop over CSV data and insert into the SQLite table
	for _, record := range records[1:] { // Skip the header row
		// Parse snapped_at (string, no need for parsing)
		snappedAt := record[0]

		// Parse price as float64 (may contain decimal points)
		price, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			log.Fatal(err)
		}

		// Handle market_cap (empty values default to 0)
		var marketCap float64
		if record[2] != "" {
			marketCap, err = strconv.ParseFloat(record[2], 64)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			marketCap = 0 // Default to 0 if the value is empty
		}

		// Handle total_volume (empty values default to 0)
		var totalVolume float64
		if record[3] != "" {
			totalVolume, err = strconv.ParseFloat(record[3], 64)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			totalVolume = 0 // Default to 0 if the value is empty
		}

		// Convert marketCap and totalVolume to integers
		marketCapInt := int(marketCap)
		totalVolumeInt := int(totalVolume)

		// Step 6: Insert the row into the database
		_, err = db.Exec("INSERT INTO crypto_data (snapped_at, price, market_cap, total_volume) VALUES (?, ?, ?, ?)", snappedAt, price, marketCapInt, totalVolumeInt)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Step 7: Notify the user the operation is complete
	fmt.Println("CSV data has been inserted into the SQLite database.")
}
