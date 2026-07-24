// Package apitest 提供 B3+ HTTP 测试的契约校验：以发布契约
// api/openapi.yaml 为准校验响应体结构（required、类型、枚举、
// additionalProperties:false、format/pattern），确保实现与契约不漂移。
// 仅用于 *_test.go。
package apitest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/goccy/go-yaml"
)

// Contract 是解析后的发布 OpenAPI 契约。
type Contract struct {
	spec map[string]any
}

var (
	loadOnce sync.Once
	loaded   *Contract
	loadErr  error
)

// Load 读取并缓存 api/openapi.yaml（仓库根目录由本文件位置推导）。
func Load(t *testing.T) *Contract {
	t.Helper()
	loadOnce.Do(func() {
		_, file, _, _ := runtime.Caller(0)
		root := filepath.Join(filepath.Dir(file), "..", "..", "..")
		raw, err := os.ReadFile(filepath.Join(root, "api", "openapi.yaml"))
		if err != nil {
			loadErr = err
			return
		}
		var spec map[string]any
		if err := yaml.Unmarshal(raw, &spec); err != nil {
			loadErr = err
			return
		}
		loaded = &Contract{spec: spec}
	})
	if loadErr != nil {
		t.Fatalf("加载发布契约失败: %v", loadErr)
	}
	return loaded
}

// ValidateJSONResponse 校验 (method, pathTemplate, status) 的 JSON 响应体
// 符合契约 Schema。pathTemplate 形如 "/api/v1/courses/{course_id}"。
func (c *Contract) ValidateJSONResponse(t *testing.T, method, pathTemplate string, status int, body []byte) {
	t.Helper()
	schema := c.responseSchema(t, method, pathTemplate, status, "application/json")
	if schema == nil {
		return
	}
	var got any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if problems := c.check(schema, got, "$"); len(problems) > 0 {
		t.Fatalf("响应违反契约 %s %s [%d]:\n%s", method, pathTemplate, status, strings.Join(problems, "\n"))
	}
}

// ValidateJSONSchema 校验任意 JSON 文档符合本地组件 Schema 引用。
// schemaRef 形如 "#/components/schemas/PersistedJobStreamEvent"。
func (c *Contract) ValidateJSONSchema(t *testing.T, schemaRef string, body []byte) {
	t.Helper()
	schema := c.resolve(map[string]any{"$ref": schemaRef})
	if schema == nil {
		t.Fatalf("契约中不存在 Schema %s", schemaRef)
	}
	var got any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("示例不是合法 JSON: %v", err)
	}
	if problems := c.check(schema, got, "$"); len(problems) > 0 {
		t.Fatalf("JSON 违反契约 %s:\n%s", schemaRef, strings.Join(problems, "\n"))
	}
}

// ValidateProblem 校验 Problem 响应满足契约 Problem Schema 的最小不变量。
func (c *Contract) ValidateProblem(t *testing.T, body []byte, wantStatus int, wantCode string) {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("Problem 不是合法 JSON: %v", err)
	}
	for _, field := range []string{"type", "title", "status", "detail", "code", "request_id"} {
		if _, ok := p[field]; !ok {
			t.Fatalf("Problem 缺少字段 %s: %v", field, p)
		}
	}
	if int(p["status"].(float64)) != wantStatus {
		t.Fatalf("Problem status = %v，期望 %d", p["status"], wantStatus)
	}
	if p["code"] != wantCode {
		t.Fatalf("Problem code = %v，期望 %s", p["code"], wantCode)
	}
	if rid, _ := p["request_id"].(string); rid == "" {
		t.Fatal("Problem.request_id 不能为空")
	}
}

// responseSchema 定位响应 Schema；找不到时返回 nil（调用方决定跳过）。
func (c *Contract) responseSchema(t *testing.T, method, pathTemplate string, status int, contentType string) map[string]any {
	t.Helper()
	paths, _ := c.spec["paths"].(map[string]any)
	item, _ := paths[pathTemplate].(map[string]any)
	if item == nil {
		t.Fatalf("契约中不存在路径 %s", pathTemplate)
	}
	op, _ := item[strings.ToLower(method)].(map[string]any)
	if op == nil {
		t.Fatalf("契约中不存在操作 %s %s", method, pathTemplate)
	}
	resps, _ := op["responses"].(map[string]any)
	resp, _ := resps[fmt.Sprintf("%d", status)].(map[string]any)
	if resp == nil {
		t.Fatalf("契约中 %s %s 没有 %d 响应", method, pathTemplate, status)
	}
	resp = c.resolve(resp)
	content, _ := resp["content"].(map[string]any)
	mt, _ := content[contentType].(map[string]any)
	if mt == nil {
		return nil // 无 JSON  body（如 204 / octet-stream）
	}
	schema, _ := mt["schema"].(map[string]any)
	return schema
}

// resolve 展开 $ref（仅限本文件内部引用）。
func (c *Contract) resolve(node map[string]any) map[string]any {
	for i := 0; i < 16; i++ {
		ref, ok := node["$ref"].(string)
		if !ok {
			return node
		}
		parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
		var cur any = c.spec
		for _, p := range parts {
			m, _ := cur.(map[string]any)
			cur = m[p]
		}
		next, _ := cur.(map[string]any)
		if next == nil {
			return node
		}
		node = next
	}
	return node
}

