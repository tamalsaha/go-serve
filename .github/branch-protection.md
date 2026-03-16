# Branch Protection Recommendations

Use these settings for the repository default branch in GitHub (currently `master`):

1. Open repository Settings > Branches.
2. Add a branch protection rule for the default branch name pattern.
3. Enable these options:
   - Require a pull request before merging
   - Require approvals (recommended: 1)
   - Require status checks to pass before merging
   - Require branches to be up to date before merging
   - Require conversation resolution before merging
   - Require linear history
4. Under required status checks, add:
   - `ci / test-and-build`

## Notes

- The `release` workflow is tag-driven (`v*`) and is not intended as a required check for pull requests.
- If the workflow name or job name changes, update the required status check string accordingly.
