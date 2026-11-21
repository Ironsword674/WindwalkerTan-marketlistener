package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	maxBytes       = 5 * 1024 * 1024
	cryptoFileName = "crypto_history.csv"
	tradFiFileName = "tradfi_history.csv"
	intervalDays   = 7
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

type CoinData struct {
	ID           string  `json:"id"`
	Symbol       string  `json:"symbol"`
	CurrentPrice float64 `json:"current_price"`
	TotalVolume  float64 `json:"total_volume"`
	MarketCap    float64 `json:"market_cap"`
	Change7d     float64 `json:"price_change_percentage_7d_in_currency"`
}

type TradFiAsset struct {
	Symbol string
	Name   string
}

func main() {
	loc := time.FixedZone("UTC+8", 8*3600)
	p := message.NewPrinter(language.English)
	caser := cases.Title(language.English)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("================================================================================")
	fmt.Println(" 🌍 全球宏观与加密资产全景监控程序 (反脆弱自愈版)")
	fmt.Printf(" 💾 加密现货数据保存至: %s\n", cryptoFileName)
	fmt.Printf(" 💾 传统金融现货数据保存至: %s\n", tradFiFileName)
	fmt.Println("================================================================================\n")

	repairAndEnsureCSV(cryptoFileName, []string{"采集时间 (UTC+8)", "类别", "代币ID", "符号", "现价 (USD)", "7天涨跌幅 (%)", "24h交易量 (USD)", "市值 (USD)"})
	repairAndEnsureCSV(tradFiFileName, []string{"采集时间 (UTC+8)", "类别", "代码", "资产名称", "现价 (USD)", "今日涨跌幅 (%)", "成交量", "市值 (USD)"})

	for {
		fmt.Printf("\n[%s] 🚀 正在发起双引擎网络请求，请稍候...\n", time.Now().In(loc).Format("2006-01-02 15:04:05"))

		successCrypto := fetchAndSaveCryptoData(p, caser, loc)
		successTradFi := fetchAndSaveTradFiData(p, caser, loc)

		now := time.Now().In(loc)
		nextRun := now.Add(time.Duration(intervalDays) * 24 * time.Hour)
		duration := nextRun.Sub(now)

		if duration <= 0 {
			fmt.Println("⚠️ 检测到系统时间异常或回调，强制重置休眠时间为 24 小时以保护 API 额度。")
			duration = 24 * time.Hour
		}

		fmt.Println("--------------------------------------------------------------------------------")
		if successCrypto && successTradFi {
			fmt.Printf("✅ 双引擎任务均成功完成。程序将休眠，等待下次执行。\n")
		} else {
			fmt.Printf("⚠️ 部分或全部任务失败。程序仍将休眠，等待下个周期重试。\n")
		}
		fmt.Printf("⏱️  下次执行时间: %s (距今约 %d 天)\n", nextRun.Format("2006-01-02 15:04:05"), intervalDays)
		fmt.Println("--------------------------------------------------------------------------------")
		fmt.Println("💡 提示: 按 Ctrl+C 可安全停止程序。")

		select {
		case <-sigChan:
			fmt.Println("\n🛑 收到退出信号，程序正在安全关闭...")
			return
		case <-time.After(duration):
			continue
		}
	}
}

