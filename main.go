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
	"strings"
    "database/sql"
	//"sort"
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


type Input struct {
    PrevOut struct {
        Addr  string `json:"addr"`
        Value int64  `json:"value"`
    } `json:"prev_out"`
}

type Output struct {
    Addr   string `json:"addr"`
    Value  int64  `json:"value"`
    Spent  bool   `json:"spent"`
}
type Transaction struct {
	TxID          string `json:"hash"`
	Confirmations int    `json:"confirmations"`
	Time          int    `json:"time"`
	Inputs        []Input `json:"inputs"` 
    Out           []Output `json:"out"`

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

type wallet float64




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
	DisplayOrigin    string
    DisplayDest      string
	
}

var client = &http.Client{
	Timeout: 10 * time.Second,
}







//Mainn Fetch wallet with transaction details 
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




//Retrievd price
func GetPrice(timestamp int64) (*HistoricalPrice , error) {

 
	 
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







//Need to remove this 
func getMidnightTimestamp(timestamp int64) string {
    t := time.Unix(timestamp, 0).UTC()
    midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
    return midnight.Format("2006-01-02 15:04:05")
}


//Fallback for price if api limit exhausted 
func GetPrice2(db *sql.DB, timestamp int64) (*HistoricalPrice, error) {
	// Convert timestamp to a UTC time string in the same format as the database
	midnight := getMidnightTimestamp(timestamp)

    

	
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
    
    fmt.Printf("\n%s╔══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╗%s\n", Headers, Reset)
    
   
    fmt.Printf("%s║ %-20s │ %-15s │ %-12s │ %-15s │ %-20s │ %-35s │ %-35s ║%s\n",
        Headers,
        "Transaction ID",
        "Amount (BTC)",
        "Confirmations",
        "USD Value",
        "Time",
        "Origin",
        "Destination",
        Reset)
    
   
    fmt.Printf("%s╠═════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╣%s\n", Headers, Reset)
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
    
    type AddressStats struct {
        totalTransactions int
        totalVolume      float64
        lastSeen         time.Time
        transactionTypes map[string]int    
        amounts          []float64         
        timeDiffs        []float64         
    }

    addressStats := make(map[string]AddressStats)
    unusualPatterns := make(map[string][]string)
    

    const (
        RAPID_TX_WINDOW    = 1 * time.Hour
        DORMANCY_PERIOD    = 30 * 24 * time.Hour
        HIGH_VALUE_THRESHOLD = 1.0 // in BTC
        UNUSUAL_VARIANCE_THRESHOLD = 2.0
    )

    
    rapidTransactions := make(map[time.Time][]string)
    var lastTxTime time.Time
    var previousAmounts []float64

    fmt.Printf("\n%s=== Security Analysis Report ===%s\n\n", Headers, Reset)

    
    for _, tx := range transactions {
        details := txDetails[tx.TxID]
        txTime := time.Unix(int64(tx.Time), 0)
        
       
        for _, input := range tx.Inputs {
            addr := input.PrevOut.Addr
            if addr == "" {
                continue
            }
            
            stats := addressStats[addr]
            stats.totalTransactions++
            stats.totalVolume += math.Abs(details.Amount)
            stats.lastSeen = txTime
            if stats.transactionTypes == nil {
                stats.transactionTypes = make(map[string]int)
            }
            stats.transactionTypes["out"]++
            stats.amounts = append(stats.amounts, math.Abs(details.Amount))
            
            if len(stats.amounts) > 1 {
                timeDiff := txTime.Sub(lastTxTime).Hours()
                stats.timeDiffs = append(stats.timeDiffs, timeDiff)
            }
            
            addressStats[addr] = stats
        }

        for _, output := range tx.Out {
            addr := output.Addr
            if addr == "" {
                continue
            }
            
            stats := addressStats[addr]
            stats.totalTransactions++
            stats.totalVolume += math.Abs(details.Amount)
            stats.lastSeen = txTime
            if stats.transactionTypes == nil {
                stats.transactionTypes = make(map[string]int)
            }
            stats.transactionTypes["in"]++
            stats.amounts = append(stats.amounts, math.Abs(details.Amount))
            
            addressStats[addr] = stats
        }

        
        if !lastTxTime.IsZero() {
            timeDiff := txTime.Sub(lastTxTime)
            if timeDiff < RAPID_TX_WINDOW {
                rapidTransactions[txTime] = append(rapidTransactions[txTime], tx.TxID)
            }
        }
        
        lastTxTime = txTime
        previousAmounts = append(previousAmounts, math.Abs(details.Amount))
    }

   
    fmt.Printf("%s1. Suspicious Pattern Detection%s\n", Headers, Reset)

    // 1. Detect addresses with unusual transaction patterns
    for addr, stats := range addressStats {
        if addr == address {
            continue // Skip entered wallet address
        }

        // Calculate transaction amount variance
        var mean, variance float64
        for _, amount := range stats.amounts {
            mean += amount
        }
        mean /= float64(len(stats.amounts))
        
        for _, amount := range stats.amounts {
            variance += math.Pow(amount-mean, 2)
        }
        variance /= float64(len(stats.amounts))
        stdDev := math.Sqrt(variance)

        // Suspicious
        if stdDev > UNUSUAL_VARIANCE_THRESHOLD && stats.totalTransactions > 3 {
            unusualPatterns[addr] = append(unusualPatterns[addr], 
                fmt.Sprintf("High variance in transaction amounts (stdDev: %.2f BTC)", stdDev))
        }

        if stats.totalVolume > HIGH_VALUE_THRESHOLD && stats.totalTransactions < 3 {
            unusualPatterns[addr] = append(unusualPatterns[addr], 
                "High volume with few transactions")
        }

        // Detect unusual timing patterns
        var avgTimeDiff float64
        if len(stats.timeDiffs) > 0 {
            for _, diff := range stats.timeDiffs {
                avgTimeDiff += diff
            }
            avgTimeDiff /= float64(len(stats.timeDiffs))
            
            if avgTimeDiff < 1.0 && len(stats.timeDiffs) > 3 {
                unusualPatterns[addr] = append(unusualPatterns[addr], 
                    "Unusually frequent transactions")
            }
        }
    }

    
    if len(unusualPatterns) > 0 {
        fmt.Printf("\n%sDetected Suspicious Patterns:%s\n", Yellow, Reset)
        for addr, patterns := range unusualPatterns {
            fmt.Printf("Address: %s\n", addr)
            for _, pattern := range patterns {
                fmt.Printf("  - %s\n", pattern)
            }
            
            // Print additional stats for suspicious addresses
            stats := addressStats[addr]
            fmt.Printf("  Statistics:\n")
            fmt.Printf("    - Total Transactions: %d\n", stats.totalTransactions)
            fmt.Printf("    - Total Volume: %.8f BTC\n", stats.totalVolume)
            fmt.Printf("    - Last Seen: %s\n", stats.lastSeen.Format("2006-01-02 15:04:05"))
            fmt.Printf("    - Transaction Types: In: %d, Out: %d\n", 
                stats.transactionTypes["in"], stats.transactionTypes["out"])
        }
    }

    // 2. Analyze Temporal Patterns NEEEEDDDDD TO BE REWORKED 
    fmt.Printf("\n%s2. Temporal Analysis%s\n", Headers, Reset)
    if len(rapidTransactions) > 0 {
        fmt.Printf("\n%sRapid Transaction Sequences:%s\n", Yellow, Reset)
        for timeWindow, txIds := range rapidTransactions {
            fmt.Printf("Time Window: %s\n", timeWindow.Format("2006-01-02 15:04:05"))
            fmt.Printf("Number of transactions: %d\n", len(txIds))
            fmt.Printf("Transaction IDs: %v\n", txIds)
        }
    }

    // 3. Volume Analysis
    fmt.Printf("\n%s3. Volume Analysis%s\n", Headers, Reset)
    var highValueTxs []string
    for _, tx := range transactions {
        details := txDetails[tx.TxID]
        if math.Abs(details.Amount) > HIGH_VALUE_THRESHOLD {
            highValueTxs = append(highValueTxs, fmt.Sprintf("TxID: %s, Amount: %.8f BTC", 
                tx.TxID, math.Abs(details.Amount)))
        }
    }
    
    if len(highValueTxs) > 0 {
        fmt.Printf("\n%sHigh-Value Transactions:%s\n", Yellow, Reset)
        for _, tx := range highValueTxs {
            fmt.Printf("- %s\n", tx)
        }
    }

    // 4. Network Analysis
    fmt.Printf("\n%s4. Network Analysis%s\n", Headers, Reset)
    var frequentInteractors []struct {
        address string
        count   int
        volume  float64
    }
    
    for addr, stats := range addressStats {
        if addr == address {
            continue
        }
        if stats.totalTransactions > 2 {
            frequentInteractors = append(frequentInteractors, struct {
                address string
                count   int
                volume  float64
            }{addr, stats.totalTransactions, stats.totalVolume})
        }
    }

    if len(frequentInteractors) > 0 {
        fmt.Printf("\n%sFrequent Interactors:%s\n", Yellow, Reset)
        for _, interactor := range frequentInteractors {
            fmt.Printf("Address: %s\n", interactor.address)
            fmt.Printf("  - Transaction Count: %d\n", interactor.count)
            fmt.Printf("  - Total Volume: %.8f BTC\n", interactor.volume)
        }
    }


}

















func formatAddresses(addresses []string, limit int) string {
    if len(addresses) == 0 {
        return "N/A"
    }
    
    
    var validAddresses []string
    for _, addr := range addresses {
        if addr != "" {
            validAddresses = append(validAddresses, addr)
        }
    }
    
    if len(validAddresses) == 0 {
        return "N/A"
    }
    
    if len(validAddresses) <= limit {
        return strings.Join(validAddresses, ", ")
    }
    
    return fmt.Sprintf("%s (+%d)", 
        strings.Join(validAddresses[:limit], ", "), 
        len(validAddresses)-limit)
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
		
		amount, err := getTransactionAmount(*address, tx.TxID)
		if err != nil {
			log.Printf("Error fetching transaction %s: %v", tx.TxID, err)
			continue
		}
	
		
		price, err := GetPrice(int64(tx.Time))
		if err != nil {
			log.Printf("Error fetching price for transaction %s: %v", tx.TxID, err)
			price, err = GetPrice2(db, int64(tx.Time))
			if err != nil {
				fmt.Printf("Error occurred while fetching fallback price for transaction %s: %v\n", tx.TxID, err)
				continue
			}
		}
	
		
		var originAddresses []string
        for _, input := range tx.Inputs {
            if input.PrevOut.Addr != "" {
                originAddresses = append(originAddresses, input.PrevOut.Addr)
            }
        }
		
		
		
		var destAddresses []string
        for _, output := range tx.Out {
            if output.Addr != "" {
                destAddresses = append(destAddresses, output.Addr)
            }
        }

		


		
		btcAmount := float64(*amount) / 100_000_000

		var displayOrigin, displayDest string
		if btcAmount > 0 {
			
			displayOrigin = formatAddresses(originAddresses, 1)
			displayDest = wallet.Address
		} else {
			
			displayOrigin = wallet.Address
			displayDest = formatAddresses(destAddresses, 1)
		}
		txDetails[tx.TxID] = TransactionDetails{
			Amount:        btcAmount,
			Price:         price.Usd,
			Time:          time.Unix(int64(tx.Time), 0),
			Confirmations: tx.Confirmations,
			DisplayOrigin:   displayOrigin,
            DisplayDest:     displayDest,
		
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
	
		fmt.Printf("║ %-20s │ %s%-15.8f%s │ %-12d │ $%-14.2f │ %-20s │ %-30s │ %-30s ║\n",
    tx.TxID[:20],
    color, details.Amount, Reset,
    details.Confirmations,
    details.Amount*details.Price,
    details.Time.Format("2006-01-02 15:04:05"),
    details.DisplayOrigin,
    details.DisplayDest)
	}
	
	fmt.Printf("%s╚══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╝%s\n", Headers, Reset)




	analyzeTransactionPatterns(wallet.Transactions,*address,txDetails)
}
