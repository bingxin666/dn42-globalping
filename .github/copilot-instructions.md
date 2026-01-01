# GitHub Copilot Instructions for dn42-globalping

## Project Overview

This repository is for the dn42-globalping project. DN42 is a decentralized network, and this project likely involves monitoring or testing connectivity across the DN42 network using Globalping services.

## Coding Standards and Best Practices

### General Guidelines

- Write clear, maintainable, and well-documented code
- Follow the principle of least surprise - code should be intuitive and predictable
- Prefer simplicity over complexity
- Write self-documenting code with meaningful variable and function names

### Code Style

- Use consistent indentation (spaces or tabs as per existing files)
- Keep functions focused and single-purpose
- Add comments for complex logic, but prefer self-documenting code
- Use meaningful commit messages that describe the "what" and "why"

### Documentation

- Update README.md when adding new features or changing functionality
- Document any configuration options or environment variables
- Include usage examples for new features
- Keep documentation in sync with code changes

### Error Handling

- Handle errors gracefully and provide meaningful error messages
- Log errors with sufficient context for debugging
- Fail fast when appropriate, but provide useful feedback

### Testing

- Write tests for new functionality when test infrastructure exists
- Ensure tests are maintainable and clearly describe what they're testing
- Test edge cases and error conditions

### Security

- Never commit secrets, API keys, or sensitive credentials
- Validate and sanitize all external inputs
- Follow security best practices for the language/framework in use
- Be cautious with network operations and external dependencies

## Repository Structure

```
dn42-globalping/
├── .github/               # GitHub configuration and workflows
│   └── copilot-instructions.md
└── README.md             # Project documentation
```

## Development Workflow

### Making Changes

1. Understand the issue or feature request fully before starting
2. Explore existing code to understand patterns and conventions
3. Make minimal, focused changes that address the specific need
4. Test changes locally when applicable
5. Update documentation to reflect changes
6. Commit with clear, descriptive messages

### Pull Requests

- Keep PRs focused on a single issue or feature
- Provide clear descriptions of changes and their rationale
- Link to related issues
- Respond to review feedback promptly

## Dependencies and Setup

Since this is a minimal repository, dependency information will be added as the project grows. Check README.md for current setup instructions.

## Additional Notes

- This project is related to DN42 (a decentralized network) and Globalping (a network monitoring service)
- Consider network reliability and latency when working on monitoring features
- Be mindful of rate limits and resource usage when interacting with external services
