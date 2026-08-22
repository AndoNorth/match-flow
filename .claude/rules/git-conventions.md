# Git Conventions

## Worktree rule

Worktrees live inside `.worktrees/<type>/<slug>`

e.g. `.worktrees/feat/gateway-realtime-endpoint`
e.g. `.worktrees/fix/ingestion-validation-panic`

## Branch naming rule

Branches follow `<type>/<slug>`

e.g. `feat/gateway-realtime-endpoint`
e.g. `fix/ingestion-validation-panic`

## Commit message format

```
<type>(<context>): [#N] <message>       # when a GH issue exists
<type>(<context>): [no-issue] <message> # when no GH issue
```

`<context>` is typically a service or area name (`gateway`, `ingestion`,
`match`, `frontend`, `docs`, `infra`).

e.g. `feat(gateway): [#4] add realtime match update endpoint`
e.g. `chore(docs): [no-issue] flesh out frontend architecture`

## PR + issue association

- Add `#N` in the PR body to link to an issue
- Use `Closes #N` or `Fixes #N` in the PR body to auto-close on merge
- Do NOT use `gh-N` anywhere - use GitHub-native `#N` syntax
