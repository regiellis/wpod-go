# FAQ

**Q: Docker won’t start containers?**
A: Ensure Docker Desktop is running and you have enough free RAM/disk.

**Q: Database connection errors?**
A: Wait a few seconds after starting—MySQL may take time to initialize.

**Q: Permission issues on Linux?**
A: Run `./manage fix-perms` to set correct permissions for `wp-content`.

**Q: Mailpit not receiving mail?**
A: Check your WordPress SMTP settings and Mailpit logs.

**Q: How do I back up my site?**
A: Use `./manage db backup` and copy your `wp-content` folder.

See [Best Practices](./index.md#best-practices) for more tips.