---
title: Release Process
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->


## Development Phase Notice

This project is currently in development phase (0.x.x versions). During this phase:
- Breaking changes may occur in any release
- APIs and interfaces are not yet stable
- We remain on version 0.x.x until the collection reaches stability

## Creating a Release

To create a new release:

1. Review all changes since the last release
   - Check git log: `git log $(git describe --tags --abbrev=0)..HEAD --oneline`
   - Review CHANGELOG.md [Unreleased] section
   
2. Determine version increment based on [Semantic Versioning](https://semver.org/):
   - **MAJOR** version for incompatible API changes
   - **MINOR** version for backwards-compatible functionality additions
   - **PATCH** version for backwards-compatible bug fixes

3. Update the version in `galaxy.yml`

4. Update CHANGELOG.md:
   - Move items from [Unreleased] to a new version section
   - Add release date
   - Ensure all significant changes are documented

5. Commit all changes with a descriptive message:
   ```bash
   git commit -m "Release version X.Y.Z"
   ```

6. Create an annotated git tag:
   ```bash
   git tag -a vX.Y.Z -m "Release version X.Y.Z - Brief description"
   ```

7. Push commits and tag:
   ```bash
   git push origin main --tags
   ```

The GitHub Actions workflow will automatically:
- Build the Ansible collection
- Publish to Ansible Galaxy

## Requirements for Successful Galaxy Import

- All roles must have a `README.md` file
- All roles must have a `meta/main.yml` file with galaxy_info
- The collection must have a valid `galaxy.yml` file
