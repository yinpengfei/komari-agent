package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const DefaultIPv4LookupURL = "https://ipv4.yin.cc/ip"
const DefaultIPv6LookupURL = "https://ipv6.yin.cc/ip"

func validateIPLookupURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return fmt.Errorf("invalid IP lookup URL; credentials and fragments are not allowed")
	}
	if u.Scheme == "https" {
		return nil
	}
	// Loopback HTTP is useful for self-hosted local relays and offline tests.
	ip, e := netip.ParseAddr(u.Hostname())
	if u.Scheme == "http" && e == nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("IP lookup URL must use HTTPS (HTTP is allowed only for a loopback IP)")
}

func queryPublicIP(client *http.Client, endpoint string, ipv4 bool) (string, error) {
	if err := validateIPLookupURL(endpoint); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("invalid IP discovery request")
	}
	// No panel token, hostname, cookies or hardware data is sent.
	req.Header.Set("User-Agent", "komari-agent-ip-discovery/1")
	req.Header.Set("Accept", "text/plain")
	resp, err := client.Do(req)
	if err != nil {
		if e, ok := err.(*url.Error); ok {
			err = e.Err
		}
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("configured IP service returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
	if err != nil {
		return "", err
	}
	if len(body) > 4096 {
		return "", fmt.Errorf("IP discovery response exceeds 4096 bytes")
	}
	text, err := parseIPServiceResponse(body, ipv4)
	if err != nil {
		return "", err
	}
	ip, err := netip.ParseAddr(text)
	if err != nil || ip.Zone() != "" || ip.Is4In6() || ip.Is4() != ipv4 || !isPublicServiceIP(ip) {
		return "", fmt.Errorf("IP service did not return a public address of the requested family")
	}
	return ip.String(), nil
}

var nonPublicServiceIPv4 = func() []netip.Prefix {
	values := []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4"}
	result := make([]netip.Prefix, 0, len(values))
	for _, v := range values {
		result = append(result, netip.MustParsePrefix(v))
	}
	return result
}()

func isPublicServiceIP(ip netip.Addr) bool {
	if ip.Is6() {
		return isPublicIPv6(ip)
	}
	if !ip.Is4() || !ip.IsGlobalUnicast() {
		return false
	}
	for _, p := range nonPublicServiceIPv4 {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// Recognize common source-IP API shapes without extracting arbitrary IPs from HTML.
func parseIPServiceResponse(body []byte, ipv4 bool) (string, error) {
	text := strings.TrimSpace(string(body))
	if strings.HasPrefix(text, "{") {
		var object map[string]json.RawMessage
		if json.Unmarshal(body, &object) != nil {
			return "", fmt.Errorf("invalid IP discovery JSON")
		}
		if raw, ok := object["ip_version"]; ok {
			var v int
			if json.Unmarshal(raw, &v) != nil || v != 0 && (ipv4 && v != 4 || !ipv4 && v != 6) {
				return "", fmt.Errorf("IP discovery response has the wrong ip_version")
			}
		}
		return addressFromObject(object, 0)
	}
	if strings.HasPrefix(text, `"`) {
		var value string
		if json.Unmarshal(body, &value) != nil {
			return "", fmt.Errorf("invalid IP discovery JSON")
		}
		return value, nil
	}
	return text, nil
}
func addressFromObject(object map[string]json.RawMessage, depth int) (string, error) {
	var found string
	for key, raw := range object {
		switch strings.ToLower(key) {
		case "ip", "query", "origin", "ip_addr", "client_ip":
			var value string
			if json.Unmarshal(raw, &value) != nil {
				return "", fmt.Errorf("IP discovery address must be a string")
			}
			if found != "" && value != found {
				return "", fmt.Errorf("IP discovery returned conflicting addresses")
			}
			found = value
		case "data":
			if depth != 0 {
				continue
			}
			var value string
			if json.Unmarshal(raw, &value) != nil {
				var nested map[string]json.RawMessage
				if json.Unmarshal(raw, &nested) == nil {
					var err error
					value, err = addressFromObject(nested, depth+1)
					if err != nil {
						return "", err
					}
				}
			}
			if value != "" {
				if found != "" && found != value {
					return "", fmt.Errorf("IP discovery returned conflicting addresses")
				}
				found = value
			}
		}
	}
	if found == "" {
		return "", fmt.Errorf("IP discovery JSON contains no supported address field")
	}
	return found, nil
}
