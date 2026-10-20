package monitoring

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

type ipLookupTransport func(*http.Request) (*http.Response, error)

func (f ipLookupTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestIPDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name, nic6, custom4, custom6, want4, want6 string
		external, legacy, fail, scanFail           bool
		calls4, calls6                             int
	}{
		{name: "NIC IPv6 avoids IPv6 lookup", nic6: "2603:c024::42", external: true, want4: "198.51.100.42", want6: "2603:c024::42", calls4: 1},
		{name: "private IPv6 falls back", nic6: "fd00::42", external: true, want4: "198.51.100.42", want6: "2603:c024::99", calls4: 1, calls6: 1},
		{name: "missing IPv6 falls back", external: true, want4: "198.51.100.42", want6: "2603:c024::99", calls4: 1, calls6: 1},
		{name: "NIC enumeration failure falls back", external: true, scanFail: true, want4: "198.51.100.42", want6: "2603:c024::99", calls4: 1, calls6: 1},
		{name: "disabled external still uses NIC IPv6", nic6: "2603:c024::42", want6: "2603:c024::42"},
		{name: "disabled external never queries", nic6: "fe80::42"},
		{name: "custom addresses win over NIC mode", nic6: "2603:c024::42", custom4: "192.0.2.1", custom6: "2001:db8::1", legacy: true, external: true, want4: "192.0.2.1", want6: "2001:db8::1"},
		{name: "custom IPv4 and automatic IPv6", custom4: "192.0.2.1", external: true, want4: "192.0.2.1", want6: "2603:c024::99", calls6: 1},
		{name: "custom IPv6 and automatic IPv4", custom6: "2001:db8::1", external: true, want4: "198.51.100.42", want6: "2001:db8::1", calls4: 1},
		{name: "legacy keeps private NIC addresses", nic6: "fd00::42", legacy: true, external: true, want4: "10.0.0.42", want6: "fd00::42"},
		{name: "legacy custom IPv4 wins", nic6: "fd00::42", custom4: "192.0.2.1", legacy: true, external: true, want4: "192.0.2.1", want6: "fd00::42"},
		{name: "legacy retains early return for one family", legacy: true, external: true, want4: "10.0.0.42"},
		{name: "lookup failure leaves missing family empty", nic6: "2603:c024::42", external: true, fail: true, want6: "2603:c024::42", calls4: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved := *flags
			old4, old6 := ipv4HTTPClient, ipv6HTTPClient
			oldList, oldAddrs, oldLegacy := listIPInterfaces, interfaceIPAddrs, listLegacyIPInterfaces
			t.Cleanup(func() {
				*flags = saved
				ipv4HTTPClient, ipv6HTTPClient = old4, old6
				listIPInterfaces, interfaceIPAddrs, listLegacyIPInterfaces = oldList, oldAddrs, oldLegacy
			})
			flags.AllowExternalIPLookup, flags.GetIpAddrFromNic = tc.external, tc.legacy
			flags.CustomIpv4, flags.CustomIpv6 = tc.custom4, tc.custom6
			flags.IncludeNics, flags.ExcludeNics = "", ""
			listLegacyIPInterfaces = func() ([]string, error) { return []string{"eth0"}, nil }
			listIPInterfaces = func() ([]net.Interface, error) {
				if tc.scanFail {
					return nil, errors.New("enumeration failed")
				}
				return []net.Interface{{Name: "eth0", Flags: net.FlagUp}}, nil
			}
			interfaceIPAddrs = func(net.Interface) ([]net.Addr, error) {
				addrs := []net.Addr{&net.IPNet{IP: net.ParseIP("10.0.0.42")}}
				if tc.nic6 != "" {
					addrs = append(addrs, &net.IPNet{IP: net.ParseIP(tc.nic6)})
				}
				return addrs, nil
			}
			calls4, calls6 := 0, 0
			client := func(host, ip string, calls *int) *http.Client {
				return &http.Client{Transport: ipLookupTransport(func(r *http.Request) (*http.Response, error) {
					*calls++
					if r.Method != "GET" || r.URL.Scheme != "https" || r.URL.Host != host || r.URL.RawQuery != "format=json" || r.Body != nil || r.Header.Get("Authorization") != "" {
						t.Fatalf("unexpected lookup request: %v", r.URL)
					}
					if tc.fail {
						return nil, errors.New("network unavailable")
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ip":"` + ip + `"}`))}, nil
				})}
			}
			ipv4HTTPClient = client("api.ipify.org", "198.51.100.42", &calls4)
			ipv6HTTPClient = client("api6.ipify.org", "2603:c024::99", &calls6)
			v4, v6, err := GetIPAddress()
			if err != nil || v4 != tc.want4 || v6 != tc.want6 || calls4 != tc.calls4 || calls6 != tc.calls6 {
				t.Fatalf("got (%q, %q), calls (%d,%d), err %v; want (%q,%q), calls (%d,%d)", v4, v6, calls4, calls6, err, tc.want4, tc.want6, tc.calls4, tc.calls6)
			}
		})
	}
}

