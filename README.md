# cf-dns-updater

This program gets the public IP address using the FritzBox UPnP API.

Then it checks Cloudflares DNS servers if the IP address is correct and updates the records using the Cloudflare API, if the IP address has changed.

## Limitations
- no subdomains are supported, the domain name has to match the zone name
- if the Cloudflare proxy is enabled, the Cloudflare API gets called every time to check the IP address
