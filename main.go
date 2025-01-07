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




type Transaction struct {
	TxID          string `json:"hash"`
	Confirmations int    `json:"confirmations"`
	Time          int    `json:"time"`
}




type WalletResponse struct {
	Address       string        `json:"address"`
	TotalReceived int64        `json:"total_received"`
	TotalSent     int64        `json:"total_sent"`
	FinalBalance  int64        `json:"final_balance"`
	TxCount       int          `json:"n_tx"`
	Transactions  []Transaction `json:"txs"`
}




type Amount float64




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




func analyzeTransactionPatterns(transactions []Transaction, address string) {
	//Get the day the transaction happened
	dailyTxCount := make(map[string]int)
	for _, tx := range transactions {
		date := time.Unix(int64(tx.Time), 0).Format("2006-01-02")
		dailyTxCount[date]++
	}

	// Find busiest day
	var maxCount int
	var busiestDay string
	for date, count := range dailyTxCount {
		if count > maxCount {
			maxCount = count
			busiestDay = date
		}
	}

	fmt.Printf("\nTransaction Pattern Analysis:\n")
	fmt.Printf("Busiest day: %s with %d transactions\n", busiestDay, maxCount)
	fmt.Printf("Average daily transactions: %.2f\n", float64(len(transactions))/float64(len(dailyTxCount)))
}


//Save to c

func main() {
	address := flag.String("wallet", "", "Bitcoin wallet address to monitor")
	//interval := flag.Int("interval", 300, "Monitoring interval in seconds")
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

	fmt.Printf("Address: %s\n", walletResponse.Address)
	fmt.Printf("Total Bitcoin Received: %.8f BTC\n", bitcoinReceived)
	fmt.Printf("Total Bitcoin Sent: %.8f BTC\n", bitcoinSent)
	fmt.Printf("Final Bitcoin Balance: %.8f BTC\n", balance)
	fmt.Printf("Number of Transactions: %d\n", walletResponse.TxCount)

	fmt.Println("Transactions:")
	for _, tx := range walletResponse.Transactions {
		amount, err := getTransactionAmount(*address, tx.TxID)
		if err != nil {
			log.Printf("Error fetching transaction %s: %v", tx.TxID, err)
			continue
		}

        t := time.Unix(int64(tx.Time), 0)

		formattedTime := t.Format("2006-01-02 15:04:05")


		fmt.Printf("TxID: %s, Amount: %.8f BTC, Confirmations: %d, Time: %s\n",
			tx.TxID, float64(*amount)/100_000_000, tx.Confirmations, formattedTime)
	}

	analyzeTransactionPatterns(walletResponse.Transactions,*address)

	
	
}