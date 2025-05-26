# Advanced Usage

WPID supports advanced workflows for power users:

- **Custom Services**: Edit `docker-compose.yml` to add Redis, Xdebug, or other services.
- **Environment Variables**: Tweak `.env` for ports, DB credentials, or debug flags.
- **Production Prep**: Use `./manage prod-check` and `./manage prod-prep` for production readiness.
- **Backups**: Use `./manage db backup` and `./manage db restore` for migrations.
- **Manual Edits**: You can manually edit config files for custom setups.

Refer to the [FAQ](./faq.md) for troubleshooting and best practices.