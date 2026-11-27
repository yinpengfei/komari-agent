package cmd

import (
	"encoding/json"
	"testing"
)

func TestExternalIPLookupDefaultAndOptOut(t *testing.T) {
	if got := RootCmd.PersistentFlags().Lookup("allow-external-ip-lookup").DefValue; got != "true" {
		t.Fatalf("external lookup default = %q, want true", got)
	}
	saved := *flags
	t.Cleanup(func() { *flags = saved })
	flags.AllowExternalIPLookup = true
	t.Setenv("AGENT_ALLOW_EXTERNAL_IP_LOOKUP", "false")
	loadFromEnv()
	if flags.AllowExternalIPLookup {
		t.Fatal("environment opt-out was ignored")
	}
	flags.AllowExternalIPLookup = true
	if err := json.Unmarshal([]byte(`{"allow_external_ip_lookup":false}`), flags); err != nil {
		t.Fatal(err)
	}
	if flags.AllowExternalIPLookup {
		t.Fatal("JSON opt-out was ignored")
	}
}

func TestIPLookupURLConfiguration(t *testing.T) {
	saved := *flags
	t.Cleanup(func() { *flags = saved })
	if got := RootCmd.PersistentFlags().Lookup("ipv4-lookup-url").DefValue; got != "https://ipv4.yin.cc/ip" {
		t.Fatal(got)
	}
	if got := RootCmd.PersistentFlags().Lookup("ipv6-lookup-url").DefValue; got != "https://ipv6.yin.cc/ip" {
		t.Fatal(got)
	}
	t.Setenv("AGENT_IPV4_LOOKUP_URL", "https://my-ip.example/ip")
	loadFromEnv()
	if flags.IPv4LookupURL != "https://my-ip.example/ip" {
		t.Fatal("environment URL was ignored")
	}
	if err := json.Unmarshal([]byte(`{"ipv4_lookup_url":"https://other.example/json","allow_external_ip_lookup":false}`), flags); err != nil {
		t.Fatal(err)
	}
	if flags.IPv4LookupURL != "https://other.example/json" || flags.AllowExternalIPLookup {
		t.Fatal("JSON override or explicit opt-out was ignored")
	}
}
