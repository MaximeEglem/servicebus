# ServiceBus CLI (`sb`) — Design Spec

**Date:** 2026-04-25
**Language:** Go
**Binary:** `sb`
**Scope:** CLI toolkit (Phase 1). Flutter integration (Phase 2, separate spec).

---

## 1. Goals

- Cross-platform single binary (Mac, Linux, Windows) with no runtime dependency
- Full Azure Service Bus management: queues, topics, subscriptions, rules
- Full messaging: send, peek, receive, complete, abandon, defer, dead letter
- Rich dead letter management as a first-class concern
- Multi-environment support via named contexts + session state
- Clean library layer (`pkg/`) importable by other Go projects and callable from Flutter via `dart:ffi`
- Self-describing for MCP and Claude Code skill integration

---

## 2. Project Structure

```
servicebus/
├── cmd/
│   └── sb/
│       └── main.go              # CLI entry point (Cobra root)
├── pkg/
│   ├── auth/                    # Credential resolution (connection string + Azure AD)
│   ├── client/                  # Core Service Bus client wrapper (shared state)
│   ├── management/              # Queue, topic, subscription, rule CRUD
│   ├── messaging/               # Send, receive, peek, complete, abandon, defer, deadletter
│   └── ffi/                     # C-exported functions for Flutter dart:ffi
├── internal/
│   ├── format/                  # Table, JSON, YAML output formatters
│   └── config/                  # Context profiles + session state
├── docs/
│   └── superpowers/specs/
└── go.mod
```

**Key principle:** `pkg/` has zero dependency on Cobra or any CLI concern. The CLI in `cmd/` is one consumer of the library. The Flutter FFI layer is another. Any Go project can import `pkg/management` or `pkg/messaging` directly.

---

## 3. Authentication

Credentials are resolved in this priority order per command:

1. `--connection-string` flag (explicit override)
2. `AZURE_SERVICEBUS_CONNECTION_STRING` env var
3. `--context` flag → named context from `~/.sb/config.yaml`
4. `current-context` from session state (`~/.sb/state.yaml`)
5. `--namespace` flag → Azure AD `DefaultAzureCredential` (covers `az login`, env vars, managed identity)

`pkg/auth` exposes a single `Resolve(opts) (credential, error)` function. No auth logic leaks into management or messaging packages.

---

## 4. Configuration File

Location: `~/.sb/config.yaml`

```yaml
current-context: dev
contexts:
  dev:
    connection-string: "Endpoint=sb://dev-ns...;SharedAccessKeyName=...;SharedAccessKey=..."
  staging:
    connection-string: "Endpoint=sb://staging-ns..."
  prod:
    namespace: prod-ns.servicebus.windows.net   # resolves via Azure AD
    use-default-credentials: true

settings:
  max-items: 50                   # default limit for list commands
  working-directory: "~/.sb/exports"
  cache-expiration-days: 30       # interactive Azure AD token cache
  thresholds:
    size-percentage: 70           # highlight queues/topics at ≥70% of max size (red)
    last-access-days: 30          # highlight queues/topics not accessed in 30d (yellow)
```

---

## 5. Session State

Location: `~/.sb/state.yaml` (separate from config, never pollutes connection settings)

```yaml
context: dev
type: dlq
```

Set with:
```
sb use dev          # set active context
sb use dev dlq      # set active context + resource type
sb use              # show current state
```

After `sb use dev dlq`, all commands default to that context and type:
```
sb -m 10                           # = sb dlq -c dev -m 10
sb -n some-queue v -f SomeException
```

Per-command overrides:
```
sb -c staging -m 10                # override context, keep type
sb q -m 10                         # override type, keep context
sb -c prod q -m 10                 # override both
```

---

## 6. Global Flags

Available on every command:

| Flag | Short | Description |
|------|-------|-------------|
| `--context` | `-c` | Override active context |
| `--connection-string` | — | Ad-hoc connection string |
| `--namespace` | — | Azure AD namespace |
| `--output` | `-o` | Output format: `table` (default), `json`, `yaml` |

---

## 7. Command Surface

### 7.1 Config

