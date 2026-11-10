package core

// Status 微信响应里的 errcode 和 errmsg，每个响应都带有这两个字段。
//
// 响应类型嵌入 Status 后才能用于 Request.Get 和 Request.Post：
//
//	type UserInfo struct {
//		core.Status
//		Nickname string `json:"nickname"`
//	}
//
// 这样响应体只需解析一遍，errcode 和业务字段同时得到。errcode 非 0 时
// Get、Post 返回 *WechatError，所以拿到的结果里 ErrCode 始终为 0，一般不需要读它。
//
// 嵌入时不要加 json 标签，否则 errcode 不会被识别。
type Status struct {
	ErrCode int    `json:"errcode,omitempty"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

func (s *Status) wechatStatus() *Status { return s }

// Response 约束 Request.Get 和 Request.Post 的类型参数：T 必须是嵌入了 Status 的结构体。
// 调用时只需要写 T，例如 Get[UserInfo](ctx)，PT 由编译器推导。
//
// wechatStatus 是未导出方法，其他包的类型只能通过嵌入 Status 来满足这个约束。
type Response[T any] interface {
	*T
	wechatStatus() *Status
}
