# WPOD: WordPress on Docker

**Need a fast way to manage local WordPress development environments? WPOD streamlines your development workflow, letting you spin up, manage, and tear down isolated WordPress instances with ease, all powered by Docker and Docker Compose.**

---

> [!NOTE]
> Hey there! I’m the creator of WPOD. While my background is primarily in Python and other web frameworks, this project gave me an exciting opportunity to dive into Go. I was immediately drawn to Go’s simplicity, impressive performance, and its strength in building robust command-line tools.
>
> Given the similarities in certain paradigms between Python and Go, I found I could pick up concepts relatively quickly. However, as with learning any new language, there were moments where I hit a "Go-specific" wall or needed to understand idiomatic approaches.
>
> In the spirit of full transparency, AI (specifically, models from co-pilot) were used as a development partner during this process. It served as a Socratic sounding board, helping me debug, understand Go's nuances (like the `embed` package intricacies we navigated!), refactor code, and explore different solutions for complex problems like cross-platform installation. It was instrumental in accelerating my learning and unblocking me in tricky situations.
>
> This was not a "copy-paste from AI" exercise nor was it an attempt at "vibe" coding. The core logic, design decisions, and iterative refinement were driven by my own understanding of applications architecture and design from my years of experience in other languages. AI was a tool, much like a search engine, patient mentor, or stack overflow, that helped bridge knowledge gaps and explore possibilities more efficiently.
>
> I believe in leveraging all available tools to learn and build effectively.
>
> To those who may have strong opinions against the use of AI: I respect your perspective. This tool is offered as is; if its development process doesn't align with your views, feel free to explore other solutions. Peace. ✌️

---

## ✨ Why WPOD?

Local WordPress development can be a pain. Different PHP versions, conflicting databases, messy configurations... WPOD cuts through the chaos!

> [!WARNING]
> I am aware that there are other tools out there that do similar things. I have used them, and they are great! But I wanted to build something that was more tailored to my needs and workflow. WPOD is my personal solution, and I hope it can help you too!

- 🎉 **Effortless Instancing:** Create pristine, isolated WordPress environments in seconds.
- 🐳 **Docker-Powered:** Leverages the magic of Docker for consistent and reproducible setups.
- 🧹 **Clean & Tidy:** Each project is self-contained. Say goodbye to global XAMPP/MAMPP clutter.
- 🔌 **Port Management:** Automatically finds and assigns available ports.
- ⚙️ **Global Configuration:** Set a default base directory for all your sites and a development domain suffix. ***(not yet...soon)***
- 🎛️ **Centralized Control:** List, manage, and status-check all your local WP sites.
- 👨‍💻 **Developer Focused:** Designed by a developer, for developers and server admin

## 🎬 Quick Demo / Screenshot

> [!TIP]
> Consider adding a short GIF or a more detailed screenshot here showing `wpod create`, then `wpod list`, and maybe the `manage` tool in action for a better visual understanding.

```bash
$ wpod create my-awesome-site
# ... friendly output ...
🎉 Instance Created Successfully!
   Name: my-awesome-site
   Directory: /path/to/your-sites/www-my-awesome-site-wordpress
   WordPress Port (on host): 12345
   Suggested Dev Hostname: my-awesome-site.minio.local
     (Ensure 127.0.0.1 my-awesome-site.minio.local is in your hosts file)
     (Ensure your host Caddy imports /path/to/.config/wpod/wpod-sites.caddy and is reloaded)
   Mailpit Web UI: http://localhost:8026 (SMTP on port 1026)

   Next steps:
     cd /path/to/your-sites/www-my-awesome-site-wordpress
     Run: ./manage up -d
     Access via browser: http://localhost:12345 (or http://my-awesome-site.minio.local if hosts/Caddy configured)

$ wpod list
╭─ List WordPress Instances
Instance Name                     Port    Created             WP Ver   DB Ver   Directory                                       Status
my-awesome-site                   12345   2023-10-27 10:00:00 6.4      8.0      .../www-my-awesome-site-wordpress               Running
another-project                   12346   2023-10-26 15:30:00 6.3      5.7      .../www-another-project-wordpress               Stopped
```

