# Dart

The `dart` target generates a single file, `bowline.dart`, with a class per type, a client per router node, and validation that matches the server's rules. The runtime is the `bowline` package on pub.dev.

```json
{ "targets": { "dart": { "out": "dart/lib/bowline.dart" } } }
```

```yaml
dependencies:
  bowline: ^1.0.0
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

Every procedure is a method on its mount's client. It takes the input class and returns a `Future` of the output. Subscriptions return a `Stream`. Uploads take a `Stream<List<int>>` and a file name. Inputs are validated before the request is sent, using the same rules as the server, so a bad input fails locally with the same issue paths.

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

`BowlineException` has the `Code`, the message, the HTTP status, the declared variant name in `type`, its `details`, and the validation `issues`. A network failure is `Code.unavailable` with status 0. A timeout from `CallOptions.timeout` is `Code.deadlineExceeded`.

## Types

Plain 64-bit integers are `int`, since the server keeps them within 53 bits. `,string` integers are `BigInt`. Timestamps are `DateTime` in UTC. Durations are `int` nanoseconds. Bytes are `Uint8List`. Enums are enhanced enums with a `value` field. Generic types take converter functions for their type parameters in `fromJson` and `toJson`. The full table is the Dart column of `spec/mapping-table.md`.

To verify: `cd examples/ledger/dart && dart test` starts the Go server and runs list, create, validation, void, and a subscription. `scripts/check-goldens.sh dart` analyzes a golden for every fidelity row with `--fatal-infos`.
