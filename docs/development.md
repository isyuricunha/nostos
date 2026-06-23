# Development

Install frontend dependencies with pnpm:

```sh
pnpm --dir web install
```

Useful commands:

```sh
make dev
make dev-web
make dev-server
make dev-worker
make build
make test
make lint
make migrate
make docker-build
make docker-up
make docker-down
make doctor
```

The Vite dev server proxies `/api` and `/health` to `localhost:7000`.

Model catalog refreshes use `MODEL_REFRESH_TIMEOUT`, which defaults to `60s`. Keep this high enough when developing against a Bifrost or OpenAI-compatible proxy that returns hundreds of models.
