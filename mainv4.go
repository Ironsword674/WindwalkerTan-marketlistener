package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
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

const (
	maxBytes     = 5 * 1024 * 1024
	dataFileName = "crypto_history.csv"
	intervalDays = 7
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

func main() {
	loc := time.FixedZone("UTC+8", 8*3600)
	p := message.NewPrinter(language.English)
	caser := cases.Title(language.English)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("================================================================================")
	fmt.Println(" 📊 加密货币核心指标连续监控程序已启动 (排雷加固版)")
	fmt.Printf(" 📌 数据来源: CoinGecko Public API\n")
	fmt.Printf(" 💾 数据将自动追加保存至: %s\n", dataFileName)
	fmt.Println("================================================================================\n")

	initCSVFileSafely()

	for {
		fmt.Printf("\n[%s] 🚀 正在发起网络请求，请稍候...\n", time.Now().In(loc).Format("2006-01-02 15:04:05"))

		success := fetchAndSaveData(p, caser, loc)

		now := time.Now().In(loc)
		nextRun := now.Add(time.Duration(intervalDays) * 24 * time.Hour)
		duration := nextRun.Sub(now)

		if duration <= 0 {
			fmt.Println("⚠️ 检测到系统时间异常或回调，强制重置休眠时间为 24 小时以保护 API 额度。")
			duration = 24 * time.Hour
		}

		fmt.Println("--------------------------------------------------------------------------------")
		if success {
			fmt.Printf("✅ 本次任务成功完成。程序将休眠，等待下次执行。\n")
		} else {
			fmt.Printf("⚠️ 本次任务失败。程序仍将休眠，等待下个周期重试。\n")
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

func fetchAndSaveData(p *message.Printer, caser cases.Caser, loc *time.Location) bool {
	requestedCoins := []string{
		"bitcoin", "ethereum", "solana", "cosmos",
		"cardano", "polkadot", "avalanche-2", "near",
	}

	coinList := strings.Join(requestedCoins, ",")
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids=%s&order=market_cap_desc&per_page=100&page=1&sparkline=false", coinList)

	resp, err := httpClient.Get(url)
	if err != nil {
		fmt.Printf("❌ 网络请求失败: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		fmt.Println("❌ API 返回 429 Too Many Requests。")
		fmt.Println("⚠️ 警告：您触发 CoinGecko 免费版的速率限制！请切勿将循环间隔修改得过短（建议保持 7 天）。")
		return false
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ API 返回异常状态码: %d\n", resp.StatusCode)
		return false
	}

	limitedReader := io.LimitReader(resp.Body, int64(maxBytes))
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		fmt.Printf("❌ 读取数据失败: %v\n", err)
		return false
	}

	if len(body) == maxBytes {
		fmt.Printf("⚠️ 严重警告：响应数据达到 %dMB 上限，JSON 可能已被截断！\n", maxBytes/(1024*1024))
	}

	var results []CoinData
	if err := json.Unmarshal(body, &results); err != nil {
		fmt.Printf("❌ 解析 JSON 失败: %v\n", err)
		return false
	}

	fmt.Println("📋 【数据获取核对报告】")
	fmt.Printf("📥 请求总数: %d 个 | ✅ 成功获取: %d 个\n", len(requestedCoins), len(results))
	fmt.Println(strings.Repeat("-", 50))

	file, err := os.OpenFile(dataFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("❌ 无法打开/创建 CSV 文件: %v\n", err)
		return false
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	nowStr := time.Now().In(loc).Format("2006-01-02 15:04:05")

	for _, coin := range results {
		priceStr := p.Sprintf("$%.2f", coin.CurrentPrice)
		volumeStr := p.Sprintf("$%.2f", coin.TotalVolume)
		marketCapStr := p.Sprintf("$%.2f", coin.MarketCap)
		changeStr := fmt.Sprintf("%+.2f%%", coin.Change7d)

		fmt.Printf("%-15s | %-8s | %15s | %22s | %22s | %12s\n",
			caser.String(coin.ID),
			strings.ToUpper(coin.Symbol),
			priceStr, volumeStr, marketCapStr, changeStr,
		)

		record := []string{
			nowStr,
			coin.ID,
			strings.ToUpper(coin.Symbol),
			fmt.Sprintf("%.2f", coin.CurrentPrice),
			fmt.Sprintf("%.2f", coin.TotalVolume),
			fmt.Sprintf("%.2f", coin.MarketCap),
			fmt.Sprintf("%.2f", coin.Change7d),
		}
		if err := writer.Write(record); err != nil {
			fmt.Printf("⚠️ 写入 CSV 记录失败 (%s): %v\n", coin.ID, err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		fmt.Printf("❌ CSV 数据刷入磁盘失败: %v\n", err)
		return false
	}

	fmt.Println("💾 数据已成功追加至本地文件。")
	return true
}

func initCSVFileSafely() {
	file, err := os.OpenFile(dataFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("❌ 初始化 CSV 文件失败: %v\n", err)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err == nil && fileInfo.Size() == 0 {
		writer := csv.NewWriter(file)
		headers := []string{
			"采集时间 (UTC+8)", "代币ID", "符号",
			"现价 (USD)", "24h交易量 (USD)", "市值 (USD)", "7天涨跌幅 (%)",
		}
		if err := writer.Write(headers); err != nil {
			fmt.Printf("❌ 写入 CSV 表头失败: %v\n", err)
			return
		}
		writer.Flush()
		fmt.Println("✅ 已创建新的历史数据文件并写入表头:", dataFileName)
	} else {
		fmt.Println("✅ 检测到已有历史数据文件，将继续安全追加数据。")
	}
}
