# Contributing

There are many ways you can contribute to Tarkov.Community.

- **Send us a Pull Request** on GitHub. Make sure you read our [Getting Started](#getting-started-with-tarkov.community-codebase) guide to learn how to setup the development environment;
- **Report issues** and bug reports on https://github.com/Spicy-Bush/fider-tarkov-community/issues;
- **Give feedback** and vote on features you'd like to see at https://tarkov.community;
- **Support us financially** by donating any amount to our [Ko-fi](https://ko-fi.com/tarkovcommunity) and help us continue our activities;

## Getting started with tarkov.community codebase

Before you start working on something that you intend to send a Pull Request on, make sure there's an [GitHub Issue](https://github.com/Spicy-Bush/fider-tarkov-community/issues) open for that or create one yourself. If it's a new feature you're working on, please share your high level thoughts on the ticket so we can agree on a solution that aligns with the overall architecture and future of Tarkov.Community.

If you have any question or need help, leave a comment on the issue and we'll do our best to help you out.

The backend of Tarkov.Community is powered by Go, while the frontend is built using TypeScript. The project also relies on technologies such as Node.js, React, and PostgreSQL.

#### 1. Install the following tools:

| Software    | How to install                                                 | What is it used for                                       |
| ----------- | -------------------------------------------------------------- | --------------------------------------------------------- |
| Go 1.25+    | https://golang.org/                                            | To compile server side code                               |
| Node.js 23+ | https://nodejs.org/ or run `nvm use` if you have nvm installed | To compile TypeScript and bundle all the client side code |
| Docker      | https://www.docker.com/                                        | To start local PostgreSQL instances                       |

#### 2. To setup your development workspace:

1. clone the repository.
2. navigate into the cloned repository.
3. run `go install github.com/cosmtrek/air` to install air, a cli tool for live reload, when you change the code, it automatically recompiles the application.
4. run `go install github.com/joho/godotenv/cmd/godotenv` to install godotenv, a cli tool to load environment variables from a `.env` so that you don't have to change your machine environment variables.
5. run `go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1` to install golangci-lint, a linter for Go apps.
6. run `npm install` to install client side packages.
7. run `docker compose up -d` to start a local PostgreSQL database on Docker.
8. run `cp .example.env .env` to create a local environment configuration file.


#### 3. To start the application

1. run `make watch` to start both the server and ui on watch mode. The application will be reloaded every time a file is changed. Alternatively, it's also possible to start Fider by running `make build` and `make run`.
2. Navigate to `http://localhost:3000/` and 🎉! You should see the sign up page of Tarkov.Community!

#### 4. To run the unit tests:

1. run `make test` to run both UI and Server unit tests.
