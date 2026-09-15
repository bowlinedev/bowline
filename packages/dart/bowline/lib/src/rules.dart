import 'error.dart';

final RegExp _uuid = RegExp(
    r'^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$');

final RegExp _email = RegExp(r'^[^\s@]+@[^\s@]+$');

bool isEmail(String s) =>
    _email.hasMatch(s) && !s.endsWith('.') && !s.contains('..');

bool isUrl(String s) {
  final uri = Uri.tryParse(s);
  return uri != null && uri.hasScheme && uri.host.isNotEmpty;
}

bool isUuid(String s) => _uuid.hasMatch(s);

void ensureValid(List<Issue> issues) {
  if (issues.isNotEmpty) {
    throw BowlineException.invalid(issues);
  }
}
