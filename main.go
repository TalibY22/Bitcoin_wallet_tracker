package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
	"math"
    "io"
    "database/sql"
	"sort"
    _ "github.com/mattn/go-sqlite3"
	
)

const (
	Red     = "\033[31m"
	Green   = "\033[32m"
	Reset   = "\033[0m"
	Blue    = "\033[34m"
	Yellow  = "\033[33m"
	Cyan    = "\033[36m"
	Headers = "\033[1;36m" 
)

type Transaction struct {
	TxID          string `json:"hash"`
	Confirmations int    `json:"confirmations"`
	Time          int    `json:"time"`
}

type WalletResponse struct {
	Address       string        `json:"address"`
	TotalReceived int64         `json:"total_received"`
	TotalSent     int64         `json:"total_sent"`
	FinalBalance  int64         `json:"final_balance"`
	TxCount       int           `json:"n_tx"`
	Transactions  []Transaction `json:"txs"`
}

type HistoricalPrice struct {
	Time         int64   
	Usd          float64 
	CurrentPrice float64
}

type Amount float64

type Price struct {
	Usd float64
}


type TransactionAnalysis struct {
    TxID          string
    Amount        float64
    Price         float64
    Time          time.Time
    Type          string    
    ProfitImpact  float64   
}


type TransactionDetails struct {
	Amount        float64
	Price         float64
	Time          time.Time
	Confirmations int
}

var client = &http.Client{
	Timeout: 10 * time.Second,
}

func fetchWallet(address string) (*WalletResponse, error) {
	url := fmt.Sprintf("https://blockchain.info/rawaddr/%s/", address)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %v", err)
	}
	defer resp.Body.Close()

	var result WalletResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return &result, nil
}

var (
	priceCache = make(map[int64]*HistoricalPrice)
	priceMutex sync.RWMutex
)





func GetPrice(timestamp int64) (*HistoricalPrice , error) {

 
	//Convert to utcccc why?? Api 
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

	// Parse the response
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

	// Create HistoricalPrice object
	price := &HistoricalPrice{
		Time: timestamp,
		Usd:  response.Data.Data[0].USD,
	}

	return price, nil
}








func getMidnightTimestamp(timestamp int64) string {
    t := time.Unix(timestamp, 0).UTC()
    midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
    return midnight.Format("2006-01-02 15:04:05")
}



func GetPrice2(db *sql.DB, timestamp int64) (*HistoricalPrice, error) {
	// Convert timestamp to a UTC time string in the same format as the database
	midnight := getMidnightTimestamp(timestamp)

    

	// Query the database
	query := `
        SELECT price 
FROM crypto_data 
WHERE snapped_at <= ? 
ORDER BY snapped_at DESC 
LIMIT 1;

    `
	var priceusd float64
	err := db.QueryRow(query, midnight).Scan(&priceusd)
	if err != nil {
		if err == sql.ErrNoRows {
			return &HistoricalPrice{}, fmt.Errorf("no price found for the given timestamp")
		}
		return &HistoricalPrice{}, err
	}

	value := &HistoricalPrice{
		Time: timestamp,
		Usd:  priceusd,
	}
   

	return value, nil
}



func getTransactionAmount(address, txid string) (*Amount, error) {
	url := fmt.Sprintf("https://blockchain.info/q/txresult/%s/%s", txid, address)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var amount Amount
	if err := json.NewDecoder(resp.Body).Decode(&amount); err != nil {
		return nil, err
	}

	return &amount, nil
}






func printTableHeader() {
	fmt.Printf("\n%s╔════════════════════════════════════════════════════════════════════════════════════════════════════════╗%s\n", Headers, Reset)
	fmt.Printf("%s║ %-20s │ %-15s │ %-12s │ %-15s │ %-20s ║%s\n",
		Headers, "Transaction ID", "Amount (BTC)", "Confirmations", "USD Value", "Time", Reset)
	fmt.Printf("%s╠════════════════════════════════════════════════════════════════════════════════════════════════════════╣%s\n", Headers, Reset)
}

