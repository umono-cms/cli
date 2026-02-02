# Umono CLI

Command-line interface for the Umono content management system.

## Installation

Install with a single command:

```bash
curl -fsSL https://umono.io/install.sh | sh
```

### Supported Platforms

| OS | Architecture |
|----|--------------|
| Linux | amd64, arm64 |
| macOS | amd64, arm64 |

### Installation Directory

The CLI is installed to `~/.local/bin` by default.

If this directory is not in your PATH, add the following line to your shell configuration file (`.bashrc`, `.zshrc`, etc.):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then restart your terminal or run:

```bash
source ~/.bashrc  # or ~/.zshrc
```

## Quick Start

Create a new website:

```bash
umono create my-website
```

## Requirements

- `curl` or `wget` (for installation)
- Linux or macOS

## License

See the [GitHub repository](https://github.com/umono-cms/cli) for details.
