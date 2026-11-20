package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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

const maxBytes = 5 * 1024 * 1024

func main() {
	loc := time.FixedZone("UTC+8", 8*3600)
	
	now := time.Now().In(loc)
	timestamp := now.Format("2006-01-02 15:04:05")
	timezone := now.Location().String() 

	p := message.NewPrinter(language.English)
	caser := cases.Title(language.English)

	fmt.Println("================================================================================")
	fmt.Println(" 📊 加密货币核心指标实时监控报告")
	fmt.Printf(" 📌 数据来源: CoinGecko Public API (api.coingecko.com)\n")
	fmt.Printf(" ⏱️ 数据快照时间: %s (%s)\n", timestamp, timezone)
	fmt.Println("================================================================================\n")

	requestedCoins := []string{
		"bitcoin", "ethereum", "solana", "cosmos",
		"cardano", "polkadot", "avalanche-2", "near",
	}

	coinList := strings.Join(requestedCoins, ",")
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids=%s&order=market_cap_desc&per_page=100&page=1&sparkline=false", coinList)

	fmt.Println("🚀 正在发起网络请求，请稍候...\n")

	client := &http.Client{Timeout: 10 * time.Second}
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

	limitedReader := io.LimitReader(resp.Body, int64(maxBytes))
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		fmt.Printf("❌ 读取数据失败: %v\n", err)
		return
	}

	if len(body) == maxBytes {
		fmt.Printf("⚠️ 严重警告：响应数据达到 %dMB 上限，JSON 可能已被截断！\n\n", maxBytes/(1024*1024))
	}

	var results []CoinData
	if err := json.Unmarshal(body, &results); err != nil {
		fmt.Printf("❌ 解析 JSON 失败 (数据可能损坏或格式错误): %v\n", err)
		return
	}

	receivedMap := make(map[string]bool)
	var receivedList []string
	for _, coin := range results {
		receivedMap[coin.ID] = true
		receivedList = append(receivedList, caser.String(coin.ID))
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

	fmt.Println("📊 实时数据详情 (按市值降序排列):")
	fmt.Printf("%-15s | %-8s | %15s | %22s | %22s | %12s\n", 
		"代币名称", "符号", "现价 (USD)", "24h 交易量 (USD)", "市值 (USD)", "7天涨跌幅")
	fmt.Println(strings.Repeat("-", 105))

	var totalVolume, totalMarketCap float64

	for _, coin := range results {
		priceStr := p.Sprintf("$%.2f", coin.CurrentPrice)
		volumeStr := p.Sprintf("$%.2f", coin.TotalVolume)
		marketCapStr := p.Sprintf("$%.2f", coin.MarketCap)
		changeStr := fmt.Sprintf("%+.2f%%", coin.Change7d)

		fmt.Printf("%-15s | %-8s | %15s | %22s | %22s | %12s\n",
			caser.String(coin.ID),
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
	fmt.Println("================================================================================")
}
