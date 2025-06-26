package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	_ "github.com/lib/pq"
)

const (
	// PostgreSQL 配置
	dbHost     = "192.168.3.80"
	dbPort     = 15432
	dbUser     = "postgres"
	dbPassword = "fast*123"
	dbName     = "icango_gis"

	// GeoServer 配置
	geoserverURL  = "http://192.168.3.80:9096/geoserver"
	geoserverUser = "admin"
	geoserverPass = "Fastgis*123"
	workspace     = "FAST"
)

func main() {

	// 手动解析命令行参数，支持参数顺序自由
	var (
		overwrite bool
		names     []string
		regexMode bool
	)

	// 简易命令行参数解析器
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--overwrite":
			overwrite = true
		case "--regex":
			regexMode = true
		default:
			names = append(names, arg)
		}
	}

	// 显示参数解析结果
	fmt.Println("✔️ 正则模式：", regexMode)
	fmt.Println("✔️ 覆盖模式：", overwrite)
	fmt.Println("✔️ 样式匹配列表：", names)

	// 正则匹配模式
	var filterRegex *regexp.Regexp
	var nameSet map[string]bool

	if regexMode {
		if len(names) == 0 {
			fmt.Println("❌ 启用了 --regex 但未提供正则表达式")
			return
		}
		var err error
		filterRegex, err = regexp.Compile(names[0])
		checkErr(err)
	} else if len(names) > 0 {
		nameSet = make(map[string]bool)
		for _, name := range names {
			nameSet[name] = true
		}
	}
	args := flag.Args()

	if regexMode {
		if len(args) == 0 {
			fmt.Println("⚠️ 启用了 --regex 但未提供表达式")
			return
		}
		var err error
		filterRegex, err = regexp.Compile(args[0])
		checkErr(err)
		fmt.Println("🔍 正则匹配样式名：", filterRegex.String())
	} else if len(args) > 0 {
		nameSet = make(map[string]bool)
		for _, name := range args {
			nameSet[name] = true
		}
		fmt.Println("🔍 精确匹配样式名：", args)
	}

	// 连接数据库
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	db, err := sql.Open("postgres", psqlInfo)
	checkErr(err)
	defer db.Close()

	rows, err := db.Query("SELECT sld_name as name,sld_content as content FROM doforgis.p_get_geo_sld()")
	checkErr(err)
	defer rows.Close()

	found := false

	for rows.Next() {
		var name, content string
		checkErr(rows.Scan(&name, &content))

		// 过滤判断
		if filterRegex != nil && !filterRegex.MatchString(name) {
			continue
		}
		if nameSet != nil && !nameSet[name] {
			continue
		}

		found = true
		exists := styleExists(name)
		content = strings.TrimSpace(content)
		var err error
		if exists && overwrite {
			err = uploadSLD(name, content, "PUT")
		} else if !exists {
			err = uploadSLD(name, content, "POST")
		} else {
			fmt.Printf("⚠️ 样式已存在，跳过：%s\n", name)
			continue
		}

		if err != nil {
			fmt.Printf("❌ 上传失败：%s (%v)\n", name, err)
		} else {
			action := "上传"
			if exists {
				action = "覆盖"
			}
			fmt.Printf("✅ %s成功：%s\n", action, name)
		}
	}

	if !found {
		fmt.Println("⚠️ 没有匹配任何样式")
	}
}

func styleExists(name string) bool {
	url := fmt.Sprintf("%s/rest/workspaces/%s/styles/%s", geoserverURL, workspace, name)

	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(geoserverUser, geoserverPass)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func uploadSLD(name, content, method string) error {
	var url string
	if method == "PUT" {
		url = fmt.Sprintf("%s/rest/workspaces/%s/styles/%s", geoserverURL, workspace, name)
	} else {
		url = fmt.Sprintf("%s/rest/workspaces/%s/styles?name=%s", geoserverURL, workspace, name)
	}

	req, err := http.NewRequest(method, url, strings.NewReader(content))
	if err != nil {
		return err
	}

	req.SetBasicAuth(geoserverUser, geoserverPass)
	req.Header.Set("Content-Type", "application/vnd.ogc.sld+xml")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("GeoServer 错误 %d: %s", resp.StatusCode, body)
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
