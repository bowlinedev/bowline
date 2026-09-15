import 'dart:convert';
import 'dart:typed_data';

import 'error.dart';

typedef DurationNs = int;

class Empty {
  const Empty();

  factory Empty.fromJson(Object? _) => const Empty();

  Map<String, Object?> toJson() => const {};

  List<Issue> validate() => const [];
}

FormatException _mismatch(String key, String want, Object? value) =>
    FormatException(
        '$key: expected $want, got ${value == null ? 'null' : value.runtimeType}');

Map<String, Object?> asObject(Object? value, [String key = 'value']) {
  if (value is Map<String, Object?>) {
    return value;
  }
  if (value is Map) {
    return value.map((k, v) => MapEntry(k.toString(), v));
  }
  throw _mismatch(key, 'object', value);
}

int asInt(Object? value, [String key = 'value']) {
  if (value is int) {
    return value;
  }
  if (value is double && value == value.truncateToDouble()) {
    return value.toInt();
  }
  throw _mismatch(key, 'integer', value);
}

BigInt asBigInt(Object? value, [String key = 'value']) {
  if (value is String) {
    final parsed = BigInt.tryParse(value);
    if (parsed != null) {
      return parsed;
    }
  }
  if (value is int) {
    return BigInt.from(value);
  }
  throw _mismatch(key, 'integer string', value);
}

double asDouble(Object? value, [String key = 'value']) {
  if (value is num) {
    return value.toDouble();
  }
  throw _mismatch(key, 'number', value);
}

bool asBool(Object? value, [String key = 'value']) {
  if (value is bool) {
    return value;
  }
  throw _mismatch(key, 'boolean', value);
}

String asString(Object? value, [String key = 'value']) {
  if (value is String) {
    return value;
  }
  throw _mismatch(key, 'string', value);
}

DateTime asTimestamp(Object? value, [String key = 'value']) {
  if (value is String) {
    final parsed = DateTime.tryParse(value);
    if (parsed != null) {
      return parsed.toUtc();
    }
  }
  throw _mismatch(key, 'RFC 3339 timestamp', value);
}

Uint8List asBytes(Object? value, [String key = 'value']) {
  if (value is String) {
    try {
      return base64Decode(base64.normalize(value));
    } on FormatException {
      throw _mismatch(key, 'base64', value);
    }
  }
  throw _mismatch(key, 'base64', value);
}

Object? asRaw(Object? value, [String key = 'value']) => value;

List<T> asList<T>(Object? value, T Function(Object?) decode,
    [String key = 'value']) {
  if (value == null) {
    return <T>[];
  }
  if (value is List) {
    return [for (final item in value) decode(item)];
  }
  throw _mismatch(key, 'array', value);
}

Map<String, T> asMap<T>(Object? value, T Function(Object?) decode,
    [String key = 'value']) {
  if (value == null) {
    return <String, T>{};
  }
  if (value is Map) {
    return {
      for (final entry in value.entries)
        entry.key.toString(): decode(entry.value)
    };
  }
  throw _mismatch(key, 'object', value);
}

T? asNullable<T>(Object? value, T Function(Object?) decode) =>
    value == null ? null : decode(value);

int readInt(Map<String, Object?> json, String key) => asInt(json[key], key);

int? readOptionalInt(Map<String, Object?> json, String key) =>
    asNullable(json[key], (v) => asInt(v, key));

BigInt readBigInt(Map<String, Object?> json, String key) =>
    asBigInt(json[key], key);

BigInt? readOptionalBigInt(Map<String, Object?> json, String key) =>
    asNullable(json[key], (v) => asBigInt(v, key));

double readDouble(Map<String, Object?> json, String key) =>
    asDouble(json[key], key);

double? readOptionalDouble(Map<String, Object?> json, String key) =>
    asNullable(json[key], (v) => asDouble(v, key));

bool readBool(Map<String, Object?> json, String key) => asBool(json[key], key);

bool? readOptionalBool(Map<String, Object?> json, String key) =>
    asNullable(json[key], (v) => asBool(v, key));

String readString(Map<String, Object?> json, String key) =>
    asString(json[key], key);

String? readOptionalString(Map<String, Object?> json, String key) =>
    asNullable(json[key], (v) => asString(v, key));

DateTime readTimestamp(Map<String, Object?> json, String key) =>
    asTimestamp(json[key], key);

DateTime? readOptionalTimestamp(Map<String, Object?> json, String key) =>
    asNullable(json[key], (v) => asTimestamp(v, key));

Uint8List readBytes(Map<String, Object?> json, String key) =>
    asBytes(json[key], key);

Uint8List? readOptionalBytes(Map<String, Object?> json, String key) =>
    asNullable(json[key], (v) => asBytes(v, key));

Object? readRaw(Map<String, Object?> json, String key) => json[key];

List<T> readList<T>(
        Map<String, Object?> json, String key, T Function(Object?) decode) =>
    asList(json[key], decode, key);

Map<String, T> readMap<T>(
        Map<String, Object?> json, String key, T Function(Object?) decode) =>
    asMap(json[key], decode, key);

String encodeTimestamp(DateTime value) => value.toUtc().toIso8601String();

String encodeBytes(List<int> value) => base64Encode(value);

List<Issue> prefixed(List<String> prefix, List<Issue> issues) => [
      for (final issue in issues)
        Issue([...prefix, ...issue.path], issue.rule, issue.message)
    ];
