# Privacy and maintenance — development version

- The configured source-IP service is attempted twice, then the matching ipify endpoint (`https://api.ipify.org` or `https://api6.ipify.org`) is tried once. Each request has an eight-second deadline. The third party sees the source IP, timing and fixed User-Agent, but receives no panel token, hostname or hardware data. IPv6 still prefers a local NIC address and explicit custom addresses take priority.
- Set `--disable-third-party-ip-fallback`, `AGENT_DISABLE_THIRD_PARTY_IP_FALLBACK=true`, or JSON `"disable_third_party_ip_fallback": true` to disable ipify while retaining the two configured-service attempts. Existing `allow_external_ip_lookup=false` still disables all HTTP IP discovery.

# Privacy and maintenance — 1.5.4 (2026-11-28)

Maintenance release: [1.5.4](https://github.com/yinpengfei/komari-agent/releases/tag/1.5.4). [Installation guide](https://github.com/yinpengfei/komari/blob/maint/1.5.1/INSTALL_MAINTENANCE.md). Source and releases use maint/1.5.1; the default branch remains main. Agent and server versions are independent.

- Missing IPv4 uses `https://ipv4.yin.cc/ip`. IPv6 prefers a local public-scope NIC candidate, then uses `https://ipv6.yin.cc/ip` only if missing. Both URLs are configurable. No built-in third-party fallback remains. Queries use the basic-info schedule (startup/reconnect and every five minutes by default), with each request limited to eight seconds and 4096 response bytes.
- Custom IPs take priority. Original `allow_external_ip_lookup` CLI/env/JSON false values still disable all HTTP address discovery. Disabling queries still permits local NIC IPv6 selection. The explicit NIC mode retains its previous behavior, including private addresses and early return after finding either family.
- Discovery carries fixed User-Agent `komari-agent-ip-discovery/1`, no panel token, cookies, hostname or hardware payload. A configured service observes the exit IP and request timing. Requests bypass HTTP_PROXY/HTTPS_PROXY, reject redirects, validate TLS normally, and reject private/special/wrong-family or conflicting response addresses. This may differ from the 1.5.3 proxy behavior; use a reachable direct service or specify IPs manually.
- Local NIC scope filtering does not verify Internet connectivity or identify a NAT66 egress. Interface include/exclude filters and deterministic selection remain unchanged.
- Metrics, hardware/OS, IPs and requested remote-control results still go to the configured panel. No server protocol change is needed for this update. Remote control remains governed by the existing disable-web-ssh setting.
- Existing automatic updates remain enabled: startup/every six hours within the current major.minor line. 1.5.* agents may install 1.5.4. Keep an exact version by disabling automatic updates. Docker requires pulling/recreating the image; its binary self-update remains disabled. Older 1.5.3 assets/tags remain available.
- Installation and updates use this fork's GitHub releases; Windows may download NSSM as before. Custom DNS fallback behavior is unchanged. These external requests are separate from IP discovery.

Configuration precedence remains defaults < CLI < environment < JSON. Consult the readme for custom URL flags and supported response shapes.

## Published 1.5.3 behavior (historical)

# Privacy and maintenance

Maintenance version **1.5.3** (2026-10-21): see its [Release](https://github.com/yinpengfei/komari-agent/releases/tag/1.5.3) for 14 platform binaries and amd64/arm64 images. [Installation and maintenance guide](https://github.com/yinpengfei/komari/blob/maint/1.5.1/INSTALL_MAINTENANCE.md). Default branch remains main; current maintenance docs live on maint/1.5.1.

This branch is maintained at https://github.com/yinpengfei/komari-agent/tree/maint/1.5.1.
Baseline: upstream commit 828afafe7fb5b1b58c1232094e1e46320719016e (v2 protocol); agent and server version numbers are independent.

- Metrics, hardware/OS details, IPs, task results, startup configuration, and requested file transfers go to the configured panel endpoint. Trust that endpoint; remote control is enabled unless `--disable-web-ssh` is set.
- Starting with 1.5.3, third-party IP discovery is enabled by default (1.5.2 defaulted to disabled). Custom IPs take precedence. Automatic IPv6 discovery first selects a public-scope local NIC candidate; only a missing IPv6 uses ipify. IPv4 uses ipify unless explicitly specified. NIC selection does not prove Internet reachability or identify a NAT66 egress address. The explicit `--get-ip-addr-from-nic` mode retains its legacy behavior, which may return private addresses and stop after finding either family.
- HTTPS GET requests use only `https://api.ipify.org?format=json` and `https://api6.ipify.org?format=json`, with User-Agent `curl/8.0.1`, no request body, panel token, hostname or hardware payload. The service can observe the source IP and request timing. Requests follow the basic-info schedule (startup/reconnect and every 5 minutes by default); a NIC IPv6 or custom address skips its corresponding lookup. Standard proxy environment settings may affect which egress address is observed.
- Disable all third-party IP discovery with `--allow-external-ip-lookup=false`, `AGENT_ALLOW_EXTERNAL_IP_LOOKUP=false`, or JSON `"allow_external_ip_lookup": false`. Existing explicit false values remain effective after upgrading. Local public IPv6 discovery continues when external lookup is disabled; manually specify IPv4 if needed. Configuration precedence remains defaults < CLI < environment < JSON.
- Public NIC IPv6 candidates conservatively use `2000::/3` excluding special protocol assignments `2001::/23`, documentation ranges `2001:db8::/32` and `3fff::/20`, and 6to4 `2002::/16`. See the [IANA registry](https://www.iana.org/assignments/iana-ipv6-special-registry/). NIC include patterns take precedence over exclusions, as in existing monitoring filters. Selection is stable by interface name then numeric address. Special-purpose addresses can be supplied explicitly.
- Automatic updates remain enabled: startup and every 6 hours, within the current major.minor version line (1.5.* for 1.5.3), querying/downloading only this fork's GitHub releases. Container binary self-update is disabled; refresh the image instead. Use `--disable-auto-update` to stop periodic GitHub requests. Do not provide GITHUB_TOKEN unless needed; it is sent to GitHub API for authenticated release requests.
- Installation uses direct GitHub downloads unless a proxy is explicitly selected. Windows additionally downloads NSSM from nssm.cc when required.
- Custom DNS fallback may contact public DNS resolvers when a configured custom resolver fails. Default operation uses system DNS.
- Existing upstream binaries do not contain these changes and must be replaced to migrate. Pin `--install-version 1.5.3` and add `--disable-auto-update` to stay on that exact version. Existing 1.5.2 tags/assets remain unchanged; agents with automatic updates enabled may update to 1.5.3 within the same 1.5.* line.
