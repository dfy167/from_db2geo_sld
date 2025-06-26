# from_db2geo_sld

✅ 使用示例
输入命令 效果
go run main.go GIS_test --overwrite ✅ 正常覆盖
go run main.go --overwrite GIS_test ✅ 顺序无关
go run main.go --regex ^test_.* --overwrite ✅ 支持正则和覆盖组合
