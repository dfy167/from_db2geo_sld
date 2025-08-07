export PGHOST=172.25.94.251
export PGPORT=20234
export PGDATABASE=sxc4ln
export PGUSER=postgres
export PGPASSWORD=fast*123
export PGCLIENTENCODING=UTF8

# 第一步：执行 SQL
psql -c "SELECT fast.gt_config('env','mode') UNION ALL (SELECT province FROM app_vdt_dbrd_issues_cluster_controlsheet_4g LIMIT 1);"
echo -e "\n✅ 环境如上，接下来进入 psql 交互模式..."

# 第二步：进入 psql 交互
psql
