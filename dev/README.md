# Docker suite for development

**NOTE**: This exists only for local development. If you're interested in using
Docker for a production setup, visit the
[docs](https://listmonk.app/docs/installation/#docker) instead.

### Objective

The purpose of this Docker suite for local development is to isolate all the dev
dependencies in a Docker environment. The containers have a host volume mounted
inside for the entire app directory. This helps us to not do a full
`docker build` for every single local change, only restarting the Docker
environment is enough.

## Setting up a dev suite

To spin up a local suite of:

- PostgreSQL
- Mailhog
- Node.js frontend app
- Golang backend app

### Verify your config file

The config file provided at `dev/config.toml` will be used when running the
containerized development stack. Make sure the values set within are suitable
for the feature you're trying to develop.

### Setup DB

Running this will build the appropriate images and initialize the database.

```bash
make init-dev-docker
```

### Fast Real-Time Development (`make dev`)

For the fastest local iteration with instant Vite HMR (< 100ms) and fast Go compilation without Docker filesystem overhead:

```bash
# Run both frontend and backend together:
make dev

# OR run in separate terminals:
make dev-frontend
make dev-backend
```

This cross-platform workflow (supported on Windows, Ubuntu/Linux, and macOS):
1. Starts **PostgreSQL**, **MailHog**, and **WAHA** in background Docker containers (`make dev-deps`).
2. Automatically runs database setup (`--install --idempotent --yes`).
3. Runs the **Vite frontend server** natively on the host OS (`make dev-frontend` -> `:8080`).
4. Runs the **Go backend server** natively on the host OS (`make dev-backend` -> `:9000`).

### Re-Initialize Development Database

To completely drop and re-initialize the development database:

```bash
make re-init-dev-db
```

### Fully Containerized Development

Running this starts your full containerized local development stack.

```bash
make dev-docker
```

Visit `http://localhost:8080` on your browser.

### Tear down

This will tear down all the data, including DB.

```bash
make rm-dev-docker
```

### See local changes in action

- Backend: Anytime you do a change to the Go app, it needs to be compiled. Just
  run `make dev-docker` again and that should automatically handle it for you.
- Frontend: Anytime you change the frontend code, you don't need to do anything.
  Since `yarn` is watching for all the changes and we have mounted the code
  inside the docker container, `yarn` server automatically restarts.
