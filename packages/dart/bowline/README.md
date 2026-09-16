# bowline

The Dart client runtime for [Bowline](https://github.com/bowlinedev/bowline).

Bowline turns your Go code into typed API clients. This package is what the generated `bowline.dart` uses to make calls.

## Install

```bash
dart pub add bowline
```

## Use

```dart
import 'package:bowline/bowline.dart';
import 'bowline.dart' as api;

final client = api.Client(Transport(baseUrl: Uri.parse('http://localhost:8080/api')));
final page = await client.invoices.list(api.ListInvoicesInput(limit: 20));
```

You do not write the types. `bowline gen` writes them from your Go code.

Errors come back as `BowlineException` with the same codes the Go server uses. Queries are sent as `GET`, mutations as `POST`. Inputs are checked before the request leaves.

Docs: [bowlinedev/bowline](https://github.com/bowlinedev/bowline)

Apache-2.0
