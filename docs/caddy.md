# Caddy & HTTPS

WPID can run a Caddy container for local HTTPS support, making it easy to develop with SSL locally. You can enable or disable Caddy during instance creation. If you choose to enable it, WPID will generate a `config/Caddyfile` for you and handle all the setup automatically.

## How Caddy Works in WPID

- **Caddy Enabled**: When enabled, Caddy acts as a reverse proxy, providing HTTPS for your local WordPress site. The configuration is stored in `config/Caddyfile` inside your instance directory. Caddy will automatically generate self-signed certificates for your `.local` domains.
- **Custom Reverse Proxy**: If you prefer to use your own Caddy, Nginx, or Apache setup, simply disable Caddy during instance creation. You can then use the generated `Caddyfile` as a template, or point your own reverse proxy to the WordPress container’s HTTP port (see the `.env` file for the correct port).

## Custom Domains and Local HTTPS

- WPID supports custom domains for each instance (e.g., `myproject.local`).
- For local development, add your chosen domain(s) to your `/etc/hosts` file, mapping them to `127.0.0.1`.
- Caddy will serve your site over HTTPS at the custom domain.
- If you use a wildcard DNS service (like `nip.io` or `sslip.io`), you can avoid editing `/etc/hosts`.

## Troubleshooting

- If HTTPS fails, check Caddy logs:

  ```sh
  docker compose logs caddy
  ```

- Make sure the ports required by Caddy (usually 80 and 443) are not in use by other services. If they are, you can:
  - Disable Caddy and use your own reverse proxy.
  - Change the ports in `.env` and `docker-compose.yml`.
- For browser trust issues with self-signed certificates, you may need to accept the certificate manually in your browser.
- If you change domains or ports, restart the Caddy container to apply changes.

## Advanced Caddy Configuration

- You can edit `config/Caddyfile` to add custom rules, redirects, or additional sites.
- For advanced HTTPS options (e.g., custom certificates, HTTP/2, etc.), refer to the [Caddy documentation](https://caddyserver.com/docs/).
- If you use your own Caddy or another reverse proxy, ensure it forwards both HTTP and HTTPS traffic to the correct container port.

See [Advanced Usage](./advanced.md) for more customization tips and best practices.