// check 递归校验 JSON 值，返回问题列表（空 = 通过）。
func (c *Contract) check(schema map[string]any, value any, path string) []string {
	schema = c.resolve(schema)
	if schema == nil {
		return nil
	}
	// anyOf/oneOf：任一分支通过即可。
	for _, comb := range []string{"anyOf", "oneOf"} {
		if branches, ok := schema[comb].([]any); ok {
			var all []string
			for _, b := range branches {
				bs, _ := b.(map[string]any)
				probs := c.check(bs, value, path)
				if len(probs) == 0 {
					return nil
				}
				all = append(all, probs...)
			}
			return []string{fmt.Sprintf("%s: 不匹配任何 %s 分支", path, comb)}
		}
	}

	types := schemaTypes(schema["type"])
	if len(types) > 0 {
		matched := false
		var typeProblems []string
		for _, ty := range types {
			if p := c.checkType(ty, schema, value, path); p == nil {
				matched = true
				break
			} else {
				typeProblems = append(typeProblems, p...)
			}
		}
		if !matched {
			return []string{fmt.Sprintf("%s: 类型不匹配（期望 %v，实际 %T %v）", path, types, value, value)}
		}
		return nil
	}
	return nil
}

// checkType 校验单类型；返回 nil 表示该类型通过（含子结构校验）。
func (c *Contract) checkType(ty string, schema map[string]any, value any, path string) []string {
	switch ty {
	case "null":
		if value != nil {
			return []string{fmt.Sprintf("%s: 期望 null", path)}
		}
		return nil
	case "string":
		s, ok := value.(string)
		if !ok {
			return []string{fmt.Sprintf("%s: 期望 string", path)}
		}
		return c.checkStringConstraints(schema, s, path)
	case "integer":
		f, ok := value.(float64)
		if !ok || f != float64(int64(f)) {
			return []string{fmt.Sprintf("%s: 期望 integer，实际 %v", path, value)}
		}
		return checkNumberConstraints(schema, f, path)
	case "number":
		f, ok := value.(float64)
		if !ok {
			return []string{fmt.Sprintf("%s: 期望 number", path)}
		}
		return checkNumberConstraints(schema, f, path)
	case "boolean":
		if _, ok := value.(bool); !ok {
			return []string{fmt.Sprintf("%s: 期望 boolean", path)}
		}
		return nil
	case "array":
		arr, ok := value.([]any)
		if !ok {
			return []string{fmt.Sprintf("%s: 期望 array", path)}
		}
		items, _ := schema["items"].(map[string]any)
		var problems []string
		for i, v := range arr {
			problems = append(problems, c.check(items, v, fmt.Sprintf("%s[%d]", path, i))...)
		}
		return problems
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return []string{fmt.Sprintf("%s: 期望 object", path)}
		}
		var problems []string
		required, _ := schema["required"].([]any)
		for _, r := range required {
			name, _ := r.(string)
			if _, present := obj[name]; !present {
				problems = append(problems, fmt.Sprintf("%s: 缺少必需字段 %s", path, name))
			}
		}
		props, _ := schema["properties"].(map[string]any)
		if addl, has := schema["additionalProperties"]; has {
			if addlBool, ok := addl.(bool); ok && !addlBool {
				for k := range obj {
					if _, known := props[k]; !known {
						problems = append(problems, fmt.Sprintf("%s: 未声明的字段 %s", path, k))
					}
				}
			}
		}
		for k, v := range obj {
			if ps, ok := props[k].(map[string]any); ok {
				problems = append(problems, c.check(ps, v, path+"."+k)...)
			}
		}
		return problems
	}
	return nil
}

func schemaTypes(t any) []string {
	switch v := t.(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

var (
	uuidRe     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	dateTimeRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)
)

func (c *Contract) checkStringConstraints(schema map[string]any, s, path string) []string {
	var problems []string
	if enum, ok := schema["enum"].([]any); ok {
		found := false
		for _, e := range enum {
			if es, ok := e.(string); ok && es == s {
				found = true
				break
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("%s: 值 %q 不在枚举内", path, s))
		}
	}
	switch schema["format"] {
	case "uuid":
		if !uuidRe.MatchString(s) {
			problems = append(problems, fmt.Sprintf("%s: 不是 UUID: %q", path, s))
		}
	case "date-time":
		if !dateTimeRe.MatchString(s) {
			problems = append(problems, fmt.Sprintf("%s: 不是 RFC 3339 时间: %q", path, s))
		}
	}
	if pattern, ok := schema["pattern"].(string); ok {
		if re, err := regexp.Compile(pattern); err == nil && !re.MatchString(s) {
			problems = append(problems, fmt.Sprintf("%s: 不匹配 pattern %s: %q", path, pattern, s))
		}
	}
	return problems
}

func checkNumberConstraints(schema map[string]any, f float64, path string) []string {
	var problems []string
	if min, ok := schema["minimum"].(float64); ok && f < min {
		problems = append(problems, fmt.Sprintf("%s: %v 小于 minimum %v", path, f, min))
	}
	if max, ok := schema["maximum"].(float64); ok && f > max {
		problems = append(problems, fmt.Sprintf("%s: %v 大于 maximum %v", path, f, max))
	}
	return problems
}
