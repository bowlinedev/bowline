import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;

import 'error.dart';
import 'sse.dart';

enum Method { get, post, put, patch, delete }

class CallOptions {
  const CallOptions({this.headers, this.timeout});

  final Map<String, String>? headers;
  final Duration? timeout;
}

class Transport {
  Transport(
      {required Uri baseUrl,
      http.Client? client,
      Map<String, String> Function()? headers})
      : _base = baseUrl.toString().replaceAll(RegExp(r'/+$'), ''),
        _client = client ?? http.Client(),
        _headers = headers;

  final String _base;
  final http.Client _client;
  final Map<String, String> Function()? _headers;

  Future<T> call<T>(
    String path,
    Method method,
    Map<String, Object?> input,
    T Function(Object?) decode, {
    CallOptions? options,
    bool rest = false,
  }) async {
    final request =
        _request(path, method, input, options, 'application/json', rest);
    final response = await _send(request, path, options);
    final body = await _read(response, path, options);
    if (response.statusCode >= 400) {
      throw _failure(response.statusCode, body, path);
    }
    return decode(body.isEmpty ? null : jsonDecode(body));
  }

  Stream<T> subscribe<T>(
    String path,
    Map<String, Object?> input,
    T Function(Object?) decode, {
    CallOptions? options,
  }) {
    final controller = StreamController<T>();
    StreamSubscription<ServerEvent>? events;
    var finished = false;
    void finish([Object? error]) {
      if (finished) {
        return;
      }
      finished = true;
      if (error != null) {
        controller.addError(error);
      }
      unawaited(events?.cancel());
      unawaited(controller.close());
    }

    controller.onListen = () async {
      try {
        final request = _request(
            path, Method.get, input, options, 'text/event-stream', false);
        final response = await _send(request, path, options);
        if (response.statusCode >= 400) {
          finish(_failure(
              response.statusCode, await _read(response, path, options), path));
          return;
        }
        events = parseServerSentEvents(response.stream).listen(
          (event) {
            switch (event.name) {
              case 'message':
                try {
                  controller.add(decode(jsonDecode(event.data)));
                } catch (e) {
                  finish(e);
                }
              case 'error':
                finish(_failure(0, event.data, path));
              case 'done':
                finish();
            }
          },
          onError: finish,
          onDone: finish,
        );
      } catch (e) {
        finish(e);
      }
    };
    controller.onCancel = () {
      finished = true;
      unawaited(events?.cancel());
    };
    return controller.stream;
  }

  Future<T> upload<T>(
    String path,
    Map<String, Object?> input,
    Stream<List<int>> file,
    String filename,
    T Function(Object?) decode, {
    CallOptions? options,
  }) async {
    final request = http.MultipartRequest('POST', Uri.parse('$_base/$path'));
    request.headers.addAll(_mergedHeaders(options, 'application/json'));
    request.files.add(http.MultipartFile.fromString('input', jsonEncode(input),
        contentType: _json));
    final bytes = await http.ByteStream(file).toBytes();
    request.files
        .add(http.MultipartFile.fromBytes('file', bytes, filename: filename));
    final response = await _send(request, path, options);
    final body = await _read(response, path, options);
    if (response.statusCode >= 400) {
      throw _failure(response.statusCode, body, path);
    }
    return decode(body.isEmpty ? null : jsonDecode(body));
  }

  static final _json = http.MediaType('application', 'json');

  Map<String, String> _mergedHeaders(CallOptions? options, String accept) {
    final headers = <String, String>{
      ...?_headers?.call(),
      ...?options?.headers
    };
    headers['accept'] = accept;
    return headers;
  }

  http.Request _request(
    String path,
    Method method,
    Map<String, Object?> input,
    CallOptions? options,
    String accept,
    bool rest,
  ) {
    final verb = method.name.toUpperCase();
    final url = Uri.parse('$_base/$path');
    final http.Request request;
    if (_sendsBody(method)) {
      request = http.Request(verb, url)..body = jsonEncode(input);
    } else if (rest) {
      request = http.Request(verb, _withQuery(url, input));
    } else {
      request = http.Request(
          verb, url.replace(queryParameters: {'input': jsonEncode(input)}));
    }
    request.headers.addAll(_mergedHeaders(options, accept));
    if (_sendsBody(method)) {
      request.headers['content-type'] = 'application/json';
    }
    return request;
  }

  static bool _sendsBody(Method method) =>
      method != Method.get && method != Method.delete;

  static Uri _withQuery(Uri url, Map<String, Object?> input) {
    final params = <String, List<String>>{};
    input.forEach((key, value) {
      if (value == null) {
        return;
      }
      final values = value is Iterable
          ? [for (final item in value) item.toString()]
          : [value.toString()];
      if (values.isNotEmpty) {
        params[key] = values;
      }
    });
    return params.isEmpty ? url : url.replace(queryParameters: params);
  }

  Future<http.StreamedResponse> _send(
      http.BaseRequest request, String path, CallOptions? options) async {
    try {
      var pending = _client.send(request);
      final timeout = options?.timeout;
      if (timeout != null) {
        pending = pending.timeout(timeout);
      }
      return await pending;
    } on TimeoutException {
      throw BowlineException(Code.deadlineExceeded, 'calling $path timed out');
    } on http.ClientException catch (e) {
      throw BowlineException(Code.unavailable, 'calling $path: ${e.message}');
    } on BowlineException {
      rethrow;
    } catch (e) {
      throw BowlineException(Code.unavailable, 'calling $path: $e');
    }
  }

  Future<String> _read(
      http.StreamedResponse response, String path, CallOptions? options) async {
    try {
      var pending = response.stream.bytesToString();
      final timeout = options?.timeout;
      if (timeout != null) {
        pending = pending.timeout(timeout);
      }
      return await pending;
    } on TimeoutException {
      throw BowlineException(Code.deadlineExceeded, 'reading $path timed out');
    } on http.ClientException catch (e) {
      throw BowlineException(Code.unavailable, 'reading $path: ${e.message}');
    }
  }

  BowlineException _failure(int status, String body, String path) {
    Object? decoded;
    try {
      decoded = jsonDecode(body);
    } on FormatException {
      decoded = null;
    }
    return BowlineException.fromEnvelope(decoded, status) ??
        BowlineException(Code.unknown, 'HTTP $status from $path',
            status: status);
  }
}
