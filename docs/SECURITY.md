## Public repository safety

This repo is public. Do not commit secrets or private data.

### Never commit
- `.env` files
- JWT secrets, ngrok tokens, API keys
- Database files (`*.db`, `*.sqlite*`)
- Private certificates/keys

### How configuration works
- Local configuration is provided via environment variables.
- Example values live in `docs/env.example`.


