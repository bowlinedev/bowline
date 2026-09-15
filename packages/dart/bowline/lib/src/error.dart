enum Code {
  canceled('CANCELED', 408),
  unknown('UNKNOWN', 500),
  invalidArgument('INVALID_ARGUMENT', 400),
  deadlineExceeded('DEADLINE_EXCEEDED', 408),
  notFound('NOT_FOUND', 404),
  alreadyExists('ALREADY_EXISTS', 409),
  permissionDenied('PERMISSION_DENIED', 403),
  resourceExhausted('RESOURCE_EXHAUSTED', 429),
  failedPrecondition('FAILED_PRECONDITION', 412),
  aborted('ABORTED', 409),
  outOfRange('OUT_OF_RANGE', 400),
  unimplemented('UNIMPLEMENTED', 404),
  internal('INTERNAL', 500),
  unavailable('UNAVAILABLE', 503),
  dataLoss('DATA_LOSS', 500),
  unauthenticated('UNAUTHENTICATED', 401);

  const Code(this.wire, this.httpStatus);

  final String wire;
  final int httpStatus;

  static Code fromWire(Object? value) {
    for (final code in values) {
      if (code.wire == value) {
        return code;
      }
    }
    return Code.unknown;
  }
}

class Issue {
  const Issue(this.path, this.rule, this.message);

  factory Issue.fromJson(Object? json) {
    final map = json is Map<String, Object?> ? json : const <String, Object?>{};
    final path = map['path'];
    return Issue(
      path is List ? path.map((e) => e.toString()).toList() : const [],
      map['rule']?.toString() ?? '',
      map['message']?.toString() ?? '',
    );
  }

  final List<String> path;
  final String rule;
  final String message;

  Map<String, Object?> toJson() =>
      {'path': path, 'rule': rule, 'message': message};

  @override
  String toString() => '${path.join('.')}: $message';

  @override
  bool operator ==(Object other) =>
      other is Issue &&
      other.rule == rule &&
      other.message == message &&
      other.path.length == path.length &&
      other.path.indexed.every((e) => path[e.$1] == e.$2);

  @override
  int get hashCode => Object.hash(path.join('.'), rule, message);
}

class BowlineException implements Exception {
  BowlineException(
    this.code,
    this.message, {
    this.status = 0,
    this.type,
    this.details,
    List<Issue>? issues,
  }) : issues = issues ?? const [];

  factory BowlineException.invalid(List<Issue> issues) => BowlineException(
        Code.invalidArgument,
        'invalid input',
        status: Code.invalidArgument.httpStatus,
        issues: issues,
      );

  final Code code;
  final String message;
  final int status;
  final String? type;
  final Object? details;
  final List<Issue> issues;

  static BowlineException? fromEnvelope(Object? body, int status) {
    if (body is! Map<String, Object?>) {
      return null;
    }
    final error = body['error'];
    if (error is! Map<String, Object?> || error['code'] is! String) {
      return null;
    }
    final issues = error['issues'];
    return BowlineException(
      Code.fromWire(error['code']),
      error['message']?.toString() ?? '',
      status: status,
      type: error['type'] as String?,
      details: error['details'],
      issues: issues is List ? issues.map(Issue.fromJson).toList() : null,
    );
  }

  @override
  String toString() => '${code.wire}: $message';
}
