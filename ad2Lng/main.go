package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"golang.org/x/time/rate"
)

// BaiduGeocodingResponse ===================== 百度地图 Geocoding 响应结构 =====================
type BaiduGeocodingResponse struct {
	Status int `json:"status"`
	Result struct {
		Location struct {
			Lng float64 `json:"lng"`
			Lat float64 `json:"lat"`
		} `json:"location"`
		Precise       int    `json:"precise"`
		Confidence    int    `json:"confidence"`
		Comprehension int    `json:"comprehension"`
		Level         string `json:"level"`
	} `json:"result"`
}

// ===================== 坐标转换工具 =====================
const xPi = math.Pi * 3000.0 / 180.0

// BD-09 -> GCJ-02
func bd09ToGcj02(bdLng, bdLat float64) (gcjLng, gcjLat float64) {
	x := bdLng - 0.0065
	y := bdLat - 0.006
	z := math.Sqrt(x*x+y*y) - 0.00002*math.Sin(y*xPi)
	theta := math.Atan2(y, x) - 0.000003*math.Cos(x*xPi)
	gcjLng = z * math.Cos(theta)
	gcjLat = z * math.Sin(theta)
	return
}

// GCJ-02 -> WGS-84（近似）
func gcj02ToWgs84(gcjLng, gcjLat float64) (wgsLng, wgsLat float64) {
	dLat, dLng := delta(gcjLat, gcjLng)
	wgsLat = gcjLat - dLat
	wgsLng = gcjLng - dLng
	return
}

func delta(lat, lng float64) (dLat, dLng float64) {
	a := 6378245.0
	ee := 0.00669342162296594323

	dLat = transformLat(lng-105.0, lat-35.0)
	dLng = transformLng(lng-105.0, lat-35.0)
	radLat := lat / 180.0 * math.Pi
	magic := math.Sin(radLat)
	magic = 1 - ee*magic*magic
	sqrtMagic := math.Sqrt(magic)
	dLat = (dLat * 180.0) / ((a * (1 - ee)) / (magic * sqrtMagic) * math.Pi)
	dLng = (dLng * 180.0) / (a / sqrtMagic * math.Cos(radLat) * math.Pi)
	return
}

func transformLat(x, y float64) float64 {
	ret := -100.0 + 2.0*x + 3.0*y + 0.2*y*y +
		0.1*x*y + 0.2*math.Sqrt(math.Abs(x))
	ret += (20.0*math.Sin(6.0*x*math.Pi) + 20.0*math.Sin(2.0*x*math.Pi)) * 2.0 / 3.0
	ret += (20.0*math.Sin(y*math.Pi) + 40.0*math.Sin(y/3.0*math.Pi)) * 2.0 / 3.0
	ret += (160.0*math.Sin(y/12.0*math.Pi) + 320*math.Sin(y*math.Pi/30.0)) * 2.0 / 3.0
	return ret
}

func transformLng(x, y float64) float64 {
	ret := 300.0 + x + 2.0*y + 0.1*x*x +
		0.1*x*y + 0.1*math.Sqrt(math.Abs(x))
	ret += (20.0*math.Sin(6.0*x*math.Pi) + 20.0*math.Sin(2.0*x*math.Pi)) * 2.0 / 3.0
	ret += (20.0*math.Sin(x*math.Pi) + 40.0*math.Sin(x/3.0*math.Pi)) * 2.0 / 3.0
	ret += (150.0*math.Sin(x/12.0*math.Pi) + 300.0*math.Sin(x/30.0*math.Pi)) * 2.0 / 3.0
	return ret
}

func queryBaiduAPI(address, city, ak string) (*BaiduGeocodingResponse, error) {
	baseURL := "https://api.map.baidu.com/geocoding/v3/"
	params := url.Values{}
	params.Add("address", address)
	params.Add("city", city)
	params.Add("output", "json")
	params.Add("ak", ak)

	fullURL := baseURL + "?" + params.Encode()
	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var geoResp BaiduGeocodingResponse
	if err := json.Unmarshal(body, &geoResp); err != nil {
		return nil, err
	}
	if geoResp.Status != 0 {
		return nil, fmt.Errorf("百度返回错误：%s", body)
	}
	return &geoResp, nil
}