```
sb config                                    # show settings + config file path
sb config set-context <name>
    [--connection-string <cs>]
    [--namespace <ns>]
    [--use-default-credentials]
sb config use-context <name>
sb config list
sb config delete-context <name>
```

`sb -s` is an alias for `sb config`.

---

### 7.2 Queues (`sb q`)

Topics mirror this exactly as `sb t`.

**List / Filter / Export**
```
sb q -c dev                                  # top max (MaxItems setting)
sb q -c dev -m 10                            # top 10
sb q -c dev -a                               # all
sb q -c dev -a -s LastAccessDate:asc
sb q -c dev -a -s SizeInMegabytes:dsc
sb q -c dev -a -s ActiveMessages:dsc

sb q -c dev -f order                         # single keyword
sb q -c dev -f email -f sms                  # multiple keywords (OR match)
sb q -c dev -f ActiveMessages:1-10           # field numeric range
sb q -c dev -f DeadLetterMessages:1-10
sb q -c dev -f SizeInMegabytes:1-10
sb q -c dev -f CreationDate:01/05/2024-01/05/2025
sb q -c dev -f LastAccessDate:01/05/2024-01/05/2025

sb q -c dev x                                # export (same flags as list/filter)
sb q -c dev x -m 10
sb q -c dev x -a -s SizeInMegabytes:dsc
```

Flags `-m`/`-a`/`-s`/`-f` and subcommand `x` are combinable freely.

**Info**
```
sb q -c dev -n some-queue-name
```

**Send**
```
sb q -c dev -n some-queue-name s -p message.json
sb q -c dev -n some-queue-name s -p message.json --delay 5
sb q -c dev -n some-queue-name s -p message.json --count 10
sb q -c dev -n some-queue-name s -p message.json --count 10 --delay 5
sb q -c dev -n some-queue-name s -p ./messages-dir/ --delay 5
```

`-p` accepts either a single JSON file or a directory of JSON files.
`--delay` is in minutes (scheduled message delivery).
`--count` repeats the same payload N times.

**Compare across contexts**

Compares all registered contexts (N-way diff). Highlights queues present in some contexts but not others, and queues whose stats diverge significantly (message counts, size).

```
sb q compare                                 # diff all registered contexts
sb q compare -f order                        # filtered by keyword
sb q compare --issues                        # only show differences/problems
sb q compare -n some-queue-name              # specific queue across all contexts
```

---

### 7.3 Subscriptions (`sb sub`)

Mirrors `sb q` with an additional `-t <topic>` required flag.

```
sb sub -c dev -t some-topic
sb sub -c dev -t some-topic -m 10
sb sub -c dev -t some-topic -a -s ActiveMessages:dsc
sb sub -c dev -t some-topic -f order
sb sub -c dev -t some-topic x
sb sub -c dev -t some-topic -n some-sub-name
sb sub -c dev -t some-topic -n some-sub-name s -p message.json --delay 5
```

---

### 7.4 Rules (`sb rule`)

```
sb rule -c dev -t some-topic -n some-sub
sb rule -c dev -t some-topic -n some-sub create <rule-name> --filter "<sql-expression>"
sb rule -c dev -t some-topic -n some-sub delete <rule-name>
```

---

### 7.5 Dead Letter Queues (`sb dlq`)

Dead letter subscriptions mirror this as `sb dls` with an additional `-t <topic>` flag.

**List / Filter / Export** (queues that have dead letters)
```
sb dlq -c dev
sb dlq -c dev -m 10
sb dlq -c dev -a
sb dlq -c dev -a -s LastAccessDate:asc
sb dlq -c dev -a -s SizeInMegabytes:dsc
sb dlq -c dev -a -s DeadLetterMessages:dsc

sb dlq -c dev -f order
sb dlq -c dev -f email -f sms
sb dlq -c dev -f ActiveMessages:1-10
sb dlq -c dev -f DeadLetterMessages:1-10
sb dlq -c dev -f SizeInMegabytes:1-10
sb dlq -c dev -f CreationDate:01/05/2024-01/05/2025
sb dlq -c dev -f LastAccessDate:01/05/2024-01/05/2025

sb dlq -c dev x
sb dlq -c dev x -m 10
sb dlq -c dev x -a -s DeadLetterMessages:dsc
```

