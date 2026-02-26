package miniprogram

import (
	"fmt"
	"strings"
)

// Validate 校验小程序配置。
func (cfg *Config) Validate() error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if strings.TrimSpace(cfg.AppID) == "" {
		return fmt.Errorf("appid is required")
	}
	if strings.TrimSpace(cfg.AppSecret) == "" {
		return fmt.Errorf("appsecret is required")
	}
	return nil
}

func validateDecodeTarget(target any) error {
	if target == nil {
		return fmt.Errorf("result is nil")
	}
	return nil
}
