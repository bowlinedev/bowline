import 'dart:async';
import 'dart:convert';

class ServerEvent {
  const ServerEvent(this.name, this.data);

  final String name;
  final String data;
}

Stream<ServerEvent> parseServerSentEvents(Stream<List<int>> bytes) async* {
  var name = '';
  final data = StringBuffer();
  var hasData = false;
  await for (final line
      in bytes.transform(utf8.decoder).transform(const LineSplitter())) {
    if (line.isEmpty) {
      if (hasData || name.isNotEmpty) {
        yield ServerEvent(name.isEmpty ? 'message' : name, data.toString());
      }
      name = '';
      data.clear();
      hasData = false;
      continue;
    }
    if (line.startsWith(':')) {
      continue;
    }
    final colon = line.indexOf(':');
    final field = colon < 0 ? line : line.substring(0, colon);
    var value = colon < 0 ? '' : line.substring(colon + 1);
    if (value.startsWith(' ')) {
      value = value.substring(1);
    }
    switch (field) {
      case 'event':
        name = value;
      case 'data':
        if (hasData) {
          data.write('\n');
        }
        data.write(value);
        hasData = true;
    }
  }
  if (hasData || name.isNotEmpty) {
    yield ServerEvent(name.isEmpty ? 'message' : name, data.toString());
  }
}
