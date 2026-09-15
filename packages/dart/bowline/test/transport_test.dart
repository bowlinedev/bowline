import 'dart:async';
import 'dart:convert';

import 'package:bowline/bowline.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:test/test.dart';

Transport transport(MockClient client,
        {Map<String, String> Function()? headers}) =>
    Transport(
        baseUrl: Uri.parse('http://api.test/api/'),
        client: client,
        headers: headers);

void main() {
  test('GET queries carry the input as a query parameter', () async {
    late http.Request seen;
    final t = transport(MockClient((request) async {
      seen = request;
      return http.Response('{"id":3,"name":"Ada"}', 200);
    }), headers: () => {'authorization': 'Bearer static'});
    final out = await t.call(
      'users.get',
      Method.get,
      {'id': 3},
      (json) => asObject(json)['name'],
      options: const CallOptions(headers: {'x-tenant': 'acme'}),
    );
    expect(out, 'Ada');
    expect(seen.method, 'GET');
    expect(seen.url.path, '/api/users.get');
    expect(seen.url.queryParameters['input'], '{"id":3}');
    expect(seen.headers['authorization'], 'Bearer static');
    expect(seen.headers['x-tenant'], 'acme');
    expect(seen.headers['accept'], 'application/json');
  });

  test('POST mutations send a JSON body with the content type', () async {
    late http.Request seen;
    final t = transport(MockClient((request) async {
      seen = request;
      return http.Response('{"id":9}', 200);
    }));
    await t.call('users.create', Method.post, {'name': 'Grace'}, asRaw);
    expect(seen.method, 'POST');
    expect(seen.body, '{"name":"Grace"}');
    expect(seen.headers['content-type'], startsWith('application/json'));
  });

  test('error envelopes become BowlineException', () async {
    final t = transport(MockClient((request) async => http.Response(
          '{"error":{"code":"NOT_FOUND","message":"user 9 not found","issues":[{"path":["id"],"rule":"exists","message":"missing"}]}}',
          404,
        )));
    try {
      await t.call('users.get', Method.get, {'id': 9}, asRaw);
      fail('expected an exception');
    } on BowlineException catch (e) {
      expect(e.code, Code.notFound);
      expect(e.status, 404);
      expect(e.message, 'user 9 not found');
      expect(e.issues, [
        const Issue(['id'], 'exists', 'missing')
      ]);
    }
  });

  test('non-envelope failures map to UNKNOWN with the status', () async {
    final t = transport(
        MockClient((request) async => http.Response('gateway down', 502)));
    await expectLater(
      t.call('users.get', Method.get, {}, asRaw),
      throwsA(isA<BowlineException>()
          .having((e) => e.code, 'code', Code.unknown)
          .having((e) => e.status, 'status', 502)),
    );
  });

  test('network failures map to UNAVAILABLE with status 0', () async {
    final t = transport(
        MockClient((request) async => throw http.ClientException('refused')));
    await expectLater(
      t.call('users.get', Method.get, {}, asRaw),
      throwsA(isA<BowlineException>()
          .having((e) => e.code, 'code', Code.unavailable)
          .having((e) => e.status, 'status', 0)),
    );
  });

  test('timeouts map to DEADLINE_EXCEEDED', () async {
    final t = transport(MockClient((request) async {
      await Future<void>.delayed(const Duration(milliseconds: 200));
      return http.Response('{}', 200);
    }));
    await expectLater(
      t.call('users.get', Method.get, {}, asRaw,
          options: const CallOptions(timeout: Duration(milliseconds: 20))),
      throwsA(isA<BowlineException>()
          .having((e) => e.code, 'code', Code.deadlineExceeded)),
    );
  });

  test('subscriptions yield message events and stop at done', () async {
    final t = transport(MockClient((request) async {
      expect(request.headers['accept'], 'text/event-stream');
      const body = ': open\n\n'
          'event: message\ndata: {"n":1}\n\n'
          ': ping\n\n'
          'event: message\ndata: {"n":2}\n\n'
          'event: done\ndata: {}\n\n'
          'event: message\ndata: {"n":3}\n\n';
      return http.Response(body, 200,
          headers: {'content-type': 'text/event-stream'});
    }));
    final seen = await t
        .subscribe('ticks', {'count': 2}, (json) => asObject(json)['n'])
        .toList();
    expect(seen, [1, 2]);
  });

  test('subscription error events throw the envelope', () async {
    final t = transport(MockClient((request) async => http.Response(
          'event: message\ndata: {"n":1}\n\nevent: error\ndata: {"error":{"code":"PERMISSION_DENIED","message":"no"}}\n\n',
          200,
        )));
    final stream = t.subscribe('ticks', {}, (json) => asObject(json)['n']);
    await expectLater(
      stream.toList(),
      throwsA(isA<BowlineException>()
          .having((e) => e.code, 'code', Code.permissionDenied)),
    );
  });

  test('uploads send the input part followed by the file part', () async {
    late http.MultipartRequest seen;
    final t = transport(MockClient.streaming((request, bodyStream) async {
      seen = request as http.MultipartRequest;
      final body = await http.ByteStream(bodyStream).bytesToString();
      expect(
          body.indexOf('name="input"'), lessThan(body.indexOf('name="file"')));
      expect(body, contains('{"label":"receipt"}'));
      expect(body, contains('hello'));
      return http.StreamedResponse(
          Stream.value(utf8.encode('{"size":5}')), 200);
    }));
    final out = await t.upload(
      'upload',
      {'label': 'receipt'},
      Stream.value(utf8.encode('hello')),
      'receipt.bin',
      (json) => asObject(json)['size'],
    );
    expect(out, 5);
    expect(seen.files.map((f) => f.field), ['input', 'file']);
    expect(seen.files.last.filename, 'receipt.bin');
  });
}
