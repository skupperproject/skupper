# AGENTS.md

Guidance and constraints for AI agents working in this repository.

## Code & Design Standards
- **Test-Only Code Separation:** Do not place test-specific helper functions, mock structures, or test-only hooks in production files. Keep test utilities in `*_test.go` files or dedicated test helper packages (`internal/.../testutils`).
- **Concise Comments:** Keep comments concise (maximum 3 lines). Focus on non-obvious logic, concurrency constraints, or architectural rationale rather than restating what the code does. Avoid verbose or boilerplate comments.

## Quality Assurance & Pre-Commit Checks
Before opening a pull request or submitting changes, ensure all checks pass:
1. **Formatting:** Run `make format` 
2. **Static Analysis:** Run `make vet` 
3. **Tests:** Run `make test` 

## Pull Request Guidelines
- Adhere to the guidelines in [`CONTRIBUTE.md`](CONTRIBUTE.md).
