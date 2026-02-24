package identity

import (
	"regexp"
	"testing"
)

func TestGenerateFingerprint(t *testing.T) {
	fp, err := GenerateFingerprint()
	if err != nil {
		t.Fatalf("生成指纹失败: %v", err)
	}

	// 指纹应该是 64 字符的十六进制字符串（SHA256）
	if len(fp) != 64 {
		t.Errorf("指纹长度错误: got %d, want 64", len(fp))
	}

	// 验证是有效的十六进制
	matched, _ := regexp.MatchString("^[a-f0-9]{64}$", fp)
	if !matched {
		t.Errorf("指纹格式无效: %s", fp)
	}

	// 多次调用应该返回相同结果
	fp2, _ := GenerateFingerprint()
	if fp != fp2 {
		t.Error("指纹不稳定，多次调用返回不同结果")
	}
}
