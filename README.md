# golang-evdev

## Development

Before committing changes, make sure to enable local git hooks:

```bash
chmod +x .githooks/pre-commit && git config core.hooksPath .githooks
```

Pre-commit hook runs:

- make lint
- make test

Commits will be rejected if checks fail.
