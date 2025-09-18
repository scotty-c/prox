# Contributing to Prox

We love your input! We want to make contributing to Prox as easy and transparent as possible, whether it's:

- Reporting a bug
- Discussing the current state of the code
- Submitting a fix
- Proposing new features
- Becoming a maintainer

## Development Process

We use GitHub to host code, to track issues and feature requests, as well as accept pull requests.

### Pull Requests

1. Fork the repo and create your branch from `main`.
2. If you've added code that should be tested, add tests.
3. If you've changed APIs, update the documentation.
4. Ensure the test suite passes.
5. Make sure your code follows the project's code style.
6. Issue that pull request!

### Testing

Before submitting a pull request, make sure all tests pass:

```bash
make test
```

For more comprehensive checks, run:

```bash
make release-check
```

This will run formatting checks, static analysis, and all tests.

## CI/CD Pipeline

The project uses GitHub Actions for Continuous Integration and Deployment:

### Automatic Processes

- All PRs and pushes to main are tested
- Tagged commits trigger automatic releases
- Release builds include binaries for multiple platforms and Docker images

### Required Permissions

For the GitHub Actions workflow to function properly:

- `contents: write` - Required for creating releases
- `packages: write` - Required for publishing Docker images to GitHub Container Registry

These permissions are already configured in the workflow file.

### Environment Setup

No additional environment setup is required for regular contributors. The GitHub Actions workflow handles everything automatically.

## Release Process

To create a new release:

1. Ensure all changes are merged to the main branch
2. Create and push a new tag:

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

The CI pipeline will automatically:
- Run tests and validation
- Build binaries for all platforms
- Create a GitHub release
- Build and push Docker images

## Code Style

- Follow Go best practices
- Run `make fmt` before committing to ensure code is properly formatted
- Use meaningful commit messages
- Comment your code where appropriate

Thank you for contributing to Prox!
