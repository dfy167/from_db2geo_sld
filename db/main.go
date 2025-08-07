package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type DBConfig struct {
	PGHOST           string
	PGPORT           string
	FileName         string
	PGDATABASE       string
	PGUSER           string
	PGPASSWORD       string
	PGCLIENTENCODING string
}

func main() {
	input := `
172.25.94.251	20221	p_bj	sxc4bj	postgres	fast*123	UTF8
172.25.94.251	20220	p_sh	sxc4sh	postgres	fast*123	UTF8
172.25.94.251	20222	p_gd	sxc4gd	postgres	fast*123	UTF8
172.25.94.251	20227	p_cq	sxc4cq	postgres	fast*123	UTF8
172.25.94.251	20223	p_hn	sxc4hn	postgres	fast*123	UTF8
172.25.94.251	20224	p_qh	sxc4qh	postgres	fast*123	UTF8
172.25.94.251	20225	p_ah	sxc4ah	postgres	fast*123	UTF8
172.25.94.251	20226	p_fj	sxc4fj	postgres	fast*123	UTF8
172.25.94.251	20228	p_gs	sxc4gs	postgres	fast*123	UTF8
172.25.94.251	20229	p_gx	sxc4gx	postgres	fast*123	UTF8
172.25.94.251	20230	p_hi	sxc4hi	postgres	fast*123	UTF8
172.25.94.251	20231	p_jx	sxc4jx	postgres	fast*123	UTF8
172.25.94.251	20219	p_tj	sxc4tj	postgres	fast*123	UTF8
172.25.94.251	20232	p_jl	sxc4jl	postgres	fast*123	UTF8
172.25.94.251	20233	p_hl	sxc4hl	postgres	fast*123	UTF8
172.25.94.251	20234	p_ln	sxc4ln	postgres	fast*123	UTF8
172.25.94.251	20235	p_hb	sxc4hb	postgres	fast*123	UTF8
172.25.94.251	20236	p_xj	sxc4xj	postgres	fast*123	UTF8
172.25.94.251	20237	p_nx	sxc4nx	postgres	fast*123	UTF8
172.25.94.251	20238	p_yn	sxc4yn	postgres	fast*123	UTF8
172.25.94.251	20239	p_zj	sxc4zj	postgres	fast*123	UTF8
172.25.94.251	20240	p_sd	sxc4sd	postgres	fast*123	UTF8
172.25.94.251	20241	p_xz	sxc4xz	postgres	fast*123	UTF8
172.25.94.251	20218	p_nm	sxc4nm	postgres	fast*123	UTF8
172.25.94.251	20242	p_ha	sxc4ha	postgres	fast*123	UTF8
172.25.94.251	20243	p_sc	sxc4sc	postgres	fast*123	UTF8
172.25.94.251	20244	p_he	sxc4he	postgres	fast*123	UTF8
172.25.94.251	20245	p_sx	sxc4sx	postgres	fast*123	UTF8
172.25.94.251	20246	p_sn	sxc4sn	postgres	fast*123	UTF8
172.25.94.251	20247	p_js	sxc4js	postgres	fast*123	UTF8
172.25.94.251	20248	p_gz	sxc4gz	postgres	fast*123	UTF8
`

	scanner := bufio.NewScanner(strings.NewReader(strings.TrimSpace(input)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			fmt.Println("字段不足7列，跳过:", line)
			continue
		}

		config := DBConfig{
			PGHOST:           fields[0],
			PGPORT:           fields[1],
			FileName:         fields[2],
			PGDATABASE:       fields[3],
			PGUSER:           fields[4],
			PGPASSWORD:       fields[5],
			PGCLIENTENCODING: fields[6],
		}

		err := writeShellFile(config)
		if err != nil {
			fmt.Println("写入文件失败：", err)
		}
	}
}

func writeShellFile(cfg DBConfig) error {
	filename := cfg.FileName + ".sh"
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	lines := []string{
		fmt.Sprintf("export PGHOST=%s", cfg.PGHOST),
		fmt.Sprintf("export PGPORT=%s", cfg.PGPORT),
		fmt.Sprintf("export PGDATABASE=%s", cfg.PGDATABASE),
		fmt.Sprintf("export PGUSER=%s", cfg.PGUSER),
		fmt.Sprintf("export PGPASSWORD=%s", cfg.PGPASSWORD),
		fmt.Sprintf("export PGCLIENTENCODING=%s", cfg.PGCLIENTENCODING),
		"",
		"# 第一步：执行 SQL",
		`psql -c "SELECT fast.gt_config('env','mode') ` +
			`UNION ALL ` +
			`(SELECT province FROM app_vdt_dbrd_issues_cluster_controlsheet_4g LIMIT 1);"`,
		`echo -e "\n✅ 环境如上，接下来进入 psql 交互模式..."`,
		"",
		"# 第二步：进入 psql 交互",
		"psql",
	}

	for _, line := range lines {
		_, _ = w.WriteString(line + "\n")
	}

	return w.Flush()
}