## 🛠️ Features

**WPOD (Main CLI - `wpod`):**
- 🚀 **`create`**: Spin up new WordPress instances with default or custom configurations.
- ⚙️ **`meta <show|edit> --json`**: View or set global configurations like `sites_base_directory` and `dev_domain_suffix`.
- 💻 **`caddy-config <regenerate|show-path>`**: Manages a Caddy configuration file snippet for host-level reverse proxying of all instances. ***(soon)***
- 📋 **`list`**: View all your managed WordPress instances, their ports, and statuses.
- 🗑️ **`delete`**: Safely remove instances, including Docker containers and volumes.
- 🩺 **`doctor`**: Check your system environment and WPOD setup.
- 🔄 **`update`**: Refresh the Docker running status for all instances.
- ➕ **`register`/`unregister`**: Manually add existing compatible WP Docker setups or remove them.
- 🧹 **`prune`**: Clean up registrations for instances whose directories are missing.
- 📍 **`locate <instance_name>`**: Quickly find the directory path of a registered instance.
- 📝 **`meta <show|edit>`**: Manage the central instance metadata file.

**Instance-Specific Tool (`./manage` inside each instance directory):**
- 🟢 **`start`**: Start instance services (WordPress, DB, etc.) in foreground or detached mode.
- 🔴 **`stop`**: Stop instance services, preserving data volumes.
- 🔄 **`restart`**: Stop and then start services.
- ⬇️ **`update`**: Pull latest Docker images and recreate services.
- 💻 **`console`**: Open a bash shell inside the WordPress container.
- 📜 **`logs`**: View/stream Docker service logs.
- ℹ️ **`status`**: Show instance and Docker container status.
- ⚙️ **`install`**: Run initial WordPress installation wizard via WP-CLI.
- 🔌 **`plugins`**: Interactively manage plugins (install, update, toggle, delete).
- 🎨 **`themes`**: Interactively manage themes (install, update, activate, delete).
- 👥 **`users`**: Interactively manage users (list, create, update, delete).
- 🗃️ **`db`**: Interactive database operations (import/export with URL updates, handles file transfer to/from container).
- 💾 **`backup`**: Create a local backup of `wp-content` and the database.
- ⏱️ **`restore`**: Restore from a local backup.
- 💨 **`cache`**: Clear WordPress object cache and transients.
- 🌐 **`open`/`browse`**: Open instance site URL in browser.
- 🔑 **`admin`**: Open instance WP Admin URL in browser.
- ✉️ **`mail`**: Open Mailpit web UI in browser.
- 🔩 **`wpcli <args...>`**: Execute any raw WP-CLI command.
- 🛡️ **`fix-perms`**: (Linux/macOS) Set host `wp-content` permissions for container `www-data` group access; provides guidance for Windows.
- 🔍 **`prod-check`**: Run checks for production readiness (non-destructive).
- 📦 **`prod-prep`**: Guide and assist in preparing an instance for production (can modify `wp-config.php` for debug flags with confirmation).

## Prerequisites

> [!IMPORTANT]
> Ensure these prerequisites are met before attempting to install or use WPOD.

