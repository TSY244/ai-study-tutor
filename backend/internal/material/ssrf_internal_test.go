package material

import (
	"context"
	"net"
	"testing"
)

func TestValidateSourceURLCanDeferDNSValidationToWorker(t *testing.T) {
	svc := &Service{
		urlAllowlist:         []string{"bilibili.com"},
		SkipURLDNSValidation: true,
	}
	svc.HostResolver = func(context.Context, string) ([]net.IP, error) {
		t.Fatal("API resolver must not run when DNS validation is deferred to Worker")
		return nil, nil
	}

	got, problem := svc.validateSourceURL(context.Background(),
		"HTTPS://WWW.BILIBILI.COM/video/BV16C7N6FEtQ/#fragment")
	if problem != nil {
		t.Fatalf("expected URL to pass API validation: %+v", problem.Problem)
	}
	if want := "https://www.bilibili.com/video/BV16C7N6FEtQ/"; got != want {
		t.Fatalf("normalized URL = %q, want %q", got, want)
	}
}
