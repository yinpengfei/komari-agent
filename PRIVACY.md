# Privacy and maintenance

This branch is maintained at https://github.com/yinpengfei/komari-agent/tree/maint/1.5.1.
Baseline: upstream commit 828afafe7fb5b1b58c1232094e1e46320719016e (v2 protocol); agent and server version numbers are independent.

- Metrics, hardware/OS details, IPs, task results, startup configuration, and requested file transfers go to the configured panel endpoint. Trust that endpoint; remote control is enabled unless `--disable-web-ssh` is set.
- Third-party IP discovery is disabled by default. The panel can observe the connection source IP. Use `--custom-ipv4`, `--custom-ipv6`, or `--get-ip-addr-from-nic` for explicit IP reporting. NIC addresses may be private/NAT addresses.
- `--allow-external-ip-lookup` explicitly enables HTTPS ipify requests and reveals this machine's source IP to ipify. No hardware/token payload is sent there.
- Automatic updates remain enabled, but query/download only this fork's GitHub releases. Use `--disable-auto-update` to stop periodic GitHub requests. Do not provide GITHUB_TOKEN unless needed; it is sent to GitHub API for authenticated release requests.
- Installation uses direct GitHub downloads unless a proxy is explicitly selected. Windows additionally downloads NSSM from nssm.cc when required.
- Custom DNS fallback may contact public DNS resolvers when a configured custom resolver fails. Default operation uses system DNS.
- Build and publish this fork's binaries before using installation/update links. Existing upstream binaries do not contain these changes.