- 🐳 **Docker & Docker Compose:** Essential! WPOD orchestrates Docker containers. [Install Docker](https://docs.docker.com/get-docker/).
- ![Go](https://raw.githubusercontent.com/devicons/devicon/master/icons/go/go-original.svg) **Go (Golang):**
  - **For running `wpod` (pre-compiled):** No Go installation needed by end-users.
  - **For building from source / development:** Go 1.21+ recommended.
- **Make:** Required if building from source using the `Makefile`. [Install GNU Make](https://www.gnu.org/software/make/).

## 🚀 Getting Started & Installation

**Method 1: Download Pre-compiled Binary (Recommended)**

1. **Download `wpod`:**
   - Go to the [Releases Page](https://github.com/regiellis/wpod-cli/releases).
   - Download the `wpod` binary for your OS/architecture (e.g., `wpod-linux-amd64`, `wpod-windows-amd64.exe`).
   - Extract if archived.

2. **Make it Executable & Place in PATH:**

   > [!NOTE]
   > Replace `./wpod-linux-amd64` with the actual name of your downloaded binary.

   - **Linux / macOS:**

     ```bash
     chmod +x ./wpod-linux-amd64
     sudo mv ./wpod-linux-amd64 /usr/local/bin/wpod
     wpod help # Verify
     ```

   - **Windows:**
     1. Rename the downloaded `.exe` to `wpod.exe` if needed.
     2. Move `wpod.exe` to a folder in your system PATH (e.g., `C:\Program Files\WPOD\`, then add that folder to PATH).
     3. Open a *new* terminal and type `wpod help`.

**Method 2: Build from Source & Use `setup` Utility (For developers)**

> [!TIP]
> Ideal for contributing, getting latest changes, or if you prefer building yourself.

1. **Clone Repository:**

   ```bash
   git clone https://github.com/regiellis/wpod-cli.git
   cd wpod-cli
   ```

2. **Build `wpod`:**

   ```bash
   task build-current # Creates ./dist/wpod (or .exe)
   ```

3. **Build `setup` Utility:**

   ```bash
   task build-setup   # Creates ./setup (or .exe)
   ```

4. **Run `setup` Utility:**
   - System-wide install: `./setup install`
     > [!CAUTION]
     > Uses `sudo` on Unix-like systems. Guides Windows users manually.
   - Local dev setup: `./setup dev` (Creates `./wpod` symlink on Unix-like systems)
   - Help: `./setup help`

## 📖 Usage

Once `wpod` is accessible:

```bash
wpod <command> [arguments...]
```

**Core Workflow:**

1. **Create an instance:**

   ```bash
   wpod create my-new-project
   ```
   *(Follow prompts. Note the suggested dev hostname and port.)*
2. **Update hosts file (if using dev domain with Caddy):**
   Add `127.0.0.1 my-new-project.wplocal` (or your chosen hostname) to your `/etc/hosts` file.
3. **Reload host Caddy (if using it):**
   `sudo caddy reload`
4. **Navigate to instance and use `manage` tool:**

   ```bash
   cd $(wpod locate my-new-project) # Or the directory shown after creation
   ./manage start
   ./manage install # Run WordPress installation wizard
   # Now access via http://my-new-project.wplocal or http://localhost:PORT
   ./manage db # For database import/export
   ./manage # For help
   ```

## ⚙️ Configuration & Templates

- Default WordPress setup (Dockerfiles, `docker-compose.yml` template, `.env-template`, the `manage` tool, and `Caddyfile.template` for instance-level Caddy) are embedded within WPOD.
- Source templates are in `cmd/wp-manager/templates/docker-default-wordpress/`.
- `wpod create` extracts these. You can customize per project.
- Global WPOD settings (like `sites_base_directory`) are stored in `~/.config/wpod/.wpod-config.json`.
- The list of managed instances is in `~/.config/wpod/.wpod-instances.json`.
- If using the host Caddy integration, the generated importable Caddy config is typically in `~/.config/wpod/wpod-sites.caddy`.

## 🤝 Contributing

Contributions are welcome! Whether it's bug reports 🐛, feature requests ✨, or pull requests ⇄, please feel free to engage.

1. Fork the repository.
2. Create your feature branch (`git checkout -b feature/AmazingFeature`).
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`).
4. Push to the branch (`git push origin feature/AmazingFeature`).
5. Open a Pull Request.

> [!NOTE]
> Please ensure your code adheres to standard Go formatting. Run `task build-current` and `wpod doctor` before submitting.

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

*Made with ❤️, Go, and a little help from my AI friend 🤖.*