func TestNICIPv6Selection(t *testing.T) {
	saved := *flags
	oldList, oldAddrs := listIPInterfaces, interfaceIPAddrs
	t.Cleanup(func() { *flags = saved; listIPInterfaces, interfaceIPAddrs = oldList, oldAddrs })
	listIPInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{
			{Name: "eth2", Flags: net.FlagUp}, {Name: "eth1", Flags: net.FlagUp},
			{Name: "eth0"}, {Name: "lo", Flags: net.FlagUp | net.FlagLoopback}, {Name: "broken", Flags: net.FlagUp},
		}, nil
	}
	interfaceIPAddrs = func(nic net.Interface) ([]net.Addr, error) {
		if nic.Name == "broken" {
			return nil, errors.New("addresses unavailable")
		}
		if nic.Name == "eth1" {
			return []net.Addr{&net.IPAddr{IP: net.ParseIP("2603:c024::20")}, &net.IPNet{IP: net.ParseIP("2603:c024::2")}}, nil
		}
		return []net.Addr{&net.IPNet{IP: net.ParseIP("2400::1")}}, nil
	}
	for _, tc := range []struct{ include, exclude, want string }{
		{"", "", "2603:c024::2"}, {"", "eth1", "2400::1"}, {"eth2", "", "2400::1"}, {"missing*", "", ""},
		// Existing NIC filtering gives an explicit include precedence over exclude.
		{"eth*", "eth1", "2603:c024::2"}, {"eth*", "eth*", "2603:c024::2"},
	} {
		flags.IncludeNics, flags.ExcludeNics = tc.include, tc.exclude
		if got := getPublicIPv6FromInterfaces(); got != tc.want {
			t.Errorf("include %q exclude %q: got %q want %q", tc.include, tc.exclude, got, tc.want)
		}
	}
}

func TestPublicIPv6Scope(t *testing.T) {
	for _, text := range []string{"::", "::1", "fe80::1", "fd00::1", "ff02::1", "::ffff:192.0.2.1", "2001:db8::1", "3fff::1", "2001:2::1", "2001:20::1", "2002::1", "192.0.2.1"} {
		if isPublicIPv6(netip.MustParseAddr(text)) {
			t.Errorf("accepted non-public IPv6 %s", text)
		}
	}
	for _, text := range []string{"2603:c024::1", "2400::1", "2001:4860::1"} {
		if !isPublicIPv6(netip.MustParseAddr(text)) {
			t.Errorf("rejected public IPv6 %s", text)
		}
	}
}

func TestIPLookupRejectsInvalidResponse(t *testing.T) {
	for _, tc := range []struct {
		body   string
		status int
		ipv4   bool
	}{
		{`{"ip":"2603:c024::1"}`, 503, false}, {`not JSON 2603:c024::1`, 200, false},
		{`{"ip":"invalid"}`, 200, false}, {`{"ip":"198.51.100.1"}`, 200, false},
		{`{"ip":"2603:c024::1"}`, 200, true}, {`{"ip":"999.1.2.3"}`, 200, true},
	} {
		client := &http.Client{Transport: ipLookupTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
		})}
		got, _ := queryPublicIP(client, "https://api.ipify.org?format=json", tc.ipv4)
		if got != "" {
			t.Errorf("accepted bad response %s: %s", tc.body, got)
		}
	}
}