func printWalletSummary(wallet *WalletResponse, currentPrice float64) {
	balance := float64(wallet.FinalBalance) / 100_000_000
	bitcoinSent := float64(wallet.TotalSent) / 100_000_000
	bitcoinReceived := float64(wallet.TotalReceived) / 100_000_000

	fmt.Printf("\n%s╔════════════════ Wallet Summary ════════════════╗%s\n", Headers, Reset)
	fmt.Printf("%s║ Address: %-37s ║%s\n", Headers, wallet.Address, Reset)
	fmt.Printf("%s║ Total Received: %-8.8f BTC              ║%s\n", Headers, bitcoinReceived, Reset)
	fmt.Printf("%s║ Total Sent: %-8.8f BTC                  ║%s\n", Headers, bitcoinSent, Reset)
	fmt.Printf("%s║ Current Balance: %-8.8f BTC             ║%s\n", Headers, balance, Reset)
	fmt.Printf("%s║ Current Value: $%-10.2f USD             ║%s\n", Headers, balance*currentPrice, Reset)
	fmt.Printf("%s║ Total Transactions: %-6d                ║%s\n", Headers, wallet.TxCount, Reset)
	fmt.Printf("%s╚══════════════════════════════════════════════╝%s\n", Headers, Reset)
}



func analyzeTransactionPatterns(transactions []Transaction, address string, txDetails map[string]TransactionDetails) {
    // Initialize analysis maps
    incomingTx := make(map[string]float64)
    outgoingTx := make(map[string]float64)
    sizeCategories := make(map[string]int)
    repeatedAddresses := make(map[string]int)
    profitByMonth := make(map[string]float64)
    
    // Transaction size clusters
    var txSizes []float64
    
    // Initialize previous maps and variables from before
   // dailyTxCount := make(map[string]int)
    monthlyTxCount := make(map[string]int)
    //hourlyDistribution := make(map[int]int)
    
    var totalProfit float64
    var highestProfit float64
    var biggestLoss float64
    var profitableMonths int
    var unprofitableMonths int

    // Process transactions
    for _, tx := range transactions {
        details := txDetails[tx.TxID]
        txTime := time.Unix(int64(tx.Time), 0)
        amount := details.Amount
        monthKey := txTime.Format("2006-01")
        
        // Incoming vs Outgoing Analysis
        if amount > 0 {
            incomingTx[monthKey] += amount
        } else {
            outgoingTx[monthKey] += -amount
        }
        
        // Size Clustering
        txSizes = append(txSizes, math.Abs(amount))
        
        // Categorize transaction sizes
        switch {
        case math.Abs(amount) < 0.001:
            sizeCategories["Micro (<0.001 BTC)"]++
        case math.Abs(amount) < 0.01:
            sizeCategories["Small (0.001-0.01 BTC)"]++
        case math.Abs(amount) < 0.1:
            sizeCategories["Medium (0.01-0.1 BTC)"]++
        case math.Abs(amount) < 1:
            sizeCategories["Large (0.1-1 BTC)"]++
        default:
            sizeCategories["Whale (>1 BTC)"]++
        }
        
        // Calculate profit/loss for this transaction
        profitImpact := amount * details.Price
        totalProfit += profitImpact
        profitByMonth[monthKey] += profitImpact
        
        if profitImpact > highestProfit {
            highestProfit = profitImpact
        }
        if profitImpact < biggestLoss {
            biggestLoss = profitImpact
        }
    }

    // Print Analysis Results
    fmt.Printf("\n%s=== Advanced Transaction Analysis ===%s\n\n", Headers, Reset)

    // 1. Transaction Size Clustering
    fmt.Printf("%sTransaction Size Distribution:%s\n", Headers, Reset)
    for category, count := range sizeCategories {
        percentage := float64(count) / float64(len(transactions)) * 100
        fmt.Printf("├─ %s: %d (%.1f%%)\n", category, count, percentage)
    }
    fmt.Printf("\n")

    // 2. Incoming vs Outgoing Analysis
    fmt.Printf("%sIncoming vs Outgoing Patterns:%s\n", Headers, Reset)
    for month := range monthlyTxCount {
        incoming := incomingTx[month]
        outgoing := outgoingTx[month]
        if incoming > 0 || outgoing > 0 {
            fmt.Printf("├─ %s:\n", month)
            fmt.Printf("│  ├─ Incoming: %.8f BTC\n", incoming)
            fmt.Printf("│  └─ Outgoing: %.8f BTC\n", outgoing)
        }
    }
    fmt.Printf("\n")

    // 3. Profit Analysis
    fmt.Printf("%sProfit Analysis:%s\n", Headers, Reset)
    fmt.Printf("├─ Total Profit/Loss: $%.2f USD\n", totalProfit)
    fmt.Printf("├─ Highest Single Profit: $%.2f USD\n", highestProfit)
    fmt.Printf("├─ Biggest Single Loss: $%.2f USD\n", biggestLoss)
    fmt.Printf("├─ Monthly Profit Breakdown:\n")
    
    // Sort months for consistent display
    var months []string
    for month := range profitByMonth {
        months = append(months, month)
    }
    sort.Strings(months)
    
    for _, month := range months {
        profit := profitByMonth[month]
        if profit > 0 {
            fmt.Printf("│  ├─ %s: %s$%.2f USD%s\n", month, Green, profit, Reset)
            profitableMonths++
        } else {
            fmt.Printf("│  ├─ %s: %s$%.2f USD%s\n", month, Red, profit, Reset)
            unprofitableMonths++
        }
    }
    
    fmt.Printf("├─ Profitable Months: %d\n", profitableMonths)
    fmt.Printf("└─ Unprofitable Months: %d\n\n", unprofitableMonths)

    // 4. Network Effect Analysis
    fmt.Printf("%sNetwork Analysis:%s\n", Headers, Reset)
    fmt.Printf("├─ Total Unique Addresses Interacted With: %d\n", len(repeatedAddresses))
    fmt.Printf("└─ Top Recurring Interactions:\n")
    
    // Sort addresses by interaction count
    type addressCount struct {
        address string
        count   int
    }
    var sortedAddresses []addressCount
    for addr, count := range repeatedAddresses {
        if count > 1 { // Only show addresses with multiple interactions
            sortedAddresses = append(sortedAddresses, addressCount{addr, count})
        }
    }
    sort.Slice(sortedAddresses, func(i, j int) bool {
        return sortedAddresses[i].count > sortedAddresses[j].count
    })
    
    // Show top 5 recurring addresses
    for i, ac := range sortedAddresses {
        if i >= 5 {
            break
        }
        fmt.Printf("    ├─ Address: %s (%d interactions)\n", ac.address[:8], ac.count)
    }
}



