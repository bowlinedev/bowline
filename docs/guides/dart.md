# Dart

The `dart` target generates one file, `bowline.dart`, with a class per type, a client per router node, and validation that mirrors the server's rules. The runtime is the `bowline` package on pub.dev.

```json
{ "targets": { "dart": { "out": "dart/lib/bowline.dart" } } }
```

```yaml
dependencies:
  bowline: ^0.5.0
```

## Calling

source: examples/ledger/dart/bin/ledger.dart:6-14

```dart
Client connect() {
  final url = Platform.environment['LEDGER_URL'] ?? 'http://localhost:8080/api';
  final token = Platform.environment['LEDGER_TOKEN'];
  return Client(Transport(
    baseUrl: Uri.parse(url),
    headers: () =>
        token == null ? const {} : {'authorization': 'Bearer $token'},
  ));
}
```

source: examples/ledger/dart/bin/ledger.dart:19-24

```dart
      case ['list']:
        final page = await client.invoices.list(ListInvoicesInput(limit: 20));
        for (final invoice in page.items) {
          out.writeln(
              '${invoice.id}\t${invoice.status.value}\t${invoice.total}\t${invoice.createdAt.toLocal()}');
        }
```

Every procedure is a method on its mount's client taking the input class and returning a `Future` of the output; subscriptions return a `Stream`, and uploads take a `Stream<List<int>>` and a file name. Inputs are validated before the request with the same rules the server enforces, so a bad input fails locally with the same issue paths.

## Errors

source: examples/ledger/dart/bin/ledger.dart:46-52

```dart
  } on BowlineException catch (e) {
    out.writeln('${e.code.wire}: ${e.message}');
    for (final issue in e.issues) {
      out.writeln('  ${issue.path.join('.')}: ${issue.message}');
    }
    return 1;
  }
```

`BowlineException` carries the `Code`, the message, the HTTP status, the declared variant name in `type`, its `details`, and validation `issues`. A network failure is `Code.unavailable` with status 0; a timeout from `CallOptions.timeout` is `Code.deadlineExceeded`.

## Types

Plain 64-bit integers are `int`, because the server keeps them within 53 bits; `,string` integers are `BigInt`. Timestamps are `DateTime` in UTC; durations are `int` nanoseconds; bytes are `Uint8List`; enums are enhanced enums with a `value`; generic types take converter functions for their parameters in `fromJson` and `toJson`. The full table is the Dart column of `spec/mapping-table.md`.

Proof: `cd examples/ledger/dart && dart test` starts the Go server and drives list, create, validation, void, and a subscription; `scripts/check-goldens.sh dart` analyzes a golden for every fidelity row with `--fatal-infos`.
