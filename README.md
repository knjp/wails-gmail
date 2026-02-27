# README

## About

**wails-gmail** is a desktop email client built with Go (backend) and React/Vite
(frontend) using the Wails framework. It synchronizes a Gmail account via the
Gmail API, caches messages in a local SQLite database, and adds AI-powered
features such as vector search, automatic importance/deadline extraction, and
summary generation via an Ollama server.

The project offers a modular channel system, customizable via JSON settings,
and can run in both development and production modes. Core functionality does
not require any external AI service.

## Development & Usage

### Running the app

The application is a Wails project; use `wails dev` to launch the desktop client
in development mode. This command starts the React/Vite frontend with hot
reload and automatically builds the Go backend. During development you can
also access the Go methods from a browser at `http://localhost:34115`.

To produce a distributable binary for your platform, run `wails build` as
usual. A Dockerfile and `docker-compose.yml` are provided for containerised
deployment, compiling the Go server and packaging the frontend assets.

### Configuration

Settings are stored in `config/settings.json` and channel definitions in
`config/channels.json` (a sample is included). You can edit those files
manually or use the in-app JSON editor (⚙️ Channel&nbsp;Settings / 🔧 App
Settings buttons).

Credentials for Gmail API (`credentials.json`) must be placed in the `config`
directory; the app will prompt for OAuth authorization on first run and store
the token in `config/token.json`.

### Features

* Gmail synchronization and basic labels (read/unread, trash).
* Custom channels using SQL-like queries.
* Local message caching with SQLite.
* AI-powered vector search, summaries, and deadline/importance extraction
  (optional, requires running an Ollama server).
* Manual importance override and message trashing.
* Web+desktop compatibility through the same API abstraction.

### ⚠️ About AI features

The app attempts to connect to the Ollama host specified in the settings file
(`ollama_host` in `config/settings.json`). If the server is not running or the
connection fails, the application will automatically skip all AI/vector-related
processing and continue working with ordinary mail synchronization only.

Vectorization and AI search are enabled **only** when the `ollama_host` is
reachable. Even if an error occurs during connection, the app will still start,
so you can use the core functionality without setting up a local AI server.

## Building

To build a redistributable, production mode package, use `wails build`.
