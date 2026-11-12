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
- 凭证缓存：access_token 和 ticket 保存在进程内存里，未过期时不加锁、不访问外部存储。同一进程内的并发获取合并为一次，获取在后台进行，单个调用方取消不会影响其他等待者。
- token 失效自动重试：遇到 40001/40014/42001 时刷新 token 并重试一次，上传请求也会重试。
- 可判断的错误：用 `errors.Is(err, core.ErrUserRefused)` 按 errcode 判断，`core.IsRetryable(ctx, err)` 判断是否值得重试，详见「错误处理」。
- 日志和错误不泄露凭证和个人信息：每次调用输出一行 debug 日志，带 appid、路径、状态码和耗时；请求体、响应体和网络错误中的 secret、access_token、session_key、openid、手机号等字段都会脱敏。

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
	if errors.Is(err, core.ErrUserRefused) {
		fmt.Println("用户未订阅或订阅次数已用完")
	} else if err != nil {
		panic(err)
	}
}
```

请求 JSON 采用 `encoding/json/v2` 默认语义：对象键顺序由编码器决定，nil map 编码为 `{}`，nil slice 编码为 `[]`，`omitempty` 按 JSON 值判断。

调用未封装的接口
----
`Request` 默认附加 access_token。`Get[T]`、`Post[T]` 解码 JSON 响应；`GetRaw`、`PostRaw` 返回原始字节，用于小程序码这类成功时返回图片、失败时返回 JSON 的接口。errcode 非 0 时，所有终结方法都返回 `*core.WechatError`。

响应类型 `T` 是嵌入了 `core.Status` 的结构体。`core.Status` 带有每个微信响应都有的 `errcode` 和 `errmsg`，嵌入后响应体只需解析一遍。没有嵌入的类型无法通过编译；接口没有业务字段时直接用 `Post[core.Status]`。

```go
package main

import (
	"context"
	"os"

	"github.com/ShinyNito/FunkWechat/v3/core"
	"github.com/ShinyNito/FunkWechat/v3/miniprogram"
)

type URLLinkResponse struct {
	core.Status
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

错误处理
----
一次调用可能返回四类错误，都可以用 `errors.Is` / `errors.AsType` 判断：

| 类型 | 含义 | 判断方式 |
|---|---|---|
| `*core.WechatError` | 微信返回了非 0 的 errcode | `errors.Is(err, core.ErrUserRefused)`，或取出 `ErrCode` |
| `*core.HTTPError` | 非 2xx 状态码且响应体里没有 errcode | `errors.AsType[*core.HTTPError](err)` |
| 网络错误 | 连接失败、超时 | `core.IsRetryable(ctx, err)` 或 `errors.Is(err, context.DeadlineExceeded)`，URL 中的凭证已脱敏 |
| `core.ErrInvalidArgument` | 参数不合法，请求没有发出 | `errors.Is(err, core.ErrInvalidArgument)` |

```go
err := client.SendSubscribeMessage(ctx, req)
switch {
case err == nil:
case errors.Is(err, core.ErrUserRefused):
	// 用户未订阅，不需要重试
case core.IsRetryable(ctx, err):
	// 微信系统繁忙（errcode -1）、HTTP 429/5xx 或网络超时，稍后重试
default:
	return err
}
```

预定义的 errcode 见 `core/errors.go`。没有预定义的错误码这样判断：

```go
if wechatErr, ok := errors.AsType[*core.WechatError](err); ok && wechatErr.ErrCode == 61501 {
	// ...
}
```

access_token 失效由 SDK 自动刷新并重试一次，不需要调用方处理；其他错误 SDK 不会自动重试。

共享凭证
----
`Cache` 是可选的。不配置时凭证只保存在进程内存里，进程重启后重新获取；稳定版接口在有效期内返回同一个 token，多个进程各自获取也互不影响。

想让多个进程共享凭证、减少对微信的请求时，可以实现 `core.Cache` 接口，例如基于 Redis：

```go
type Cache interface {
	Get(ctx context.Context, key string) (value string, ok bool, err error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}
```

配置后 SDK 仍然优先读内存，不会每个请求都访问 `Cache`：进程启动和凭证到期时读取，从 `Cache` 读到的值每分钟重新确认一次。`Get` 返回 err 时 SDK 输出一条 Warn 日志并改为向微信获取。

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
