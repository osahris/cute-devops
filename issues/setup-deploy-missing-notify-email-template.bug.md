---
status: draft
---

<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# `setup_deploy`: email notifications fail on a missing template

## Symptom

With `setup_deploy_notify_email_to` set (i.e. email notifications enabled),
the role fails:

```
TASK [setup_deploy : Template email notification configuration]
fatal: [...]: Could not find or access 'notify_email.env.j2'
```

Default runs (`setup_deploy_notify_email_to: ""`) are unaffected — the task
is `when`-gated — so the bug stays latent until someone turns email on.

## Root cause

`roles/setup_deploy/tasks/notify.yaml` templates `src: notify_email.env.j2`
to `/etc/deploy/notify_email.env`, but no such template exists in the role
(there was no `templates/` directory at all). `notify-email.sh` sources that
env file for `NOTIFY_EMAIL_TO`, `NOTIFY_EMAIL_FROM`, and the rest.

## Fix

Add `roles/setup_deploy/templates/notify_email.env.j2` rendering the
`setup_deploy_notify_email_*` variables into the `NOTIFY_EMAIL_*` shell vars
`notify-email.sh` expects, with the managed-file header. Verify with a run
that sets `setup_deploy_notify_email_to`.

Note: `notify.yaml` and `systemd.yaml` both create `/etc/deploy` and copy
`notify.sh` / `notify-email.sh` — overlapping work. Worth de-duping while
fixing this, but not required to close the bug.
