// 密钥存取：展示清单由平台组件声明，页面只能看到脱敏值；
// tmdb/bangumi provider 读同一份 credentials.json。

package credentials

import (
	"context"
	"errors"
	"sort"
	"strings"

	"mygo-host/platform"
)

type KeyStatus struct {
	Name   string `json:"name"`
	Label  string `json:"label"`
	Set    bool   `json:"set"`
	Masked string `json:"masked,omitempty"`
}

type SetKeyParams struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Keys 返回密钥的脱敏状态(永不回传明文)。
func (s *Service) Keys(context.Context) ([]KeyStatus, error) {
	specs := append([]platform.KeySpec(nil), s.keys...)
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	out := make([]KeyStatus, 0, len(specs))
	for _, spec := range specs {
		v, ok := s.creds.Lookup(spec.Name)
		st := KeyStatus{Name: spec.Name, Label: spec.Label, Set: ok && v != ""}
		if st.Set {
			st.Masked = maskKey(v)
		}
		out = append(out, st)
	}
	return out, nil
}

// SetKey 保存/清除密钥;空串等价于清除。
func (s *Service) SetKey(_ context.Context, p SetKeyParams) error {
	known := false
	for _, spec := range s.keys {
		if spec.Name == p.Name {
			known = true
			break
		}
	}
	if !known {
		return errors.New("未知密钥: " + p.Name)
	}
	v := strings.TrimSpace(p.Value)
	if len(v) > 1024 {
		return errors.New("密钥过长(上限 1024 字符)")
	}
	return s.creds.Set(p.Name, v)
}

func maskKey(v string) string {
	if len(v) <= 4 {
		return "••••"
	}
	return "••••" + v[len(v)-4:]
}