func repairAndEnsureCSV(fileName string, headers []string) {
	if _, err := os.Stat(fileName); err == nil {
		fmt.Printf("✅ 检测到正常文件: %s，将继续安全追加数据。\n", fileName)
		return
	}

	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	pattern := baseName + ".*"
	
	matches, err := filepath.Glob(pattern)
	if err == nil && len(matches) > 0 {
		for _, match := range matches {
			if match != fileName {
				fmt.Printf("⚠️ 发现异常文件: %s (可能是被误改名或云端同步干扰)\n", match)
				fmt.Printf("🔧 正在执行自愈：将其强制重命名回正确的 %s ...\n", fileName)
				
				if err := os.Rename(match, fileName); err != nil {
					fmt.Printf("❌ 自愈失败，无法重命名: %v\n", err)
				} else {
					fmt.Printf("✅ 自愈成功！已恢复文件: %s\n", fileName)
					return
				}
			}
		}
	}

	fmt.Printf("ℹ️  未找到历史文件，正在创建全新的: %s\n", fileName)
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("❌ 创建文件 %s 失败: %v\n", fileName, err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write(headers); err != nil {
		fmt.Printf("❌ 写入 %s 表头失败: %v\n", fileName, err)
		return
	}
	writer.Flush()
	fmt.Printf("✅ 已创建新文件并写入表头: %s\n", fileName)
}

func fetchAndSaveCryptoData(p *message.Printer, caser cases.Caser, loc *time.Location) bool {
	requestedCoins := []string{
		"bitcoin", "ethereum", "solana", "cosmos", "cardano", 
		"polkadot", "avalanche-2", "near", "pax-gold",
		"starknet", "matic-network", 
		"celestia", "neutron-3", "agoric", "persistence", 
		"juno-network", 
	}

	coinList := strings.Join(requestedCoins, ",")
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids=%s&order=market_cap_desc&per_page=100&page=1&sparkline=false", coinList)

	resp, err := httpClient.Get(url)
	if err != nil {
		fmt.Printf("❌ [Crypto] 网络请求失败: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		fmt.Println("❌ [Crypto] API 返回 429 Too Many Requests。触发 CoinGecko 限流。")
		return false
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ [Crypto] API 返回异常状态码: %d\n", resp.StatusCode)
		return false
	}

	limitedReader := io.LimitReader(resp.Body, int64(maxBytes))
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		fmt.Printf("❌ [Crypto] 读取数据失败: %v\n", err)
		return false
	}

	var results []CoinData
	if err := json.Unmarshal(body, &results); err != nil {
		fmt.Printf("❌ [Crypto] 解析 JSON 失败: %v\n", err)
		return false
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].MarketCap > results[j].MarketCap
	})

	fmt.Println("📋 【Crypto 现货数据获取核对报告 (已按市值强制降序)】")
	fmt.Printf("📥 请求总数: %d 个 | ✅ 成功获取: %d 个\n", len(requestedCoins), len(results))

	file, err := os.OpenFile(cryptoFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("❌ [Crypto] 无法打开 CSV 文件: %v\n", err)
		return false
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	nowStr := time.Now().In(loc).Format("2006-01-02 15:04:05")

	for _, coin := range results {
		priceStr := p.Sprintf("$%.2f", coin.CurrentPrice)
		volumeStr := p.Sprintf("$%.2f", coin.TotalVolume)
		marketCapStr := p.Sprintf("$%.2f", coin.MarketCap)
		changeStr := fmt.Sprintf("%+.2f%%", coin.Change7d)

		fmt.Printf("  %-15s | %-6s | %12s | %18s | %18s | %10s\n",
			caser.String(coin.ID), strings.ToUpper(coin.Symbol), priceStr, volumeStr, marketCapStr, changeStr)

		record := []string{
			nowStr, "Crypto", coin.ID, strings.ToUpper(coin.Symbol),
			fmt.Sprintf("%.2f", coin.CurrentPrice), fmt.Sprintf("%.2f", coin.Change7d),
			fmt.Sprintf("%.2f", coin.TotalVolume), fmt.Sprintf("%.2f", coin.MarketCap),
		}
		writer.Write(record)
	}

	if err := writer.Error(); err != nil {
		fmt.Printf("❌ [Crypto] CSV 刷入磁盘失败: %v\n", err)
		return false
	}
	fmt.Println("💾 [Crypto] 数据已成功追加。")
	return true
}

