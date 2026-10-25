# AES: persistent retry and retention

[Türkçe](../tr-tr/4%20-%20aes.md)

AES saves tap events to a database, except request/response logs, which are
discarded by default and retained only when selected by `request_rules`.
Matching event deliveries are tracked
separately by message ID + subject + consumer group. Failure in one group does
not reopen successful deliveries in other groups.

## Rule manifest and JSON Schema

- [Active manifest](../../aes.rules.json): event rules control retry/cleanup.
  Missing or empty `request_rules` disables request/response log persistence.
- [Example manifest](../../aes.rules.example.json): runnable examples, not
  enabled automatically.
- [JSON Schema](../../aes.rules.schema.json): field descriptions, types,
  required fields, defaults, enums and numeric limits.

Keep the manifest beside `aes.env`. `E5_AES_RULES_FILE` overrides its path,
which is resolved relative to the working directory. The launcher/IDE must load
environment variables; AES does not read an env file itself.

[VS Code workspace settings](../../.vscode/settings.json) associate the active
and example manifests with the local schema, enabling completion, hover
descriptions and validation without downloading the schema.

**Do not add `$schema` to the manifest:** AES rejects unknown root fields.
The association belongs in editor settings. For a custom filename, add it to
the `json.schemas` `fileMatch` list. Schema defaults are informational; AES
applies defaults when fields are omitted.

```json
{
  "rules": [
    {
      "event_subject_key": "order.created",
      "instance_group": "^orders$",
      "instance_name": "*",
      "try_after": 30,
      "try_count": 5,
      "consumption_timeout": 120,
      "on_consumption_timeout": "try",
      "remove_successful_timeout": 3600,
      "remove_failed_timeout": 86400
    },
    {
      "event_subject_key": { "regex": "^payment\\." }
    }
  ]
}
```

| Field | Default | Meaning |
|---|---|---|
| `event_subject_key` | Required | Exact string or `{"regex":"..."}` containing a Go regex |
| `instance_group` | `"*"` | Initial delivery's consumer group filter: `"*"` or Go regex |
| `instance_name` | `"*"` | Initial delivery's consumer name filter: `"*"` or Go regex |
| `try_after` | `0` | Delay after failure/timeout in seconds; `0` disables retry |
| `try_count` | `0` | Maximum retries, excluding the original delivery; `0` means unlimited |
| `consumption_timeout` | `0` | Result wait time after delivery in seconds; `0` disables timeout |
| `on_consumption_timeout` | `"keep"` | `keep`, `try` or `remove` when the timeout expires |
| `remove_successful_timeout` | `3600` | Retention seconds after success; `0` means never delete |
| `remove_failed_timeout` | `0` | Retention seconds after terminal failure; `0` means never delete |

The **first matching rule wins**. All subject, group and name filters must
match. Plain subject strings are literal: `order.created` is not a regex.
For filters, `"*"` is a special match-all value, not a regex. Use `^...$` for
a full regex match; escape backslashes in JSON (`"^payment\\."`).

All durations are whole seconds in `0..9223372036`; `try_count` is a
non-negative `int64`. `try_count=0` does not enable retry unless `try_after>0`.
Use the lowercase field names shown above, including
`on_consumption_timeout`, not `ON_CONSUMTATION_TIMEOUT`.

Unknown fields, invalid regexes, negative values and out-of-range durations
fail startup. JSON Schema validates structure, not Go regex syntax; AES
compiles regexes at startup. Rules are loaded once; there is no hot reload.

Unmatched event deliveries are logged but are not automatically retried or cleaned.
Each tracked delivery snapshots its policy. Changing rules or restarting AES
does not reset existing counters, deadlines or policies.

### Consumption timeout

- `keep`: retain the record without automatic retry or cleanup. A later
  consumption result can still update it.
- `try`: treat the timeout as a failure and apply `try_after` and `try_count`.
  If retry is disabled or exhausted, apply failed retention.
- `remove`: delete the record and associated logs without retry. This is an
  explicit data-loss policy, not the default.

With `consumption_timeout=0`, no timeout action runs.
Sending the last allowed retry does not immediately make the delivery a
terminal failure: AES waits for its result or configured timeout.
Success stops retry even before the limit is reached, then starts successful
retention. Pending or unlimited retries are not removed by failed retention.

### Example manifest behavior

1. `order.created`: tracks initial deliveries to names starting with
   `orders-worker-` in group `orders`. Retries after 30 seconds, up to five
   times. Missing results after 120 seconds count as failures. Retains successes
   for one hour and terminal failures for one day.
2. `^payment\\.`: matches subjects starting with `payment.` across all groups
   and instances. Explicit failures retry every 60 seconds without a limit.
   Missing results after 300 seconds are kept without automatic action.
   Successes are retained for one hour.
3. `temporary.notification`: never retries. Removes records with no result
   after 600 seconds, retaining successful records for 60 seconds. Omitted
   fields use their defaults.
4. Request `audit/payment-details`: retains request/response logs for one day.
5. Request `^audit/permanent/`: retains matching request/response logs indefinitely.
   Other requests are not saved.

Copy the desired rules into the active manifest, or set
`E5_AES_RULES_FILE=aes.rules.example.json` and restart AES. The latter enables
**all** example rules, including the destructive `remove` policy.

## Request/response log retention

Use the optional `request_rules` list independently of event retry rules:

