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
	Timeout: 20 * time.Second,
}


var Suspiciouswallets []string



//INside a for loop 

//for every suspicious wallet search up the wallet




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



func Getsuswallets(){



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





//FUnc analyse pattern 2222









//Need to study this              
func analyzeTransactionPatterns(transactions []Transaction, address string, txDetails map[string]TransactionDetails) {
    type AddressStats struct {
        totalTransactions int
        totalVolume       float64
        lastSeen          time.Time
        transactionTypes  map[string]int
        amounts           []float64
        timeDiffs         []float64
        dailyVolume       map[string]float64 
    }

    addressStats := make(map[string]AddressStats)
    unusualPatterns := make(map[string][]string)

    const (
        RAPID_TX_WINDOW          = 24 * time.Hour
        DORMANCY_PERIOD          = 30 * 24 * time.Hour
        HIGH_VALUE_THRESHOLD     = 1.0 
        UNUSUAL_VARIANCE_THRESHOLD = 2.0
        TEMPORAL_SPAM_THRESHOLD   = 3.0  
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

            
            day := txTime.Format("2006-01-02")
            if stats.dailyVolume == nil {
                stats.dailyVolume = make(map[string]float64)
            }
            stats.dailyVolume[day] += math.Abs(details.Amount)

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

    
    for addr, stats := range addressStats {
        if addr == address {
            continue 
        }

        
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

       
        if stdDev > UNUSUAL_VARIANCE_THRESHOLD && stats.totalTransactions > 3 {
            unusualPatterns[addr] = append(unusualPatterns[addr], 
                fmt.Sprintf("High variance in transaction amounts (stdDev: %.2f BTC)", stdDev))
        }

        
        if stats.totalVolume > HIGH_VALUE_THRESHOLD && stats.totalTransactions < 3 {
            unusualPatterns[addr] = append(unusualPatterns[addr], 
                "High volume with few transactions")
        }

        
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

        
        for day, dailyVolume := range stats.dailyVolume {
            if dailyVolume > HIGH_VALUE_THRESHOLD && len(stats.amounts) > TEMPORAL_SPAM_THRESHOLD {
                unusualPatterns[addr] = append(unusualPatterns[addr], 
                    fmt.Sprintf("High volume on %s, possibly suspicious activity", day))
            }
        }
    }

    

    //Get suspicious wallets from here 
    
    if len(unusualPatterns) > 0 {
        fmt.Printf("\n%sDetected Suspicious Patterns:%s\n", Yellow, Reset)
        for addr, patterns := range unusualPatterns {
            
            
            
            //Append the suspicious wallets
           // Suspiciouswallets =append(Suspiciouswallets,addr)
        
           
           
            fmt.Printf("Address: %s\n", addr)
            for _, pattern := range patterns {
                fmt.Printf("  - %s\n", pattern)
            }

            
            stats := addressStats[addr]
            fmt.Printf("  Statistics:\n")
            fmt.Printf("    - Total Transactions: %d\n", stats.totalTransactions)
            fmt.Printf("    - Total Volume: %.8f BTC\n", stats.totalVolume)
            fmt.Printf("    - Last Seen: %s\n", stats.lastSeen.Format("2006-01-02 15:04:05"))
            fmt.Printf("    - Transaction Types: In: %d, Out: %d\n", 
                stats.transactionTypes["in"], stats.transactionTypes["out"])
        }
    }

    
    fmt.Printf("\n%s2. Temporal Analysis%s\n", Headers, Reset)
    if len(rapidTransactions) > 0 {
        fmt.Printf("\n%sRapid Transaction Sequences:%s\n", Yellow, Reset)
        for timeWindow, txIds := range rapidTransactions {
            fmt.Printf("Time Window: %s\n", timeWindow.Format("2006-01-02 15:04:05"))
            fmt.Printf("Number of transactions: %d\n", len(txIds))
            fmt.Printf("Transaction IDs: %v\n", txIds)
        }
    }

    
    fmt.Printf("\n%s3. Volume Analysis%s\n", Headers, Reset)
    var highValueTxs []string
    var spikeTransactions []string
    var previousTxValue float64
    for _, tx := range transactions {
        details := txDetails[tx.TxID]
        if math.Abs(details.Amount) > HIGH_VALUE_THRESHOLD {
            highValueTxs = append(highValueTxs, fmt.Sprintf("TxID: %s, Amount: %.8f BTC", 
                tx.TxID, math.Abs(details.Amount)))
        }

        if previousTxValue != 0 && math.Abs(details.Amount) > previousTxValue * 1.5 {
            spikeTransactions = append(spikeTransactions, fmt.Sprintf("TxID: %s, Amount: %.8f BTC (Spike)", 
                tx.TxID, math.Abs(details.Amount)))
        }
        previousTxValue = math.Abs(details.Amount)
    }
    
    if len(highValueTxs) > 0 {
        fmt.Printf("\n%sHigh-Value Transactions:%s\n", Yellow, Reset)
        for _, tx := range highValueTxs {
            fmt.Printf("- %s\n", tx)
        }
    }
    if len(spikeTransactions) > 0 {
        fmt.Printf("\n%sSpike Transactions:%s\n", Yellow, Reset)
        for _, tx := range spikeTransactions {
            fmt.Printf("- %s\n", tx)
        }
    }

    
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


        if stats.totalTransactions > 4 {
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




func analyzeWalletBehavior(transactions []Transaction, address string, txDetails map[string]TransactionDetails) {
    // Analysis structures
    type AddressInteraction struct {
        totalVolume   float64
        frequency     int
        lastSeen      time.Time
        firstSeen     time.Time
        inVolume      float64
        outVolume     float64
        inCount       int
        outCount      int
        addresses     map[string]bool
    }

    // Initialize tracking maps
    dailyActivity := make(map[string]float64)
    monthlyActivity := make(map[string]float64)
    addressInteractions := make(map[string]*AddressInteraction)
    

    const (
        HIGH_VALUE_TX        = 1.0  // BTC
        SUSPICIOUS_FREQUENCY = 10    // transactions per day
        WHALE_THRESHOLD      = 5.0   // BTC
        INACTIVE_PERIOD      = 30 * 24 * time.Hour
        SATOSHI_TO_BTC      = 1e-8
    )

    var (
        totalVolume          float64
        maxDailyVolume      float64
        maxDailyVolumeDate  string
        maxTxValue          float64
        maxTxID             string
        maxTxTime           time.Time
        uniqueCounterparties = make(map[string]bool)
    )

   //First loop
    for _, tx := range transactions {
        txTime := time.Unix(int64(tx.Time), 0)
        dayKey := txTime.Format("2006-01-02")
        monthKey := txTime.Format("2006-01")
        
        var txVolume float64
        isOutgoing := false
        var counterpartyAddresses []string

        // Determine transaction direction and collect counterparties
        for _, input := range tx.Inputs {
            if input.PrevOut.Addr == address {
                isOutgoing = true
                txVolume = float64(input.PrevOut.Value) * SATOSHI_TO_BTC
            } else if input.PrevOut.Addr != "" {
                counterpartyAddresses = append(counterpartyAddresses, input.PrevOut.Addr)
                uniqueCounterparties[input.PrevOut.Addr] = true
            }
        }

        if !isOutgoing {
            for _, output := range tx.Out {
                if output.Addr == address {
                    txVolume = float64(output.Value) * SATOSHI_TO_BTC
                } else if output.Addr != "" {
                    counterpartyAddresses = append(counterpartyAddresses, output.Addr)
                    uniqueCounterparties[output.Addr] = true
                }
            }
        }

        // Update daily and monthly volumes
        dailyActivity[dayKey] += txVolume
        monthlyActivity[monthKey] += txVolume
        
        if dailyActivity[dayKey] > maxDailyVolume {
            maxDailyVolume = dailyActivity[dayKey]
            maxDailyVolumeDate = dayKey
        }

        if txVolume > maxTxValue {
            maxTxValue = txVolume
            maxTxID = tx.TxID
            maxTxTime = txTime
        }

        // Update address interactions
        for _, addr := range counterpartyAddresses {
            if addr == "" || addr == address {
                continue
            }

            if _, exists := addressInteractions[addr]; !exists {
                addressInteractions[addr] = &AddressInteraction{
                    firstSeen:  txTime,
                    addresses:  make(map[string]bool),
                }
            }

            interaction := addressInteractions[addr]
            interaction.lastSeen = txTime
            interaction.frequency++
            
            if isOutgoing {
                interaction.outVolume += txVolume
                interaction.outCount++
            } else {
                interaction.inVolume += txVolume
                interaction.inCount++
            }
            
            interaction.totalVolume += txVolume
            interaction.addresses[addr] = true
        }

        totalVolume += txVolume
    }

    // Print comprehensive analysis
    fmt.Printf("\n%s=== Comprehensive Wallet Analysis ===%s\n\n", Yellow, Reset)
    
    // Volume Analysis
    fmt.Printf("%s1. Volume Statistics%s\n", Cyan, Reset)
    fmt.Printf("- Total Volume: %.8f BTC\n", totalVolume)
    fmt.Printf("- Highest Daily Volume: %.8f BTC on %s\n", maxDailyVolume, maxDailyVolumeDate)
    fmt.Printf("- Largest Single Transaction: %.8f BTC (%s at %s)\n", 
        maxTxValue, maxTxID, maxTxTime.Format("2006-01-02 15:04:05"))
    fmt.Printf("- Average Transaction Size: %.8f BTC\n", totalVolume/float64(len(transactions)))
    
    // Temporal Analysis
    fmt.Printf("\n%s2. Activity Patterns%s\n", Cyan, Reset)
    activeHours := make(map[int]int)
    for _, tx := range transactions {
        hour := time.Unix(int64(tx.Time), 0).Hour()
        activeHours[hour]++
    }
    
    var mostActiveHour int
    var maxHourlyTx int
    for hour, count := range activeHours {
        if count > maxHourlyTx {
            mostActiveHour = hour
            maxHourlyTx = count
        }
    }
    fmt.Printf("- Most Active Hour: %02d:00 UTC (%d transactions)\n", mostActiveHour, maxHourlyTx)
    
    
    fmt.Printf("\n%s3. Counterparty Analysis%s\n", Cyan, Reset)
    fmt.Printf("- Total Unique Counterparties: %d\n", len(uniqueCounterparties))
    
    var frequentPartners []string
    var highValuePartners []string
    var suspiciousAddrs []string
    
    for addr, interaction := range addressInteractions {
        timeDiff := interaction.lastSeen.Sub(interaction.firstSeen)
        
        // Identify frequent partners
        if interaction.frequency >= 5 {
            frequentPartners = append(frequentPartners, fmt.Sprintf(
                "%s (%d transactions, %.8f BTC)", 
                addr, interaction.frequency, interaction.totalVolume))
        }
        
        // Identify high-value partners
        if interaction.totalVolume >= WHALE_THRESHOLD {
            highValuePartners = append(highValuePartners, fmt.Sprintf(
                "%s (%.8f BTC)", addr, interaction.totalVolume))
        }
        
        // Identify suspicious patterns
        txPerHour := float64(interaction.frequency) / (timeDiff.Hours() + 1)
        if txPerHour >= 5 {
            suspiciousAddrs = append(suspiciousAddrs, fmt.Sprintf(
                "%s (%d transactions in %s)", 
                addr, interaction.frequency, timeDiff.String()))
        }
    }
    
    if len(frequentPartners) > 0 {
        fmt.Printf("\n%sFrequent Transaction Partners:%s\n", Yellow, Reset)
        for _, partner := range frequentPartners {
            fmt.Printf("- %s\n", partner)
        }
    }
    
    if len(highValuePartners) > 0 {
        fmt.Printf("\n%sHigh-Value Partners (>%.1f BTC):%s\n", Yellow, WHALE_THRESHOLD, Reset)
        for _, partner := range highValuePartners {
            fmt.Printf("- %s\n", partner)
        }
    }
    
    if len(suspiciousAddrs) > 0 {
        fmt.Printf("\n%sPotentially Suspicious Activity:%s\n", Red, Reset)
        for _, suspicious := range suspiciousAddrs {
            fmt.Printf("- %s\n", suspicious)
        }
    }

    // Usage Patterns
    fmt.Printf("\n%s4. Usage Patterns%s\n", Cyan, Reset)
    fmt.Printf("- Active Days: %d\n", len(dailyActivity))
    fmt.Printf("- Active Months: %d\n", len(monthlyActivity))
    fmt.Printf("- Average Daily Volume: %.8f BTC\n", totalVolume/float64(len(dailyActivity)))
    fmt.Printf("- Transactions per Day: %.2f\n", float64(len(transactions))/float64(len(dailyActivity)))

    // Risk Assessment
    riskScore := 0
    riskFactors := make([]string, 0)
    
    if len(suspiciousAddrs) > 0 {
        riskScore++
        riskFactors = append(riskFactors, "High frequency trading patterns detected")
    }
    if maxDailyVolume > WHALE_THRESHOLD {
        riskScore++
        riskFactors = append(riskFactors, "Large daily volume spikes")
    }
    if float64(len(transactions))/float64(len(dailyActivity)) > SUSPICIOUS_FREQUENCY {
        riskScore++
        riskFactors = append(riskFactors, "Unusually high transaction frequency")
    }
    
    fmt.Printf("\n%s5. Risk Assessment%s\n", Headers, Reset)
    switch {
    case riskScore >= 2:
        fmt.Printf("%sHIGH RISK - Multiple suspicious patterns detected%s\n", Red, Reset)
    case riskScore == 1:
        fmt.Printf("%sMEDIUM RISK - Some unusual patterns detected%s\n", Yellow, Reset)
    default:
        fmt.Printf("%sLOW RISK - No significant suspicious patterns detected%s\n", Green, Reset)
    }
    
    if len(riskFactors) > 0 {
        fmt.Printf("Risk factors identified:\n")
        for _, factor := range riskFactors {
            fmt.Printf("- %s\n", factor)
        }
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




	//analyzeTransactionPatterns(wallet.Transactions,*address,txDetails)
    analyzeWalletBehavior(wallet.Transactions,*address,txDetails)
    
    
    fmt.Println(Suspiciouswallets)
}
