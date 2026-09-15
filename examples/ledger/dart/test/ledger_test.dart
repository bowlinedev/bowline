import 'dart:async';
import 'dart:io';

import 'package:bowline/bowline.dart';
import 'package:http/http.dart' as http;
import 'package:ledger_dart/bowline.dart';
import 'package:test/test.dart';

Future<int> freePort() async {
  final socket = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
  final port = socket.port;
  await socket.close();
  return port;
}

Future<void> waitForHealth(Uri url) async {
  final deadline = DateTime.now().add(const Duration(seconds: 90));
  while (DateTime.now().isBefore(deadline)) {
    try {
      final response = await http.get(url);
      if (response.statusCode == 200) {
        return;
      }
    } on http.ClientException {
      await Future<void>.delayed(const Duration(milliseconds: 200));
    } on SocketException {
      await Future<void>.delayed(const Duration(milliseconds: 200));
    }
  }
  throw StateError('ledger server did not become healthy');
}

void main() {
  late Process server;
  late Client client;
  late Uri base;

  setUpAll(() async {
    final port = await freePort();
    server = await Process.start(
      'go',
      ['run', './cmd/server'],
      workingDirectory: '..',
      environment: {'ADDR': '127.0.0.1:$port'},
    );
    server.stdout.drain<void>();
    server.stderr.drain<void>();
    base = Uri.parse('http://127.0.0.1:$port/api');
    await waitForHealth(base.replace(path: '${base.path}/health'));
    client = Client(Transport(baseUrl: base));
  });

  tearDownAll(() async {
    server.kill();
    await server.exitCode.timeout(const Duration(seconds: 10), onTimeout: () {
      server.kill(ProcessSignal.sigkill);
      return -1;
    });
  });

  test('lists the seeded invoices', () async {
    final page = await client.invoices.list(ListInvoicesInput(limit: 20));
    final ids = page.items.map((i) => i.id).toList();
    expect(ids, containsAll([3, 4]));
    final paid = page.items.firstWhere((i) => i.id == 4);
    expect(paid.status, Status.paid);
    expect(paid.total, 'USD 9999.00');
    expect(paid.createdAt.isUtc, isTrue);
  });

  test('validation issues surface locally with server paths', () async {
    try {
      await client.invoices.create(CreateInvoiceInput(
        customerId: 1,
        lines: [Line(description: '', quantity: 0, unitPrice: 'USD 1.00')],
      ));
      fail('expected a validation failure');
    } on BowlineException catch (e) {
      expect(e.code, Code.invalidArgument);
      expect(e.issues.map((i) => i.path.join('.')),
          containsAll(['lines.0.description', 'lines.0.quantity']));
    }
  });

  test('the server rejects an input the client did not check', () async {
    final raw = Transport(baseUrl: base);
    try {
      await raw.call(
          'invoices.create',
          Method.post,
          {
            'customerId': 1,
            'lines': [
              {'description': '', 'quantity': 0, 'unitPrice': 'USD 1.00'}
            ]
          },
          asRaw);
      fail('expected a validation failure');
    } on BowlineException catch (e) {
      expect(e.status, 400);
      expect(e.issues.map((i) => i.path.join('.')),
          containsAll(['lines.0.description', 'lines.0.quantity']));
    }
  });

  test('creates, watches, and voids', () async {
    final events = <Invoice>[];
    final subscription = client.invoices
        .watch(WatchInput())
        .listen(events.add, onError: (Object e) {});
    await Future<void>.delayed(const Duration(milliseconds: 300));
    final created = await client.invoices.create(CreateInvoiceInput(
      customerId: 1,
      lines: [
        Line(description: 'Widgets', quantity: 3, unitPrice: 'USD 10.00')
      ],
      note: 'from dart',
    ));
    expect(created.total, 'USD 30.00');
    expect(created.note, 'from dart');
    final voided =
        await client.invoices.void_(VoidInvoiceInput(id: created.id));
    expect(voided.status, Status.void_);
    await Future<void>.delayed(const Duration(milliseconds: 300));
    await subscription.cancel();
    expect(events.map((e) => e.id), contains(created.id));
  });

  test('a locked invoice surfaces FAILED_PRECONDITION with typed details',
      () async {
    try {
      await client.invoices.void_(VoidInvoiceInput(id: 4));
      fail('expected InvoiceLocked');
    } on BowlineException catch (e) {
      expect(e.code, Code.failedPrecondition);
      expect(e.status, 412);
      expect(e.type, 'InvoiceLocked');
      final details = InvoiceLocked.fromJson(asObject(e.details));
      expect(details.id, 4);
      expect(details.status, Status.paid);
    }
  });
}
