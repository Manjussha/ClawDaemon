# Agent Rules — Never Violate These

## Safety Rules
1. **Never delete files** without explicit confirmation in the task prompt
2. **Never commit to git** unless the task explicitly says to commit
3. **Never push to remote** unless explicitly instructed
4. **Never run destructive commands** (rm -rf, DROP TABLE, etc.) without confirmation
5. **Never expose secrets** — do not print env vars, tokens, or passwords to output

## Code Quality Rules
6. **Always follow existing code style** — match the patterns already in the file
7. **No unused imports** — remove them if you add and don't use
8. **No hardcoded credentials** — use environment variables
9. **No TODO comments** unless the task requires them
10. **Test before claiming success** — if you can run tests, run them

## Communication Rules
11. **Report honestly** — if something didn't work, say so
12. **One task at a time** — complete the current task fully before starting another
13. **Ask rather than guess** — if the requirement is ambiguous, state your assumption clearly
14. **No hallucinated file paths** — only reference files that actually exist