```json
{
  "rules": [],
  "request_rules": [
    {
      "request_subject_key": { "regex": "^postral/important/" },
      "remove_timeout": 30
    },
    {
      "request_subject_key": "audit/payment-details",
      "remove_timeout": 0
    }
  ]
}
```

| Field | Default | Meaning |
|---|---|---|
| `request_subject_key` | Required | Exact subject string or `{"regex":"..."}` containing a Go regex |
| `remove_timeout` | `3600` | Retention seconds from the first observed request log; `0` means retain indefinitely |

The first matching request rule wins. There are no consumer group/name filters
or retry options; requests are never resent. One retention policy applies to
the request lifecycle by `messageId`:
`REQUEST_RECEIVED`, `REQUEST_ROUTED`, `REQUEST_NO_TARGET`,
`RESPONSE_RECEIVED`, `RESPONSE_DELIVERED`.

**The default is not to save requests.** Missing/empty `request_rules` or no
matching rule means their bodies are never written to the database. Event
rules do not apply to requests.

For selected requests, `aes_request` stores the subject, timeout and deadline.
Subjectless responses inherit the subject from this persistent mapping.
A response alone never starts retention: responses with an unseen or already
cleaned request are discarded.

Positive retention starts when **AES receives the first request log**, not when
a response arrives. Later logs do not extend the deadline. Requests without
a response are still cleaned when due, together with their request/response
logs and metadata. A response arriving at/after the deadline is not saved;
late responses do not reopen deleted requests. `remove_timeout=0` means
indefinite retention, not the default discard behavior.

Restart AES to reload rules. Existing selected requests retain their snapshotted
timeouts and deadlines. Legacy request/response logs without retention
metadata are **not retroactively deleted**.

## Connections and database

AES uses two connections:

- Tap: `E5_EXHAUST_HOST`, `E5_EXHAUST_PORT`, `E5_EXHAUST_KEY`,
  `E5_EXHAUST_TLS`, `E5_EXHAUST_CA_FILE`, `E5_EXHAUST_RECONNECT`.
- Retry publisher: `E5_HOST` (localhost), `E5_PORT` (3535), `E5_TLS` (false),
  `E5_AES_AUTH_KEY` (empty). The CA file is shared via `E5_EXHAUST_CA_FILE`.
  When broker authentication is enabled, the AES identity needs publish
  permission for the relevant subjects.

TLS settings must match each server port; certificate verification is not
skipped. Tap carries the original body. `E5_EXHAUST_INCLUDE_CONTENT` controls
**console-only** masking and does not affect the body stored by AES.

Schema v1 uses MySQL: `E5_AES_DB_DRIVER=mysql`, default port `3306`.
Other `E5_AES_DB_*` connection settings remain available. The former PostgreSQL
default lacked an implemented driver/DSN and is not supported. SQLite is used
only in isolated automated tests.

With `E5_AES_DB_GENERATE_IF_NOT_EXIST=true`, AES creates:

- `aes_event_tap`: event logs.
- `aes_delivery`: body, state, retry counter, timestamps and policy snapshot.
- `aes_attempt`: attempt message ID to delivery mapping.
- `aes_request`: subject, retention timeout and deadline for selected requests only.
- `aes_schema_version`: one row, `id=1`, `version=1`.

There is no migration chain in the first release. Unsupported schema versions
are rejected. When automatic creation is disabled, AES checks the existing
version table.
For an existing v1 installation, start once with
`E5_AES_DB_GENERATE_IF_NOT_EXIST=true` to create `aes_request`. The schema remains
version `1` for the first release; no migration chain is introduced.

## Retry protocol and cleanup

AES sends `RETRY_EVENT` to the broker. The broker delivers a normal `EVENT`
to exactly one active listener in the failed group. This may be a different
instance from the original consumer. Rule name filters select the initial
delivery; they do not pin retries to that instance. Normal `EVENT` broadcasts
still reach all listening groups.

Each retry has a new `messageId`; `originalMessageId` identifies the original
event. Consumers must report the **received attempt messageId** and subject in
their consumption result. The broker derives the group from the actual
connection.

`RETRY_ACCEPTED` means queued for the consumer, not successfully consumed.
`RETRY_NO_TARGET`, `RETRY_ERROR` and failures before sending do not consume the
retry limit; AES tries again after `try_after`. An ambiguous connection failure
after sending preserves the reserved attempt and waits for a result/timeout.
The same protection applies if AES crashes during publishing; with timeout
disabled, the ambiguous attempt remains retained.

A late success can complete the delivery. Old/duplicate errors do not schedule
extra retries or reopen successful records.

One scheduler goroutine runs every second, processing up to 100 due requests
and 100 due event deliveries.
Durations specify the earliest action time; database/network load may delay
execution. Cleanup deletes terminal deliveries, attempt mappings and that
group's logs together. The shared `EVENT_RECEIVED` log is deleted after the
last tracked group is cleaned. Other groups' logs are preserved.

Exhaust/tap is not a durable, acknowledged queue: disconnects or buffer overflow
can lose events. AES tracks events it receives and successfully persists.
Retry may cause duplicate processing; consumer-side idempotency using
`originalMessageId` is recommended.

Start with `go run ./cmd/aes` after loading environment variables.
SIGINT/SIGTERM stops the scheduler, closes tap and releases the database.
