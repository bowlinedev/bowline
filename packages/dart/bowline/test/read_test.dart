import 'package:bowline/bowline.dart';
import 'package:test/test.dart';

void main() {
  final json = <String, Object?>{
    'id': 3,
    'big': '9007199254740993',
    'ratio': 0.5,
    'whole': 2.0,
    'ok': true,
    'name': 'Ada',
    'at': '2026-01-02T03:04:05Z',
    'atFraction': '2026-01-02T03:04:05.123456Z',
    'atOffset': '2026-01-02T05:04:05+02:00',
    'blob': 'aGVsbG8=',
    'raw': {'any': 1},
    'tags': ['a', 'b'],
    'counts': {'x': 1, 'y': 2},
    'missing': null,
  };

  test('reads every scalar', () {
    expect(readInt(json, 'id'), 3);
    expect(readInt(json, 'whole'), 2);
    expect(readBigInt(json, 'big'), BigInt.parse('9007199254740993'));
    expect(readDouble(json, 'ratio'), 0.5);
    expect(readBool(json, 'ok'), isTrue);
    expect(readString(json, 'name'), 'Ada');
    expect(readBytes(json, 'blob'), [104, 101, 108, 108, 111]);
    expect(readRaw(json, 'raw'), {'any': 1});
  });

  test('parses RFC 3339 with and without fractions and offsets as UTC', () {
    expect(readTimestamp(json, 'at'), DateTime.utc(2026, 1, 2, 3, 4, 5));
    expect(readTimestamp(json, 'atFraction').microsecond, 456);
    expect(readTimestamp(json, 'atOffset'), DateTime.utc(2026, 1, 2, 3, 4, 5));
    expect(readTimestamp(json, 'at').isUtc, isTrue);
    expect(encodeTimestamp(DateTime.utc(2026, 1, 2, 3, 4, 5)),
        '2026-01-02T03:04:05.000Z');
  });

  test('reads collections and optionals', () {
    expect(readList(json, 'tags', asString), ['a', 'b']);
    expect(readMap(json, 'counts', asInt), {'x': 1, 'y': 2});
    expect(readList(json, 'absent', asString), isEmpty);
    expect(readOptionalString(json, 'missing'), isNull);
    expect(readOptionalInt(json, 'absent'), isNull);
    expect(readOptionalTimestamp(json, 'at'), isNotNull);
    expect(asNullable(null, asInt), isNull);
  });

  test('names the key on a shape mismatch', () {
    expect(
      () => readInt(json, 'name'),
      throwsA(isA<FormatException>()
          .having((e) => e.message, 'message', contains('name'))),
    );
    expect(() => readBytes(json, 'name'), throwsA(isA<FormatException>()));
    expect(() => readTimestamp(json, 'name'), throwsA(isA<FormatException>()));
    expect(() => asObject(3, 'x'), throwsA(isA<FormatException>()));
  });

  test('prefixes issue paths', () {
    final issues = prefixed([
      'items',
      '0'
    ], [
      const Issue(['name'], 'required', 'is required')
    ]);
    expect(issues.single.path, ['items', '0', 'name']);
  });

  test('empty round-trips', () {
    expect(const Empty().toJson(), isEmpty);
    expect(Empty.fromJson({'x': 1}).validate(), isEmpty);
  });
}
