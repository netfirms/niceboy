# 🤝 Contributing to niceboy

First off, thank you for considering contributing to `niceboy`! It's people like you who make the project better.

## 🏗️ Development Environment

### Setup

1. Fork and clone the repository.
2. Install dependencies:
   ```bash
   go mod download
   ```
3. Run the bot in development mode:
   ```bash
   go run cmd/niceboy/main.go
   ```
4. Run tests to ensure everything is working:
   ```bash
   go test ./...
   ```

### Technical Context

Before contributing, please review [ARCHITECTURE.md](ARCHITECTURE.md) and [docs/SPECIFICATION.md](docs/SPECIFICATION.md). We adhere to a strict modular adapter pattern for exchanges and a plug-and-play registry for strategies.

## 📜 Coding Style

- Follow the standard formatting tools (`gofmt` or `rustfmt`).
- Write meaningful commit messages.
- Ensure all new features are accompanied by tests.

## 🚀 Pull Request Process

1. Create a new branch for your feature or bugfix.
2. Submit a Pull Request (PR) with a clear description of the changes.
3. Link the PR to any relevant issues.
4. Pass all CI checks before requesting a review.

## ⚖️ License

By contributing to `niceboy`, you agree that your contributions will be licensed under the project's [LICENSE](LICENSE).

---

*Happy Coding!*
