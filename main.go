package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type CoinData struct {
	ID                   string  `json:"id"`
	Symbol               string  `json:"symbol"`
	CurrentPrice         float64 `json:"current_price"`
	TotalVolume          float64 `json:"total_volume"` 
}

func main() {
	coins := []string{
		"bitcoin", "ethereum", "solana", "cosmos", 
		"cardano", "polkadot", "avalanche-2", "near",
	}
	
	coinList := strings.Join(coins, ",")
  
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids=%s&order=market_cap_desc&per_page=100&page=1&sparkline=false", coinList)

	fmt.Println("🚀 正在向数据聚合器请求今日统计信息...\n")

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close() 

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ 读取数据失败: %v\n", err)
		return
	}

	var results []CoinData
	err = json.Unmarshal(body, &results)
	if err != nil {
		fmt.Printf("❌ 解析数据失败: %v\n", err)
		return
	}
  
	fmt.Printf("%-12s | %-6s | %-15s | %-15s\n", "币种", "符号", "当前价格 (USD)", "24h 交易量 (USD)")
	fmt.Println(strings.Repeat("-", 55))

	var totalMarketVolume float64

	for _, coin := range results {
		priceStr := fmt.Sprintf("$%,.2f", coin.CurrentPrice)
		volumeStr := fmt.Sprintf("$%,.2f", coin.TotalVolume)
		
		fmt.Printf("%-12s | %-6s | %-15s | %-15s\n", 
			strings.Title(coin.ID), 
			strings.ToUpper(coin.Symbol), 
			priceStr, 
			volumeStr)
			
		totalMarketVolume += coin.TotalVolume
	}

	fmt.Println(strings.Repeat("-", 55))
	fmt.Printf("💡 这 8 大主流链今日总交易量合计: $%,.2f USD\n", totalMarketVolume)
}
