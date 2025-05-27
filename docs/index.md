# WPOD: WordPress in Docker

WPOD is a modern CLI tool for managing local WordPress environments using Docker. This documentation covers all major commands, workflows, troubleshooting, and best practices for using WPOD in real-world development.

---

## What is WPOD?

WPOD lets you create, manage, and destroy fully isolated WordPress stacks on your local machine. Each instance includes WordPress, MySQL, Adminer, Mailpit, and optionally Caddy for HTTPS. It is designed for developers who want:

- Fast, repeatable WordPress environments
- No global PHP/MySQL/Apache required
- Easy switching between projects
- Safe, isolated testing and development

---

## Creating a New Instance

Run:

```sh
wpod create
```

You will be prompted for an instance name, and whether to customize ports, credentials, and Caddy (reverse proxy) settings. WPOD checks for port conflicts and guides you through safe choices.

**Tips:**

- If ports 80/443 are in use, WPOD will suggest disabling the built-in Caddy and using your own reverse proxy.
- You can always edit the generated `.env` and `docker-compose.yml` for advanced tweaks.

**Common errors:**

- _"Port already in use"_: Choose a different port or stop the conflicting service.
- _"Permission denied"_: Ensure you have write access to the target directory.

---

## Instance Directory Structure

Each instance contains:

- `docker-compose.yml` — Defines all services (WordPress, DB, Adminer, Mailpit, Caddy)
- `.env` — All environment variables for the stack
- `config/Caddyfile` — Caddy reverse proxy config (used if Caddy is enabled)
- `wp-content/` — Your themes, plugins, uploads
- `wordpress/` — WordPress core files
- `db/` — Database data (persisted in Docker volume)
- `manage` — Instance management CLI

---

## Managing an Instance

From inside the instance directory, use the `manage` tool:

- `./manage start` — Start all services in the background
- `./manage stop` — Stop all services (data is preserved)
- `./manage status` — Show which containers are running, with health info
- `./manage update` — Pull latest images and restart
- `./manage console` — Open a shell in the WordPress container
- `./manage logs` — View logs for all services
- `./manage install` — Run the WordPress install wizard (WP-CLI)
- `./manage plugins` — Install, update, activate, or delete plugins interactively
- `./manage themes` — Manage themes interactively
- `./manage users` — List, create, or update users
- `./manage db` — Import/export, backup/restore the database
- `./manage mail` — Open Mailpit web UI (view outgoing emails)
- `./manage admin` — Open WP Admin in your browser
- `./manage open` — Open the site in your browser

**Suggestions:**

- Use `./manage logs` if a service fails to start (look for MySQL or WordPress errors)
- Use `./manage db backup` before risky changes
- Use `./manage plugins` and `./manage themes` to avoid manual WP Admin work

---

## Caddy & HTTPS

WPOD can run a Caddy container for local HTTPS, or you can use the generated Caddyfile with your own host-level Caddy/Nginx/Apache. During creation, you’ll be prompted to enable/disable Caddy. If you use your own reverse proxy, point it to the WordPress container’s port (see `.env`).

**Troubleshooting:**

- If HTTPS doesn’t work, check Caddy logs (`docker compose logs caddy`) or your host proxy config.
- For custom domains, add entries to your `/etc/hosts` or use a wildcard DNS for `.local` domains.

---

## Advanced Usage

- **Customizing Services:** Edit `docker-compose.yml` to add Redis, Xdebug, or other services.
- **Environment Variables:** All service settings are in `.env`. Change ports, DB credentials, or WP debug flags here.
- **Production Prep:** Use `./manage prod-check` and `./manage prod-prep` for production readiness.
- **Backups:** Use `./manage db backup` and `./manage db restore` for safe migrations.

---

## Troubleshooting & FAQ

- **Docker won’t start containers:** Check Docker Desktop is running, and you have enough free RAM/disk.
- **Database connection errors:** Wait a few seconds after `start`—MySQL may take time to initialize.
- **Permission issues on Linux:** Use `./manage fix-perms` to set correct file permissions for `wp-content`.
- **Mailpit not receiving mail:** Check your WordPress SMTP settings and Mailpit logs.

---

## Best Practices

- Use a separate instance for each project/client.
- Regularly backup your database and `wp-content`.
- Use the CLI for all management tasks—avoid manual container or file edits unless necessary.
- Keep WPOD and Docker images up to date (`wpod update`, `./manage update`).

---

## More Documentation

- [Getting Started](getting-started.md): Step-by-step setup
- [Usage Guide](usage.md): Common workflows
- [CLI Reference](cli.md): All commands and options
- [Caddy & Reverse Proxy](caddy.md): HTTPS and domain setup
- [Advanced Topics](advanced.md): Customization, production, and more
- [FAQ](faq.md): Answers to common questions

---

WPOD is open source and welcomes issues, feedback, and contributions. For more, visit the [GitHub repo](https://github.com/regiellis/wpod-go).
