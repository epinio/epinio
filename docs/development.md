# Development Guidelines

## Commit Message Guidelines:

All new commits need to follow the Conventional Commit guidelines:
https://www.conventionalcommits.org/en/v1.0.0/#summary

### Setup a git hook for commit linting

We use [gitlint](https://jorisroovers.com/gitlint/) to lint all new commits.

The hook can be setup by running the following command inside of the `carrier` git directory:

```bash
gitlint install-hook
```