// processLocations 封装的处理函数
func processLocations(ctx context.Context, db *pgxpool.Pool, ak string, limiter *rate.Limiter) error {
	// 查询未处理地址记录
	rows, err := db.Query(ctx, `SELECT id, address, city FROM gis.location_info WHERE wgs_lng IS NULL`)
	if err != nil {
		return fmt.Errorf("查询数据库失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var address, city string
		if err := rows.Scan(&id, &address, &city); err != nil {
			log.Printf("读取记录失败 ID %d: %v\n", id, err)
			continue
		}

		// 限速控制：3 QPS
		if err := limiter.Wait(ctx); err != nil {
			log.Printf("等待速率控制器失败: %v\n", err)
			continue
		}

		// 请求百度API
		geoResp, err := queryBaiduAPI(address, city, ak)
		if err != nil {
			log.Printf("地址解析失败 ID %d: %v\n", id, err)
			continue
		}

		// 坐标转换：BD09 → GCJ02 → WGS84
		bdLng := geoResp.Result.Location.Lng
		bdLat := geoResp.Result.Location.Lat
		gcjLng, gcjLat := bd09ToGcj02(bdLng, bdLat)
		wgsLng, wgsLat := gcj02ToWgs84(gcjLng, gcjLat)

		// 更新数据库
		_, err = db.Exec(ctx, `
			UPDATE location_info 
			SET bd_lng = $1, bd_lat = $2, wgs_lng = $3, wgs_lat = $4, confidence = $5, level = $6, comprehension = $7, precise = $8
			WHERE id = $9
		`, bdLng, bdLat, wgsLng, wgsLat, geoResp.Result.Confidence, geoResp.Result.Level, geoResp.Result.Comprehension, geoResp.Result.Precise, id)

		if err != nil {
			log.Printf("更新失败 ID %d: %v\n", id, err)
			continue
		}

		log.Printf("✅ 更新成功 ID %d -> WGS(%.6f, %.6f) Level: %s", id, wgsLng, wgsLat, geoResp.Result.Level)
	}
	return nil
}

// ===================== 主程序入口 =====================
func main() {
	ctx := context.Background()
	limiter := rate.NewLimiter(rate.Every(time.Second/2), 1)
	// address := "长沙市雨花区上海城30栋"
	// city := "长沙市"
	ak := "6je2ZCe86LWHFOiL9dRYTU09xccrj3fH" // <<< 替换为你的 AK

	// 控制速率：每秒最多 3 个请求
	if err := limiter.Wait(ctx); err != nil {
		log.Printf("等待速率控制器失败: %v\n", err)
		// continue
	}
	// 连接数据库
	dbpool, err := pgxpool.New(ctx, "postgres://postgres:3@localhost:5432/beijing?sslmode=disable")
	if err != nil {
		log.Fatal("连接池创建失败:", err)
	}
	defer dbpool.Close()

	// 调用封装函数
	if err := processLocations(ctx, dbpool, ak, limiter); err != nil {
		log.Fatal("处理过程出错:", err)
	}

	// // 调用百度API
	// geoResp, err := queryBaiduAPI(address, city, ak)
	// if err != nil {
	// 	log.Printf("地址解析失败: %v\n", err)
	// 	// continue
	// }
	// fmt.Printf("🎯 解析精度: precise = %d, confidence = %d, comprehension = %d, level = %s\n",
	// 	geoResp.Result.Precise,
	// 	geoResp.Result.Confidence,
	// 	geoResp.Result.Comprehension,
	// 	geoResp.Result.Level)

	// bdLng := geoResp.Result.Location.Lng
	// bdLat := geoResp.Result.Location.Lat

	// // 转换坐标
	// gcjLng, gcjLat := bd09ToGcj02(bdLng, bdLat)
	// wgsLng, wgsLat := gcj02ToWgs84(gcjLng, gcjLat)

	// fmt.Printf("📍 原始 BD-09 坐标：经度 %.8f, 纬度 %.8f\n", bdLng, bdLat)
	// fmt.Printf("🌏 转换后 WGS-84 坐标：经度 %.8f, 纬度 %.8f\n", wgsLng, wgsLat)
}
