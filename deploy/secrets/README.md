# Runtime secrets

Run `./deploy/scripts/init.sh` from the repository root. It creates:

- `postgres_password.txt`: a generated database password.
- `anthropic_api_key.txt`: an empty `0600` file by default.

Only the Worker service mounts `anthropic_api_key.txt`. Put the Claude API key
in that file when AI operations are required. Never commit either `.txt` file.

