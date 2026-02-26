package officialaccount

import "github.com/ShinyNito/FunkWechat/core"

type TypedRequest[T any] = core.TypedRequest[T]

func Request[T any](c *Client) *TypedRequest[T] {
	return core.NewTypedRequest[T](c.apiClient)
}
