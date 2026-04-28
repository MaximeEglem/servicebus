# servicebus

ServiceBus CLI for Azure Service Bus management and messaging.

The binary name is sb.

## What It Covers

- Context and credential management
- Queue and topic listing, filtering, sorting, export
- Subscription and rule management
- Dead-letter queue and dead-letter subscription operations
- Message send, purge, replay, and export workflows
- Session shortcuts via sb use
- JSON schema and markdown doc generation
- MCP server mode over stdio

## Build

Build in project root:

```bash
make build
```

This creates:

```bash
./sb
```

Optional cross-platform binaries:

```bash
make build-all
```

Check the current project version embedded in the binary:

```bash
./sb version
./sb version --verbose
```

`make build` now discovers the version automatically from git tags.
If the current commit is not tagged yet, the binary reports an untagged development version derived from git metadata.
The default output prints the semantic version only; use `--verbose` to include full build metadata, commit, and build time.

## Versioning

Releases follow semantic versioning with git tags in the form `vMAJOR.MINOR.PATCH`.

- `make release-patch`: backward-compatible bug fixes
- `make release-minor`: backward-compatible features
- `make release-major`: breaking changes

Each release command:

1. Requires a clean git working tree
2. Finds the latest semantic version tag automatically
3. Computes the next version
4. Creates an annotated git tag
5. Pushes the tag to `origin`

Examples:

```bash
make release-patch
make release-minor
make release-major
```

## Install (Optional)

If sb is not found in PATH, install it to a PATH directory:

```bash
mkdir -p ~/.local/bin
cp ./sb ~/.local/bin/sb
chmod +x ~/.local/bin/sb
```

Verify:

```bash
command -v sb
```

## Authentication Model

Credential resolution priority:

1. --connection-string flag
2. AZURE_SERVICEBUS_CONNECTION_STRING env var
3. Named context from config
4. Namespace with Azure AD default credential

## Config and Contexts

Set context with connection string:

```bash
sb config set-context dev --connection-string "Endpoint=sb://my-ns.servicebus.windows.net/;SharedAccessKeyName=...;SharedAccessKey=..."
```

Set context with namespace (Azure AD):

```bash
sb config set-context prod --namespace my-ns.servicebus.windows.net --use-default-credentials
```

Set default context:

```bash
sb config use-context dev
```

List contexts:

```bash
sb config list
```

Delete context:

```bash
sb config delete-context dev
```

Show config summary:

```bash
sb config
```

Open config file (where connection strings are saved):

```bash
sb config open
```

Config file path:

```text
~/.sb/config.yaml
```

Session file path:

```text
~/.sb/state.yaml
```

## Session Shortcuts With sb use

Show current session:

```bash
sb use
```

Set active context only:

```bash
sb use dev
```

Set active context and active resource type:

```bash
sb use dev dlq
```

When session type is set, root command can dispatch directly:

```bash
sb -m 10
```

## Global Flags

Available for most commands:

- -c, --context
- --connection-string
- --namespace
- -o, --output (table, json, yaml)

Common list flags:

- -m, --max
- -a, --all
- -s, --sort
- -f, --filter (repeatable)
- -n, --name
- -t, --topic (where relevant)

## Queue Commands

List queues:

```bash
sb q -c dev
```

List all, sorted:

```bash
sb q -c dev -a -s SizeInMegabytes:dsc
```

Filter queues:

```bash
sb q -c dev -f order
sb q -c dev -f ActiveMessages:1-100
sb q -c dev -f LastAccessDate:01/01/2025-31/12/2025
```

Queue details:

```bash
sb q -c dev -n orders
```

Send message(s):

```bash
sb q -c dev -n orders s -p ./message.json
sb q -c dev -n orders s -p ./message.json --count 10
sb q -c dev -n orders s -p ./message.json --delay 5
sb q -c dev -n orders s -p ./messages/
```

Export queue list to JSON:

```bash
sb q -c dev x
sb q -c dev x --out ./queues.json
```

Compare queues across contexts:

```bash
sb q compare
sb q compare --issues
sb q compare -n orders
```

## Topic Commands

List topics:

```bash
sb t -c dev
```

Filter/sort topics:

```bash
sb t -c dev -a -s SizeInMegabytes:dsc -f billing
```

Topic details:

```bash
sb t -c dev -n events
```

Export topic list:

```bash
sb t -c dev x
sb t -c dev x --out ./topics.json
```

## Subscription Commands

List subscriptions for a topic:

```bash
sb sub -c dev -t events
```

Subscription details:

```bash
sb sub -c dev -t events -n email-sub
```

Send to topic (subscription workflow):

