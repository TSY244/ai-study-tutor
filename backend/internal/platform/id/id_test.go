package id

import "testing"

func TestSanitizeRequestID(t *testing.T) {
	gen := &Fixed{Values: []string{"fresh-id"}}
	if got := SanitizeRequestID("client-id-123", gen); got != "client-id-123" {
		t.Fatalf("合法值应保留：%q", got)
	}
	for _, bad := range []string{"", "with space", "with\nnewline", "中文字符"} {
		if got := SanitizeRequestID(bad, gen); got != "fresh-id" {
			t.Fatalf("非法值 %q 应被替换：%q", bad, got)
		}
	}
	long := make([]byte, MaxRequestIDLength+1)
	for i := range long {
		long[i] = 'a'
	}
	if got := SanitizeRequestID(string(long), gen); got != "fresh-id" {
		t.Fatal("超长值应被替换")
	}
}

func TestRandomGeneratorProducesUniqueUUIDs(t *testing.T) {
	gen := Random{}
	a, b := gen.New(), gen.New()
	if a == b || len(a) != 36 {
		t.Fatalf("UUID 生成异常：%q %q", a, b)
	}
}
