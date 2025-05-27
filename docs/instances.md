# Managing Instances

Each WordPress project is an "instance" managed by WPOD. Instances are fully isolated and have their own configuration, files, and database.

## Creating an Instance

Run:
```sh
wpod create
```
Follow the prompts to set up a new instance.

## Instance Structure

- `docker-compose.yml` — Service definitions
- `.env` — Environment variables
- `config/Caddyfile` — Caddy config (if enabled)
- `wp-content/` — Themes, plugins, uploads
- `wordpress/` — Core files
- `db/` — Database data
- `manage` — CLI tool

## Managing an Instance

Use the `manage` tool inside the instance directory for all operations. See [CLI](./cli.md) for details.