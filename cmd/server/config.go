package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
)

type config struct {
	address   string
	database  string
	selfcheck bool
}

func parseConfig() (config, error) {
	defaultAddress := "127.0.0.1:19081"
	if value := os.Getenv("PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return config{}, fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
		}
		defaultAddress = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	var result config
	flag.StringVar(&result.address, "addr", defaultAddress, "HTTP 监听地址")
	flag.StringVar(&result.database, "db", "stage-rigging-clearance.db", "SQLite 数据库路径")
	flag.BoolVar(&result.selfcheck, "selfcheck", false, "运行真实 HTTP 闭环自检后退出")
	flag.Parse()
	if _, _, err := net.SplitHostPort(result.address); err != nil {
		return config{}, fmt.Errorf("-addr 必须是 host:port: %w", err)
	}
	if result.selfcheck {
		host, _, _ := net.SplitHostPort(result.address)
		if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
			return config{}, fmt.Errorf("selfcheck 只允许使用回环监听地址")
		}
	}
	return result, nil
}
