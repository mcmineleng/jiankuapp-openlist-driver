package jiankuapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/stream"
)

// JiankuAPP 驱动实例
type JiankuAPP struct {
	model.Storage
	Addition
}

func (d *JiankuAPP) Config() driver.Config {
	return config
}

func (d *JiankuAPP) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *JiankuAPP) Init(ctx context.Context) error {
	return nil
}

func (d *JiankuAPP) Drop(ctx context.Context) error {
	return nil
}

// parseTree 把数组形式的树拍平成 map[AppName]fields
func parseTree(data []byte) (map[string]map[string]interface{}, error) {
	var arr []map[string]map[string]interface{}
	if err := json.Unmarshal(data, &arr); err != nil {
		return nil, err
	}
	root := make(map[string]map[string]interface{}, len(arr))
	for _, item := range arr {
		for appName, fields := range item {
			root[appName] = fields
		}
	}
	return root, nil
}

// List 列目录：每次调用都重新读取 JSON
func (d *JiankuAPP) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	data, err := d.readJSON(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 JSON 失败: %w", err)
	}

	root, err := parseTree(data)
	if err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	dirPath := dir.GetPath()

	// 根目录：返回所有 App 目录
	if dirPath == "" || dirPath == "/" {
		var objs []model.Obj
		for appName := range root {
			objs = append(objs, &model.Object{
				Name:     appName,
				Path:     "/" + appName,
				IsFolder: true,
				Modified: time.Now(),
			})
		}
		return objs, nil
	}

	// 子目录：查找对应 App 的字段
	appName := strings.TrimPrefix(dirPath, "/")
	fields, ok := root[appName]
	if !ok {
		return nil, fmt.Errorf("目录不存在: %s", appName)
	}

	var objs []model.Obj
	for key, val := range fields {
		if key == "README" {
			objs = append(objs, d.newFileObj("README.md", dirPath+"/README.md", val))
			continue
		}
		objs = append(objs, d.newFileObj(key, dirPath+"/"+key, val))
	}
	return objs, nil
}

// Link 提供文件下载：README.md 直接返回内容，其他字段作为 URL 重定向
func (d *JiankuAPP) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	data, err := d.readJSON(ctx)
	if err != nil {
		return nil, err
	}

	root, err := parseTree(data)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(strings.TrimPrefix(file.GetPath(), "/"), "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效文件路径: %s", file.GetPath())
	}

	appName := parts[0]
	fileName := parts[1]
	fields, ok := root[appName]
	if !ok {
		return nil, fmt.Errorf("App 不存在: %s", appName)
	}

	fieldKey := fileName
	if fileName == "README.md" {
		fieldKey = "README"
	}

	val, ok := fields[fieldKey]
	if !ok {
		return nil, fmt.Errorf("字段不存在: %s", fieldKey)
	}

	// README.md 直接返回内容
	if fileName == "README.md" {
		content := ""
		switch v := val.(type) {
		case string:
			content = v
		default:
			b, _ := json.Marshal(v)
			content = string(b)
		}
		return &model.Link{
			RangeReader: stream.GetRangeReaderFromMFile(int64(len(content)), bytes.NewReader([]byte(content))),
		}, nil
	}

	// 其他字段：字段值本身就是 URL，直接重定向
	if s, ok := val.(string); ok {
		return &model.Link{
			URL: s,
		}, nil
	}

	// 兜底：非字符串（理论上不会出现）仍按内容返回
	b, _ := json.Marshal(val)
	return &model.Link{
		RangeReader: stream.GetRangeReaderFromMFile(int64(len(b)), bytes.NewReader(b)),
	}, nil
}

// readJSON 统一读取 JSON 数据（本地或远程）
func (d *JiankuAPP) readJSON(ctx context.Context) ([]byte, error) {
	addr := strings.TrimSpace(d.Addition.AppTreeURL)
	if addr == "" {
		return nil, fmt.Errorf("APP树地址为空")
	}

	if strings.HasPrefix(addr, "//") {
		addr = "https:" + addr
	}

	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	}

	return os.ReadFile(addr)
}

// newFileObj 创建文件 Obj
func (d *JiankuAPP) newFileObj(name, path string, val interface{}) model.Obj {
	content := ""
	switch v := val.(type) {
	case string:
		content = v
	default:
		b, _ := json.Marshal(v)
		content = string(b)
	}
	return &model.Object{
		Name:     name,
		Path:     path,
		Size:     int64(len(content)),
		IsFolder: false,
		Modified: time.Now(),
	}
}