func fetchAndSaveTradFiData(p *message.Printer, caser cases.Caser, loc *time.Location) bool {
	tradFiAssets := []TradFiAsset{
		{Symbol: "XAUUSD=X", Name: "现货黄金 (XAU/USD - 宏观真实定价锚)"},
		{Symbol: "XAGUSD=X", Name: "现货白银 (XAG/USD - 宏观真实定价锚)"},
		{Symbol: "MSTR", Name: "MicroStrategy (微策略 - 企业级 BTC 代理)"},
		{Symbol: "IBIT", Name: "iShares Bitcoin Trust (贝莱德 BTC ETF)"},
		{Symbol: "FBTC", Name: "Fidelity Wise Origin Bitcoin Fund (富达 BTC ETF)"},
		{Symbol: "GBTC", Name: "Grayscale Bitcoin Trust (灰度 BTC ETF)"},
		{Symbol: "ETHA", Name: "iShares Ethereum Trust (贝莱德 ETH ETF)"},
		{Symbol: "FETH", Name: "Fidelity Ethereum Fund (富达 ETH ETF)"},
		{Symbol: "ETHE", Name: "Grayscale Ethereum Trust (灰度 ETH ETF)"},
		{Symbol: "ARKK", Name: "ARK Innovation ETF (木头姐旗舰基金)"},
		{Symbol: "FXI", Name: "iShares China Large-Cap ETF (中国大盘股)"},
		{Symbol: "CRCL", Name: "Circle Internet Group (USDC 发行方)"},
	}

	symbols := make([]string, 0, len(tradFiAssets))
	for _, asset := range tradFiAssets {
		symbols = append(symbols, asset.Symbol)
	}
	symbolList := strings.Join(symbols, ",")

	url := fmt.Sprintf("https://query1.finance.yahoo.com/v7/finance/quote?symbols=%s", symbolList)

	resp, err := httpClient.Get(url)
	if err != nil {
		fmt.Printf("❌ [TradFi] 网络请求失败: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ [TradFi] API 返回异常状态码: %d\n", resp.StatusCode)
		return false
	}

	limitedReader := io.LimitReader(resp.Body, int64(maxBytes))
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		fmt.Printf("❌ [TradFi] 读取数据失败: %v\n", err)
		return false
	}

	var yfResp struct {
		QuoteResponse struct {
			Result []struct {
				Symbol                   string  `json:"symbol"`
				ShortName                string  `json:"shortName"`
				RegularMarketPrice       float64 `json:"regularMarketPrice"`
				RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
				RegularMarketVolume      int64   `json:"regularMarketVolume"`
				MarketCap                float64 `json:"marketCap"`
			} `json:"result"`
			Error interface{} `json:"error"`
		} `json:"quoteResponse"`
	}

	if err := json.Unmarshal(body, &yfResp); err != nil {
		fmt.Printf("❌ [TradFi] 解析 JSON 失败: %v\n", err)
		return false
	}

	if yfResp.QuoteResponse.Error != nil {
		fmt.Printf("❌ [TradFi] API 返回业务错误: %v\n", yfResp.QuoteResponse.Error)
		return false
	}

	resultMap := make(map[string]struct {
		Price  float64
		Change float64
		Vol    int64
		MCap   float64
	})
	for _, item := range yfResp.QuoteResponse.Result {
		resultMap[item.Symbol] = struct {
			Price  float64
			Change float64
			Vol    int64
			MCap   float64
		}{
			Price:  item.RegularMarketPrice,
			Change: item.RegularMarketChangePercent,
			Vol:    item.RegularMarketVolume,
			MCap:   item.MarketCap,
		}
	}

	fmt.Println("📋 【TradFi 现货数据获取核对报告】")
	fmt.Printf("📥 请求总数: %d 个 | ✅ 成功获取: %d 个\n", len(tradFiAssets), len(yfResp.QuoteResponse.Result))

	file, err := os.OpenFile(tradFiFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("❌ [TradFi] 无法打开 CSV 文件: %v\n", err)
		return false
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	nowStr := time.Now().In(loc).Format("2006-01-02 15:04:05")

	for _, asset := range tradFiAssets {
		data, exists := resultMap[asset.Symbol]
		if !exists {
			record := []string{nowStr, "TradFi", asset.Symbol, asset.Name, "N/A", "N/A", "N/A", "N/A"}
			writer.Write(record)
			continue
		}

		record := []string{
			nowStr, "TradFi", asset.Symbol, asset.Name,
			fmt.Sprintf("%.2f", data.Price), fmt.Sprintf("%.2f", data.Change),
			fmt.Sprintf("%d", data.Vol), fmt.Sprintf("%.0f", data.MCap),
		}
		writer.Write(record)
	}

	if err := writer.Error(); err != nil {
		fmt.Printf("❌ [TradFi] CSV 刷入磁盘失败: %v\n", err)
		return false
	}
	fmt.Println("💾 [TradFi] 数据已成功追加。")
	return true
}
