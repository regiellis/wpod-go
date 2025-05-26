# Getting Started with WPID

WPID makes it easy to spin up local WordPress environments using Docker. Follow these steps to get started:

1. **Install Docker**: Make sure Docker Desktop (or Docker Engine) is installed and running on your system.
2. **Download WPID**: Clone or download the WPID repository.
3. **Create a New Instance**:
   ```sh
   wpid create
   ```
   Follow the prompts to set up your first WordPress instance.
4. **Start Your Instance**:
   ```sh
   cd <instance-directory>
   ./manage start
   ```
5. **Access WordPress**: Open your browser and go to the URL provided in the setup output.

For more details, see the [Usage](./usage.md) and [CLI](./cli.md) sections.