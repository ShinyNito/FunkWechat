FunkWechat
==========

轻量封装的微信相关 SDK，包含核心 HTTP 客户端、AccessToken 管理、小程序能力和常用加解密/签名工具。

功能
----
- 核心客户端：统一的 GET/POST/上传封装，自动携带 access_token
- Token 管理：小程序 access_token 获取与缓存，支持自定义缓存实现
- 工具集：AES-CBC 解密、PKCS7 padding、签名校验等
- 响应封装：统一处理 errcode/errmsg，提供 map/JSON 解析辅助

快速开始
--------
```go
mp, err := miniprogram.New(&miniprogram.Config{
    AppID:     "<your-appid>",
    AppSecret: "<your-secret>",
    Cache:     core.NewMemoryCache(), // 可替换为自定义实现
})
if err != nil {
    // handle invalid config
}

client := mp.GetClient()
resp, err := client.Request().
    Path("/cgi-bin/token").
    Query("grant_type", "client_credential").
    Get(context.Background())
if err != nil {
    // handle error
}
fmt.Println(string(resp))
```

许可证
------
本项目采用 MIT License，详见 `LICENSE`。
