package http_client

import (
	"fmt"
	"time"

	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/util/log"
	"github.com/go-resty/resty/v2"
)

// GetHttpClient 获取请求客户端
func GetHttpClient(proxys ...string) *resty.Client {
	client := resty.New()
	// 优先使用传入的代理，否则使用全局代理
	if len(proxys) > 0 {
		proxy := proxys[0]
		client.SetProxy(proxy)
	} else if config.Proxy != "" {
		client.SetProxy(config.Proxy)
	}
	client.SetTimeout(time.Second * 5)
	return client
}

// TestProxy 测试代理连通性
func TestProxy() {
	if config.Proxy == "" {
		return
	}

	log.Sugar.Infof("[代理测试] 检测代理可用性: %s", config.Proxy)

	client := GetHttpClient()
	client.SetTimeout(time.Second * 60)
	resp, err := client.R().Get("https://httpbin.org/ip")

	if err != nil {
		log.Sugar.Errorf("[代理测试] 代理连接失败: %v", err)
		return
	}

	if resp.StatusCode() != 200 {
		log.Sugar.Errorf("[代理测试] 代理返回异常状态码: %d", resp.StatusCode())
		return
	}

	log.Sugar.Infof("[代理测试] 代理连接成功")
	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("代理返回数据:\n%s\n", string(resp.Body()))
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}
