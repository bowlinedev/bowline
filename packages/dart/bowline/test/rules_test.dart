import 'package:bowline/bowline.dart';
import 'package:test/test.dart';

void main() {
  test('isEmail', () {
    expect(isEmail('ada@example.com'), isTrue);
    expect(isEmail('nope'), isFalse);
    expect(isEmail('a@b.'), isFalse);
    expect(isEmail('a b@example.com'), isFalse);
  });

  test('isUrl', () {
    expect(isUrl('https://example.com/x'), isTrue);
    expect(isUrl('example.com'), isFalse);
    expect(isUrl('mailto:x'), isFalse);
  });

  test('isUuid', () {
    expect(isUuid('123e4567-e89b-12d3-a456-426614174000'), isTrue);
    expect(isUuid('123e4567e89b12d3a456426614174000'), isFalse);
  });

  test('ensureValid throws INVALID_ARGUMENT with the issues', () {
    ensureValid(const []);
    expect(
      () => ensureValid([
        const Issue(['id'], 'required', 'is required')
      ]),
      throwsA(isA<BowlineException>()
          .having((e) => e.code, 'code', Code.invalidArgument)
          .having((e) => e.issues.single.rule, 'rule', 'required')),
    );
  });
}
