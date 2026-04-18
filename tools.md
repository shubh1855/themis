# Themis Tool Documentation

This document provides a comprehensive list of all tools available to the AI agents in the Themis ecosystem. These tools allow agents to interact with the web, manage files, execute processes, and handle development workflows.

There are currently **46 tools** registered in the system.

## 🌐 Web Tools
Tools for interacting with external web resources.

| Tool | Description |
| :--- | :--- |
| `web_search` | Performs a web search and returns ranked results with titles and snippets. |
| `fetch_url` | Fetches the content of a URL and extracts readable text and metadata. |
| `fetch_json` | Fetches and parses JSON data from a specific API endpoint or URL. |
| `download_file` | Downloads a file from a URL and saves it to a secure location in the workspace. |
| `scrape_page` | Scrapes a webpage using CSS-like selectors to extract specific elements. |

## 📦 Package Registry Tools
Tools for searching and looking up package metadata across various ecosystems.

| Ecosystem | Tools | Description |
| :--- | :--- | :--- |
| **NPM** | `npm_search`, `npm_lookup` | Search for Node.js packages or retrieve specific package metadata. |
| **Pip (PyPI)** | `pip_search`, `pip_lookup` | Search for Python packages or retrieve detailed metadata from PyPI. |
| **Cargo** | `cargo_search`, `crate_lookup` | Search for Rust crates on crates.io or retrieve crate specifications. |
| **Go** | `go_search`, `go_lookup` | Find Go modules or look up specific module documentation and versions. |

## 📂 File Management Tools
Complete set of tools for manipulating the local filesystem.

| Tool | Description |
| :--- | :--- |
| `create_file` | Creates a new file with the specified content. Fails if the file exists. |
| `write_file` | Writes content to a file, creating it or overwriting it if it already exists. |
| `append_file` | Appends content to the end of an existing file. |
| `read_file` | Reads the full content of a specified file. |
| `edit_file` | Performs a search-and-replace operation on a file. |
| `mkdir` | Creates a directory and any necessary parent directories. |
| `delete_file` | Deletes a specified file. |
| `move_file` | Moves or renames a file or directory. |
| `copy_file` | Copies a file from a source to a destination. |
| `list_dir` | Lists the names, sizes, and types of entries in a directory. |
| `tree` | Generates a visual text representation of a directory's structure. |
| `glob_search` | Finds files matching a specific glob pattern (e.g., `*.go`). |

## ⚙️ Process Management Tools
Tools for executing commands and managing life-cycles of processes.

| Tool | Description |
| :--- | :--- |
| `run_cmd` | Executes a shell command and waits for its output. |
| `run_file` | Executes a specific script (Python, JS, Go, etc.) using its default interpreter. |
| `start_background` | Starts a command in the background and returns a PID for tracking. |
| `stop_background` | Terminates a running background process by its PID. |
| `logs_process` | Retrieves the stdout/stderr logs for a specific background process. |
| `wait_port` | Polls a local port until it becomes active (useful for waiting for servers). |

## 🌿 Git Tools
Integrated Git functionality for managing source control.

| Tool | Description |
| :--- | :--- |
| `git_status` | Shows the current status of the working directory. |
| `git_diff` | Displays the differences between files or commits. |
| `git_log` | Shows the commit history. |
| `git_branch` | Lists, creates, or deletes branches. |
| `git_checkout` | Switches to a different branch or restores files. |
| `git_commit` | Records changes to the repository with a commit message. |
| `git_clone` | Clones a repository into a new directory. |

## 🧪 Testing & Quality Tools
Tools for ensuring code quality and performance.

| Tool | Description |
| :--- | :--- |
| `run_tests` | Runs the test suite for the current project. |
| `run_linter` | Executes a linter (like golangci-lint) to check for style and bugs. |
| `coverage_report` | Generates a report on test coverage. |
| `benchmark_cmd` | Runs performance benchmarks for the project. |

## 🗄️ Database Tools
Tools for interacting with SQL databases.

| Tool | Description |
| :--- | :--- |
| `sql_query` | Executes a SQL query and returns the results. |
| `db_tables` | Lists all tables in the current database schema. |
| `db_schema` | Retrieves the schema (create statements) for a specific table. |
| `db_migrate` | Runs database migration scripts. |
