package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
    
	
)


//Colors for the ouput
const (
	Red    = "\033[31m"
	Green  = "\033[32m"
	Reset  = "\033[0m"
)

//Structure to store the Trasactions
type Transaction struct {
	TxID          string `json:"hash"`
	Confirmations int    `json:"confirmations"`
	Time          int    `json:"time"`
}

//Structure to store wallet reponse 
type WalletResponse struct {
	Address       string        `json:"address"`
	TotalReceived int64         `json:"total_received"`
	TotalSent     int64         `json:"total_sent"`
	FinalBalance  int64         `json:"final_balance"`
	TxCount       int           `json:"n_tx"`
	Transactions  []Transaction `json:"txs"`
}

//Structure to show price for a particular date 
type HistoricalPrice struct {
	Time         int64   `json:"time"`
	Usd          float64 `json:"USD"`
	Ksh          float64 `json:"KSH"`
	CurrentPrice float64
}

//
type Amount float64



// Fetch wallet
func fetchTransactions(address string) (*WalletResponse, error) {
	url := fmt.Sprintf("https://blockchain.info/rawaddr/%s", address)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var result WalletResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return &result, nil
}






//Fetch btc amount
func getTransactionAmount(address, txid string) (*Amount, error) {
	url := fmt.Sprintf("https://blockchain.info/q/txresult/%s/%s", txid, address)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transaction: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read transaction: %v", err)
	}

	var amount Amount
	err = json.Unmarshal(body, &amount)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transaction: %v", err)
	}

	return &amount, nil
}





//Monitoring wallet 
func monitorWallet(address string, interval int) {
	var previousBalance float64
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		walletResponse, err := fetchTransactions(address)
		if err != nil {
			log.Printf("Error monitoring wallet: %v", err)
			continue
		}

		currentBalance := float64(walletResponse.FinalBalance) / 100_000_000
		if previousBalance != 0 && currentBalance != previousBalance {
			fmt.Printf("[ALERT] Wallet balance changed! Previous: %.8f BTC, Current: %.8f BTC\n",
				previousBalance, currentBalance)
		}
		previousBalance = currentBalance
	}
}



// Fetch bitcoin price
func GetPrice(timestamp int64) (*HistoricalPrice, error) {
	date := time.Unix(timestamp, 0).UTC().Format("02-01-2006")
	url := fmt.Sprintf("https://min-api.cryptocompare.com/data/v2/histoday?fsym=BTC&tsym=USD&limit=1&toTs=%d", timestamp)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch historical price data: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data struct {
			Data []struct {
				Time int64   `json:"time"`
				USD  float64 `json:"close"`
			} `json:"Data"`
		} `json:"Data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if len(response.Data.Data) == 0 {
		return nil, fmt.Errorf("no historical data available for date %s", date)
	}

	price := &HistoricalPrice{
		Time: timestamp,
		Usd:  response.Data.Data[0].USD,
	}

	return price, nil
}








// Transaction Report
func analyzeTransactionPatterns(transactions []Transaction, address string) {
	dailyTxCount := make(map[string]int)
	for _, tx := range transactions {
		date := time.Unix(int64(tx.Time), 0).Format("2006-01-02")
		dailyTxCount[date]++
	}

	var maxCount int
	var busiestDay string
	for date, count := range dailyTxCount {
		if count > maxCount {
			maxCount = count
			busiestDay = date
		}
	}

	fmt.Printf("\n--- Transaction Pattern Analysis ---\n")
	fmt.Printf("Busiest day: %s with %d transactions\n", busiestDay, maxCount)
	fmt.Printf("Average daily transactions: %.2f\n", float64(len(transactions))/float64(len(dailyTxCount)))
}





//Calculate profit 






func main() {
	address := flag.String("wallet", "", "Bitcoin wallet address to monitor")
	flag.Parse()

	if *address == "" {
		log.Fatal("Please provide a wallet address using the -wallet flag")
	}

	walletResponse, err := fetchTransactions(*address)
	if err != nil {
		log.Fatalf("Error fetching transactions: %v", err)
	}

	balance := float64(walletResponse.FinalBalance) / 100_000_000
	bitcoinSent := float64(walletResponse.TotalSent) / 100_000_000
	bitcoinReceived := float64(walletResponse.TotalReceived) / 100_000_000
	priceToday, err := GetPrice(int64(time.Now().Unix()))


	// Display wallet summary
	fmt.Printf("\n--- Wallet Summary ---\n")
	fmt.Printf("Address: %s\n", walletResponse.Address)
	fmt.Printf("Total Bitcoin Received: %.8f BTC\n", bitcoinReceived)
	fmt.Printf("Total Bitcoin Sent: %.8f BTC\n", bitcoinSent)
	fmt.Printf("Final Bitcoin Balance: %.8f BTC\n", balance)
	fmt.Printf("Value of Bitcoin Balance:  %.2f USD\n", balance * priceToday.Usd)

	fmt.Printf("Number of Transactions: %d\n", walletResponse.TxCount)


	// Display transaction details
	fmt.Println("\n--- Transactions ---")
	for _, tx := range walletResponse.Transactions {
		amount, err := getTransactionAmount(*address, tx.TxID)
		if err != nil {
			log.Printf("Error fetching transaction %s: %v", tx.TxID, err)
			continue
		}

		t := time.Unix(int64(tx.Time), 0)
		priceTest, err := GetPrice(int64(tx.Time))


		if err != nil {
			fmt.Println("Could not fetch price")
		}

		formattedTime := t.Format("2006-01-02 15:04:05")
		btcAmount := float64(*amount) / 100_000_000

	    bitcoin_value := btcAmount * priceTest.Usd
		var color string
		if btcAmount < 0 {
			color = Red
		} else {
			color = Green
		}

	
		
		
		fmt.Printf("TxID: %s, Amount: %s%.8f BTC%s, Confirmations: %d, Time: %s, Value Of BTC Sent: %.2f USD\n",
			tx.TxID, color, btcAmount, Reset, tx.Confirmations, formattedTime, bitcoin_value)
	}

	analyzeTransactionPatterns(walletResponse.Transactions, *address)
	
}