func main() {
	address := flag.String("wallet", "", "Bitcoin wallet address to monitor")
	flag.Parse()

	if *address == "" {
		log.Fatal("Please provide a wallet address using the -wallet flag")
	}
    


    db, err := sql.Open("sqlite3", "btcprice.db")
	if err != nil {
		fmt.Println("Error opening database:", err)
		return
	}
	defer db.Close()

	

    priceToday, err := GetPrice(int64(time.Now().Unix()))
    
    if err != nil {
        fmt.Println("Api Limit Exhausted local database will be used:", err)

		priceToday,err = GetPrice2(db,int64(time.Now().Unix()))

		if err != nil{
 
			fmt.Println("error occured:",err)
			return
		}
    }

	wallet, err := fetchWallet(*address)
	if err != nil {
		log.Fatalf("Error fetching wallet: %v", err)
	}

	printWalletSummary(wallet,priceToday.Usd)

	//var wg sync.WaitGroup
	txDetails := make(map[string]TransactionDetails)
	//var txMutex sync.Mutex

	printTableHeader()

	for _, tx := range wallet.Transactions {
		// Fetch the transaction amount
		amount, err := getTransactionAmount(*address, tx.TxID)
		if err != nil {
			log.Printf("Error fetching transaction %s: %v", tx.TxID, err)
			continue
		}
	
		// Fetch the price
		price, err := GetPrice(int64(tx.Time))
		if err != nil {
			log.Printf("Error fetching price for transaction %s: %v", tx.TxID, err)
			price, err = GetPrice2(db, int64(tx.Time))
			if err != nil {
				fmt.Printf("Error occurred while fetching fallback price for transaction %s: %v\n", tx.TxID, err)
				continue
			}
		}
	
		// Convert amount to BTC and store transaction details
		btcAmount := float64(*amount) / 100_000_000
		txDetails[tx.TxID] = TransactionDetails{
			Amount:        btcAmount,
			Price:         price.Usd,
			Time:          time.Unix(int64(tx.Time), 0),
			Confirmations: tx.Confirmations,
		}
	}
	
	// Process and display the transaction details
	for _, tx := range wallet.Transactions {
		details, ok := txDetails[tx.TxID]
		if !ok {
			continue
		}
	
		color := Green
		if details.Amount < 0 {
			color = Red
		}
	
		fmt.Printf("║ %-20s │ %s%-15.8f%s │ %-12d │ $%-14.2f │ %-20s ║\n",
			tx.TxID[:20],
			color, details.Amount, Reset,
			details.Confirmations,
			details.Amount*details.Price,
			details.Time.Format("2006-01-02 15:04:05"))
	}
	
	fmt.Printf("%s╚════════════════════════════════════════════════════════════════════════════════════════════════════════╝%s\n", Headers, Reset)




	analyzeTransactionPatterns(wallet.Transactions,*address,txDetails)
}
