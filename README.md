FunkWechat v2
=============

强类型、链式、无兼容层的微信 SDK。

> v2 是一次性重构版本，不兼容 v1 API。

特性
----
- 强类型链式请求：`Request[T](client).Path(...).Post(ctx)`
- 统一核心内核：请求执行、上传、微信错误解码、token 管理
- Token 刷新去重：并发场景下单飞（singleflight-style）
- 默认支持 query 脱敏日志

安装
----
```bash
go get github.com/ShinyNito/FunkWechat/v2
```

小程序示例
----
```go
package main

import (
	"context"
	"fmt"

	"github.com/ShinyNito/FunkWechat/v2/miniprogram"
)

func main() {
	client, err := miniprogram.New(miniprogram.Config{
		AppID:     "your-appid",
		AppSecret: "your-secret",
	})
	if err != nil {
		panic(err)
	}

	session, err := client.Code2Session(context.Background(), miniprogram.Code2SessionRequest{
		JSCode: "login-code",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(session.OpenID)

	type SendResp struct {
		MsgID int `json:"msgid"`
	}
	sendResp, err := miniprogram.Request[SendResp](client).
		Path("/cgi-bin/message/send").
		Body(map[string]any{"touser": "openid"}).
		Post(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println(sendResp.MsgID)
}
```

公众号示例
----
```go
package main

import (
	"context"
	"fmt"

		"github.com/ShinyNito/FunkWechat/v2/officialaccount"
)

func main() {
	client, err := officialaccount.New(officialaccount.Config{
		AppID:     "your-appid",
		AppSecret: "your-secret",
	})
	if err != nil {
		panic(err)
	}

	ticket, err := client.GetTicket(context.Background(), officialaccount.GetTicketRequest{})
	if err != nil {
		panic(err)
	}
	fmt.Println(ticket.Ticket)

	sign, err := client.GetJssdkSign(context.Background(), officialaccount.JssdkSignRequest{
		URL: "https://example.com/page#hash",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(sign.Signature)
}
```

许可证
------
MIT
