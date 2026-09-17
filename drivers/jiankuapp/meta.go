package jiankuapp

import (
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

// Addition 驱动配置项：只有一个「APP树地址」
type Addition struct {
	driver.RootPath
	AppTreeURL string `json:"app_tree_url" type:"string" required:"true" help:"APP树地址，支持本地路径或 HTTP URL"`
}

// 配置声明
var config = driver.Config{
	Name:        "JiankuAPP",
	LocalSort:   false,
	OnlyProxy:   false,
	NoCache:     true, // 关键：禁用目录缓存，每次 List 都重新读取
	NoUpload:    true,
	NeedMs:      false,
	DefaultRoot: "/", // 根目录默认值，修复 GetRooter 报错
}

// 注册驱动
func init() {
	op.RegisterDriver(func() driver.Driver {
		return &JiankuAPP{}
	})
}