**Info** (summary of a specific queue's dead letter sub-queue, with message-level filters)
```
sb dlq -c dev -n some-queue-name
sb dlq -c dev -n some-queue-name -f SomeException
sb dlq -c dev -n some-queue-name -f 01/04/2025
sb dlq -c dev -n some-queue-name -f 01/04/2025-01/05/2025
sb dlq -c dev -n some-queue-name -f ErrorType:SomeException
sb dlq -c dev -n some-queue-name -f ErrorMessage:SomeException
sb dlq -c dev -n some-queue-name -f ErrorDate:01/04/2025-01/05/2025
```

**View messages** (`v`)
```
sb dlq -c dev -n some-queue-name v
sb dlq -c dev -n some-queue-name v -f SomeException
sb dlq -c dev -n some-queue-name v -f 01/04/2025
sb dlq -c dev -n some-queue-name v -f 100-200          # sequence number range
sb dlq -c dev -n some-queue-name v -f 200              # single sequence number
```

**Purge messages** (`p`)
```
sb dlq -c dev -n some-queue-name p
sb dlq -c dev -n some-queue-name p -f SomeException
sb dlq -c dev -n some-queue-name p -f 01/04/2025
sb dlq -c dev -n some-queue-name p -f 01/04/2025-01/05/2025
sb dlq -c dev -n some-queue-name p -f 100-200
```

**Replay** (requeue to main queue) (`r`)
```
sb dlq -c dev -n some-queue-name r
sb dlq -c dev -n some-queue-name r --delay 5
sb dlq -c dev -n some-queue-name r --ttl 1
sb dlq -c dev -n some-queue-name r -f SomeException
sb dlq -c dev -n some-queue-name r -f 01/04/2025
sb dlq -c dev -n some-queue-name r -f 01/04/2025-01/05/2025
sb dlq -c dev -n some-queue-name r -f 100-200
```

`--delay` is in minutes. `--ttl` is in days.

**Export messages** (`x`)
```
sb dlq -c dev -n some-queue-name x
sb dlq -c dev -n some-queue-name x -f SomeException
sb dlq -c dev -n some-queue-name x -f 01/04/2025
sb dlq -c dev -n some-queue-name x -f 01/04/2025-01/05/2025
sb dlq -c dev -n some-queue-name x -f 100-200
```

Exports write JSON to `WorkingDirectory` (from config) by default, overridable with `--out <path>`.

---

### 7.6 Flag Reference

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--context` | `-c` | string | Named context (connection) |
| `--max` | `-m` | int | Max results (default: `MaxItems` setting) |
| `--all` | `-a` | bool | Return all results, ignore `MaxItems` |
| `--sort` | `-s` | string | `<Field>:<asc\|dsc>` |
| `--filter` | `-f` | string | Keyword, field range, or error field. Repeatable. |
| `--name` | `-n` | string | Queue / subscription name |
| `--topic` | `-t` | string | Topic name (for `sub`, `dls`, `rule`) |
| `--path` | `-p` | string | JSON file or directory path (send) |
| `--count` | — | int | Repeat send N times |
| `--delay` | — | int | Scheduled delivery delay in minutes |
| `--ttl` | — | int | Time-to-live in days |
| `--output` | `-o` | string | `table` (default), `json`, `yaml` |
| `--issues` | — | bool | Compare: show only differences |

---

## 8. Filter Syntax

All `-f` values follow a consistent pattern:

| Pattern | Meaning |
|---------|---------|
| `-f order` | Keyword match on name (case-insensitive) |
| `-f email -f sms` | Multiple keywords, OR match |
| `-f ActiveMessages:1-10` | Numeric field in range [1, 10] |
| `-f SizeInMegabytes:1-10` | Numeric field in range |
| `-f DeadLetterMessages:1-10` | Numeric field in range |
| `-f CreationDate:01/05/2024-01/05/2025` | Date field in range |
| `-f LastAccessDate:01/05/2024-01/05/2025` | Date field in range |
| `-f ErrorType:SomeException` | DLQ message field exact match |
| `-f ErrorMessage:SomeException` | DLQ message field contains |
| `-f ErrorDate:01/04/2025-01/05/2025` | DLQ message field date range |
| `-f 01/04/2025` | DLQ message enqueued on date |
| `-f 100-200` | DLQ message sequence number range |
| `-f 200` | DLQ message exact sequence number |

The filter parser disambiguates by context (numeric vs date vs keyword vs field prefix).

---

## 9. Output & Highlighting

**Table output** (default): human-readable, with threshold highlighting:
- Queue/topic size ≥ `Thresholds.SizePercentage`% of max → red row
- Last access > `Thresholds.LastAccessDays` days ago → yellow row

**JSON / YAML output** (`-o json`, `-o yaml`): machine-readable, no highlighting. Used by agents, scripts, and the Flutter integration.

**Export files**: written as JSON arrays to `WorkingDirectory`. Filename format: `<type>-<name>-<timestamp>.json`.

---

## 10. Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Command error (bad flags, missing args) |
| `2` | Connection / auth error |
| `3` | Resource not found |
| `4` | Operation failed (e.g. partial purge) |

---

## 11. Machine-Readable Help (MCP & Skill Integration)

### `sb schema`
Outputs the full command/flag/example tree as JSON. Used to auto-generate MCP tool descriptions and Claude Code skills.

```
sb schema                    # full tree
sb schema --command dlq      # subtree for dead letter commands
```

Output structure:
```json
{
  "version": "1.0.0",
  "commands": [
    {
      "name": "dlq",
      "description": "Manage dead letter queues",
      "flags": [
        { "name": "context", "short": "c", "type": "string", "description": "Active context name" },
        { "name": "max",     "short": "m", "type": "int",    "description": "Max results to return" }
      ],
      "subcommands": [
        {
          "name": "v",
          "description": "View dead letter messages",
          "examples": [
            "sb dlq -c dev -n some-queue v",
            "sb dlq -c dev -n some-queue v -f SomeException",
            "sb dlq -c dev -n some-queue v -f 100-200"
          ]
        }
      ]
    }
  ]
}
```

### `sb docs`
Outputs the full command reference as Markdown. Pipe to a file or paste into a skill body.

```
sb docs                      # full markdown
sb docs --command dlq        # dead letter section only
sb docs > COMMANDS.md
```

### `sb mcp`
Starts an MCP stdio server. Tool definitions are auto-generated from `sb schema` — always in sync with the binary.

```
sb mcp                       # start MCP server on stdio
```

### Consistency requirements for agent use
- Every command supports `-o json`
- All errors go to stderr; data goes to stdout
- Exit codes are always set (see Section 10)
- `sb schema` output is stable and versioned

---

## 12. Library Layer (`pkg/`)

The `pkg/` packages are the stable public API, usable without the CLI:

```go
import "github.com/user/sb/pkg/management"
import "github.com/user/sb/pkg/messaging"
import "github.com/user/sb/pkg/auth"

cred, _ := auth.Resolve(auth.Options{ConnectionString: "..."})
client  := management.NewClient(cred)
queues, _ := client.ListQueues(ctx, management.ListOptions{Max: 10})
```

### Flutter FFI (`pkg/ffi/`)
Compiled as a shared library (`go build -buildmode=c-shared`) producing `.so` (Linux), `.dylib` (Mac), `.dll` (Windows). Flutter calls these via `dart:ffi` — no subprocess spawning required.

C-exported functions follow a consistent pattern:
```c
// All functions return JSON strings; caller frees with sb_free()
char* sb_queue_list(const char* opts_json);
char* sb_dlq_view(const char* opts_json);
char* sb_message_send(const char* opts_json);
void  sb_free(char* ptr);
```

Input and output are JSON strings to keep the FFI boundary thin and language-agnostic.

---

## 13. Out of Scope (Phase 1)

- Flutter app and method channel integration (separate spec)
- Web UI
- Message schema validation
- Pub/Sub emulation
- Multi-namespace federation
