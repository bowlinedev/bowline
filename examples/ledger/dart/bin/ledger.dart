import 'dart:io';

import 'package:bowline/bowline.dart';
import 'package:ledger_dart/bowline.dart';

Client connect() {
  final url = Platform.environment['LEDGER_URL'] ?? 'http://localhost:8080/api';
  final token = Platform.environment['LEDGER_TOKEN'];
  return Client(Transport(
    baseUrl: Uri.parse(url),
    headers: () =>
        token == null ? const {} : {'authorization': 'Bearer $token'},
  ));
}

Future<int> run(List<String> args, Client client, StringSink out) async {
  try {
    switch (args) {
      case ['list']:
        final page = await client.invoices.list(ListInvoicesInput(limit: 20));
        for (final invoice in page.items) {
          out.writeln(
              '${invoice.id}\t${invoice.status.value}\t${invoice.total}\t${invoice.createdAt.toLocal()}');
        }
      case ['create', final description, final quantity]:
        final invoice = await client.invoices.create(CreateInvoiceInput(
          customerId: 1,
          lines: [
            Line(
                description: description,
                quantity: int.parse(quantity),
                unitPrice: 'USD 10.00')
          ],
        ));
        out.writeln('created ${invoice.id} ${invoice.total}');
      case ['void', final id]:
        final invoice =
            await client.invoices.void_(VoidInvoiceInput(id: int.parse(id)));
        out.writeln('voided ${invoice.id} ${invoice.status.value}');
      default:
        out.writeln(
            'usage: ledger list | create <description> <quantity> | void <id>');
        return 2;
    }
    return 0;
  } on BowlineException catch (e) {
    out.writeln('${e.code.wire}: ${e.message}');
    for (final issue in e.issues) {
      out.writeln('  ${issue.path.join('.')}: ${issue.message}');
    }
    return 1;
  }
}

Future<void> main(List<String> args) async {
  exitCode = await run(args, connect(), stdout);
}
