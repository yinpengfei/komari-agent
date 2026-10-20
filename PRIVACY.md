# Privacy and maintenance

Stable release **1.5.2** is published (2026-10-21), with 14 platform binaries and amd64/arm64 images. [Installation and maintenance guide](https://github.com/yinpengfei/komari/blob/maint/1.5.1/INSTALL_MAINTENANCE.md). Default branch remains main; current maintenance docs live on maint/1.5.1.

This branch is maintained at https://github.com/yinpengfei/komari-agent/tree/maint/1.5.1.
Baseline: upstream commit 828afafe7fb5b1b58c1232094e1e46320719016e (v2 protocol); agent and server version numbers are independent.

- Metrics, hardware/OS details, IPs, task results, startup configuration, and requested file transfers go to the configured panel endpoint. Trust that endpoint; remote control is enabled unless `--disable-web-ssh` is set.
- Third-party IP discovery is disabled by default. The panel can observe the connection source IP. Use `--custom-ipv4`, `--custom-ipv6`, or `--get-ip-addr-from-nic` for explicit IP reporting. NIC addresses may be private/NAT addresses.
- `--allow-external-ip-lookup` explicitly enables HTTPS ipify requests and reveals this machine's source IP to ipify. No hardware/token payload is sent there.
- Automatic updates remain enabled: startup and every 6 hours, within the current major.minor version line (1.5.* for 1.5.2), querying/downloading only this fork's GitHub releases. Container binary self-update is disabled; refresh the image instead. Use `--disable-auto-update` to stop periodic GitHub requests. Do not provide GITHUB_TOKEN unless needed; it is sent to GitHub API for authenticated release requests.
- Installation uses direct GitHub downloads unless a proxy is explicitly selected. Windows additionally downloads NSSM from nssm.cc when required.
- Custom DNS fallback may contact public DNS resolvers when a configured custom resolver fails. Default operation uses system DNS.
- This fork's 1.5.2 binaries and images are already published. Existing upstream binaries do not contain these changes and must be replaced to migrate. Pin --install-version 1.5.2 and add --disable-auto-update if you need to stay on that exact version.
