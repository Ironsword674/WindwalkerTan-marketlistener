package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"golang.org/x/text/message"
)

type CoinData struct {
	ID           string  `json:"id"`
	Symbol       string  `json:"symbol"`
	CurrentPrice float64 `json:"current_price"`
	TotalVolume  float64 `json:"total_volume"`
	MarketCap    float64 `json:"market_cap"`
	Change7d     float64 `json:"price_change_percentage_7d_in_currency"`
}

func main() {
	p := message.NewPrinter(message.MatchLanguage("en"))

	requestedCoins := []string{
		"bitcoin", "ethereum", "solana", "cosmos",
		"cardano", "polkadot", "avalanche-2", "near",
	}

	coinList := strings.Join(requestedCoins, ",")
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids=%s&order=market_cap_desc&per_page=100&page=1&sparkline=false", coinList)

	fmt.Println("🚀 正在向 CoinGecko 请求实时数据...\n")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("❌ 网络请求失败 (可能超时或无网络): %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ API 返回异常状态码: %d\n", resp.StatusCode)
		return
	}

	maxBytes := 5 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, int64(maxBytes))
	
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		fmt.Printf("❌ 读取数据失败 (可能超出 5MB 限制): %v\n", err)
		return
	}

	var results []CoinData
	err = json.Unmarshal(body, &results)
	if err != nil {
		fmt.Printf("❌ 解析 JSON 失败: %v\n", err)
		return
	}

	receivedMap := make(map[string]bool)
	var receivedList []string
	for _, coin := range results {
		receivedMap[coin.ID] = true
		receivedList = append(receivedList, strings.Title(coin.ID))
	}

	var missingList []string
	for _, req := range requestedCoins {
		if !receivedMap[req] {
			missingList = append(missingList, req)
		}
	}

	fmt.Println("📋 【数据获取核对报告】")
	fmt.Printf("📥 请求总数: %d 个\n", len(requestedCoins))
	fmt.Printf("✅ 成功获取: %d 个\n", len(receivedList))
	
	if len(missingList) > 0 {
		fmt.Printf("❌ 缺失数据: %d 个 -> [%s]\n", len(missingList), strings.Join(missingList, ", "))
		fmt.Println("⚠️ 提示：缺失的币种可能已下架、改名，或 CoinGecko 暂时未返回其数据。")
	} else {
		fmt.Println("✅ 缺失数据: 0 个 (数据 100% 完整)")
	}
	fmt.Println(strings.Repeat("-", 50))

	fmt.Println("📊 实时数据详情:")
	fmt.Printf("%-15s | %-8s | %15s | %22s | %22s | %12s\n", 
		"代币名称", "符号", "现价 (USD)", "24h 交易量 (USD)", "市值 (USD)", "7天涨跌幅")
	fmt.Println(strings.Repeat("-", 105)) // 稍微加宽以适应千位分隔符

	var totalVolume, totalMarketCap float64

	for _, coin := range results {
		priceStr := p.Sprintf("$%.2f", coin.CurrentPrice)
		volumeStr := p.Sprintf("$%.2f", coin.TotalVolume)
		marketCapStr := p.Sprintf("$%.2f", coin.MarketCap)
		changeStr := fmt.Sprintf("%+.2f%%", coin.Change7d)

		fmt.Printf("%-15s | %-8s | %15s | %22s | %22s | %12s\n",
			strings.Title(coin.ID),
			strings.ToUpper(coin.Symbol),
			priceStr,
			volumeStr,
			marketCapStr,
			changeStr,
		)
		
		totalVolume += coin.TotalVolume
		totalMarketCap += coin.MarketCap
	}

	fmt.Println(strings.Repeat("-", 105))
	fmt.Printf("💡 成功获取的代币 24h 总交易量合计: %s USD\n", p.Sprintf("$%.2f", totalVolume))
	fmt.Printf("💡 成功获取的代币 总市值合计: %s USD\n", p.Sprintf("$%.2f", totalMarketCap))
}