```bash
sb sub -c dev -t events -n email-sub s -p ./message.json
```

## Rule Commands

List rules:

```bash
sb rule -c dev -t events -n email-sub
```

Create rule:

```bash
sb rule -c dev -t events -n email-sub create my-rule --filter "sys.Label = 'important'"
```

Delete rule:

```bash
sb rule -c dev -t events -n email-sub delete my-rule
```

## DLQ Commands (Queue Dead Letters)

List queues with dead letters:

```bash
sb dlq -c dev
```

View dead-letter messages in one queue:

```bash
sb dlq -c dev -n orders v
sb dlq -c dev -n orders v -f ErrorType:NullReferenceException
sb dlq -c dev -n orders v -f 100-200
sb dlq -c dev -n orders v -f 01/04/2025
```

Purge dead-letter messages:

```bash
sb dlq p -c dev -n orders
sb dlq p -c dev -n orders -f TimeoutException
sb dlq p -c dev -n orders --yes
sb dlq p -c dev -n orders --all --yes --workers 16 --batch-size 500
sb dlq p -c dev -n orders --all --yes --progress-every 10000 --checkpoint-file ./.sb-dlq-orders.ckpt
sb dlq p -c dev -n orders --all --yes --checkpoint-file ./.sb-dlq-orders.ckpt --resume
```

Replay dead-letter messages to main queue:

```bash
sb dlq r -c dev -n orders
sb dlq r -c dev -n orders --delay 5
sb dlq r -c dev -n orders --ttl 7
sb dlq r -c dev -n orders --all --workers 16 --batch-size 500 --retry-max-retries 8
sb dlq r -c dev -n orders --all --progress-every 10000 --checkpoint-file ./.sb-dlq-replay.ckpt
sb dlq r -c dev -n orders --all --checkpoint-file ./.sb-dlq-replay.ckpt --resume
```

Export dead-letter messages:

```bash
sb dlq -c dev -n orders x
sb dlq -c dev -n orders x --out ./orders-dlq.json
```

## DLS Commands (Subscription Dead Letters)

List subscriptions with dead letters:

```bash
sb dls -c dev -t events
```

View dead-letter messages for one subscription:

```bash
sb dls -c dev -t events -n email-sub v
```

Purge dead-letter messages:

```bash
sb dls p -c dev -t events -n email-sub
sb dls p -c dev -t events -n email-sub --yes
sb dls p -c dev -t events -n email-sub --all --yes --workers 16 --batch-size 500
sb dls p -c dev -t events -n email-sub --all --yes --progress-every 10000 --checkpoint-file ./.sb-dls-purge.ckpt
sb dls p -c dev -t events -n email-sub --all --yes --checkpoint-file ./.sb-dls-purge.ckpt --resume
```

Replay dead-letter messages to topic:

```bash
sb dls r -c dev -t events -n email-sub
sb dls r -c dev -t events -n email-sub --delay 5 --ttl 7
sb dls r -c dev -t events -n email-sub --all --workers 16 --batch-size 500
sb dls r -c dev -t events -n email-sub --all --progress-every 10000 --checkpoint-file ./.sb-dls-replay.ckpt
sb dls r -c dev -t events -n email-sub --all --checkpoint-file ./.sb-dls-replay.ckpt --resume
```

High-volume options for dlq/dls purge/replay:

- --workers: async worker count (default 8)
- --batch-size: receive batch size (default 200)
- --retry-max-retries: max retries for transient failures (default 5)
- --retry-delay-ms: initial retry delay in milliseconds (default 200)
- --retry-max-delay-ms: max retry delay in milliseconds (default 5000)
- --progress-every: print progress every N successful operations
- --checkpoint-file: write checkpoint for resume support
- --resume: resume from checkpoint file

Export dead-letter messages:

```bash
sb dls -c dev -t events -n email-sub x
```

## Output Formats

Default output is table. You can switch to JSON or YAML:

```bash
sb q -c dev -o json
sb q -c dev -o yaml
```

## Utilities

Generate CLI schema JSON:

```bash
sb schema
```

Generate markdown docs:

```bash
sb docs
```

Run MCP server over stdio:

```bash
sb mcp
```

## Troubleshooting

Command not found:

- Build first with make build.
- Run local binary with ./sb.
- Or install sb to a PATH directory.

No credentials resolved:

- Pass --connection-string directly.
- Or set AZURE_SERVICEBUS_CONNECTION_STRING.
- Or create/select a context with sb config set-context and sb config use-context.

Can’t find where connection strings are stored:

- Run sb config open.
- File is stored in ~/.sb/config.yaml.