# 24-calculate Production Deployment

This project deploys only the Go backend. The WeChat mini game frontend is not deployed by this server workflow.

## Current Server Layout

```text
/data/website/24-calculate/server       # Git checkout
/data/website/24-calculate/avatars      # Avatar volume
/data/backups/24-calculate/mysql        # Migration backups
/data/docker-container/services/nginx/sites/calc-api.pdurl.cn.conf
```

Runtime containers:

```text
twenty_four_calculate_api
twenty_four_calculate_worker
```

Shared services are not managed by this compose project:

```text
MySQL: 172.17.0.1:3306
Redis: docker-container-redis-1:6379, logical DB 3
Docker network: docker-container_backend
```

## Deployment Model

GitHub Actions builds and publishes the production image to GHCR:

```text
ghcr.io/chenbb0128/24-calculate-backend:<commit-sha>
```

The production server no longer compiles Go during normal deployment. It only:

1. Verifies the requested SHA is the current `origin/master`.
2. Pulls the immutable GHCR image for that SHA.
3. Fast-forwards the server checkout.
4. Backs up MySQL.
5. Runs Goose migrations.
6. Recreates only the API and worker containers.
7. Checks `/ready` locally and through `https://calc-api.pdurl.cn/ready`.

Do not run `docker compose down -v` on the production server.

## Admin dashboard

The Go image includes the one-shot `/app/admin-seed` command and the Vue admin
dashboard at `/app/admin`. The API serves it at `/admin/`; the existing Nginx
configuration can keep proxying `/` to the API container. The dashboard uses
same-origin requests to `/api/v1/admin/*` and does not contain provider
credentials. The production workflow builds `frontend/admin-vue` and stages
its `dist` output into the image before publishing it.

The deployment script runs migrations automatically. If both
`GO_SERVICE_ADMIN_USERNAME` and `GO_SERVICE_ADMIN_PASSWORD` are present in the
server-only `backend/deployments/.env`, it also runs the one-shot admin seed
after migrations. The seed refuses to overwrite an existing username, so
subsequent deployments leave the existing password unchanged and continue.

For the first deployment, add the administrator values to the server-only
`.env` (use an 8-character-or-longer password), then push the release. Do not
commit or paste the filled-in file into a ticket:

```bash
cd /data/website/24-calculate/server/backend/deployments
# Edit .env and add these two server-only values:
# GO_SERVICE_ADMIN_USERNAME=admin
# GO_SERVICE_ADMIN_PASSWORD=<your 8-character-or-longer password>
```

The deployment script reads the values from `.env` and never prints the
password. If both values are absent, the release still deploys but skips the
administrator seed.

## GitHub Secrets

The production workflow needs these repository secrets:

```text
PROD_HOST=116.62.159.237
PROD_PORT=22
PROD_USER=calculate-deploy
PROD_SSH_KEY=<private key for github-actions-24-calculate>
PROD_KNOWN_HOSTS=<ssh-keyscan output for the server>
```

`GITHUB_TOKEN` is used automatically for GHCR push/pull during the workflow.

## Server Install

Install the restricted deploy user and commands from the server as root:

```bash
cd /data/website/24-calculate/server
bash backend/deployments/server/install-deploy-components \
  backend/deployments/server/24-calculate-deploy-entrypoint \
  backend/deployments/server/deploy-24-calculate \
  /tmp/github-actions-24-calculate.pub
```

The installer creates `calculate-deploy`, appends one forced-command SSH key, and allows that user to run only `/usr/local/sbin/deploy-24-calculate` through sudo.

## Manual Deploy

A manual server-side deploy can still be run by root when needed:

```bash
cd /data/website/24-calculate/server
git fetch --prune origin master
sha=$(git rev-parse origin/master)
printf '%s\n%s\n' '<ghcr-user>' '<ghcr-token>' | /usr/local/sbin/deploy-24-calculate "$sha"
```

For normal releases, use the GitHub Actions workflow instead.

## Health Checks

```bash
curl --fail http://127.0.0.1:18082/health
curl --fail http://127.0.0.1:18082/ready
curl --fail https://calc-api.pdurl.cn/health
curl --fail https://calc-api.pdurl.cn/ready
```
