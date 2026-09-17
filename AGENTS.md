# Repository Instructions

## Upstream maintenance: linear patch stack

This private fork is maintained as a linear patch stack on top of `upstream/main`.

### Invariants

- Treat `upstream/main` as the only upstream source of truth.
- Keep `main` shaped as `upstream/main` followed only by this fork's private commits.
- Keep private changes atomic, independently testable, and free of upstream merge commits.
- Keep `backend/cmd/server/VERSION` as the final private release commit.
- Treat `origin/main` as the publication target, not as an input to upstream synchronization.
- Preserve a local backup branch until the rewritten branch has been validated and accepted.

Verify the shape with:

```bash
git log --reverse --no-merges upstream/main..main
test "$(git rev-list --merges upstream/main..main --count)" -eq 0
```

### Synchronizing upstream

When asked to sync, update, or merge upstream changes, use this rebase workflow instead of merging `upstream/main` into `main`:

1. Require a clean worktree and fetch both remotes.
2. Record the old upstream base with `git merge-base main upstream/main`.
3. Create a uniquely named backup branch at the current `main`.
4. Create a uniquely named `sync/*` candidate branch from `main`.
5. Rebase the candidate onto `upstream/main` and resolve each private commit separately.
6. If an equivalent private change has entered upstream, drop that patch and report it.
7. Keep the private version update as the final commit; update its value only when the requested release version is known.
8. Compare old and rebased patch stacks with `git range-diff <old-base>..<backup> upstream/main..<candidate>`.
9. Run the repository's configured backend and frontend checks. Report unavailable tooling explicitly.
10. Confirm there are no merge commits or unexpected tree changes, then move local `main` to the validated candidate.
11. Retain the backup branch. Delete it only after explicit approval or a verified deployment.

A typical synchronization starts with:

```bash
git status --short --branch
git fetch upstream --prune
git fetch origin --prune
old_base=$(git merge-base main upstream/main)
git branch backup/main-before-sync-<upstream-sha> main
git switch -c sync/upstream-<upstream-sha> main
git rebase upstream/main
```

Use `git rerere` when useful for recurring conflicts. Conflict resolution must preserve the intent of both current upstream behavior and each still-required private patch.

### Remote history

Rebasing changes private commit IDs. Update `origin/main` only when the user explicitly requests it, and then use:

```bash
git push --force-with-lease origin main
```

Before that push, state that remote history will be rewritten and confirm the backup branch exists. Use ordinary pushes for backup branches and tags.
