# steprunner Specification

This document defines the **dibbla-task.yaml** input format and the **GitHub Actions workflow commands** output format used by the `steprunner` package.

---

## Table of Contents

- [dibbla-task.yaml Format (v1)](#dibbla-taskyaml-format-v1)
  - [Top-Level Fields](#top-level-fields)
  - [Environment Variables Section](#environment-variables-section)
  - [Tools Section](#tools-section)
  - [Steps Section](#steps-section)
  - [Step Types](#step-types)
  - [Platform Support](#platform-support)
  - [Dependency Resolution](#dependency-resolution)
  - [Working Directory](#working-directory)
  - [Field Reference](#field-reference)
  - [Full Example](#full-example)
- [Output Format](#output-format)
  - [GitHub Actions Workflow Commands](#github-actions-workflow-commands)
  - [Command Reference](#command-reference)
  - [Step Lifecycle Output](#step-lifecycle-output)
  - [Output Example](#output-example)
  - [Parsing the Output](#parsing-the-output)
  - [Regex Patterns for Consumers](#regex-patterns-for-consumers)
- [Frontend Preview Integration](#frontend-preview-integration)
  - [Preview API](#preview-api)
  - [Live Execution Events](#live-execution-events)
  - [StepEvent Structure](#stepevent-structure)

---

## dibbla-task.yaml Format (v1)

A `dibbla-task.yaml` file defines a pipeline of steps to execute, along with optional tool requirements. The format is YAML-based and designed to be simple, cross-platform, and human-readable.

### Top-Level Fields

| Field     | Type              | Required | Description                                      |
|-----------|-------------------|----------|--------------------------------------------------|
| `version` | string            | Yes      | Format version. Currently only `"1"`.            |
| `env`     | map[string]object | No       | Pipeline-level environment variable declarations (see [Environment Variables](#environment-variables-section)). |
| `tools`   | map[string]object | No       | Named tool definitions (see [Tools](#tools-section)). |
| `steps`   | list[object]      | Yes      | Ordered list of steps (see [Steps](#steps-section)). Must contain at least one step. |

### Environment Variables Section

The `env` map declares pipeline-level environment variables that are available to all steps during execution. Each key is the variable name and the value is an object describing the variable.

```yaml
env:
  <VARIABLE_NAME>:
    description: "<human-readable description>"
    default: "<default value>"
    required: true | false
```

| Field         | Type   | Required | Default | Description                                                                 |
|---------------|--------|----------|---------|-----------------------------------------------------------------------------|
| `description` | string | No       | —       | Human-readable description of the variable. Displayed in frontend setup forms. |
| `default`     | string | No       | —       | Default value used when no value is provided at runtime.                    |
| `required`    | bool   | No       | `false` | If `true`, execution fails before any step runs unless a value is provided (via `WithEnv()` or from a default). |

**Variable name constraints:**

Variable names must match the regex `^[A-Za-z_][A-Za-z0-9_]*$`. Names starting with a digit or containing hyphens/spaces are rejected at parse time.

**Resolution order:**

1. The `default` value from the declaration is applied first.
2. User-provided values (via `WithEnv()` or frontend input) override defaults.
3. If a variable is `required: true` and has no value after resolution, execution fails with `ErrMissingRequiredEnv` before any step runs.

**Behavior:**

- Resolved env vars are merged into the runner-level environment, making them available to all `command` and `write_env` steps.
- Step-level `env` fields can override pipeline-level env vars for that specific step.
- Variables provided via `WithEnv()` that are not declared in the `env` section are passed through unchanged.

**Frontend integration:**

The `Preview()` API returns an `EnvVars` field containing an ordered list of `EnvVarSummary` entries. Each entry includes the variable name, description, default value, and whether it is required. This enables the frontend to render input forms with labels, help text, placeholders, and required indicators during integration setup.

### Tools Section

The `tools` map defines named tools that can be referenced by `tool_check` steps. Each key is a tool name (used as an identifier), and the value describes how to detect and install it.

```yaml
tools:
  <tool-name>:
    check: "<command>"
    privileged: true | false
    bin_path: "<path>"
    install:
      darwin: "<command>"
      linux: "<command>"
      windows: "<command>"
    install_fallback:
      darwin: "<command>"
      linux: "<command>"
      windows: "<command>"
```

| Field              | Type              | Required | Default | Description                                                                 |
|--------------------|-------------------|----------|---------|-----------------------------------------------------------------------------|
| `check`            | string            | Yes      | —       | Shell command to verify the tool is installed. A zero exit code means present. |
| `install`          | map[string]string | No       | —       | Per-platform install commands. Keys are OS identifiers: `darwin`, `linux`, `windows`. |
| `install_fallback` | map[string]string | No       | —       | Per-platform fallback install commands. Tried when the primary `install` fails (e.g. due to permission errors). Fallback commands never run with elevated privileges and are typically used for unprivileged installs to the user's home directory. |
| `bin_path`         | string            | No       | —       | Directory where the tool's binary is located after installation. Supports `~` expansion. After any install (primary or fallback), this path is prepended to the process `PATH` and persisted to `~/.dibbla/env` so that external processes (e.g. code editors launched via "Open With") can also find the tool. |
| `privileged`       | bool              | No       | `false` | If `true`, the primary install command runs with elevated privileges. On macOS this shows the native system authentication dialog via `osascript`. If the user cancels, the step fails with an appropriate error. Does not apply to `install_fallback`. |

**Detection logic:**

1. The tool name is looked up in `PATH` via `exec.LookPath`.
2. The `check` command is executed. A zero exit code confirms the tool is available.
3. If both fail and an `install` command exists for the current OS, it is executed. If `privileged: true`, the install command runs with elevated privileges (on macOS, this triggers the native system authentication dialog via `osascript`).
4. If the primary install fails and an `install_fallback` command exists for the current OS, it is executed without elevated privileges.
5. If `bin_path` is set, the expanded path is prepended to the process `PATH` and appended to `~/.dibbla/env`.
6. After installation, the `check` command is re-run to verify success.

**Fallback installation:**

Some users (e.g. non-admin accounts on shared machines) cannot install tools via package managers that require write access to system directories. The `install_fallback` field provides an alternative install command that installs the tool to a user-local directory (typically under `~/.local/`) without requiring elevated privileges.

When the primary `install` command fails, the runner automatically tries `install_fallback`. The `bin_path` field tells the runner where the fallback-installed binary lives so it can be added to `PATH`.

The `~/.dibbla/env` file accumulates `export PATH=...` entries from all tools with `bin_path`. It is sourced by "Open With" terminal commands (e.g. Claude Code) so that externally launched processes inherit the enriched PATH. Entries are written idempotently — duplicate paths are not added.

```yaml
tools:
  go:
    check: "go version"
    bin_path: "~/.local/go/bin"
    install:
      darwin: "brew install go"
      windows: "winget install --id GoLang.Go -e --accept-source-agreements --accept-package-agreements"
    install_fallback:
      darwin: 'curl -fsSL "https://go.dev/dl/go1.26.2.darwin-$(uname -m | sed s/x86_64/amd64/).tar.gz" | tar -xz -C ~/.local'
```

**Privileged installation:**

Some tools (e.g. Homebrew) require administrator privileges to install. Setting `privileged: true` on a tool definition tells the runner to request elevated privileges when executing the install command.

On macOS, this is implemented using `osascript` with `do shell script ... with administrator privileges`, which presents the native macOS password dialog that users are familiar with from System Settings and other macOS applications. If the user cancels the authentication dialog, the install step fails gracefully.

For tools that need `privileged: true`, the install command should also set `NONINTERACTIVE=1` (or equivalent) to suppress interactive prompts, since the command runs without a terminal:

```yaml
tools:
  brew:
    check: "brew --version"
    privileged: true
    install:
      darwin: 'NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"'
```

### Steps Section

Steps are executed in dependency order. Each step is an object with the following fields:

```yaml
steps:
  - id: "<slug>"
    name: "<display name>"
    type: "command" | "tool_check" | "write_env"
    tool: "<tool-name>"             # only for tool_check
    run: "<shell command>"          # string or platform map, only for command
    mode: "foreground" | "background" | "terminal"  # only for command
    platforms:                      # optional platform filter
      - "darwin"
      - "linux"
      - "windows"
    env:
      KEY: "value"
    env_file: "<filename>"          # only for write_env
    working_dir: "<path>"
    depends_on:
      - "<step-id>"
    continue_on_error: false
```

| Field              | Type                       | Required                  | Default        | Description                                                                 |
|--------------------|----------------------------|---------------------------|----------------|-----------------------------------------------------------------------------|
| `id`               | string                     | Yes                       | —              | Unique stable identifier (slug). Used for dependency references and frontend tracking. |
| `name`             | string                     | Yes                       | —              | Human-readable display label.                                               |
| `type`             | string                     | Yes                       | —              | Step type: `"command"`, `"tool_check"`, or `"write_env"`.                   |
| `tool`             | string                     | Yes (if type=tool_check)  | —              | References a key in the `tools` map.                                        |
| `run`              | string \| map[string]string | Yes (if type=command)    | —              | Shell command. Either a plain string (all platforms) or a per-platform map (see [Platform Support](#platform-support)). |
| `mode`             | string                     | No (only for command)     | `"foreground"` | Execution mode: `"foreground"` blocks until done, `"background"` starts and moves on, `"terminal"` opens in a new terminal window. See [Execution Modes](#execution-modes). |
| `platforms`        | list[string]               | No                        | all            | Restricts the step to the listed platforms. Valid values: `darwin`, `linux`, `windows`. If omitted, the step runs on all platforms. |
| `env`              | map[string]string          | No                        | —              | Additional environment variables for this step. Merged with runner-level env; step-level takes precedence. |
| `env_file`         | string                     | No                        | `".env"`       | Output filename for `write_env` steps. Relative to the working directory.   |
| `working_dir`      | string                     | No                        | —              | Override the working directory for this step.                               |
| `depends_on`       | list[string]               | No                        | `[]`           | Step IDs that must complete successfully before this step runs.             |
| `continue_on_error`| bool                       | No                        | `false`        | If `true`, the pipeline continues even if this step fails. Downstream steps are not skipped. |

### Execution Modes

The `mode` field controls how a `command` step is executed. It is only valid on steps with `type: command`. If omitted, the default is `foreground`.

| Mode         | Behavior                                                                                     |
|--------------|----------------------------------------------------------------------------------------------|
| `foreground` | **Default.** The runner blocks until the command completes. Success = exit code 0.           |
| `background` | The runner starts the command, waits 2 seconds for immediate crashes, then moves on. The command continues running independently. Useful for services (e.g. a backend server). |
| `terminal`   | The runner opens the command in a **new native terminal window** and moves on immediately. On macOS this opens Terminal.app; on Linux it tries gnome-terminal, konsole, xfce4-terminal, or xterm. Useful for dev servers where the user wants live output, HMR status, and the ability to Ctrl+C. |

**Background mode:**

```yaml
- id: run-backend
  name: "Start Backend Server"
  type: command
  mode: background
  run: "PORT=8080 ./bin/server"
  depends_on: ["build"]
```

The process runs independently of the pipeline. If it exits within the 2-second grace period (e.g. due to a missing binary or crash), the step fails. Output streaming continues through the configured formatter while the process is running.

**Terminal mode:**

```yaml
- id: start-frontend-dev
  name: "Start Vite Dev Server"
  type: command
  mode: terminal
  run: "npm run dev"
  working_dir: "./frontend"
  depends_on: ["install-deps"]
```

On macOS, this opens a new Terminal.app window with the command running. The terminal inherits the enriched PATH and step-specific environment variables. The user can see live output, interact with the process, and use Ctrl+C to stop it.

### Step Types

#### `command`

Runs a shell command via the platform shell:

- **macOS / Linux**: `/bin/sh -c "<run>"`
- **Windows**: `cmd /C "<run>"`

The `run` field is required and can be either:

- A **plain string** — the same command runs on all platforms:

  ```yaml
  run: "make build"
  ```

- A **platform map** — different commands per OS:

  ```yaml
  run:
    darwin: "brew install foo"
    linux: "sudo apt-get install -y foo"
    windows: "choco install foo"
  ```

When `run` is a platform map, the runner resolves the command for the current OS at execution time. If no entry exists for the current platform and the step was not filtered out by `platforms`, the step fails with `ErrNoPlatformCommand`.

Stdout and stderr are streamed in real time through the configured formatter.

#### `tool_check`

Checks if a tool is available and optionally installs it if missing. The `tool` field is required and must reference a key in the top-level `tools` map.

The check follows this sequence:

1. Look for the binary in `PATH` (`exec.LookPath`).
2. Run the tool's `check` command.
3. If the tool is not found and `install` commands are defined, run the platform-appropriate install command.
4. Re-run the `check` command to verify installation succeeded.

#### `write_env`

Writes all resolved pipeline-level environment variables (from the top-level `env` section) to a `.env` file. This step type requires no additional fields — `env_file` is optional and defaults to `".env"`.

The generated file uses standard `.env` format:

```
# Description of the variable
VARIABLE_NAME=value
```

Variables are written in alphabetical order. If a variable declaration includes a `description`, it is written as a `#` comment above the variable line.

**Typical usage:**

```yaml
env:
  DATABASE_URL:
    description: "PostgreSQL connection string"
    default: "postgres://localhost:5432/mydb"

steps:
  - id: write-env
    name: "Write .env File"
    type: write_env

  - id: start-app
    name: "Start Application"
    type: command
    run: "npm start"
    depends_on: ["write-env"]
```

### Platform Support

The steprunner supports three platforms: `darwin` (macOS), `linux`, and `windows`. Platform awareness is expressed at two levels:

#### 1. Per-step command variants (`run` as platform map)

When a step needs different commands on different OSes, `run` accepts a map keyed by platform name instead of a plain string:

```yaml
- id: install-deps
  name: "Install Dependencies"
  type: command
  run:
    darwin: "brew install pkg-config openssl"
    linux: "sudo apt-get install -y pkg-config libssl-dev"
    windows: "choco install openssl"
```

At runtime, the runner resolves the command for the current platform. If no entry exists for the current OS, the step fails.

#### 2. Step-level platform filter (`platforms` field)

When a step should only run on certain platforms, set the `platforms` field:

```yaml
- id: codesign
  name: "Code Sign (macOS)"
  type: command
  platforms: ["darwin"]
  run: "codesign --sign - target/release/app"
```

If `platforms` is omitted, the step runs on all platforms. Steps filtered out by `platforms` are completely removed from the execution plan — they do not appear in the run result.

#### Platform filtering and dependency pruning

When steps are filtered out for the current platform, any `depends_on` references to those steps are automatically pruned from remaining steps. This means:

```yaml
steps:
  - id: install-mac
    platforms: ["darwin"]
    run: "brew install openssl"

  - id: install-linux
    platforms: ["linux"]
    run: "apt install libssl-dev"

  - id: build
    run: "make build"
    depends_on: ["install-mac", "install-linux"]
```

On macOS, `install-linux` is removed and `build` depends only on `install-mac`. On Linux, `install-mac` is removed and `build` depends only on `install-linux`. The dependency graph stays valid on every platform.

#### Combining both features

For maximum flexibility, combine platform maps with platform filters:

```yaml
# Same step, different commands:
- id: install-deps
  name: "Install Dependencies"
  type: command
  run:
    darwin: "brew install foo"
    linux: "apt install foo"

# Platform-exclusive steps:
- id: configure-launchd
  name: "Configure launchd"
  type: command
  platforms: ["darwin"]
  run: "launchctl load ~/Library/LaunchAgents/com.app.plist"

- id: configure-systemd
  name: "Configure systemd"
  type: command
  platforms: ["linux"]
  run: "sudo systemctl enable myapp"
```

### Dependency Resolution

Steps are executed in an order that respects `depends_on` constraints, determined by topological sort (Kahn's algorithm). Platform-filtered steps are removed before the sort.

**Rules:**

- A step will not run until all of its dependencies have completed successfully.
- If a dependency fails and does **not** have `continue_on_error: true`, all downstream steps are **skipped**.
- If a dependency fails but **does** have `continue_on_error: true`, downstream steps still run.
- Circular dependencies are detected and produce an error before any steps execute.
- Steps with no dependencies run in the order they appear in the file.
- Dependencies on platform-filtered steps are automatically pruned.

### Working Directory

When `Run()` is called with a file path, the working directory defaults to the directory containing the `dibbla-task.yaml` file. This ensures that all steps — including `write_env` — operate relative to the task file's location.

The default can be overridden by:

- Calling `WithWorkDir()` at the API level (overrides the file-based default).
- Setting `working_dir` on a specific step (overrides the runner-level working directory for that step only).

When using `RunReader()` or `RunString()`, there is no file path to derive from, so the process working directory is used unless `WithWorkDir()` is set.

### Field Reference

Quick reference for required fields by step type:

| Field         | `command` | `tool_check` | `write_env` |
|---------------|-----------|--------------|-------------|
| `id`          | Required  | Required     | Required    |
| `name`        | Required  | Required     | Required    |
| `type`        | Required  | Required     | Required    |
| `run`         | Required  | —            | —           |
| `mode`        | Optional  | —            | —           |
| `tool`        | —         | Required     | —           |
| `platforms`   | Optional  | Optional     | Optional    |
| `env`         | Optional  | —            | —           |
| `env_file`    | —         | —            | Optional    |
| `working_dir` | Optional  | —            | Optional    |
| `depends_on`  | Optional  | Optional     | Optional    |
| `continue_on_error` | Optional | Optional | Optional  |

### Full Example

```yaml
version: "1"

env:
  DATABASE_URL:
    description: "PostgreSQL connection string for the application database"
    default: "postgres://localhost:5432/mydb"
  API_KEY:
    description: "Third-party API key for the payment provider"
    required: true
  LOG_LEVEL:
    description: "Logging verbosity (debug, info, warn, error)"
    default: "info"

tools:
  brew:
    check: "brew --version"
    privileged: true
    install:
      darwin: 'NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"'
  go:
    check: "go version"
    install:
      darwin: "brew install go"
      linux: "sudo apt-get install -y golang"
      windows: "choco install golang"
  node:
    check: "node --version"
    install:
      darwin: "brew install node"
      linux: "curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash - && sudo apt-get install -y nodejs"
      windows: "choco install nodejs"

steps:
  - id: write-env
    name: "Write .env File"
    type: write_env

  - id: verify-brew
    name: "Verify Homebrew"
    type: tool_check
    tool: brew
    platforms: ["darwin"]

  - id: verify-go
    name: "Verify Go"
    type: tool_check
    tool: go
    depends_on: ["verify-brew"]

  - id: verify-node
    name: "Verify Node.js"
    type: tool_check
    tool: node
    depends_on: ["verify-brew"]

  - id: install-native-deps
    name: "Install Native Dependencies"
    type: command
    run:
      darwin: "brew install pkg-config openssl"
      linux: "sudo apt-get install -y pkg-config libssl-dev"
      windows: "choco install openssl"
    depends_on: ["verify-go"]

  - id: build-backend
    name: "Build Backend"
    type: command
    run: "go build -o ./bin/server ./cmd/server"
    env:
      CGO_ENABLED: "0"
    depends_on: ["install-native-deps", "write-env"]

  - id: build-frontend
    name: "Build Frontend"
    type: command
    run: "npm ci && npm run build"
    working_dir: "./frontend"
    depends_on: ["verify-node", "write-env"]

  - id: test
    name: "Run Tests"
    type: command
    run: "go test ./..."
    depends_on: ["build-backend"]

  - id: lint
    name: "Run Linter"
    type: command
    run: "golangci-lint run ./..."
    depends_on: ["build-backend"]
    continue_on_error: true

  - id: codesign
    name: "Code Sign (macOS)"
    type: command
    platforms: ["darwin"]
    run: "codesign --sign - ./bin/server"
    depends_on: ["build-backend"]

  - id: package
    name: "Package Release"
    type: command
    run:
      darwin: "tar -czf release.tar.gz -C ./bin server"
      linux: "tar -czf release.tar.gz -C ./bin server"
      windows: "Compress-Archive -Path .\\bin\\server.exe -DestinationPath release.zip"
    depends_on: ["test", "lint", "build-frontend", "codesign"]

  - id: run-server
    name: "Start Backend Server"
    type: command
    mode: background
    run: "PORT=8080 ./bin/server"
    depends_on: ["build-backend"]

  - id: start-dev
    name: "Start Frontend Dev Server"
    type: command
    mode: terminal
    run: "npm run dev"
    working_dir: "./frontend"
    depends_on: ["build-frontend", "run-server"]
```

---

## Output Format

### GitHub Actions Workflow Commands

The default output formatter emits [GitHub Actions workflow commands](https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/workflow-commands-for-github-actions) to stdout. This format was chosen because:

1. It is a well-documented, widely understood standard.
2. It provides structured annotations (errors, warnings, notices) alongside raw output.
3. It supports collapsible groups for step-level organization.
4. It is trivially parseable by any consumer (line-by-line, prefix-based).
5. It works as both human-readable terminal output and machine-parseable structured data.

### Command Reference

| Command                                    | Description                          | When Emitted                          |
|--------------------------------------------|--------------------------------------|---------------------------------------|
| `::group::{title}`                         | Start a collapsible group            | Step begins execution                 |
| `::endgroup::`                             | End the current group                | Step finishes (success or failure)    |
| `::error title={title}::{message}`         | Error annotation                     | Step fails, tool not found, etc.      |
| `::warning title={title}::{message}`       | Warning annotation                   | Tool missing (before install attempt), step skipped |
| `::notice title={title}::{message}`        | Informational notice                 | Tool found, tool installed            |
| `::debug::{message}`                       | Debug-level message                  | Command about to run, verbose detail  |

Lines that do not start with `::` are raw stdout/stderr output from the executing command, belonging to the currently open group.

### Step Lifecycle Output

Each step produces this output sequence:

```
::group::{step name}
::debug::{context information}
{...raw command output lines...}
::notice title={...}::{...}          (on success)
::error title={...}::{...}           (on failure)
::endgroup::
```

**Invariants:**

- Every `::group::` has exactly one matching `::endgroup::`.
- Groups are never nested (steps run sequentially).
- Error/warning/notice annotations appear between `::group::` and `::endgroup::`.
- The number of `::group::`/`::endgroup::` pairs equals the number of executed steps (including skipped steps, but not platform-filtered steps).
- Steps filtered out by `platforms` produce no output at all.

### Output Example

```
::group::Verify Go
::debug::Checking if 'go' is available...
go version go1.24.0 darwin/arm64
::notice title=Tool Check::go is installed at /opt/homebrew/bin/go
::endgroup::
::group::Verify Node.js
::debug::Checking if 'node' is available...
v22.2.0
::notice title=Tool Check::node is installed at /opt/homebrew/bin/node
::endgroup::
::group::Install Native Dependencies
::debug::Running: brew install pkg-config openssl
::endgroup::
::group::Build Backend
::debug::Running: go build -o ./bin/server ./cmd/server
::endgroup::
::group::Run Tests
::debug::Running: go test ./...
ok      example.com/server       0.042s
ok      example.com/server/api   0.018s
::endgroup::
::group::Run Linter
::debug::Running: golangci-lint run ./...
main.go:12:2: ineffectual assignment to err (ineffassign)
::error title=Step Failed::Step "Run Linter" failed with exit code 1
::endgroup::
::group::Code Sign (macOS)
::debug::Running: codesign --sign - ./bin/server
::endgroup::
::group::Package Release
::debug::Running: tar -czf release.tar.gz -C ./bin server
::endgroup::
```

### Parsing the Output

Every line falls into one of two categories:

1. **Workflow commands** -- lines starting with `::`. These carry structured metadata.
2. **Raw output** -- all other lines. These are stdout/stderr from the current step's command.

The general workflow command syntax is:

```
::command parameter1={value1},parameter2={value2}::{message}
```

For commands without parameters (like `group`, `endgroup`, `debug`):

```
::command::{message}
```

### Regex Patterns for Consumers

These patterns can be used to parse the output stream:

```
Group start:     ^::group::(.+)$
Group end:       ^::endgroup::$
Error:           ^::error title=([^:]+)::(.+)$
Warning:         ^::warning title=([^:]+)::(.+)$
Notice:          ^::notice title=([^:]+)::(.+)$
Debug:           ^::debug::(.+)$
Raw output:      ^(?!::)(.*)$
```

---

## Frontend Preview Integration

The package supports a two-phase workflow for GUI applications:

1. **Preview** -- parse the file and display a step checklist before execution.
2. **Execute** -- run steps with real-time status updates to the UI.

### Preview API

Call `Preview()` or `PreviewReader()` to parse a `dibbla-task.yaml` file without executing anything. The returned `PreviewResult` contains:

| Field             | Type              | Description                                             |
|-------------------|-------------------|---------------------------------------------------------|
| `File`            | `*StepFile`       | The fully parsed task file.                             |
| `ExecutionOrder`  | `[]string`        | Step IDs in topologically sorted execution order.       |
| `StepSummaries`   | `[]StepSummary`   | Ordered list of step metadata for UI rendering.         |
| `EnvVars`         | `[]EnvVarSummary` | Sorted list of declared environment variables for frontend setup forms. |

Each `StepSummary` contains:

| Field       | Type       | Description                                    |
|-------------|------------|------------------------------------------------|
| `ID`        | `string`   | Stable step identifier.                        |
| `Name`      | `string`   | Display label.                                 |
| `Type`      | `StepType` | `"command"`, `"tool_check"`, or `"write_env"`. |
| `DependsOn` | `[]string` | Dependency step IDs.                           |
| `Tool`      | `string`   | Tool name (for tool_check steps).              |
| `Platforms` | `[]string` | Platform filter (empty means all platforms).   |

Each `EnvVarSummary` contains:

| Field         | Type     | Description                                                          |
|---------------|----------|----------------------------------------------------------------------|
| `Name`        | `string` | Variable name (e.g. `"DATABASE_URL"`).                               |
| `Description` | `string` | Human-readable description for frontend labels/help text.            |
| `Default`     | `string` | Default value (can be used as placeholder in input fields).          |
| `Required`    | `bool`   | Whether the variable must be set before execution can proceed.       |

The `EnvVars` list is sorted alphabetically by name for deterministic rendering.

**Platform awareness in preview:**

Preview returns **all** steps regardless of the current platform, including their `Platforms` metadata. This allows the frontend to display which steps apply to which platforms (e.g. greying out non-applicable steps). The `ExecutionOrder` reflects the topological order of all steps.

During execution, platform filtering happens automatically — only applicable steps run.

### Live Execution Events

During execution, use `NewCallbackFormatter` to receive structured events. Compose it with `NewMultiFormatter` to get both stdout logging and GUI events:

```go
ghFmt := output.NewGHActionsFormatter(os.Stdout)
cbFmt := output.NewCallbackFormatter(func(e steprunner.StepEvent) {
    // Push to frontend via Wails events, websocket, channel, etc.
    runtime.EventsEmit(ctx, "step-update", e)
})
multi := output.NewMultiFormatter(ghFmt, cbFmt)

result, err := steprunner.Run(ctx, "dibbla-task.yaml",
    steprunner.WithFormatter(multi),
)
```

### StepEvent Structure

Each callback receives a `StepEvent`:

| Field       | Type         | Description                                              |
|-------------|--------------|----------------------------------------------------------|
| `StepID`    | `string`     | Stable step identifier. Empty for global events.         |
| `StepName`  | `string`     | Display label.                                           |
| `Status`    | `StepStatus` | Lifecycle state (see below).                             |
| `Output`    | `string`     | A line of stdout/stderr output.                          |
| `Error`     | `string`     | Error message (when status is `failed`).                 |
| `Timestamp` | `time.Time`  | When the event was created.                              |

**Step statuses:**

| Status      | Meaning                                          |
|-------------|--------------------------------------------------|
| `pending`   | Step has not started yet (set by frontend).       |
| `running`   | Step execution has begun.                        |
| `success`   | Step completed with exit code 0.                 |
| `failed`    | Step completed with non-zero exit code or error. |
| `skipped`   | Step was skipped due to a failed dependency.     |

Note: Steps filtered out by `platforms` do not appear in execution results or events at all — they are removed before execution begins.
