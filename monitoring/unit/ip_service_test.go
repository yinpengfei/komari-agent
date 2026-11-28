package monitoring

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestIPLookupRetryAndFallback(t *testing.T) {
	for _, tc := range []struct {
		name      string
		failures  int
		fallback  bool
		ipv4      bool
		wantCalls int
		wantIP    string
	}{
		{"first attempt succeeds", 0, true, true, 1, "8.8.8.8"},
		{"retry succeeds", 1, true, true, 2, "8.8.8.8"},
		{"IPv4 fallback succeeds", 2, true, true, 3, "8.8.8.8"},
		{"IPv6 fallback succeeds", 2, true, false, 3, "2603:c024::99"},
		{"fallback disabled", 2, false, true, 2, ""},
		{"all attempts fail", 3, true, true, 3, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: ipLookupTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				wantHost := "self.example"
				if calls == 3 {
					wantHost = "api6.ipify.org"
					if tc.ipv4 {
						wantHost = "api.ipify.org"
					}
				}
				if r.URL.Host != wantHost || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Body != nil {
					t.Fatalf("unexpected request %s", r.URL.Host)
				}
				if calls <= tc.failures {
					return nil, errors.New("unavailable")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.wantIP))}, nil
			})}
			got, err := queryPublicIPWithFallback(client, "https://self.example/ip", tc.ipv4, tc.fallback)
			if got != tc.wantIP || calls != tc.wantCalls || (err != nil) != (tc.wantIP == "") {
				t.Fatalf("got %q, %d calls, error %v", got, calls, err)
			}
		})
	}
	client := &http.Client{Transport: rejectIPLookup{t}}
	if _, err := queryPublicIPWithFallback(client, "http://invalid.example", true, true); err == nil {
		t.Fatal("invalid configured URL must not trigger fallback")
	}
}

func TestCommonIPServiceFormats(t *testing.T) {
	for _, body := range []string{"8.8.8.8\n", `"8.8.8.8"`, `{"ip":"8.8.8.8"}`, `{"IP":"8.8.8.8"}`, `{"query":"8.8.8.8"}`, `{"origin":"8.8.8.8"}`, `{"ip_addr":"8.8.8.8"}`, `{"data":{"ip":"8.8.8.8"}}`, `{"data":"8.8.8.8"}`, `{"ip":"8.8.8.8","ip_version":4,"location":{"city":"test"}}`} {
		client := &http.Client{Transport: ipLookupTransport(func(r *http.Request) (*http.Response, error) {
			if r.Body != nil || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
				t.Fatal("unexpected outbound data")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		got, err := queryPublicIP(client, "https://ipv4.yin.cc/ip", true)
		if err != nil || got != "8.8.8.8" {
			t.Errorf("%s: %q %v", body, got, err)
		}
	}
}

func TestIPServiceRejectsUnsafeBodies(t *testing.T) {
	for _, body := range []string{`{"ip":"10.0.0.1"}`, `{"ip":"100.64.1.1"}`, `{"ip":"127.0.0.1"}`, `{"ip":"8.8.8.8","ip_version":6}`, `{"ip":"8.8.8.8","origin":"1.1.1.1"}`, `{"data":{"ip":"8.8.8.8","origin":"1.1.1.1"}}`, `{"ip":"8.8.8.8","data":{"ip":"8.8.8.8","origin":"1.1.1.1"}}`, `{"origin":"8.8.8.8, 1.1.1.1"}`, `{"ip":"8.8.8.8"} {}`, `<html>8.8.8.8</html>`, strings.Repeat(" ", 4097) + "8.8.8.8"} {
		client := &http.Client{Transport: ipLookupTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		if got, err := queryPublicIP(client, "https://ipv4.yin.cc/ip", true); err == nil || got != "" {
			t.Errorf("accepted unsafe response: %.90s", body)
		}
	}
}

func TestIPServiceURLPolicy(t *testing.T) {
	for _, url := range []string{"http://ip.yin.cc/ip", "https://user:secret@ip.yin.cc/ip", "https://ip.yin.cc/ip#token", "file:///tmp/ip", "https:///ip"} {
		if validateIPLookupURL(url) == nil {
			t.Errorf("accepted %s", url)
		}
	}
	for _, url := range []string{DefaultIPv4LookupURL, DefaultIPv6LookupURL, "http://127.0.0.1:8081/ip", "http://[::1]:8081/ip"} {
		if err := validateIPLookupURL(url); err != nil {
			t.Errorf("%s: %v", url, err)
		}
	}
	for _, client := range []*http.Client{ipv4HTTPClient, ipv6HTTPClient} {
		if client.Transport.(*http.Transport).Proxy != nil {
			t.Fatal("IP discovery must not use a proxy egress")
		}
		if client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
			t.Fatal("IP discovery must not follow redirects")
		}
	}
}
