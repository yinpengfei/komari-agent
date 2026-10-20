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
