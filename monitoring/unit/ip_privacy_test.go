package monitoring

import (
	"net/http"
	"testing"
)

type rejectIPLookup struct{ t *testing.T }

func (r rejectIPLookup) RoundTrip(*http.Request) (*http.Response, error) {
	r.t.Fatal("unexpected third-party IP lookup")
	return nil, nil
}

func TestPrivateIPDiscoveryDoesNotContactThirdParties(t *testing.T) {
	saved := *flags
	old4, old6 := ipv4HTTPClient, ipv6HTTPClient
	t.Cleanup(func() { *flags = saved; ipv4HTTPClient, ipv6HTTPClient = old4, old6 })
	ipv4HTTPClient = &http.Client{Transport: rejectIPLookup{t}}
	ipv6HTTPClient = &http.Client{Transport: rejectIPLookup{t}}
	flags.AllowExternalIPLookup = false
	flags.GetIpAddrFromNic = false
	for _, custom := range []bool{false, true} {
		flags.CustomIpv4, flags.CustomIpv6 = "", ""
		if custom {
			flags.CustomIpv4, flags.CustomIpv6 = "192.0.2.1", "2001:db8::1"
		}
		v4, v6, err := GetIPAddress()
		if err != nil || v4 != flags.CustomIpv4 || v6 != flags.CustomIpv6 {
			t.Fatalf("unexpected IP result: %q %q %v", v4, v6, err)
		}
	}
}
