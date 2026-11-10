FunkWechat v3
=============

强类型、无第三方依赖的微信服务端 SDK，支持小程序和公众号。需要 Go 1.27。

```bash
go get github.com/ShinyNito/FunkWechat/v3
```

特性
----
- 泛型终结方法：`c.Request(path).Body(...).Post[T](ctx)`，所有请求共用一条执行路径。
- 稳定版 access_token：使用 `/cgi-bin/stable_token` 普通模式获取。与其他服务、其他实例并存时不会互相作废 token，多实例部署无需额外处理。
- 凭证缓存：access_token 和 ticket 优先读缓存，同一进程内的并发获取合并为一次。获取在后台进行，单个调用方取消不会影响其他等待者。
- token 失效自动重试：遇到 40001/42001 时刷新 token 并重试一次，上传请求也会重试。
- 错误带接口路径：`*core.WechatError` 包含 `Path`、`ErrCode`、`ErrMsg`。
- 日志和错误不泄露凭证：每次调用输出一行 debug 日志，带 appid、路径、状态码和耗时；请求体、响应体和网络错误中的 secret、access_token、session_key 等字段都会脱敏。

小程序
----
```go
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/ShinyNito/FunkWechat/v3/core"
	"github.com/ShinyNito/FunkWechat/v3/miniprogram"
)

func main() {
	ctx := context.Background()
	client, err := miniprogram.New(core.AppConfig{
		AppID:     "your-appid",
		AppSecret: "your-secret",
	})
	if err != nil {
		panic(err)
	}

	session, err := client.Code2Session(ctx, miniprogram.Code2SessionRequest{JSCode: "login-code"})
	if err != nil {
		panic(err)
	}

	err = client.SendSubscribeMessage(ctx, miniprogram.SubscribeMessageRequest{
		ToUser:     session.OpenID,
		TemplateID: "your-template-id",
		Page:       "/pages/index/index",
		Data: map[string]miniprogram.SubscribeMessageValue{
			"thing1": {Value: "订单已发货"},
		},
		MiniProgramState: miniprogram.MiniProgramStateFormal,
	})
	if wechatErr, ok := errors.AsType[*core.WechatError](err); ok && wechatErr.ErrCode == core.ErrCodeUserRefused {
		fmt.Println("用户未订阅或订阅次数已用完")
	} else if err != nil {
		panic(err)
	}
}
```

调用未封装的接口
----
`Request` 默认附加 access_token。`Get[T]`、`Post[T]` 解码 JSON 响应；`GetRaw`、`PostRaw` 返回原始字节，用于小程序码这类成功时返回图片、失败时返回 JSON 的接口。errcode 非 0 时，所有终结方法都返回 `*core.WechatError`。

```go
package main

import (
	"context"
	"os"

	"github.com/ShinyNito/FunkWechat/v3/miniprogram"
)

type URLLinkResponse struct {
	URLLink string `json:"url_link"`
}

func Example(ctx context.Context, client *miniprogram.Client) error {
	link, err := client.Request("/wxa/generate_urllink").
		Body(map[string]any{"path": "/pages/index/index"}).
		Post[URLLinkResponse](ctx)
	if err != nil {
		return err
	}
	_ = link.URLLink

	code, err := client.Request("/wxa/getwxacodeunlimit").
		Body(map[string]any{"scene": "id=1"}).
		PostRaw(ctx)
	if err != nil {
		return err
	}
	return os.WriteFile("code.png", code.Body, 0o644)
}
```

上传素材时用 `File` 和 `Field` 构造 multipart 请求，不能和 `Body` 同时使用：

```go
client.Request("/cgi-bin/media/upload").
	Query("type", "image").
	File("media", "a.jpg", content).
	Post[UploadResponse](ctx)
```

共享缓存
----
默认把凭证缓存在进程内存里。多个进程想共享同一份 access_token 时，可以实现 `core.Cache` 接口，例如基于 Redis：

```go
type Cache interface {
	Get(ctx context.Context, key string) (string, bool)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}
```

缓存键为 `funkwechat:access_token:<appid>` 和 `funkwechat:ticket:<appid>:<type>`。

公众号
----
```go
package main

import (
	"context"
	"fmt"

	"github.com/ShinyNito/FunkWechat/v3/core"
	"github.com/ShinyNito/FunkWechat/v3/officialaccount"
)

func main() {
	ctx := context.Background()
	client, err := officialaccount.New(core.AppConfig{
		AppID:     "your-appid",
		AppSecret: "your-secret",
	})
	if err != nil {
		panic(err)
	}

	sign, err := client.GetJssdkSign(ctx, officialaccount.JssdkSignRequest{
		URL: "https://example.com/page#hash",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(sign.Signature)
}
```

解密小程序用户数据
----
前端传来 `encryptedData` 和 `iv` 时，用 `wxcrypto.DecryptUserData` 解密。获取手机号优先使用 `GetPhoneNumber`。

```go
package main

import (
	"context"
	"fmt"

	"github.com/ShinyNito/FunkWechat/v3/miniprogram"
	"github.com/ShinyNito/FunkWechat/v3/wxcrypto"
)

type UserInfo struct {
	NickName  string `json:"nickName"`
	Watermark struct {
		AppID string `json:"appid"`
	} `json:"watermark"`
}

func DecryptUserInfo(ctx context.Context, client *miniprogram.Client, jsCode, encryptedData, iv string) (UserInfo, error) {
	session, err := client.Code2Session(ctx, miniprogram.Code2SessionRequest{JSCode: jsCode})
	if err != nil {
		return UserInfo{}, err
	}
	info, err := wxcrypto.DecryptUserData[UserInfo](session.SessionKey, encryptedData, iv)
	if err != nil {
		return UserInfo{}, err
	}
	// 校验 watermark.appid，防止使用其他应用的数据
	if info.Watermark.AppID != client.Config().AppID {
		return UserInfo{}, fmt.Errorf("invalid watermark appid: %s", info.Watermark.AppID)
	}
	return info, nil
}
```

验证公众号消息推送的签名
----
服务器配置校验和明文模式推送用 `wxcrypto.VerifySignature`，安全模式推送用 `wxcrypto.VerifyMsgSignature`。

```go
package main

import (
	"net/http"

	"github.com/ShinyNito/FunkWechat/v3/wxcrypto"
)

const verifyToken = "your-wechat-token"

// GET /wechat/callback?signature=...&timestamp=...&nonce=...&echostr=...
func HandleWechatVerify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if !wxcrypto.VerifySignature(q.Get("signature"), q.Get("timestamp"), q.Get("nonce"), verifyToken) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	_, _ = w.Write([]byte(q.Get("echostr")))
}
```

许可证
------
MIT
