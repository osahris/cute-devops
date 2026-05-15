# gitflower

A Git workflow tool.

## Quick start

```bash
make build         # produce ./gitflower
make test          # unit + TUI integration + PTY e2e (everything Go)
make e2e-format    # format-pipeline golden-diff against a generated fixture
make e2e           # both of the above
make help          # the rest
```

The `review` subcommand is the headline feature:

```bash
gitflower review feature                              # full TUI
gitflower review --no-tui feature                     # write the .review and exit
gitflower review --base main --read-delay 500ms feature
```

See `issues/dot-review.feature.md` (in the parent repo) for the on-disk
file format and `apps/gitflower/tui/` for the TUI internals.

---

## Original design notes


The idea of this app is provide a fully git based development platform. Other than say central Git hosting sites, like GitHub, GitLab or Gitea/Forgejo, we support a local repo first, git centric, home directory based user experience.

For example, our issue tracker is just markdown files in the `issues/` folder in the home directory (should be configurable, but this is the default). Issues are tracked in `issues/` branches automatically. 

Releases are also just put into the `releases/vX.Y.Z/` folder and get a normal `vX.Y.Z` and the build files are managed with git annex. Our hooks provide us with automatic release branch management. 

The idea is that we provide hooks and that we have an opinionated way of managing a git repo as a software project.

Those workflows are managed by git hooks that are put into the repo. We provide a router, similar to a web servers url router, for the branches. For example, we can hook up all branches under `issues/{issueID}` to a specific issue tracker workflow, that updates the web UI. Our hook only allows explicitly specified branches, you can still configure free branches without rules, but they still need to be configured.

We also provide a web interface similar to Git hosting sites with a Code, News, Reflog, Merge requests, News, Wiki, Static sites in the future. But unlike the central sites, all data is stored directly in the repo. 

Gitflower comes AI excluded. There is a sister project called vibejector where you can plug in AI agents onto git flower, but those are just additional workflows from the gitflower perspective.
